#!/usr/bin/env python3
"""Observe the actual detached helper's flock in a CLI-created scratch project."""
import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

binary = sys.argv[1]
environment = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_"))}
environment.update({"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "review-test", "GIT_AUTHOR_EMAIL": "review@test.invalid",
    "GIT_COMMITTER_NAME": "review-test", "GIT_COMMITTER_EMAIL": "review@test.invalid",
    "HANDOFF_TEST_NOTIFY": os.devnull, "GOGC": sys.argv[2] if len(sys.argv) > 2 else "1"})

def command(directory, *arguments):
    result = subprocess.run(arguments, cwd=directory, env=environment, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f"{arguments}: {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result.stdout.strip(), result.stderr.strip()

with tempfile.TemporaryDirectory(prefix="handoff-h9b-lock-gc-") as directory:
    root = Path(directory)
    print("Scratch project:", directory, flush=True)
    print("GOGC:", environment["GOGC"], flush=True)
    command(directory, "git", "init", "-q", "-b", "main")
    command(directory, "git", "commit", "-q", "--allow-empty", "-m", "fixture")
    command(directory, binary, "init", "--roles", "implementer,reviewer")
    popups = root / "popups.txt"
    environment["HANDOFF_TEST_POPUP"] = str(popups)
    body = root / "body.md"
    # A legal body large enough to force collections while popup loads the record.
    body.write_text("Review fixture.\n" + ("x" * 80 + "\n") * 11_000)
    receipt, stderr = command(directory, binary, "note", "--as", "implementer",
        "--item", "W1", "--to", "operator", "--title", "Scratch question",
        "--body", str(body))
    print(receipt, stderr, sep="\n", flush=True)
    lock = root / ".handoff/notified/decide-note-1.lock"
    done = root / ".handoff/notified/decide-note-1"
    deadline = time.monotonic() + 8
    while not popups.exists():
        if time.monotonic() > deadline:
            raise RuntimeError("helper never displayed its fake dialog")
        time.sleep(.02)
    try:
        with lock.open("r+b") as observed:
            while time.monotonic() < deadline:
                assert not done.exists(), "dialog closed before the lock check"
                try:
                    fcntl.flock(observed, fcntl.LOCK_EX | fcntl.LOCK_NB)
                except BlockingIOError:
                    time.sleep(.02)
                else:
                    fcntl.flock(observed, fcntl.LOCK_UN)
                    print("FAIL: helper's lock is free while its fake dialog remains unanswered.", flush=True)
                    # A second, independent CLI process will now start another helper.
                    command(directory, binary, "note", "--as", "implementer", "--item", "W2", "--to", "reviewer",
                        "--title", "Unrelated fixture", "--body", str(body))
                    second_deadline = time.monotonic() + 2
                    while time.monotonic() < second_deadline:
                        if popups.read_text().count("handoff:") >= 2:
                            print("CONFIRMED: second helper displayed the same undecided question.", flush=True)
                            raise AssertionError("two pop-ups for the same note while first remains open")
                        time.sleep(.02)
                    raise AssertionError("lock was released, but duplicate popup was not observed")
        print("PASS: lock remained held throughout the observation window.", flush=True)
    finally:
        # This only releases the never-clicking test dialog, not an operator decision.
        Path(str(popups) + ".release").touch()
        until = time.monotonic() + 3
        while not done.exists() and time.monotonic() < until:
            time.sleep(.02)
        # Both detached helpers poll every 100 ms; let each finish its marker
        # write before TemporaryDirectory removes the disposable project.
        time.sleep(.5)
        entries = [json.loads(line) for line in (root / ".handoff/channel.jsonl").read_text().splitlines()]
        assert not any(entry["kind"] == "decision" for entry in entries)
        print("No decision written. Record writes used the CLI only.", flush=True)
