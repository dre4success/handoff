#!/usr/bin/env python3
"""Direct generated-hook and policy probes; no model sessions, no record edits."""
import json
import os
import pty
import select
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import tomllib

binary = str(Path(sys.argv[1]).resolve())
mode = sys.argv[2]
env = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_"))}
env.update({"GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "review-test", "GIT_AUTHOR_EMAIL": "review@test.invalid",
    "GIT_COMMITTER_NAME": "review-test", "GIT_COMMITTER_EMAIL": "review@test.invalid",
    "HANDOFF_TEST_NOTIFY": os.devnull})

def run(root, args, expected=0, input=None):
    p = subprocess.run(args, cwd=root, env=env, input=input, text=True,
        capture_output=True, timeout=20)
    if expected is not None and p.returncode != expected:
        raise AssertionError(f"{shlex.join(args)}: exit {p.returncode}\n{p.stdout}\n{p.stderr}")
    return p

def until(what, check):
    deadline = time.monotonic() + 8
    while not check():
        if time.monotonic() > deadline:
            raise AssertionError("timed out: " + what)
        time.sleep(.02)

def setup(root):
    run(root, ["git", "init", "-q", "-b", "main"])
    run(root, ["git", "commit", "-q", "--allow-empty", "-m", "fixture"])
    run(root, [binary, "init", "--roles", "implementer,reviewer"])
    body = root / "body.md"
    body.write_text("Disposable reviewer fixture.\n")
    return body

def generated(root, executable):
    out = run(root, [executable, "settings", "codex", "--as", "reviewer"]).stdout
    hooks, rules = out.split(".codex/hooks.json:\n", 1)[1].split("\n\n.codex/rules/handoff.rules:\n", 1)
    return json.loads(hooks), rules

def present(root, pid):
    try:
        return json.loads((root / ".handoff/presence/reviewer.json").read_text())["pid"] == pid
    except (FileNotFoundError, json.JSONDecodeError):
        return False

