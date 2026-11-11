#!/usr/bin/env bash
# The reviewer's probes for H26 round 2 (8162dbf): a tell is for the session told. Run as
#   S=<scratchpad> B=<handoff built at the commit> BASE=<handoff built at 1af34c7> OLD=<handoff v0.1.2> WT=<worktree> bash probes2.sh > probes2.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; BASE=${BASE:?handoff at the base}; OLD=${OLD:?handoff v0.1.2}; WT=${WT:?worktree}
mkdir -p "$S/h26"
WORK=$(mktemp -d "$S/h26/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { sed -E "s|$BASE|handoff-base|g; s|$OLD|handoff-v0.1.2|g; s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# runin <env...> -- <cmd...>: a command in a session's environment
runin() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ env ${envs[*]} $*"; env "${envs[@]}" "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# term <env...> -- <cmd...>: a command at a terminal (script(1)), as the operator's own
term() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ [terminal] ${envs[*]:+env ${envs[*]} }$*"; env ${envs[@]+"${envs[@]}"} script -q /dev/null "$@" < /dev/null > "$WORK/o" 2>&1; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }
# hook <json> <env...> -- <hook args...>: a hook, fed what the CLI writes to its stdin
hook() { local json=$1; shift; local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ echo '$json' | env ${envs[*]} handoff hook $*"; printf '%s' "$json" | env "${envs[@]}" "$B" hook "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
stop() { printf '{"session_id":"%s","hook_event_name":"Stop"}' "$1"; }
sleeper() { sleep 600 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
N=0
newrepo() { N=$((N+1)); REPO="$WORK/repo$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; }
body() { printf '%s\n' "$1" > "$WORK/body.md"; echo "$WORK/body.md"; }
lines() { wc -l < .handoff/channel.jsonl | tr -d ' '; }

start() { printf '{"session_id":"%s","hook_event_name":"SessionStart","source":"%s"}' "$1" "$2"; }
rec() { echo; echo "# the record:"; sed -E 's/"ts":"[^"]*",//; s/"v":1,//' .handoff/channel.jsonl | clean; echo "# the cursors: $(for f in .handoff/cursors/*.json; do [ -f "$f" ] && printf '%s %s  ' "$(basename "$f" .json)" "$(sed -E 's/.*"id":([0-9]+).*/#\1/' "$f")"; done)"; }
main() {
printf '# command: the reviewer'"'"'s round 2 probes for handoff tell; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"

###############################################################################
echo "## R1. Round 1's P1, again: does an old tell reach a session started later?"
echo; echo "### R1a. The session told ends before its turn does; a new session takes the role"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-night CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "Carry on while I'm away. Merge when the reviewer approves."
kill $A; wait $A 2>/dev/null
C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-morning CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
hook "$(stop s-morning)" CLAUDE_CODE_SESSION_ID=s-morning CLAUDE_PID=$C -- listen --timeout 3s
run "$B" show 1
rec
kill $C; wait $C 2>/dev/null

echo; echo "### R1b. The operator moves the role before the first session's turn ends; then gives it back"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$B" role reviewer | sed -n 1,3p
term -- "$B" tell "Carry on while I'm away."
runin CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
echo "# s-two has the role and has not listened yet. The operator gives it back to s-one, which was the one told:"
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$B" role reviewer | sed -n 1,3p
hook "$(stop s-one)" CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- listen --timeout 3s
echo "# and the plain case: a second tell, to s-one; the role moves to s-two, which listens:"
term -- "$B" tell "Second."
runin CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
hook "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 3s
echo "# the role goes back to s-one once more, after s-two's listener passed the tell over:"
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$B" role reviewer | sed -n 1,3p
hook "$(stop s-one)" CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- listen --timeout 3s
rec
kill $A $C; wait $A $C 2>/dev/null

echo; echo "### R1c. A Codex thread that took its role by a command and ran no hook"
newrepo
runin CODEX_THREAD_ID=t-gone -- "$B" role reviewer | sed -n 1,3p
term -- "$B" tell "Carry on while I'm away."
runin CODEX_THREAD_ID=t-new -- "$B" role reviewer | sed -n 1,3p
runin CODEX_THREAD_ID=t-new -- "$B" wait --timeout 2s
hook "$(stop t-new)" LC_ALL=C -- listen --timeout 2s
rec

###############################################################################
echo; echo "## R2. The session told gets its tell, each way a session listens"
echo; echo "### R2a. Claude Code: told in the middle of a turn, it listens when the turn ends; after a resume (same id, a new process); with a plain wait in the session"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "One: read at the turn's end."
hook "$(stop s-impl)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- listen --timeout 3s
term -- "$B" tell "Two: read after a resume."
kill $A; wait $A 2>/dev/null
echo "# the session's process ended; the session is resumed in a new process, with the same id:"
C=$(sleeper)
hook "$(start s-impl resume)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$C -- brief | sed -n 1,4p
hook "$(stop s-impl)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$C -- listen --timeout 3s
term -- "$B" tell "Three: read by a plain wait in the session."
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$C -- "$B" wait --timeout 3s
echo; echo "### R2b. Codex: the role taken by a command (CODEX_THREAD_ID), the hook given the id on stdin alone"
runin CODEX_THREAD_ID=t-rev -- "$B" role reviewer | sed -n 1,3p
term -- "$B" tell "Four: both told."
hook "$(stop t-rev)" LC_ALL=C -- listen --timeout 3s
term -- "$B" tell "Five: read by a plain wait in the thread."
runin CODEX_THREAD_ID=t-rev -- "$B" wait --timeout 3s
echo; echo "### R2c. A listener already running when the tell is written wakes with it"
( printf '%s' "$(stop s-impl)" | env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$C "$B" hook listen --timeout 20s > "$WORK/l1.out" 2>&1; echo "# exit $?" >> "$WORK/l1.out" ) & L1=$!
( printf '%s' "$(stop t-rev)" | "$B" hook listen --timeout 20s > "$WORK/l2.out" 2>&1; echo "# exit $?" >> "$WORK/l2.out" ) & L2=$!
sleep 2
term -- "$B" tell "Six: both listening."
wait $L1 $L2
echo "# the implementer's:"; cat "$WORK/l1.out"; echo "# the reviewer's:"; cat "$WORK/l2.out"
rec
kill $C; wait $C 2>/dev/null

###############################################################################
echo; echo "## R3. The role's other entries around a tell that is passed over: nothing else is lost, and order holds"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-old CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin HANDOFF_ROLE=reviewer -- "$B" note --item W1 --to implementer --title "before the tell" --body "$(body b1)"
term -- "$B" tell "For s-old alone."
runin HANDOFF_ROLE=reviewer -- "$B" note --item W1 --to implementer --title "after the tell" --body "$(body b2)"
kill $A; wait $A 2>/dev/null
C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "For s-new."
hook "$(stop s-new)" CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$C -- listen --timeout 3s
echo "# a wait with --emit outside any session, already running, passes over s-new's next tell and goes on to the note after it:"
( "$B" wait --for implementer --emit --timeout 6s > "$WORK/emit.out" 2>&1; echo "# exit $?" >> "$WORK/emit.out" ) & E=$!
sleep 1.5
term -- "$B" tell "For s-new, while an outside wait streams."
runin HANDOFF_ROLE=reviewer -- "$B" note --item W1 --to implementer --title "after that tell" --body "$(body b3)"
wait $E; echo "# the --emit wait printed:"; cat "$WORK/emit.out"
rec
kill $C; wait $C 2>/dev/null

###############################################################################
echo; echo "## R4. /clear in Claude Code: the role moves to the new id; a tell to the old id is passed over (the implementer's choice 2)"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-before CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "Told before the clear."
hook "$(start s-after clear)" CLAUDE_CODE_SESSION_ID=s-after CLAUDE_PID=$A -- brief | sed -n 1,4p
cat .handoff/sessions/*.json | clean
hook "$(stop s-after)" CLAUDE_CODE_SESSION_ID=s-after CLAUDE_PID=$A -- listen --timeout 3s
term -- "$B" tell "Told after the clear."
hook "$(stop s-after)" CLAUDE_CODE_SESSION_ID=s-after CLAUDE_PID=$A -- listen --timeout 3s
rec
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## R5. A role held by a listener alone (the implementer's 3): the tell names no session and goes by role"
echo; echo "### R5a. A wait outside any session, and a session whose role its settings fix (HANDOFF_ROLE), both listening"
newrepo
"$B" wait --for implementer --timeout 20s > "$WORK/w1.out" 2>&1 & W1=$!
A=$(sleeper)
( printf '%s' "$(stop s-fixed)" | env HANDOFF_ROLE=reviewer CLAUDE_CODE_SESSION_ID=s-fixed CLAUDE_PID=$A "$B" hook listen --timeout 20s > "$WORK/w2.out" 2>&1; echo "# exit $?" >> "$WORK/w2.out" ) & W2=$!
sleep 2
term -- "$B" tell "To whoever listens."
wait $W1; echo "# exit $?" >> "$WORK/w1.out"; wait $W2
echo "# the wait outside a session:"; cat "$WORK/w1.out"; echo "# the session with a fixed role:"; cat "$WORK/w2.out"
rec
echo; echo "### R5b. The same session, in the middle of a turn (no listener): not told at all, and says so"
term -- "$B" tell "Anyone?"
echo; echo "### R5c. An unnamed tell left unread (its listener killed in the second before it read), then a session given the role: this is the window the implementer names"
"$B" wait --for implementer --timeout 20s > /dev/null 2>&1 & W1=$!
sleep 1.3; kill -STOP $W1
term -- "$B" tell "Unnamed, and its listener stopped."
kill -KILL $W1; wait $W1 2>/dev/null
C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-later CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
hook "$(stop s-later)" CLAUDE_CODE_SESSION_ID=s-later CLAUDE_PID=$C -- listen --timeout 3s
echo; echo "### R5d. A session with a fixed role, started later, meets a tell that names an earlier session: passed over"
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
term -- "$B" tell "For s-rev alone." 
rm -f .handoff/sessions/s-rev.json
echo "# (s-rev's binding removed, as if it had ended and been dropped)"
hook "$(stop s-fixed2)" HANDOFF_ROLE=reviewer CLAUDE_CODE_SESSION_ID=s-fixed2 CLAUDE_PID=$A -- listen --timeout 3s
rec
kill $A $C; wait $A $C 2>/dev/null

###############################################################################
echo; echo "## R6. A wait outside any session, for a role a session holds (the implementer's 1): it passes over that session's tell and moves the cursor, so the session never gets it"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "For s-impl."
runin HANDOFF_ROLE=reviewer -- "$B" note --item W1 --to implementer --title "a note" --body "$(body n)"
run "$B" wait --for implementer --timeout 2s
hook "$(stop s-impl)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- listen --timeout 3s
echo "# ^ the session gets neither: the outside wait took the note, as before H26, and passed over the tell, printing nothing of it"
rec
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## R7. An older handoff (v0.1.2) on a record whose tells name sessions"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-told CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell "Named for s-told."
kill $A; wait $A 2>/dev/null
C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-later CLAUDE_PID=$C -- "$OLD" role implementer | sed -n 1,3p
echo; echo "\$ echo '$(stop s-later)' | env CLAUDE_CODE_SESSION_ID=s-later CLAUDE_PID=<pid> handoff-v0.1.2 hook listen --timeout 3s"
printf '%s' "$(stop s-later)" | env CLAUDE_CODE_SESSION_ID=s-later CLAUDE_PID=$C "$OLD" hook listen --timeout 3s; echo "# exit $?"
echo "# ^ v0.1.2 ignores the field and delivers by role, as row 93 says: the promise holds only once every session runs the new handoff"
run "$OLD" show 1
runin HANDOFF_ROLE=reviewer -- "$OLD" note --item W1 --to implementer --title "by v0.1.2, after" --body "$(body x)"
run "$B" inbox --for implementer --quiet
rec
kill $C; wait $C 2>/dev/null

###############################################################################
echo; echo "## R8. The message's refusal (round 1's P6)"
newrepo
A=$(sleeper)
CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" role implementer > /dev/null
term -- "$B" tell "- no merges tonight"
term -- "$B" tell --as operator "Carry on."
kill $A; wait $A 2>/dev/null
}
main 2>&1 | clean
