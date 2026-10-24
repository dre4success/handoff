#!/usr/bin/env python3
"""H10a independent probes, using CLI-created disposable records only."""
import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import time

binary = str(Path(sys.argv[1]).resolve())
base_env = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_"))}
base_env.update({"GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "review-test", "GIT_AUTHOR_EMAIL": "review@test.invalid",
    "GIT_COMMITTER_NAME": "review-test", "GIT_COMMITTER_EMAIL": "review@test.invalid",
    "HANDOFF_TEST_NOTIFY": os.devnull})

def run(root, args, env=None, expected=0):
    result = subprocess.run(args, cwd=root, env=env or base_env, text=True,
        capture_output=True, timeout=20)
    if expected is not None and result.returncode != expected:
        raise AssertionError(f"{shlex.join(args)}: exit {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result

def setup(root):
    run(root, ["git", "init", "-q", "-b", "main"])
    run(root, ["git", "commit", "-q", "--allow-empty", "-m", "fixture"])
    run(root, [binary, "init", "--roles", "implementer,reviewer"])
    body = root / "body.md"
    body.write_text("Disposable reviewer fixture.\n")
    return body

def until(description, check):
    deadline = time.monotonic() + 8
    while not check():
        if time.monotonic() > deadline:
            raise AssertionError("timed out: " + description)
        time.sleep(.02)

def process_alive(pid):
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False

if sys.argv[2] == "orphan":
    with tempfile.TemporaryDirectory(prefix="handoff-h10a-orphan-") as scratch:
        root = Path(scratch)
        body = setup(root)
        env = dict(base_env, HANDOFF_ROLE="reviewer")
        wrapper_code = """import os, pathlib, subprocess, sys, time
with open('hook.out', 'w') as out, open('hook.err', 'w') as err:
    child = subprocess.Popen([sys.argv[1], 'hook', 'listen', '--timeout', '60s'], stdin=subprocess.DEVNULL, stdout=out, stderr=err)
    pathlib.Path('hook.pid').write_text(str(child.pid))
    time.sleep(60)
"""
        wrapper = subprocess.Popen([sys.executable, "-c", wrapper_code, binary], cwd=root, env=env)
        hook_pid = None
        try:
            until("hook pid", lambda: (root / "hook.pid").exists())
            hook_pid = int((root / "hook.pid").read_text())
            presence = root / ".handoff/presence/reviewer.json"
            until("hook owns the role", lambda: presence.exists() and json.loads(presence.read_text())["pid"] == hook_pid)
            os.kill(hook_pid, signal.SIGSTOP)
            wrapper.terminate()
            wrapper.wait(timeout=5)
            print("Original session parent exited while its hook was stopped.", flush=True)
            result = run(root, [binary, "note", "--as", "implementer", "--item", "W1",
                "--to", "reviewer", "--body", str(body)])
            print(result.stdout.strip(), flush=True)
            os.kill(hook_pid, signal.SIGCONT)
            cursor = root / ".handoff/cursors/reviewer.json"
            # An orphan should exit before it reads or acknowledges new work.
            until("orphan exit or cursor movement", lambda: cursor.exists() or not process_alive(hook_pid))
            time.sleep(.2)
            print("Orphan stderr:", (root / "hook.err").read_text().strip(), flush=True)
            replacement = run(root, [binary, "wait", "--as", "reviewer", "--for", "reviewer", "--timeout", "1s"], expected=None)
            print("Replacement wait exit:", replacement.returncode, flush=True)
            print("Replacement stdout:", repr(replacement.stdout), flush=True)
            if replacement.returncode != 0 or "#1 note W1" not in replacement.stdout:
                raise AssertionError("orphan consumed the entry; the replacement listener never received it")
            print("PASS: replacement receives the entry.", flush=True)
        finally:
            if hook_pid:
                for sig in (signal.SIGCONT, signal.SIGTERM):
                    try:
                        os.kill(hook_pid, sig)
                    except ProcessLookupError:
                        pass
            if wrapper.poll() is None:
                wrapper.terminate()
            wrapper.wait(timeout=5)

elif sys.argv[2] == "spaces":
    with tempfile.TemporaryDirectory(prefix="handoff-h10a-spaces-") as scratch:
        root = Path(scratch)
        body = setup(root)
        install = root / "install with spaces"
        install.mkdir()
        spaced_binary = install / "handoff"
        shutil.copy2(binary, spaced_binary)
        env = dict(base_env, HANDOFF_ROLE="reviewer")
        run(root, [binary, "note", "--as", "implementer", "--item", "W1",
            "--to", "reviewer", "--body", str(body)])
        brief = run(root, [str(spaced_binary), "hook", "brief"], env).stdout
        # Follow exactly the command printed in the duty brief, substituting only the id.
        line = next(line for line in brief.splitlines() if "read it with `" in line)
        command = line.split("`", 2)[1].replace("<id>", "1")
        print("Brief command:", command, flush=True)
        result = run(root, ["/bin/sh", "-c", command], env, expected=None)
        print("Shell exit:", result.returncode, flush=True)
        print("Shell stderr:", result.stderr.strip(), flush=True)
        if result.returncode != 0 or "#1 note W1" not in result.stdout:
            raise AssertionError("the brief's printed show command must run and read the fixture note")
        print("PASS: the shell runs the brief's quoted command and reads the note.", flush=True)

elif sys.argv[2] == "worktree":
    with tempfile.TemporaryDirectory(prefix="handoff-h10a-worktree-") as scratch:
        root = Path(scratch)
        setup(root)
        linked = root / "linked"
        run(root, ["git", "worktree", "add", "--detach", str(linked), "HEAD"])
        result = run(linked, [binary, "settings", "claude", "--as", "reviewer"])
        settings = json.loads(result.stdout[result.stdout.index("\n{") + 1:])
        print("Settings location:", linked / ".claude/settings.local.json", flush=True)
        print("Record location:", root / ".handoff", flush=True)
        print("Printed deny rules:", settings["permissions"]["deny"], flush=True)
        expected = "Edit(/" + str((root / ".handoff").resolve()) + "/**)"
        assert expected in settings["permissions"]["deny"], expected
        print("PASS: emitted Edit deny names the common record by its real absolute path.", flush=True)

elif sys.argv[2] == "ask-operator":
    with tempfile.TemporaryDirectory(prefix="handoff-h10a-ask-operator-") as scratch:
        root = Path(scratch)
        body = setup(root)
        env = dict(base_env, HANDOFF_ROLE="reviewer")
        brief = run(root, [binary, "hook", "brief"], env).stdout
        line = next(line for line in brief.splitlines() if line.startswith("To ask the operator"))
        command = line.split("`", 2)[1].replace("<question>", "Fixture question").replace("<file>", shlex.quote(str(body))).replace("<item>", "W1")
        print("Brief command:", command, flush=True)
        result = run(root, ["/bin/sh", "-c", command], env, expected=None)
        print("Shell exit:", result.returncode, flush=True)
        print("Shell stderr:", result.stderr.strip(), flush=True)
        assert result.returncode == 0, "following the brief's ask-the-operator command must post an operator note"
        assert "note W1 reviewer → operator" in result.stdout, result.stdout
        print("Posted:", result.stdout.strip(), flush=True)
        print("PASS: the brief's operator note command posts.", flush=True)

