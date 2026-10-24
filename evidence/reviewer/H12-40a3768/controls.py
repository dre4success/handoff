#!/usr/bin/env python3
"""Delivery checks for bound waits and explicit-role compatibility."""
import importlib.util
import json
from pathlib import Path
import subprocess
import time

spec = importlib.util.spec_from_file_location("p", Path(__file__).resolve().parent.parent / "H12-50a8c2d/probes.py")
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)
p.BINARY = "/private/tmp/handoff-review-H12-r3-bin"

for name, args, outside, destination, expected in [
    ("hook", ["hook", "listen"], False, "reviewer", 0),
    ("plain", ["wait"], False, "reviewer", 2),
    ("as-own-role", ["wait", "--as", "reviewer"], False, "reviewer", 2),
    ("for-own-role", ["wait", "--for", "reviewer"], False, "reviewer", 2),
    ("for-another-role", ["wait", "--for", "reviewer-2"], False, "reviewer-2", 0),
    ("outside-explicit-role", ["wait", "--as", "reviewer"], True, "reviewer", 0),
]:
    root = p.fixture()
    def cli(args, sid=None):
        r = subprocess.run([p.BINARY, *args], cwd=root, env=p.BASE_ENV | ({"CODEX_THREAD_ID": sid} if sid else {}), text=True, capture_output=True, timeout=5)
        assert r.returncode == 0, r.stderr
        return r
    cli(["role", "reviewer"], "old")
    env = p.BASE_ENV | {"HANDOFF_TEST_STARTUP_DELAY": "1500ms"}
    if not outside:
        env["CODEX_THREAD_ID"] = "old"
    old = subprocess.Popen([p.BINARY, *args, "--timeout", "5s"], cwd=root, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    old.stdin.write(json.dumps(dict(session_id="old", hook_event_name="Stop")))
    old.stdin.close()
    old.stdin = None
    try:
        time.sleep(.6)
        cli(["role", "reviewer"], "new")
        body = root / "note.md"
        body.write_text("Delivery control " + name + ".\n")
        cli(["note", "--as", "implementer", "--to", destination, "--item", "CONTROL", "--body", str(body)])
        out, err = old.communicate(timeout=4)
        print(name, "old exit:", old.returncode, "stdout:", out.strip(), "stderr:", err.strip(), flush=True)
        assert old.returncode == expected
        if destination == "reviewer" and not outside:
            assert "note CONTROL" not in out + err
            heard = cli(["wait", "--timeout", "1s"], "new")
            assert "note CONTROL" in heard.stdout
        else:
            assert "note CONTROL" in out
        print("PASS:", name, flush=True)
    finally:
        if old.poll() is None:
            old.terminate()
            old.communicate(timeout=3)
