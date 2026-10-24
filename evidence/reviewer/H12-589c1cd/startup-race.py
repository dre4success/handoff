#!/usr/bin/env python3
"""Bounded integration probe for role transfer overlapping listener startup."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time

spec = importlib.util.spec_from_file_location("p", Path(__file__).resolve().parent.parent / "H12-50a8c2d/probes.py")
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)
p.BINARY = "/private/tmp/handoff-review-H12-r2-bin"
kind = sys.argv[1] if len(sys.argv) > 1 else "hook"
root = Path(tempfile.mkdtemp(prefix="handoff-h12-startup-race-", dir="/private/tmp"))
for args in (["init", "-q", "-b", "main"], ["-c", "user.name=Reviewer", "-c", "user.email=reviewer@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture"]):
    subprocess.run(["git", *args], cwd=root, env=p.BASE_ENV, check=True)

def command(args, sid=None, check=True):
    result = subprocess.run([p.BINARY, *args], cwd=root, env=p.BASE_ENV | ({"CODEX_THREAD_ID": sid} if sid else {}), capture_output=True, text=True, timeout=15)
    if check:
        assert result.returncode == 0, result.stderr
    return result

padding = [f"pad{i:03}" for i in range(300)]
command(["init", "--roles", ",".join(["implementer", "reviewer", *padding]), "--reviewers", "reviewer"])
for role in padding:
    command(["role", role], "zz-" + role)
print("kind:", kind, "scratch:", root, "; seeded 300 unrelated bindings through the CLI to widen the read/transfer overlap", flush=True)
found = False
for attempt in range(30):
    command(["role", "reviewer"], "aa-original")
    replacement = subprocess.Popen([p.BINARY, "role", "reviewer"], cwd=root, env=p.BASE_ENV | {"CODEX_THREAD_ID": "bb-replacement"}, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    delay = [0, .0005, .001, .002, .004, .008][attempt % 6]
    time.sleep(delay)
    if kind == "hook":
        old = subprocess.Popen([p.BINARY, "hook", "listen", "--timeout", "5s"], cwd=root, env=p.BASE_ENV, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        old.stdin.write(json.dumps(dict(session_id="aa-original", hook_event_name="Stop")))
        old.stdin.close()
        old.stdin = None
    else:
        old = subprocess.Popen([p.BINARY, "wait", "--timeout", "5s"], cwd=root, env=p.BASE_ENV | {"CODEX_THREAD_ID": "aa-original"}, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        replacement.communicate(timeout=10)
        assert replacement.returncode == 0
        time.sleep(1.5)
        print("attempt", attempt, "delay", delay, "old listener exit", old.poll(), flush=True)
        if old.poll() is None:
            body = root / "note.md"
            body.write_text("Only the replacement should receive this.\n")
            note = command(["note", "--as", "implementer", "--to", "reviewer", "--item", "STARTUP", "--body", str(body)])
            out, err = old.communicate(timeout=6)
            print("posted:", note.stdout.strip(), flush=True)
            print("displaced listener:", old.returncode, out, err, flush=True)
            if "note STARTUP" in out + err:
                print("REPRODUCED: displaced", kind, "consumed the replacement's note after a startup/transfer race.", flush=True)
                replacement_wait = command(["wait", "--timeout", "1s"], "bb-replacement", check=False)
                print("replacement wait:", replacement_wait.returncode, replacement_wait.stdout, replacement_wait.stderr, flush=True)
                assert replacement_wait.returncode == 3 and "note STARTUP" not in replacement_wait.stdout
                found = True
                break
    finally:
        if old.poll() is None:
            old.terminate()
            old.communicate(timeout=5)
print("found:", found, flush=True)
raise SystemExit(0 if found else 1)
