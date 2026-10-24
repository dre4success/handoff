#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import time

prior = Path(__file__).resolve().parent.parent / "H12-50a8c2d/probes.py"
spec = importlib.util.spec_from_file_location("prior", prior)
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)
p.BINARY = "/private/tmp/handoff-review-H12-r3-bin"

def moved_wait():
    root = p.fixture()
    first, second = {"CODEX_THREAD_ID": "old"}, {"CODEX_THREAD_ID": "new"}
    p.run(root, ["role", "reviewer"], first)
    old = subprocess.Popen([p.BINARY, "wait", "--timeout", "10s"], cwd=root, env=p.BASE_ENV | first, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        p.wait_presence(root)
        p.run(root, ["role", "reviewer"], second)
        out, err = old.communicate(timeout=3)
        print("displaced wait:", old.returncode, out, err, flush=True)
        assert old.returncode == 2 and "no longer holds reviewer" in err
        body = root / "body.md"
        body.write_text("For the replacement.\n")
        p.run(root, ["note", "--as", "implementer", "--to", "reviewer", "--item", "R1", "--body", str(body)])
        heard = p.run(root, ["wait", "--timeout", "1s"], second)
        assert "note R1" in heard.stdout
        print("PASS: displaced wait exits 2 and replacement receives the note.", flush=True)
    finally:
        if old.poll() is None:
            old.terminate()
            old.communicate(timeout=3)

def common_parent():
    root = p.fixture()
    first = {"CODEX_THREAD_ID": "codex-one"}
    p.run(root, ["role", "implementer"], first)
    for thread, source in [("codex-one", "startup"), ("codex-two", "startup"), ("codex-three", "clear")]:
        result = subprocess.run([p.BINARY, "hook", "brief", "--prefer", "implementer"], cwd=root, env=p.BASE_ENV,
                                input=json.dumps(dict(session_id=thread, hook_event_name="SessionStart", source=source)),
                                capture_output=True, text=True, timeout=5)
        print(thread, source, "exit", result.returncode, result.stdout, result.stderr, flush=True)
        assert result.returncode == 0
        assert ("you are implementer" in result.stdout) == (thread == "codex-one")
    assert "You are implementer" in p.run(root, ["role"], first).stdout
    p.run(root, ["role", "reviewer"], {"CODEX_THREAD_ID": "codex-two"})
    assert "You are implementer" in p.run(root, ["role"], first).stdout
    assert "You are reviewer" in p.run(root, ["role"], {"CODEX_THREAD_ID": "codex-two"}).stdout
    print("PASS: shared hook host does not move roles, including clear and --prefer; two Codex ids retain separate roles.", flush=True)

if __name__ == "__main__":
    {"wait": moved_wait, "shared": common_parent}[sys.argv[1]]()
