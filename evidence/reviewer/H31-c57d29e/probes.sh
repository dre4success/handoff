#!/usr/bin/env bash
# probes.sh <handoff binary> <empty scratch dir>: the reviewer's offline cases for H31 (c57d29e).
# Each case writes a transcript for a made-up session under a scratch settings folder and runs
# `handoff role reviewer` as that session in a scratch project. Scratch HOME, CLAUDE_CONFIG_DIR and
# CODEX_HOME; alerts go to a scratch file.
set -u
BIN=$1; T=$(cd "$2" && pwd -P)
export HOME=$T/home HANDOFF_TEST_NOTIFY=$T/alerts.txt HANDOFF_TEST_AWAKE=$T/awake.txt GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_ROLE CLAUDE_PROJECT_DIR
mkdir -p "$HOME" "$T/cfg" "$T/codex"
g() { git -c user.name=probe -c user.email=probe@example.invalid -c protocol.file.allow=always "$@"; }
scrub() { sed -e "s|$T|<scratch>|g" -e "s|$BIN|<the build>|g"; }
P=$T/proj; mkdir -p "$P" && g -C "$P" init -q -b main && g -C "$P" commit -q --allow-empty -m start && (cd "$P" && "$BIN" init > /dev/null)
n=0
mkt() { # mkt <id> <first cwd, as JSON>: a Claude Code transcript whose first lines name no folder
  mkdir -p "$T/cfg/projects/-x"
  printf '%s\n' '{"type":"mode","mode":"default"}' '{"type":"session-owner"}' "{\"type\":\"user\",\"cwd\":$2,\"message\":{\"role\":\"user\",\"content\":\"hello\"}}" '{"type":"assistant","cwd":"/somewhere/else"}' > "$T/cfg/projects/-x/$1.jsonl"
}
ask() { # ask <label> <first cwd as JSON> [folder to run in]: is the session warned?
  n=$((n+1)); id="s-$n"; mkt "$id" "$2"
  out=$(cd "${3:-$P}" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1)
  if printf '%s' "$out" | grep -q '^handoff: warning: this session started in'; then w="WARNED"; else w="not warned"; fi
  printf '%-78s %s\n' "$1" "$w"
}
q() { printf '"%s"' "$1"; }

printf '# command: bash probes.sh <the build of c57d29e> <an empty scratch folder>\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the Mac of the operator (darwin arm64, %s, git %s)\n# commit: c57d29eed941ff63acb91cce953dfc62d421fc94\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')"
echo "## N1. the same folder, spelt another way"
ln -s "$P" "$T/link-to-proj"; mkdir -p "$P/web/src"
ask "a. started in a link to the project" "$(q "$T/link-to-proj")"
ask "b. started in the project; the command runs through a link to it" "$(q "$P")" "$T/link-to-proj"
ask "c. started in the project, spelt in capitals (this volume ignores case)" "$(q "$(dirname "$P")/PROJ")"
ask "d. started in the project, spelt with /./ and a last /" "$(q "$P/./web/../")"
ask "e. started in a folder in the project, through the link" "$(q "$T/link-to-proj/web/src")"

echo; echo "## N2. repositories"
mkdir -p "$T/lib" && g -C "$T/lib" init -q -b main && g -C "$T/lib" commit -q --allow-empty -m lib
g -C "$P" submodule --quiet add "$T/lib" vendor/lib 2>&1 | scrub; g -C "$P" commit -q -m "a submodule"
ask "a. started in a submodule of the project (its git folder is in the project's)" "$(q "$P/vendor/lib")"
echo "   what handoff says when run there: $(cd "$P/vendor/lib" && "$BIN" inbox --as reviewer 2>&1 | head -1 | scrub | cut -c1-170)"
mkdir -p "$P/nested" && g -C "$P/nested" init -q -b main
ask "b. started in a repository of its own inside the project" "$(q "$P/nested")"
g init -q --bare "$T/bare.git"
ask "c. started in a bare repository" "$(q "$T/bare.git")"
mkdir -p "$T/dangling" && echo "gitdir: $T/nowhere/.git" > "$T/dangling/.git"
ask "d. started in a folder whose .git file points nowhere" "$(q "$T/dangling")"
g -C "$P" worktree add -q "$T/tree" -b side; mkdir -p "$T/tree/a/b"
ask "e. started in a folder in another worktree of the project" "$(q "$T/tree/a/b")"
mkdir -p "$T/above/inner" && g -C "$T/above" init -q -b main && g -C "$T/above/inner" init -q -b main && g -C "$T/above/inner" commit -q --allow-empty -m s && (cd "$T/above/inner" && "$BIN" init > /dev/null)
n=$((n+1)); id="s-$n"; mkt "$id" "$(q "$T/above")"
out=$(cd "$T/above/inner" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1)
printf '%-78s %s\n' "f. started in the folder above, which is a repository too" "$(printf '%s' "$out" | grep -q '^handoff: warning' && echo WARNED || echo "not warned")"
mkdir -p "$T/locked" && chmod 000 "$T/locked"
ask "g. started in a folder that can't be entered" "$(q "$T/locked")"; chmod 755 "$T/locked"
ask "h. the folder named is a file" "$(q "$P/.gitmodules")"
ask "i. started in the project's .handoff folder" "$(q "$P/.handoff")"
ask "j. started in the project's .git folder" "$(q "$P/.git")"
ask "k. started in the home folder, no repository" "$(q "$HOME")"

