#!/usr/bin/env python3
import os
import sys
import json
import tempfile
import subprocess
from pathlib import Path
from datetime import datetime, timedelta

# Minimal port of NaturalCommitEngine from spec
class NaturalCommitEngine:
    FORMAT_STYLES = ["CASUAL_SHORT", "CONVENTIONAL", "TWO_LINE"]

    @staticmethod
    def determine_format(timestamp_epoch: int) -> str:
        slot = (int(timestamp_epoch) // 7200) % 3
        return NaturalCommitEngine.FORMAT_STYLES[slot]

    @staticmethod
    def get_last_queued_commit(sandbox_path: str) -> str:
        q = os.path.join(sandbox_path, "queue.json")
        if os.path.exists(q):
            try:
                with open(q) as f:
                    d = json.load(f)
                    qc = d.get("queued_commits", [])
                    if qc:
                        return qc[-1].get("commit_message", "").split("\n")[0]
            except Exception:
                pass
        return ""

    @classmethod
    def generate_commit_message(cls, repo_path: str, sandbox_path: str, author_date_iso: str) -> str:
        dt = datetime.fromisoformat(author_date_iso)
        epoch = int(dt.timestamp())
        style = cls.determine_format(epoch)
        last = cls.get_last_queued_commit(sandbox_path)
        try:
            st = subprocess.check_output(["git", "status", "--porcelain"], cwd=repo_path).decode().strip()
            files = len(st.splitlines()) if st else 1
        except Exception:
            files = 1
        if style == "CASUAL_SHORT":
            return f"update {files} project file{'s' if files>1 else ''} and fix edge cases"
        elif style == "CONVENTIONAL":
            return f"refactor(core): optimize working directory assets ({files} file{'s' if files>1 else ''})"
        elif style == "TWO_LINE":
            title = "refactor local module interfaces"
            if last:
                body = f"Extends changes from earlier commit ('{last}') to maintain local system consistency."
            else:
                body = "Improves execution speed and cleans up unused internal buffers."
            return f"{title}\n\n{body}"
        return "update codebase settings"

def run_git_cmd(args, cwd):
    return subprocess.run(args, cwd=cwd, check=True, text=True, capture_output=True).stdout.strip()

def main():
    repo = Path(tempfile.mkdtemp(prefix="flare-demo-"))
    sb = repo / ".flare-sandbox"
    sb.mkdir()
    (sb / "queue.json").write_text('{"queued_commits":[]}')
    run_git_cmd(["git","init"], cwd=repo)
    run_git_cmd(["git","config","user.name","FLARE Demo"], cwd=repo)
    run_git_cmd(["git","config","user.email","flare@local"], cwd=repo)
    (repo/"README.md").write_text("# FLARE + GitHub Demo\n")
    run_git_cmd(["git","add","."] ,cwd=repo)
    run_git_cmd(["git","commit","-m","initial commit"], cwd=repo)
    print("Repo:", repo)
    print("Initial:", run_git_cmd(["git","log","--oneline","-1"], cwd=repo))

    for i in range(3):
        (repo/f"change_{i}.txt").write_text(f"change {i}")
        run_git_cmd(["git","add","."] ,cwd=repo)
        t = (datetime.now()+timedelta(hours=i*2)).isoformat()
        msg = NaturalCommitEngine.generate_commit_message(str(repo), str(sb), t)
        run_git_cmd(["git","commit","-m", msg], cwd=repo)
        (sb/"queue.json").write_text(json.dumps({"queued_commits":[{"commit_message":msg}]}))

    print("\n--- FLARE natural commits ---")
    print(run_git_cmd(["git","log","--oneline","-n","4"], cwd=repo))

if __name__=='__main__':
    main()
