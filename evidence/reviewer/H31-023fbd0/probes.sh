#!/usr/bin/env bash
# probes.sh <the build of 023fbd0> <an empty scratch folder>: the reviewer's offline cases for H31 round 3 (round 2's, with M5 for Antigravity).
# Scratch HOME and CODEX_HOME; alerts go to a scratch file.
set -u
BIN=$1; T=$(cd "$2" && pwd -P)
export HOME=$T/home CODEX_HOME=$T/codex HANDOFF_TEST_NOTIFY=$T/alerts.txt HANDOFF_TEST_AWAKE=$T/awake.txt GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_ROLE CLAUDE_PROJECT_DIR CLAUDECODE CLAUDE_CONFIG_DIR ANTIGRAVITY_CONVERSATION_ID
mkdir -p "$HOME" "$CODEX_HOME"
g() { git -c user.name=probe -c user.email=probe@example.invalid -c protocol.file.allow=always "$@"; }
scrub() { sed -e "s|$T|<scratch>|g" -e "s|/private/tmp/claude[^ ]*|handoff|g"; }
printf '# command: bash probes.sh <the build of 023fbd0> <an empty scratch folder>\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the Mac of the operator (darwin arm64, %s, git %s)\n# commit: 023fbd038e348404ff2c1d50e608829ba461b5db\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')"
P=$T/proj; mkdir -p "$P/web/src" "$T/out" && g -C "$P" init -q -b main && g -C "$P" commit -q --allow-empty -m start && (cd "$P" && "$BIN" init > /dev/null)
n=0
stand() { # stand <folder>: a process in that folder, standing for Claude Code's; prints its pid
  (cd "$1" 2>/dev/null && exec sleep 300) > /dev/null 2>&1 & echo $!
}
w() { grep -q '^handoff: warning: this session started in' && echo WARNED || echo "not warned"; }
cmd() { # cmd <label> <folder the session's process is in> [folder the command runs in]
  n=$((n+1)); pid=$(stand "$2"); sleep 0.2
  printf '%-74s %s\n' "$1" "$(cd "${3:-$P}" && CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" role reviewer 2>&1 | w)"; kill $pid 2>/dev/null
}
hook() { # hook <label> <CLAUDE_PROJECT_DIR> <folder the session's process is in>
  n=$((n+1)); pid=$(stand "$3"); sleep 0.2
  (cd "$P" && CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" role reviewer > /dev/null 2>&1)
  printf '%-74s %s\n' "$1" "$(cd "$P" && printf '{"session_id":"s-%s","hook_event_name":"SessionStart","source":"compact"}' "$n" | env ${2:+CLAUDE_PROJECT_DIR="$2"} CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" hook brief 2>&1 | w)"; kill $pid 2>/dev/null
}
echo "## M1. a Claude Code command: the folder of the session's process (lsof)"
ln -s "$P" "$T/link"; g -C "$P" worktree add -q "$T/tree" -b side; mkdir -p "$P/nested" && g -C "$P/nested" init -q -b main
cmd "a. its process is in the project" "$P"
cmd "b. in a folder in the project" "$P/web/src"
cmd "c. in the project, entered through a link" "$T/link"
cmd "d. in another worktree of the project" "$T/tree"
cmd "e. in a folder in no repository" "$T/out"
cmd "f. in the folder above the project" "$T"
cmd "g. in a repository of its own inside the project" "$P/nested"
cmd "h. in the project; the command runs in a folder in the project" "$P" "$P/web/src"
n=$((n+1)); pid=$(stand "$T/out"); kill $pid; sleep 0.3
printf '%-74s %s\n' "i. its process has ended" "$(cd "$P" && CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" role reviewer 2>&1 | w)"
n=$((n+1)); printf '%-74s %s\n' "j. CLAUDE_PID is not a number" "$(cd "$P" && CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=abc "$BIN" role reviewer 2>&1 | w)"
n=$((n+1)); mkdir -p "$T/gone"; pid=$(stand "$T/gone"); sleep 0.2; rmdir "$T/gone"
printf '%-74s %s\n' "k. its process is in a folder since removed" "$(cd "$P" && CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" role reviewer 2>&1 | w)"; kill $pid 2>/dev/null
n=$((n+1)); pid=$(stand "$T/out"); sleep 0.2; mkdir -p "$T/fakebin"; printf '#!/bin/sh\nsleep 30\n' > "$T/fakebin/lsof"; chmod +x "$T/fakebin/lsof"
echo "l. an lsof first on PATH that sleeps 30 s is not the one run (the build names /usr/sbin/lsof):"
s=$(python3 -I -c 'import time; print(time.time())'); r=$(cd "$P" && PATH="$T/fakebin:$PATH" CLAUDE_CODE_SESSION_ID=s-$n CLAUDE_PID=$pid "$BIN" role reviewer 2>&1 | w); e=$(python3 -I -c 'import time,sys; print("%.1f" % (time.time()-float(sys.argv[1])))' "$s")
printf '%-74s %s\n' "   handoff role took $e s" "$r"; kill $pid 2>/dev/null
echo "m. time of handoff role with the lsof call, best of 5: $(python3 -I -c '
import subprocess, sys, time, os
p = subprocess.Popen(["sleep", "60"], cwd=sys.argv[2]); b = 1e9
for _ in range(5):
    t = time.time(); subprocess.run([sys.argv[1], "role"], cwd=sys.argv[3], env=dict(os.environ, CLAUDE_CODE_SESSION_ID="s-1", CLAUDE_PID=str(p.pid)), capture_output=True); b = min(b, time.time() - t)