echo; echo "## N3. transcripts"
ask "a. the first cwd is null; the next line names /somewhere/else (gone)" "null"
n=$((n+1)); id="s-$n"; mkdir -p "$T/out"
printf '%s\n' '{"type":"user","cwd":7}' "{\"type\":\"user\",\"cwd\":\"$T/out\"}" > "$T/cfg/projects/-x/$id.jsonl"
out=$(cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1)
printf '%-78s %s\n' "b. the first cwd is a number; the next line names a folder outside" "$(printf '%s' "$out" | grep -q '^handoff: warning' && echo WARNED || echo "not warned")"
n=$((n+1)); id="s-$n"
printf '%s\n' "{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"{\\\"cwd\\\":\\\"$T/out\\\"}\",\"cwd\":\"$T/out\"}}" "{\"type\":\"user\",\"cwd\":\"$P\"}" > "$T/cfg/projects/-x/$id.jsonl"
out=$(cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1)
printf '%-78s %s\n' "c. an outside cwd inside a message only; the next line names the project" "$(printf '%s' "$out" | grep -q '^handoff: warning' && echo WARNED || echo "not warned")"
n=$((n+1)); id="s-$n"; mkt "$id" "$(q "$T/out")"; chmod 000 "$T/cfg/projects/-x/$id.jsonl"
out=$(cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1)
printf '%-78s %s\n' "d. a transcript that can't be read" "$(printf '%s' "$out" | grep -q '^handoff: warning' && echo WARNED || echo "not warned")"
n=$((n+1)); id="s-$n"; mkdir -p "$T/cfg/projects/-x/$id.jsonl"
out=$(cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role reviewer 2>&1; echo "exit $?")
printf '%-78s %s\n' "e. a folder in the transcript's place" "$(printf '%s' "$out" | grep -q '^handoff: warning' && echo WARNED || echo "not warned") ($(printf '%s' "$out" | tail -1))"
n=$((n+1)); id="s-$n"; mkt "$id" "$(q "$T/out")"
(cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role implementer > /dev/null 2>&1)
hook() { printf '{"session_id":"%s","hook_event_name":"SessionStart","source":"%s","transcript_path":"%s","cwd":"%s"}' "$id" "$1" "$2" "$P" | (cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" hook brief 2>&1) | grep -q '^handoff: warning' && echo WARNED || echo "not warned"; }
printf '%-78s %s\n' "f. the start brief (a hook), given the session's transcript" "$(hook compact "$T/cfg/projects/-x/$id.jsonl")"
printf '%-78s %s\n' "g. the start brief, given a path named for the session where no file is" "$(hook resume "$T/cfg/projects/-elsewhere/$id.jsonl")"
printf '%-78s %s\n' "h. the start brief, given no transcript" "$(hook resume "")"

echo; echo "## N4. where the warning stands in what each command prints (first 60 characters of each line)"
id="s-$n"
for c in "role implementer" "role" "brief"; do echo "-- handoff $c"; (cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" $c 2>&1) | head -6 | scrub | cut -c1-60 | sed 's/^$/(an empty line)/'; done
echo "-- the whole warning:"; (cd "$P" && CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role 2>&1) | grep '^handoff: warning' | scrub

echo; echo "## N5. with no session"
echo "handoff role: $(cd "$P" && CLAUDE_CONFIG_DIR=$T/cfg "$BIN" role 2>&1 | head -1 | cut -c1-110)"
echo "HANDOFF_ROLE=reviewer handoff brief: $(cd "$P" && HANDOFF_ROLE=reviewer CLAUDE_CONFIG_DIR=$T/cfg "$BIN" brief 2>&1 | grep -c '^handoff: warning') warnings"

echo; echo "## N6. time, with many folders to look through (handoff role, asked; best of 5, in ms)"
best() { python3 -I -c '
import subprocess, sys, time, os
b = 1e9
for _ in range(5):
    t = time.time(); subprocess.run(sys.argv[2:], cwd=sys.argv[1], capture_output=True); b = min(b, time.time() - t)
print("%d" % (b * 1000))' "$@"; }
id="s-$n"
echo "Claude Code, 2 project folders: $(CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg best "$P" "$BIN" role) ms"
python3 -I -c '
import os, sys
for i in range(3000): os.makedirs("%s/cfg/projects/-p%d" % (sys.argv[1], i))
for y in range(3):
    for m in range(1, 13):
        for d in range(1, 29):
            p = "%s/codex/sessions/%d/%02d/%02d" % (sys.argv[1], 2024 + y, m, d); os.makedirs(p)
            for k in range(20): open("%s/rollout-2026-01-01T00-00-%02d-0199-%d-%d-%d-%d.jsonl" % (p, k, y, m, d, k), "w").close()
' "$T"
echo "Claude Code, 3000 project folders: $(CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$$ CLAUDE_CONFIG_DIR=$T/cfg best "$P" "$BIN" role) ms"
mkdir -p "$T/codex/sessions/2026/11/15"; printf '{"type":"session_meta","payload":{"id":"0199-probe","cwd":"%s"}}\n' "$T/out" > "$T/codex/sessions/2026/11/15/rollout-2026-11-15T00-00-00-0199-probe.jsonl"
(cd "$P" && CODEX_THREAD_ID=0199-probe CODEX_HOME=$T/codex "$BIN" role reviewer-2 > /dev/null 2>&1)
echo "Codex, 1008 day folders with 20160 rollouts: $(CODEX_THREAD_ID=0199-probe CODEX_HOME=$T/codex best "$P" "$BIN" role) ms; warned: $(cd "$P" && CODEX_THREAD_ID=0199-probe CODEX_HOME=$T/codex "$BIN" role 2>&1 | grep -c '^handoff: warning')"
echo "with no session id (nothing looked for): $(best "$P" "$BIN" inbox --as reviewer) ms for handoff inbox"
echo; echo "alerts sent to the scratch file: $(cat "$T/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
