package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitTrackedChangesLeavesUntrackedFilesAlone(t *testing.T) {
	repository := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repository}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run("init", "-q")
	run("config", "user.name", "FLARE tests")
	run("config", "user.email", "flare-tests@example.invalid")
	tracked := filepath.Join(repository, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-m", "initial")
	before := run("rev-parse", "HEAD")
	if err := os.WriteFile(tracked, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	untracked := filepath.Join(repository, "secret-like-untracked.txt")
	if err := os.WriteFile(untracked, []byte("must not be staged\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := commitTrackedChanges(repository, false); err != nil {
		t.Fatal(err)
	}
	if after := run("rev-parse", "HEAD"); after == before {
		t.Fatal("expected tracked changes to create a checkpoint commit")
	}
	if status := run("status", "--porcelain"); !strings.Contains(status, "?? secret-like-untracked.txt") {
		t.Fatalf("expected untracked file to remain uncommitted; status: %q", status)
	}
}

func TestCommitTrackedChangesPushesToConfiguredUpstream(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "working")
	remote := filepath.Join(root, "remote.git")
	if err := os.Mkdir(repository, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(directory string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", directory}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run(root, "init", "--bare", "--initial-branch=main", remote)
	run(repository, "init", "--initial-branch=main")
	run(repository, "config", "user.name", "FLARE tests")
	run(repository, "config", "user.email", "flare-tests@example.invalid")
	tracked := filepath.Join(repository, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(repository, "add", "tracked.txt")
	run(repository, "commit", "-m", "initial")
	run(repository, "remote", "add", "origin", remote)
	run(repository, "push", "-u", "origin", "main")
	if err := os.WriteFile(tracked, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := commitTrackedChanges(repository, true); err != nil {
		t.Fatal(err)
	}
	localHead := run(repository, "rev-parse", "HEAD")
	remoteHead := run(root, "--git-dir", remote, "rev-parse", "refs/heads/main")
	if localHead != remoteHead {
		t.Fatalf("expected upstream HEAD %s to match local HEAD %s", remoteHead, localHead)
	}
}

func TestCommitTrackedChangesDoesNotCommitWithoutUpstream(t *testing.T) {
	repository := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repository}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run("init", "-q")
	run("config", "user.name", "FLARE tests")
	run("config", "user.email", "flare-tests@example.invalid")
	tracked := filepath.Join(repository, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-m", "initial")
	before := run("rev-parse", "HEAD")
	if err := os.WriteFile(tracked, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := commitTrackedChanges(repository, true); err == nil {
		t.Fatal("expected push without a configured upstream to fail")
	}
	if after := run("rev-parse", "HEAD"); after != before {
		t.Fatalf("expected no commit without an upstream; HEAD changed from %s to %s", before, after)
	}
}
