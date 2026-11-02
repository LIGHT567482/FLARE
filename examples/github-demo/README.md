# FLARE + GitHub Live Example

This example shows how FLARE's **Natural Commit Engine** behaves when working with a local Git repository (e.g. one cloned from GitHub).

## What FLARE does with Git

When FLARE runs against a repository:
1. Detects local file changes (staged/unstaged) via `git status --porcelain`
2. Stages changes automatically (`git add -A`)
3. Generates a **human-like** commit message based on time-slot rotation (every 2 hours)
4. Creates a local commit with that natural message
5. On reconnection (online), pushes to the configured remote (e.g. `git@github.com:user/repo.git` or `https://github.com/...`)
6. Maintains local audit logs + offline sync queue

## Natural Commit Format Rotation (12 commits / 24h)

| Slot | Style | Example |
|---|---|---|
| 08:00 (evening/morning slots as in spec) | Casual Short | `update 2 project files and fix edge cases` |
| 10:00 | Conventional | `refactor(core): optimize working directory assets (2 files)` |
| 12:00 | Two-Line | `refactor local module interfaces` + body referencing last commit |

The engine avoids machine markers like `[bot]`, `[skip ci]`, `chore: automated sync`, etc. Messages look hand-written.

## Run the live simulation

```bash
cd /home/light/Desktop/FLARE/examples/github-demo
python3 flare_git_demo.py
```

Output shows FLARE generating rotated natural commit messages locally—just like it would in a real GitHub workflow.

## Real workflow with GitHub

```bash
# Clone your GitHub repo
git clone https://github.com/your-org/your-app.git
cd your-app

# Start flared (daemon)
flared  # runs in background, watches repo + schedules commits

# Make changes locally (FLARE will commit naturally on its schedule)
# When online, FLARE pushes to origin (branch tracked)
git push  # or FLARE triggers git push via sync engine
```

In production, FLARE treats `origin` (GitHub/GitLab/Codeberg) as an async replication target: local commits happen first (offline-first), push happens when network is available.
