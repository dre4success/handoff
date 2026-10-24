#!/usr/bin/env python3
"""Exercise decision races through a PTY in disposable, CLI-created records."""
import json
import os
from pathlib import Path
import pty
import re
import select
import subprocess
import sys
import tempfile
import time

binary = sys.argv[1]
environment = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_"))}
environment.update({"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "review-test", "GIT_AUTHOR_EMAIL": "review@test.invalid",
    "GIT_COMMITTER_NAME": "review-test", "GIT_COMMITTER_EMAIL": "review@test.invalid",
    "HANDOFF_TEST_NOTIFY": os.devnull})

def command(directory, arguments):
    result = subprocess.run(arguments, cwd=directory, env=environment, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f"{arguments}: {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result.stdout.strip()

def handoff(directory, *arguments):
    return command(directory, [binary, *arguments])

def setup(directory):
    command(directory, ["git", "init", "-q", "-b", "main"])
    command(directory, ["git", "commit", "-q", "--allow-empty", "-m", "A"])
    first = command(directory, ["git", "rev-parse", "HEAD"])
    command(directory, ["git", "commit", "-q", "--allow-empty", "-m", "B"])
    second = command(directory, ["git", "rev-parse", "HEAD"])
    handoff(directory, "init", "--roles", "implementer,reviewer", "--reviewers", "reviewer")
    body = Path(directory, "body.md")
    body.write_text("Review fixture.\n")
    receipt = handoff(directory, "post", "--as", "implementer", "--item", "W1", "--to", "reviewer",
        "--commit", first, "--title", "W1 initial", "--body", str(body))
    request = re.search(r"#(\d+)", receipt).group(1)
    handoff(directory, "reply", "--as", "reviewer", request, "--verdict", "APPROVE", "--commit", first, "--body", str(body))
    return first, second, request, str(body)

def at_prompt(directory, arguments, prompt, action, answer):
    master, slave = pty.openpty()
    child = subprocess.Popen([binary, *arguments], cwd=directory, env=environment,
        stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)
    output = bytearray()
    deadline = time.monotonic() + 15
    try:
        while prompt.encode() not in output:
            if time.monotonic() >= deadline:
                raise RuntimeError("prompt not reached: " + output.decode(errors="replace"))
            if select.select([master], [], [], 0.2)[0]:
                output.extend(os.read(master, 4096))
        action()
        os.write(master, answer.encode())
        while child.poll() is None:
            if time.monotonic() >= deadline:
                raise RuntimeError("terminal did not finish")
            if select.select([master], [], [], 0.2)[0]:
                try:
                    data = os.read(master, 4096)
                except OSError:
                    break
                if not data:
                    break
                output.extend(data)
        code = child.wait(timeout=5)
    finally:
        if child.poll() is None:
            child.kill()
            child.wait()
        os.close(master)
    print(output.decode(errors="replace"))
    return code

def no_decisions(directory):
    # Read-only verification; records are created and changed only by the CLI.
    entries = [json.loads(line) for line in Path(directory, ".handoff/channel.jsonl").read_text().splitlines()]
    assert not any(entry["kind"] == "decision" for entry in entries), entries

with tempfile.TemporaryDirectory(prefix="handoff-h9-stale-") as directory:
    first, second, request, body = setup(directory)
    def supersede():
        receipt = handoff(directory, "post", "--as", "implementer", "--item", "W1", "--to", "reviewer",
            "--commit", second, "--title", "W1 replacement", "--body", body, "--supersedes", request)
        replacement = re.search(r"#(\d+)", receipt).group(1)
        handoff(directory, "reply", "--as", "reviewer", replacement, "--verdict", "APPROVE", "--commit", second, "--body", body)
    code = at_prompt(directory, ["decide", "W1", "--approve"], "[y/N]", supersede, "y\n")
    assert code == 2, code
    no_decisions(directory)
    print("PASS: decide by item does not retarget a confirmation to a superseding commit.")

with tempfile.TemporaryDirectory(prefix="handoff-h9-retract-") as directory:
    first, second, request, body = setup(directory)
    def retract():
        handoff(directory, "reply", "--as", "reviewer", request, "--verdict", "CHANGES", "--commit", first, "--body", body)
    code = at_prompt(directory, [], "[a/d/s]", retract, "a\n\n")
    assert code == 0, code
    no_decisions(directory)
    print("PASS: bare handoff refuses to write after the standing approval is retracted.")
