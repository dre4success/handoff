#!/usr/bin/env python3
"""H11 independent CLI probes; records are created/written only by handoff."""
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile

binary = str(Path(sys.argv[1]).resolve())
mode = sys.argv[2]
env = {k: v for k, v in os.environ.items() if not k.startswith(("HANDOFF_", "GIT_"))}
env.update({"GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "review-test", "GIT_AUTHOR_EMAIL": "review@test.invalid",
    "GIT_COMMITTER_NAME": "review-test", "GIT_COMMITTER_EMAIL": "review@test.invalid",
    "HANDOFF_TEST_NOTIFY": os.devnull})

def run(root, args, expected=0):
    p = subprocess.run(args, cwd=root, env=env, text=True, capture_output=True, timeout=20)
    if p.returncode != expected:
        raise AssertionError(f"{shlex.join(args)}: exit {p.returncode}\n{p.stdout}\n{p.stderr}")
    return p.stdout

def repository(root, branch="main", commit=True):
    root.mkdir()
    run(root, ["git", "init", "-q", "-b", branch])
    if commit:
        run(root, ["git", "commit", "-q", "--allow-empty", "-m", "fixture"])
    return root

def config(root):
    return json.loads((root / ".handoff/config.json").read_text())

def snapshot(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
        for p in root.rglob("*") if p.is_file() and ".handoff" not in p.relative_to(root).parts}

with tempfile.TemporaryDirectory(prefix="handoff-h11-" + mode + "-") as scratch:
    scratch = Path(scratch).resolve()
    if mode == "ignore":
        root = repository(scratch / "project")
        (root / ".gitignore").write_text("# existing project rule\ncache/\n")
        (root / "README.md").write_text("Existing project.\n")
        run(root, ["git", "add", ".gitignore", "README.md"])
        run(root, ["git", "commit", "-q", "-m", "existing files"])
        before = snapshot(root)
        output = run(root, [binary, "init"])
        assert snapshot(root) == before, "init changed files outside .handoff, including Git metadata"
        c = config(root)
        assert c["roles"] == [{"name": "claude", "reviews": True}, {"name": "codex", "reviews": True}], c
        assert c["main"] == "main", c
        for role in ("claude", "codex"):
            settings = run(root, [binary, "settings", role])
            assert f"is then {role}" in settings, settings
        body = scratch / "body.md"
        body.write_text("Fixture note.\n")
        run(root, [binary, "note", "--as", "claude", "--item", "W1", "--to", "codex", "--body", str(body)])
        assert run(root, ["git", "status", "--porcelain", "--untracked-files=all"]) == ""
        run(root, ["git", "add", "-A"])
        assert run(root, ["git", "ls-files", "--", ".handoff"]) == ""
        print(output.strip())
        print("Ignored paths:\n" + run(root, ["git", "status", "--porcelain", "--ignored"]).strip())
        existing = (root / ".handoff/config.json").read_bytes()
        run(root, [binary, "init"], expected=2)
        assert (root / ".handoff/config.json").read_bytes() == existing
        print("PASS: only .handoff changes, default role/settings values are correct, git add -A stages no record, repeat init preserves config.")
    elif mode == "branches":
        for branch in ("master", "trunk"):
            root = repository(scratch / branch, branch)
            run(root, [binary, "init"])
            assert config(root)["main"] == branch, config(root)
        root = repository(scratch / "both", "master")
        run(root, ["git", "branch", "main"])
        run(root, [binary, "init"])
        assert config(root)["main"] == "main", config(root)
        root = repository(scratch / "unborn", "dev", commit=False)
        run(root, [binary, "init"])
        assert config(root)["main"] == "dev", config(root)
        root = repository(scratch / "detached", "trunk")
        run(root, ["git", "checkout", "-q", "--detach"])
        run(root, [binary, "init"], expected=2)
        assert not (root / ".handoff").exists()
        run(root, [binary, "init", "--main", "release"])
        assert config(root)["main"] == "release"
        root = repository(scratch / "common", "trunk")
        linked = scratch / "linked"
        run(root, ["git", "worktree", "add", "--detach", str(linked), "HEAD"])
        run(linked, [binary, "init"])
        assert config(root)["main"] == "trunk", config(root)
        assert not (linked / ".handoff").exists()
        print("PASS: main/master precedence, trunk fallback, unborn branch, detached refusal and explicit override, and linked-worktree common branch selection.")
    elif mode == "reviewers":
        for label, flags, wanted in [
            ("default", [], [False, True, True]),
            ("override", ["--reviewers", "reviewer-2"], [False, False, True])]:
            root = repository(scratch / label)
            run(root, [binary, "init", "--roles", "implementer,reviewer,reviewer-2", *flags])
            c = config(root)
            print(label + ":", c["roles"])
            assert [r["reviews"] for r in c["roles"]] == wanted, c
            run(root, [binary, "settings", "claude"], expected=2)
            run(root, [binary, "settings", "claude", "--as", "implementer"])
        print("PASS: custom defaults and explicit reviewer override; custom role names still require an explicit settings role.")
    elif mode == "post":
        root = repository(scratch / "project")
        run(root, [binary, "init"])
        run(root, ["git", "checkout", "-q", "-b", "feature"])
        (root / "change.txt").write_text("A small change.\n")
        run(root, ["git", "add", "change.txt"])
        run(root, ["git", "commit", "-q", "-m", "fixture change"])
        sha = run(root, ["git", "rev-parse", "HEAD"]).strip()
        body = scratch / "body with spaces.md"
        body.write_text("Please review the fixture commit.\n")
        brief = run(root, [binary, "hook", "brief", "--as", "claude"])
        line = next(x for x in brief.splitlines() if x.startswith("To ask for a review"))
        command = line.split("`", 2)[1]
        for key, value in {"<item>": "W1", "<sha>": sha, "<title>": "Independent review", "<file>": shlex.quote(str(body))}.items():
            command = command.replace(key, value)
        print("Printed command with placeholders filled:", command)
        print(run(root, ["/bin/sh", "-c", command]).strip())
        shown = run(root, [binary, "show", "1"])
        assert "from: claude → codex" in shown and "commit: " + sha in shown, shown
        codex_brief = run(root, [binary, "hook", "brief", "--as", "codex"])
        assert " post --as codex --item <item> --to claude " in codex_brief, codex_brief
        reply = [binary, "reply", "1", "--verdict", "QUESTION", "--commit", sha, "--body", str(body)]
        run(root, [*reply, "--as", "claude"], expected=2)
        print(run(root, [*reply, "--as", "codex"]).strip())
        print("PASS: the generated post command records the exact commit for the other reviewing agent; the reverse brief addresses claude. Both roles can review, but the requester cannot reply to its own request; its addressee can.")
