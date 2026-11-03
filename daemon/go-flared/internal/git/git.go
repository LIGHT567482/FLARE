package git

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	commitInterval = 2 * time.Hour
	pushInterval   = 30 * time.Second
)

func Worker() {
	autoPush := os.Getenv("FLARE_GIT_AUTO_PUSH") == "true"
	if os.Getenv("FLARE_GIT_AUTO_COMMIT") != "true" {
		if autoPush {
			log.Printf("[git] automatic push requires FLARE_GIT_AUTO_COMMIT=true")
		}
		return
	}
	repositories := filepath.SplitList(os.Getenv("FLARE_GIT_REPOS"))
	if len(repositories) == 0 || repositories[0] == "" {
		log.Printf("[git] automatic commits enabled but FLARE_GIT_REPOS is empty")
		return
	}

	waitingForClean := make(map[string]bool, len(repositories))
	if autoPush {
		for _, repository := range repositories {
			clean, err := trackedWorkingTreeClean(repository)
			if err != nil {
				waitingForClean[repository] = true
				log.Printf("[git] automatic push for %q is waiting for a clean tracked working tree: %v", repository, err)
			} else if !clean {
				waitingForClean[repository] = true
				log.Printf("[git] automatic push for %q is waiting for a clean tracked working tree; existing changes will not be committed", repository)
			}
		}
	}

	run := func() {
		for _, repository := range repositories {
			if waitingForClean[repository] {
				clean, err := trackedWorkingTreeClean(repository)
				if err != nil {
					log.Printf("[git] cannot check whether %q is clean: %v", repository, err)
					continue
				}
				if !clean {
					continue
				}
				waitingForClean[repository] = false
				log.Printf("[git] %q is clean; automatic commit-and-push is armed for future tracked changes", repository)
				continue
			}
			if err := commitTrackedChanges(repository, autoPush); err != nil {
				log.Printf("[git] checkpoint failed for %q: %v", repository, err)
			}
		}
	}
	run()
	interval := commitInterval
	if autoPush {
		interval = pushInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}

func trackedWorkingTreeClean(repository string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status, err := runGit(ctx, repository, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, fmt.Errorf("inspect tracked working tree: %w", err)
	}
	return strings.TrimSpace(status) == "", nil
}

func commitTrackedChanges(repository string, autoPush bool) error {
	if strings.TrimSpace(repository) == "" {
		return errors.New("repository path is empty")
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return fmt.Errorf("resolve repository path: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve repository symlinks: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	top, err := runGit(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("verify Git repository: %w", err)
	}
	top, err = filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return fmt.Errorf("resolve Git root: %w", err)
	}
	if top != root {
		return fmt.Errorf("configured path %q is not the Git repository root %q", root, top)
	}

	status, err := runGit(ctx, root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return fmt.Errorf("inspect working tree: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	remote, mergeRef := "", ""
	if autoPush {
		remote, mergeRef, err = configuredUpstream(ctx, root)
		if err != nil {
			return err
		}
	}
	if _, err := runGit(ctx, root, "add", "-u", "--", "."); err != nil {
		return fmt.Errorf("stage tracked changes: %w", err)
	}
	_, err = runGit(ctx, root, "diff", "--cached", "--quiet")
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("inspect staged changes: %w", err)
		}
	}
	if err == nil {
		return nil
	}
	message := "chore(flare): local checkpoint " + time.Now().UTC().Format(time.RFC3339)
	if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
		return fmt.Errorf("create local checkpoint commit: %w", err)
	}
	if autoPush {
		if _, err := runGit(ctx, root, "push", "--porcelain", remote, "HEAD:"+mergeRef); err != nil {
			return fmt.Errorf("push checkpoint to %s %s: %w", remote, mergeRef, err)
		}
	}
	return nil
}

func configuredUpstream(ctx context.Context, repository string) (string, string, error) {
	branch, err := runGit(ctx, repository, "branch", "--show-current")
	if err != nil {
		return "", "", fmt.Errorf("resolve current branch: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", "", errors.New("cannot push a detached HEAD")
	}
	remote, err := runGit(ctx, repository, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return "", "", fmt.Errorf("resolve upstream remote for branch %q: %w", branch, err)
	}
	remote = strings.TrimSpace(remote)
	if remote == "" || remote == "." {
		return "", "", fmt.Errorf("branch %q has no pushable remote upstream", branch)
	}
	mergeRef, err := runGit(ctx, repository, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return "", "", fmt.Errorf("resolve upstream branch for %q: %w", branch, err)
	}
	mergeRef = strings.TrimSpace(mergeRef)
	if !strings.HasPrefix(mergeRef, "refs/heads/") {
		return "", "", fmt.Errorf("branch %q has an invalid upstream ref %q", branch, mergeRef)
	}
	return remote, mergeRef, nil
}

func runGit(ctx context.Context, repository string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
