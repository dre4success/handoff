#!/usr/bin/env python3
"""Independent CLI probes; scratch records are written only by handoff."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

BINARY = "/private/tmp/handoff-review-H12-bin"
BASE_ENV = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_")) and k not in ("CLAUDE_CODE_SESSION_ID", "CLAUDE_PID", "CODEX_THREAD_ID")}
BASE_ENV.update(HANDOFF_TEST_NOTIFY=os.devnull, GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)

def run(cwd, args, extra=None, expected=0):
    cmd = [BINARY, *args]
    result = subprocess.run(cmd, cwd=cwd, env=BASE_ENV | (extra or {}), text=True, capture_output=True, timeout=15)
    print("$", " ".join(args), "env", extra or {}, flush=True)
    print("exit:", result.returncode, flush=True)
    print(result.stdout + result.stderr, end="", flush=True)
    assert result.returncode == expected, result
    return result

def fixture():
    root = Path(tempfile.mkdtemp(prefix="handoff-h12-review-", dir="/private/tmp"))
    for args in (["init", "-q", "-b", "main"], ["-c", "user.name=Reviewer", "-c", "user.email=reviewer@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture"]):
        subprocess.run(["git", *args], cwd=root, env=BASE_ENV, check=True)
    run(root, ["init", "--roles", "implementer,reviewer,reviewer-2", "--reviewers", "reviewer,reviewer-2", "--main", "main"])
    return root

def wait_presence(root):
    path = root / ".handoff/presence/reviewer.json"
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        if path.exists():
            return
        time.sleep(.025)
    raise AssertionError("listener did not register")

def move(kind):
    root = fixture()
    parents = [subprocess.Popen(["sleep", "60"]) for _ in range(2)]
    listener = None
    try:
        first = dict(CLAUDE_CODE_SESSION_ID="original-reviewer", CLAUDE_PID=str(parents[0].pid))
        second = dict(CLAUDE_CODE_SESSION_ID="replacement-reviewer", CLAUDE_PID=str(parents[1].pid))
        run(root, ["role", "reviewer"], first)
        args = ["wait", "--timeout", "30s"] if kind == "wait" else ["hook", "listen", "--timeout", "30s"]
        listener = subprocess.Popen([BINARY, *args], cwd=root, env=BASE_ENV | first, text=True, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        listener.stdin.write(json.dumps(dict(session_id="original-reviewer", hook_event_name="Stop")))
        listener.stdin.close()
        listener.stdin = None
        wait_presence(root)
        run(root, ["role", "reviewer"], second)
        time.sleep(2)
        print("original listener after role transfer:", listener.poll(), flush=True)
        if kind == "wait":
            assert listener.poll() is None, "expected reproduction: old wait still listening"
            run(root, ["wait", "--timeout", "1s"], second, expected=4)
        else:
            assert listener.poll() == 0, "hook failed to stop after role transfer"
        body = root / "note.md"
        body.write_text("For the replacement reviewer.\n")
        run(root, ["note", "--as", "implementer", "--to", "reviewer", "--item", "R1", "--body", str(body)])
        if kind == "wait":
            out, err = listener.communicate(timeout=5)
            print("original listener delivered after losing role:", listener.returncode, out, err, flush=True)
            assert listener.returncode == 0 and "note R1" in out
            replacement = run(root, ["wait", "--timeout", "1s"], second, expected=3)
            assert "note R1" not in replacement.stdout
            print("REPRODUCED: displaced normal wait held the lock and consumed the replacement's note.", flush=True)
        else:
            result = run(root, ["wait", "--timeout", "1s"], second)
            assert "note R1" in result.stdout
            print("PASS: displaced hook stopped; replacement received the note.", flush=True)
        print("scratch fixture:", root, flush=True)
    finally:
        if listener is not None and listener.poll() is None:
            listener.terminate()
            listener.communicate(timeout=5)
        for parent in parents:
            parent.terminate()
            parent.wait()

def shared_codex_parent():
    root = fixture()
    first = dict(CODEX_THREAD_ID="codex-thread-one")
    run(root, ["role", "implementer"], first)
    def hook(thread):
        result = subprocess.run([BINARY, "hook", "brief"], cwd=root, env=BASE_ENV,
                                input=json.dumps(dict(session_id=thread, hook_event_name="SessionStart")),
                                capture_output=True, text=True, timeout=5)
        print("SessionStart", thread, "hook parent:", os.getpid(), "exit:", result.returncode, flush=True)
        print(result.stdout + result.stderr, end="", flush=True)
        assert result.returncode == 0
        return result.stdout
    assert "you are implementer" in hook("codex-thread-one")
    bindings = [json.loads(p.read_text()) for p in (root / ".handoff/sessions").glob("*.json")]
    assert bindings[0]["pid"] == os.getpid()
    assert "you are implementer" in hook("codex-thread-two")
    bindings = [json.loads(p.read_text()) for p in (root / ".handoff/sessions").glob("*.json")]
    print("bindings after second independent thread starts:", bindings, flush=True)
    assert len(bindings) == 1 and bindings[0]["session"] == "codex-thread-two"
    run(root, ["inbox"], first, expected=2)
    print("REPRODUCED: another thread with the same hook parent silently took the first thread's role.", flush=True)
    print("Stand-in parent only; no live Codex sessions launched. Shared-parent applicability is Source/Inferred from the installed CLI and OpenAI maintainer's runtime description.", flush=True)
    print("scratch fixture:", root, flush=True)

if __name__ == "__main__":
    if sys.argv[1] == "shared-codex-parent":
        shared_codex_parent()
    else:
        move(sys.argv[1])