with tempfile.TemporaryDirectory(prefix="handoff-h10b-" + mode + "-") as scratch:
    root = Path(scratch).resolve()
    body = setup(root)
    if mode == "rules":
        install = root / "install with spaces and 'quote'"
        install.mkdir()
        installed = install / "handoff"
        shutil.copy2(binary, installed)
        hooks, rules = generated(root, str(installed))
        policy = root / "handoff.rules"
        policy.write_text(rules)
        print("Installed:", installed, flush=True)
        print("Generated rules:\n" + rules, flush=True)
        for args, expected in [(["show", "1"], "allow"),
                (["claim", "--as", "reviewer", "1"], "allow"),
                (["reply", "--as", "reviewer", "1", "--verdict", "APPROVE"], "allow"),
                (["decide", "W1", "--approve"], "forbidden"),
                ([], "allow")]:
            p = run(root, ["codex", "execpolicy", "check", "--pretty", "--rules", str(policy), "--", str(installed), *args])
            decision = json.loads(p.stdout).get("decision")
            print("argv:", args, "decision:", decision, flush=True)
            assert decision == expected, p.stdout
        command = hooks["hooks"]["SessionStart"][0]["hooks"][0]["command"]
        p = run(root, ["/bin/sh", "-c", command], input="{}")
        print("Generated SessionStart command:", command, flush=True)
        assert "you are reviewer" in p.stdout, p.stdout
        assert "claim --as reviewer" in p.stdout, p.stdout
        print("PASS: policy checker reads the generated rules; shell executes the quoted hook without HANDOFF_ROLE.", flush=True)
    elif mode == "listen":
        hooks, _ = generated(root, binary)
        handler = hooks["hooks"]["Stop"][0]["hooks"][0]
        assert handler["timeout"] == 86400 and not handler.get("async", False)
        command = handler["command"]
        print("Generated Stop command:", command, flush=True)
        for n in (1, 2):
            p = subprocess.Popen(["/bin/sh", "-c", command], cwd=root, env=env,
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                p.stdin.write("{}")
                p.stdin.close()
                p.stdin = None
                until("exec-form shell replaced by listening hook", lambda: present(root, p.pid))
                other = run(root, [binary, "wait", "--as", "reviewer", "--for", "reviewer", "--timeout", "1s"], expected=4)
                print("Second wait exit:", other.returncode, flush=True)
                run(root, [binary, "note", "--as", "implementer", "--item", "W" + str(n), "--to", "reviewer", "--body", str(body)])
                out, err = p.communicate(timeout=8)
                print("Stop", n, "exit:", p.returncode, "stderr:", err.strip(), flush=True)
                assert p.returncode == 2 and not out and f"#{n} note W{n}" in err
            finally:
                if p.poll() is None:
                    p.terminate()
                p.wait(timeout=5)
        print("PASS: generated Stop receives both notes across two turns and holds one listener per role.", flush=True)
    elif mode == "parent":
        hooks, _ = generated(root, binary)
        command = hooks["hooks"]["Stop"][0]["hooks"][0]["command"]
        wrapper_code = """import pathlib, subprocess, sys, time
with open('hook.out', 'w') as out, open('hook.err', 'w') as err:
    child = subprocess.Popen(['/bin/sh', '-c', sys.argv[1]], stdin=subprocess.DEVNULL, stdout=out, stderr=err)
    pathlib.Path('hook.pid').write_text(str(child.pid))
    time.sleep(60)
"""
        parent = subprocess.Popen([sys.executable, "-c", wrapper_code, command], cwd=root, env=env)
        hook_pid = None
        try:
            until("hook pid", lambda: (root / "hook.pid").exists())
            hook_pid = int((root / "hook.pid").read_text())
            until("generated exec hook listening", lambda: present(root, hook_pid))
            os.kill(hook_pid, signal.SIGSTOP)
            parent.terminate()
            parent.wait(timeout=5)
            run(root, [binary, "note", "--as", "implementer", "--item", "W1", "--to", "reviewer", "--body", str(body)])
            os.kill(hook_pid, signal.SIGCONT)
            # Wait beyond the hook's one-second polling interval.
            time.sleep(1.3)
            got = run(root, [binary, "wait", "--as", "reviewer", "--for", "reviewer", "--timeout", "1s"])
            print("Replacement wait:", got.stdout.strip(), flush=True)
            assert "#1 note W1" in got.stdout, got.stdout
            assert not (root / "hook.err").read_text(), "orphan tried to continue a dead parent"
            print("PASS: exec preserves the intended parent relationship and the orphan leaves the note.", flush=True)
        finally:
            if hook_pid:
                for sig in (signal.SIGCONT, signal.SIGTERM):
                    try:
                        os.kill(hook_pid, sig)
                    except ProcessLookupError:
                        pass
            if parent.poll() is None:
                parent.terminate()
            parent.wait(timeout=5)
    elif mode == "bare":
        run(root, [binary, "note", "--as", "reviewer", "--item", "D1", "--to", "operator", "--body", str(body)])
        _, rules = generated(root, binary)
        policy = root / "handoff.rules"
        policy.write_text(rules)
        checked = run(root, ["codex", "execpolicy", "check", "--rules", str(policy), "--", binary])
        print("Policy on bare command:", checked.stdout.strip(), flush=True)
        assert json.loads(checked.stdout)["decision"] == "allow"
        settings = run(root, [binary, "settings", "codex", "--as", "reviewer"]).stdout
        config_text = settings.split(".codex/config.toml:\n", 1)[1].split("\n\n.codex/hooks.json:\n", 1)[0]
        role_env = tomllib.loads(config_text)["shell_environment_policy"]["set"]
        assert role_env["HANDOFF_ROLE"] == "reviewer", role_env
        print("Parsed generated config:", role_env, flush=True)
        for label, process_env, expect_prompt in [("generated role", dict(env, **role_env), False), ("The operator control, no role", env, True)]:
            master, slave = pty.openpty()
            child = subprocess.Popen([binary], cwd=root, env=process_env, stdin=slave, stdout=slave, stderr=slave)
            os.close(slave)
            received = b""
            skipped = False
            try:
                deadline = time.monotonic() + 5
                while True:
                    if time.monotonic() > deadline:
                        raise AssertionError("PTY did not finish: " + repr(received))
                    if select.select([master], [], [], .1)[0]:
                        try:
                            part = os.read(master, 65536)
                        except OSError:
                            break
                        if not part:
                            break
                        received += part
                    prompted = b"Approve, deny or skip?" in received
                    if prompted and not expect_prompt:
                        raise AssertionError("configured agent reached the operator's UI: " + repr(received))
                    if prompted and expect_prompt and not skipped:
                        os.write(master, b"s\n")
                        skipped = True
                    if child.poll() is not None:
                        break
                child.wait(timeout=5)
                print(label + ":\n" + received.decode(errors="replace"), flush=True)
                assert child.returncode == 0
                if expect_prompt:
                    assert skipped, "The operator control should reach the prompt and skip"
                else:
                    assert b"inbox for reviewer" in received, received
            finally:
                if child.poll() is None:
                    child.terminate()
                child.wait(timeout=5)
                os.close(master)
        print("PASS: the role parsed from printed config keeps the agent in its inbox; the operator's no-role terminal still prompts. Only Skip was entered.", flush=True)