p.kill(); print("%d ms" % (b * 1000))' "$BIN" "$T/out" "$P")"

echo; echo "## M2. a Claude Code hook: CLAUDE_PROJECT_DIR (the start brief after /compact)"
hook "a. CLAUDE_PROJECT_DIR outside; the process is outside too" "$T/out" "$T/out"
hook "b. CLAUDE_PROJECT_DIR is the project; the process is outside" "$P" "$T/out"
hook "c. CLAUDE_PROJECT_DIR outside; the process is in the project" "$T/out" "$P"
hook "d. no CLAUDE_PROJECT_DIR; the process is outside (falls to lsof)" "" "$T/out"
hook "e. CLAUDE_PROJECT_DIR is not absolute" "out" "$P"

echo; echo "## M3. Codex: the rollout's last turn_context"
roll() { # roll <id> <lines...>: a rollout for id
  d="$CODEX_HOME/sessions/2026/11/15"; mkdir -p "$d"; f="$d/rollout-2026-11-15T00-00-00-$1.jsonl"; shift; printf '%s\n' "$@" > "$f"; echo "$f"
}
meta() { printf '{"timestamp":"t","type":"session_meta","payload":{"id":"x","cwd":"%s"}}' "$1"; }
turn() { printf '{"timestamp":"t","type":"turn_context","payload":{"cwd":"%s","model":"m"}}' "$1"; }
codex() { # codex <label> <id> [extra env]
  printf '%-74s %s\n' "$1" "$(cd "$P" && env ${3:-NOTHING=1} CODEX_THREAD_ID=$2 "$BIN" role reviewer-2 2>&1 | w)"
}
roll 019a-a "$(meta "$T/out")" "$(turn "$T/out")" > /dev/null;                       codex "a. begun outside, one turn outside" 019a-a
roll 019a-b "$(meta "$T/out")" "$(turn "$T/out")" "$(turn "$P")" > /dev/null;        codex "b. begun outside, the last turn in the project" 019a-b
roll 019a-c "$(meta "$P")" "$(turn "$P")" "$(turn "$T/out")" > /dev/null;            codex "c. begun in the project, the last turn outside" 019a-c
roll 019a-d "$(meta "$T/out")" > /dev/null;                                           codex "d. begun outside, no turn yet (only the first line)" 019a-d
roll 019a-e "$(meta "$P")" "$(turn "$P")" '{"timestamp":"t","type":"response_item","payload":{"type":"message","content":"{\"type\":\"turn_context\",\"payload\":{\"cwd\":\"'"$T/out"'\"}}"}}' > /dev/null; codex "e. the last turn in the project, then a message quoting an outside turn" 019a-e
roll 019a-f "$(meta "$P")" "$(turn "$P")" '{"timestamp":"t", "type": "turn_context", "payload": {"cwd": "'"$T/out"'"}}' > /dev/null; codex "f. the last turn outside, written with spaces after the colons" 019a-f
f=$(roll 019a-g "$(meta "$P")" "$(turn "$T/out")"); python3 -I -c 'import sys; open(sys.argv[1], "a").write("{\"timestamp\":\"t\",\"type\":\"response_item\",\"payload\":{\"x\":\"" + "y" * (17 << 20) + "\"}}\n")' "$f"; codex "g. the last turn outside, with 17 MiB written after it" 019a-g
f=$(roll 019a-h "$(meta "$P")" "$(turn "$T/out")"); python3 -I -c 'import sys; open(sys.argv[1], "a").write("{\"timestamp\":\"t\",\"type\":\"response_item\",\"payload\":{\"x\":\"" + "y" * (15 << 20) + "\"}}\n")' "$f"; codex "h. the same with 15 MiB after it" 019a-h
echo "i. time of handoff role for h, best of 5: $(python3 -I -c '
import subprocess, sys, time, os
b = 1e9
for _ in range(5):
    t = time.time(); subprocess.run([sys.argv[1], "role"], cwd=sys.argv[2], env=dict(os.environ, CODEX_THREAD_ID="019a-h"), capture_output=True); b = min(b, time.time() - t)
