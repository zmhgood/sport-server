#!/usr/bin/env python3
"""Read-only release gate: fetch objects, validate explicit SHAs; never deploy."""
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path("/opt/sport")
REPOS = {"backend": "server", "admin": "admin", "miniprogram": "mini"}
BASELINE = ROOT / "releases" / "baseline-20261010.json"


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], text=True,
                                   stderr=subprocess.DEVNULL).strip()


def validate(shas):
    if len(shas) != 3 or any(not re.fullmatch(r"[0-9a-f]{40}", s) for s in shas):
        raise ValueError("Require explicit full server/admin/mini commit SHAs")
    baseline = json.loads(BASELINE.read_text())["commits"]
    checked = {}
    for (name, directory), sha in zip(REPOS.items(), shas):
        repo = ROOT / directory
        git(repo, "fetch", "origin", "+refs/heads/master:refs/remotes/origin/master",
            "--tags")
        if git(repo, "rev-parse", sha + "^{commit}") != sha:
            raise ValueError(name + ": commit mismatch")
        for old, new in [(baseline[name], sha), (sha, "refs/remotes/origin/master")]:
            if subprocess.run(["git", "-C", str(repo), "merge-base", "--is-ancestor", old, new],
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
                raise ValueError(name + ": version lacks repair baseline or is not on origin/master")
        checked[name] = sha
    return checked


if __name__ == "__main__":
    try:
        print(json.dumps({"verified": validate(sys.argv[1:]), "deployed": False}))
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print("Release verification failed: " + (str(exc) if isinstance(exc, ValueError)
                                                else type(exc).__name__), file=sys.stderr)
        sys.exit(1)