print("%d ms" % (b * 1000))' "$BIN" "$P")"
pid=$(stand "$T/out"); codex "j. a Codex session whose shell also has a Claude Code session's variables" 019a-b "CLAUDE_PID=$pid CLAUDE_CODE_SESSION_ID=s-outer"; kill $pid 2>/dev/null

echo; echo "## M5. Antigravity (new in round 3)"
agy() { printf '%-74s %s\n' "$1" "$(cd "$P" && env $3 "$BIN" role $2 2>&1 | w)"; }
roll 0a1b-agy "$(meta "$T/out")" "$(turn "$T/out")" > /dev/null
agy "a. an Antigravity session, a Codex rollout named for its id, last turn outside" reviewer "ANTIGRAVITY_CONVERSATION_ID=0a1b-agy"
agy "b. the same id as a Codex session (the rollout is read)" reviewer-2 "CODEX_THREAD_ID=0a1b-agy"
pid=$(stand "$T/out"); sleep 0.2
agy "c. an Antigravity agent started from a Claude Code session's shell (process outside)" reviewer "ANTIGRAVITY_CONVERSATION_ID=c-3 CLAUDE_CODE_SESSION_ID=s-outer2 CLAUDE_PID=$pid"; kill $pid 2>/dev/null
(cd "$P" && ANTIGRAVITY_CONVERSATION_ID=c-4 "$BIN" role reviewer > /dev/null 2>&1)
printf '%-74s %s\n' "d. an Antigravity SessionStart hook run outside any repository, stdin's cwd the project" "$(cd "$T/out" && printf '{"conversationId":"c-4","hookEventName":"SessionStart","cwd":"%s"}' "$P" | ANTIGRAVITY_CONVERSATION_ID=c-4 "$BIN" hook brief 2>&1 | grep -c 'handoff: warning') warning; the brief is there: $(cd "$T/out" && printf '{"conversationId":"c-4","hookEventName":"SessionStart","cwd":"%s"}' "$P" | ANTIGRAVITY_CONVERSATION_ID=c-4 "$BIN" hook brief 2>&1 | grep -c 'you are reviewer')"

echo; echo "## M4. the words"
pid=$(stand "$T/out"); sleep 0.2; echo "a. Claude Code:"; (cd "$P" && CLAUDE_CODE_SESSION_ID=s-words CLAUDE_PID=$pid "$BIN" role implementer 2>&1 | grep '^handoff: warning' | scrub); kill $pid
echo "b. Codex:"; (cd "$P" && CODEX_THREAD_ID=019a-a "$BIN" role reviewer-2 2>&1 | grep '^handoff: warning' | scrub)
Q="$T/it's a proj"; mkdir -p "$Q" && g -C "$Q" init -q -b main && g -C "$Q" commit -q --allow-empty -m s && (cd "$Q" && "$BIN" init > /dev/null)
pid=$(stand "$T/out"); sleep 0.2; echo "c. a project whose path a shell would split; the command it offers, run by sh -c with claude swapped for pwd:"
c=$(cd "$Q" && CLAUDE_CODE_SESSION_ID=s-q CLAUDE_PID=$pid "$BIN" role reviewer 2>&1 | grep '^handoff: warning' | sed -n 's/.*(`\(.*\)`).*/\1/p'); kill $pid
echo "   $(printf '%s' "$c" | scrub)"; echo "   -> $(sh -c "$(printf '%s' "$c" | sed 's/&& claude --resume .*/\&\& pwd/')" 2>&1 | scrub)"
echo; echo "alerts written to the scratch file: $(cat "$T/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
