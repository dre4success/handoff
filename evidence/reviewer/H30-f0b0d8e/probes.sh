#!/usr/bin/env bash
# The reviewer's probes for H30 round 2 (f0b0d8e): a role follows a Claude Code session
# sent to the background. Round 1's M1 to M11, with each line now written with the time it is made, and M12 to M14 for the wait, the time and the bindings of other builds. Hook input and transcripts are made here, in the
# shape Claude Code 2.1.325 gave them in real-probe.log. Run as
#   S=<scratchpad> B=<handoff built at the commit> OLD=<handoff built at 2336ecd, the code of v0.1.4> WT=<worktree> bash probes.sh > probes.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; OLD=${OLD:?older handoff}; WT=${WT:?worktree}
mkdir -p "$S/h30r2"
WORK=$(mktemp -d "$S/h30r2/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { perl -pe "s|\Q$OLD\E|handoff-v0.1.4|g; s|\Q$B\E|handoff|g; s|\Q$HOME\E|~|g; s|\Q$WORK\E|<scratch>|g; s/\"pid\":[0-9]+/\"pid\":<pid>/g; s/\"host\":[0-9]+/\"host\":<pid>/g; s/pid [0-9]+/pid <pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g; s/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/<ts>/g"; }
short() { perl -pe "s|\Q$OLD\E|handoff|g; s|\Q$B\E|handoff|g"; }
sleeper() { sleep 900 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
# ended <pid>: end a process and wait until it is gone
ended() { kill "$1" 2>/dev/null; while kill -0 "$1" 2>/dev/null; do sleep 0.05; done; }
body() { printf '%s\n' "${1:-ok}" > "$WORK/body.md"; echo "$WORK/body.md"; }
N=0
# newrepo: a scratch project and a transcripts folder beside it, as Claude Code keeps one per project
newrepo() { N=$((N+1)); REPO="$WORK/m$N"; TR="$WORK/transcripts/-scratch-m$N"; mkdir -p "$TR"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; : > "$WORK/alerts.txt"; echo "# a scratch project"; }
# give <session> <pid> <role> [binary]: the operator gives a session its role, and the session runs handoff role
give() { echo; echo "\$ [session $1] handoff role $3"; env CLAUDE_CODE_SESSION_ID="$1" CLAUDE_PID="$2" "${4:-$B}" role "$3" 2>&1 | head -1; }
# now_ms: the time now, as Claude Code writes it in a transcript
now_ms() { perl -MTime::HiRes=time -MPOSIX -e '$t=time; printf "%s.%03dZ", strftime("%Y-%m-%dT%H:%M:%S", gmtime($t)), ($t*1000)%1000'; }
# transcript <id> [next] [dir] [time]: a session's transcript; with next, its last lines say it went on in that session, at that time (now, if none is given; "none" leaves the time out)
transcript() { local id=$1 next=${2:-} dir=${3:-$TR} at=${4:-$(now_ms)} ts; ts="\"timestamp\":\"$at\","; [ "$at" = none ] && ts=""; { printf '{"type":"user","sessionId":"%s","message":{"role":"user","content":"Run the command."}}\n' "$id"; printf '{"type":"system","subtype":"informational","content":"Backgrounding after the current tool finishes\xe2\x80\xa6","sessionId":"%s"}\n' "$id"; [ -n "$next" ] && printf '{"type":"continued-in",%s"sessionId":"%s","continuedInSessionId":"%s"}\n' "$ts" "$id" "$next"; printf '{"type":"cost-state","sessionId":"%s","totalCostUSD":0.01}\n{"type":"file-history-snapshot","sessionId":"%s"}\n{"type":"last-prompt","lastPrompt":"Run the command.","sessionId":"%s"}\n{"type":"cost-state","sessionId":"%s","totalCostUSD":0.01}\n' "$id" "$id" "$id" "$id"; } > "$dir/$id.jsonl"; }
# input <session> <event> [source] [transcript]: what Claude Code writes to a hook's stdin
input() { local t=${4-$TR/$1.jsonl}; printf '{"session_id":"%s","transcript_path":"%s","cwd":"%s","hook_event_name":"%s"%s}' "$1" "$t" "$REPO" "$2" "${3:+,\"source\":\"$3\"}"; }
# hook <session> <pid> <event> <source> <brief|listen|notify> [binary] [transcript]: that session's hook
hook() { local t; if [ $# -ge 7 ]; then t=$7; else t=$TR/$1.jsonl; fi; echo; echo "\$ [session $1, at ${3}${4:+ ($4)}] handoff hook $5"; input "$1" "$3" "$4" "$t" | env CLAUDE_CODE_SESSION_ID="$1" CLAUDE_PID="$2" "${6:-$B}" hook $5 $([ "$5" = listen ] && echo --timeout 2s) > "$WORK/o" 2>&1; c=$?; short < "$WORK/o" | cut -c1-150 | head -3; echo "# exit $c"; }
# timed <label> <cmd...>: a command, its first line and how long it took, to a tenth of a second
timed() { local label=$1 t0 t1 o; shift; o=$(mktemp "$WORK/timed.XXXXXX"); t0=$(perl -MTime::HiRes=time -e 'print time'); "$@" > "$o" 2>&1; local c=$?; t1=$(perl -MTime::HiRes=time -e 'print time'); printf '%s: exit %s after %.1f s: %s\n' "$label" "$c" "$(echo "$t1 - $t0" | bc -l)" "$(short < "$o" | head -1 | cut -c1-90)"; }
bindings() { echo "# bindings: $(cat "$REPO"/.handoff/sessions/*.json 2>/dev/null | tr '\n' ' ')"; }
# moved <session>: one line on who holds what
who() { echo "# roles now: $(for f in "$REPO"/.handoff/sessions/*.json; do [ -f "$f" ] && perl -ne 'print "$2 has $1; " if /"role":"([^"]+)","session":"([^"]+)"/' "$f"; done)"; }
# nomove <what>: after `give s-a <dead pid> implementer`, session s-b's SessionStart and Stop must leave the role where it is
nomove() { local what=$1 t=${2-$TR/s-b.jsonl}; local out1 out2; out1=$(input s-b SessionStart fork "$t" | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief 2>&1 | head -1 | cut -c1-58); out2=$(input s-b Stop "" "$t" | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook listen --timeout 1s 2>&1; echo "exit $?"); printf '%-66s | %-58s | %-42s | %s\n' "$what" "$out1" "$(echo $out2 | cut -c1-42)" "$(for f in "$REPO"/.handoff/sessions/*.json; do perl -ne 'print "$2 has $1; " if /"role":"([^"]+)","session":"([^"]+)"/' "$f"; done)"; }
# fresh <old id> : a new project, the role given to the old session, whose process then ends
fresh() { newrepo > /dev/null; OLDPID=$(sleeper); env CLAUDE_CODE_SESSION_ID="${1:-s-a}" CLAUDE_PID=$OLDPID "$B" role implementer > /dev/null 2>&1; ended $OLDPID; }
term() { shift; echo; echo "\$ [terminal] handoff $*"; script -q /dev/null "$B" "$@" < /dev/null > "$WORK/o" 2>&1; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }

main() {
printf '# command: the reviewer'"'"'s probes for H30 round 2, a role follows a session sent to the background; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"
NEWPID=$(sleeper)

###############################################################################
echo "## M1. /background: the old session's process has ended, and its transcript names the new session"
for bin in "$OLD" "$B"; do
  echo; echo "# --- with $bin"
  newrepo; OLDPID=$(sleeper)
  give s-old $OLDPID implementer "$bin"
  ended $OLDPID; transcript s-old s-new; transcript s-new
  env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$NEWPID "$bin" note --as reviewer --to implementer --item W1 --title "before" --body "$(body)" 2>&1 | sed 's/^/  note to implementer, before the new session starts: /'
  hook s-new $NEWPID SessionStart fork brief "$bin"
  hook s-new $NEWPID SessionStart fork listen "$bin"
  bindings
  env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$NEWPID "$bin" note --as reviewer --to implementer --item W1 --title "after" --body "$(body later)" 2>&1 | sed 's/^/  note to implementer, after: /'
  echo; echo "\$ [session s-new] handoff inbox   (no --as)"; env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$bin" inbox 2>&1 | head -2
done

###############################################################################
echo; echo "## M2. The old process still runs when the new session starts, as in the real runs"
echo "# (a) it ends one second after the new session's hooks start; the brief, the listener and a permission-prompt hook start together"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer; SINCE=$(perl -ne 'print $1 if /"since":"([^"]+)"/' .handoff/sessions/s-old.json)
transcript s-old s-new; transcript s-new
( sleep 1; kill $OLDPID ) &
( input s-new SessionStart fork > "$WORK/in.b"; timed "  brief " env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook brief < "$WORK/in.b" ) > "$WORK/t.b" &
( input s-new SessionStart fork > "$WORK/in.l"; timed "  listen" env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook listen --timeout 2s < "$WORK/in.l" ) > "$WORK/t.l" &
( printf '{"session_id":"s-new","transcript_path":"%s","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' "$TR/s-new.jsonl" > "$WORK/in.n"; timed "  notify" env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook notify < "$WORK/in.n" ) > "$WORK/t.n" &
wait; cat "$WORK/t.b" "$WORK/t.l" "$WORK/t.n"
who; bindings | perl -pe 's/"since":"[^"]+"/"since":"<ts>"/'
echo "# the binding's since is the time the role was given: $([ "$(perl -ne 'print $1 if /"since":"([^"]+)"/' .handoff/sessions/s-new.json)" = "$SINCE" ] && echo yes || echo NO); alerts: $(cut -c1-80 "$WORK/alerts.txt" | tr '\n' ' ')"
echo; echo "# (b) it never ends: the hooks wait five seconds, and the running session keeps its role"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer
transcript s-old s-new; transcript s-new
input s-new SessionStart fork > "$WORK/in.b"; timed "  brief " env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook brief < "$WORK/in.b"
who
echo "# a note in that time:"
env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$NEWPID "$B" note --as reviewer --to implementer --item W1 --title "meanwhile" --body "$(body)" 2>&1 | sed 's/^/  the reviewer: /'
ended $OLDPID; echo "# the old process ends later; the new session's next hook:"
hook s-new $NEWPID Stop "" listen
who
echo; echo "# (c) while a hook waits, the sessions' lock is free, and a role given meanwhile is kept"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer
transcript s-old s-new; transcript s-new
( input s-new SessionStart fork > "$WORK/in.b"; timed "  s-new's brief" env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook brief < "$WORK/in.b" ) > "$WORK/t.b" &
sleep 1; XPID=$(sleeper)
timed "  a third session's handoff role implementer, one second into the wait" env CLAUDE_CODE_SESSION_ID=s-x CLAUDE_PID=$XPID "$B" role implementer
ended $OLDPID; wait; cat "$WORK/t.b"; who
echo; echo "# (d) the same, and the role given meanwhile is the waiting session's own"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer
transcript s-old s-new; transcript s-new
( input s-new SessionStart fork > "$WORK/in.b"; timed "  s-new's brief" env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook brief < "$WORK/in.b" ) > "$WORK/t.b" &
sleep 1
timed "  handoff role reviewer in s-new itself, one second into the wait" env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" role reviewer
wait; cat "$WORK/t.b"; who; ended $OLDPID
echo
echo "# (e) with a permission-prompt hook as the first after the old process has ended:"
newrepo; OLDPID=$(sleeper); give s-old $OLDPID reviewer; ended $OLDPID; transcript s-old s-new; transcript s-new
echo; echo "\$ [session s-new, at Notification (permission_prompt)] handoff hook notify"
printf '{"session_id":"s-new","transcript_path":"%s","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' "$TR/s-new.jsonl" | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook notify; echo "# exit $?"
who; echo "# alerts file: $(cat "$WORK/alerts.txt" 2>/dev/null | cut -c1-120)"

###############################################################################
echo; echo "## M3. Nothing else moves a role. In each line: implementer is given to s-a, whose process ends; then a line is written, with the time it is written unless the case says otherwise; then s-b's SessionStart and Stop"
printf '%-66s | %-58s | %-42s | %s\n' "case" "s-b's brief at SessionStart begins" "its Stop hook" "who has a role after"
fresh; transcript s-a; transcript s-b
nomove "/fork: s-a's transcript has no such line"
fresh; transcript s-a s-c; transcript s-b; transcript s-c
nomove "the line names another session, s-c"
fresh; transcript s-a s-c; transcript s-b; transcript s-c
nomove "the hook gives s-c's transcript, which the line names" "$TR/s-c.jsonl"
fresh; transcript s-a s-b; mkdir -p "$WORK/transcripts/-other"; transcript s-b "" "$WORK/transcripts/-other"
nomove "s-b's transcript is in another folder than s-a's" "$WORK/transcripts/-other/s-b.jsonl"
fresh; transcript s-a s-b; transcript s-b
nomove "the hook gives no transcript" ""
fresh; transcript s-a s-b; transcript s-b
nomove "the hook's transcript has another name (s-b.txt)" "$TR/s-b.txt"
fresh "../s-a"; transcript s-b; mkdir -p "$TR/sub"; transcript s-a s-b "$TR/..";
nomove "the old id is ../s-a, and ../s-a.jsonl names s-b"
fresh; transcript s-b; printf '{"type":"assistant","sessionId":"s-a","message":{"content":"{\\"type\\":\\"continued-in\\",\\"continuedInSessionId\\":\\"s-b\\"}"}}\n{"type":"user","message":{"type":"continued-in","continuedInSessionId":"s-b"}}\n' > "$TR/s-a.jsonl"
nomove "the line is only quoted inside a message in s-a's transcript"
fresh; transcript s-b; printf '{"type":"continued-in","sessionId":"s-a"}\n{"type":"continued-in","continuedInSessionId":""}\n{"type":"continued-in","continuedInSessionId":7}\n{"type":"continued-in","continuedInSessionId":"s-b"\n' > "$TR/s-a.jsonl"
nomove "the line names nobody, or is not whole JSON"
fresh; transcript s-a s-b; transcript s-b; perl -e 'print((qq({"type":"assistant","message":{"content":").("x" x 1000).qq("}}\n)) x 70)' >> "$TR/s-a.jsonl"
nomove "70 KiB of transcript follow the line (s-a was used again)"
fresh; transcript s-a s-b; transcript s-b; printf '{"type":"continued-in","timestamp":"2026-11-13T23:00:00.000Z","sessionId":"s-a","continuedInSessionId":"s-z"}\n' >> "$TR/s-a.jsonl"
nomove "a later line in s-a's transcript names s-z"
fresh; transcript s-a s-a; transcript s-b
nomove "s-a's transcript names itself"
fresh; transcript s-a s-1; for i in 1 2 3 4 5 6 7; do transcript s-$i s-$((i+1)); done; transcript s-8 s-b; transcript s-b
nomove "nine moves from s-a to s-b (the limit is eight)"
fresh; rm -f "$TR/s-a.jsonl"; mkdir "$TR/s-a.jsonl"; transcript s-b
nomove "s-a.jsonl is a folder"
fresh; transcript s-a s-b "$TR" "2026-11-13T22:18:15.118Z"; transcript s-b
nomove "the line is older than the role"
fresh; transcript s-a s-b "$TR" none; transcript s-b
nomove "the line has no time"
fresh; transcript s-a s-b "$TR" "yesterday"; transcript s-b
nomove "the line's time is not a time"
fresh; transcript s-a s-1; transcript s-1 s-b "$TR" "2026-11-13T22:18:15.118Z"; transcript s-b
nomove "two moves, the second line older than the role"
fresh; transcript s-a s-b; transcript s-b; perl -pi -e 's/"since":"[^"]+"/"since":"long ago"/' .handoff/sessions/s-a.json
nomove "the binding's since is not a time"
# a binding with no process: a Codex session's `handoff role`, which records none
newrepo > /dev/null; env CODEX_THREAD_ID=s-a "$B" role implementer > /dev/null 2>&1; transcript s-a s-b; transcript s-b
nomove "s-a's binding names no process (given in a Codex session)"
# the old pid belongs to another process now
newrepo > /dev/null; env CLAUDE_CODE_SESSION_ID=s-a CLAUDE_PID=$NEWPID "$B" role implementer > /dev/null 2>&1; OTHER=$(sleeper); NEWPID_SAVE=$NEWPID; NEWPID=$OTHER; transcript s-a s-b; transcript s-b
nomove "s-a's process id is some running process's (each hook waits 5 s)"; NEWPID=$NEWPID_SAVE
# two sessions' transcripts both name s-b
newrepo > /dev/null; P1=$(sleeper); P2=$(sleeper); env CLAUDE_CODE_SESSION_ID=s-a CLAUDE_PID=$P1 "$B" role implementer > /dev/null 2>&1; env CLAUDE_CODE_SESSION_ID=s-a2 CLAUDE_PID=$P2 "$B" role reviewer > /dev/null 2>&1; ended $P1; ended $P2; transcript s-a s-b; transcript s-a2 s-b; transcript s-b
nomove "two ended sessions with roles both name s-b"
# settings that fix the role
fresh; transcript s-a s-b; transcript s-b
o1=$(input s-b SessionStart fork | env HANDOFF_ROLE=reviewer CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief 2>&1 | head -1 | cut -c1-58)
printf '%-66s | %-58s | %-42s | %s\n' "HANDOFF_ROLE=reviewer in s-b's settings" "$o1" "(not run)" "$(for f in "$REPO"/.handoff/sessions/*.json; do perl -ne 'print "$2 has $1; " if /"role":"([^"]+)","session":"([^"]+)"/' "$f"; done)"
# a Codex hook: its transcript is a rollout file, not named after the thread
newrepo > /dev/null; OLDPID=$(sleeper); env CLAUDE_CODE_SESSION_ID=s-a CLAUDE_PID=$OLDPID "$B" role implementer > /dev/null 2>&1; ended $OLDPID; transcript s-a s-b; cp "$TR/s-a.jsonl" "$TR/rollout-2026-11-13T22-18-15-s-b.jsonl"
o1=$(printf '{"session_id":"s-b","transcript_path":"%s","hook_event_name":"SessionStart","source":"startup"}' "$TR/rollout-2026-11-13T22-18-15-s-b.jsonl" | env CODEX_THREAD_ID=s-b "$B" hook brief 2>&1 | head -1 | cut -c1-58)
printf '%-66s | %-58s | %-42s | %s\n' "a Codex thread s-b, its rollout file beside s-a's transcript" "$o1" "(not run)" "$(for f in "$REPO"/.handoff/sessions/*.json; do perl -ne 'print "$2 has $1; " if /"role":"([^"]+)","session":"([^"]+)"/' "$f"; done)"

###############################################################################
echo; echo "## M4. What does move it besides one /background"
fresh; transcript s-a s-1; for i in 1 2 3 4 5 6; do transcript s-$i s-$((i+1)); done; transcript s-7 s-b; transcript s-b
echo "# eight moves from s-a to s-b, none of the sessions between ran a hook:"
hook s-b $NEWPID Stop "" listen; who
fresh; transcript s-b; transcript s-a s-b "$WORK"; ln -s "$WORK/s-a.jsonl" "$TR/s-a.jsonl"
echo; echo "# s-a.jsonl is a symbolic link to a file kept elsewhere:"
hook s-b $NEWPID SessionStart fork brief; who
fresh; transcript s-b; perl -e 'print((qq({"type":"assistant","message":{"content":").("x" x 1000).qq("}}\n)) x 3000)' > "$TR/s-a.jsonl"; printf '{"TYPE":"continued-in","TimeStamp":"%s","ContinuedInSessionID":"s-b"}\n{"type":"cost-state"}\n' "$(now_ms)" >> "$TR/s-a.jsonl"
echo; echo "# a 3 MB transcript; the line's keys in other letter case (Go's JSON reading takes them):"
hook s-b $NEWPID SessionStart fork brief; who

fresh; transcript s-a s-b "$TR" "2031-01-01T00:00:00.000+01:00"; transcript s-b
echo; echo "# the line's time is years ahead, and written with an offset:"
hook s-b $NEWPID SessionStart fork brief; who

###############################################################################
echo; echo "## M5. After the move, the operator gives the role to another session: the moved-to session does not take it back"
fresh; transcript s-a s-b; transcript s-b
hook s-b $NEWPID SessionStart fork brief; who
XPID=$(sleeper); give s-x $XPID implementer
hook s-b $NEWPID Stop "" listen
hook s-b $NEWPID SessionStart resume brief
who
echo; echo "# And when the operator gives the role to another session before the new session's first hook:"
fresh; transcript s-a s-b; transcript s-b
give s-x $XPID implementer
hook s-b $NEWPID SessionStart fork brief; who
echo; echo "# And when the operator gives a different role to another session first (handoff role drops bindings of ended sessions):"
fresh; transcript s-a s-b; transcript s-b
give s-x $XPID reviewer
hook s-b $NEWPID SessionStart fork brief; who

###############################################################################
echo; echo "## M6. An old line, and a role given after it"
echo "# s-a is sent to the background and goes on as s-b; the role moves. Later the operator"
echo "# resumes s-a, which keeps its id and its transcript (real-probe.log, R8), and gives it the role again."
fresh; transcript s-a s-b; transcript s-b
hook s-b $NEWPID SessionStart fork brief
APID=$(sleeper); give s-a $APID implementer
printf '{"type":"user","sessionId":"s-a","message":{"role":"user","content":"you are implementer"}}\n{"type":"assistant","sessionId":"s-a","message":{"content":"ok"}}\n' >> "$TR/s-a.jsonl"
who
echo "# while s-a runs, s-b's hooks move nothing:"
hook s-b $NEWPID Stop "" listen; who
ended $APID; echo "# s-a's process ends (the operator closes it). s-b, still in the background, ends a turn. In round 1 it took the role back here:"
hook s-b $NEWPID Stop "" listen; who

###############################################################################
echo; echo "## M7. Many hooks at once"
echo "# 20 rounds. Each: the role at an ended s-a; then 12 hooks of s-b at once (brief, listen, notify), and in"
echo "# the odd rounds a third session running handoff role implementer at the same moment."
bad=0; moved=0; taken=0
for round in $(seq 1 20); do
  fresh; transcript s-a s-b; transcript s-b
  for i in 1 2 3 4; do
    input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief > /dev/null 2>&1 &
    input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook listen --timeout 1s > /dev/null 2>&1 &
    input s-b Notification | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook notify > /dev/null 2>&1 &
  done
  if [ $((round % 2)) = 1 ]; then env CLAUDE_CODE_SESSION_ID=s-x CLAUDE_PID=$XPID "$B" role implementer > /dev/null 2>&1 & fi
  wait
  files=$(ls .handoff/sessions/*.json 2>/dev/null | wc -l | tr -d ' ')
  holder=$(cat .handoff/sessions/*.json | perl -ne 'print "$2" if /"role":"implementer","session":"([^"]+)"(,"pid":(\d+))?/ and $h=$1 and print $h; ')
  holders=$(cat .handoff/sessions/*.json | grep -c '"role":"implementer"')
  [ "$files" = 1 ] && [ "$holders" = 1 ] || { bad=$((bad+1)); echo "#   round $round: $files bindings: $(cat .handoff/sessions/*.json | tr '\n' ' ')"; }
  grep -q '"session":"s-b"' .handoff/sessions/*.json && moved=$((moved+1)); grep -q '"session":"s-x"' .handoff/sessions/*.json && taken=$((taken+1))
  ls .handoff/sessions | grep -v -E '^(\.lock|s-[abx]\.json)$' | sed 's/^/#   stray file: /'
done
echo "# rounds with anything but one binding holding implementer: $bad of 20; it ended with s-b in $moved, with the third session in $taken"

###############################################################################
echo; echo "## M8. The listeners. The old session's listener runs under its CLI's process; the CLI ends; the new session listens"
newrepo
# a stand-in for the old CLI: a shell that starts the session's Stop hook as its own child, as the CLI does, then idles
transcript s-old
printf '{"session_id":"s-old","transcript_path":"%s","hook_event_name":"Stop"}' "$TR/s-old.jsonl" > "$WORK/in.old"
( exec sh -c 'echo $$ > "$1/oldcli.pid"; sleep 1.5; env CLAUDE_CODE_SESSION_ID=s-old CLAUDE_PID=$$ "$2" hook listen < "$1/in.old" > "$1/listen.old" 2>&1 & echo $! > "$1/listener.old"; while :; do sleep 1; done' sh "$WORK" "$B" ) 2>/dev/null &
disown
sleep 0.4; OLDCLI=$(cat "$WORK/oldcli.pid"); echo $OLDCLI >> "$WORK/pids"
env CLAUDE_CODE_SESSION_ID=s-old CLAUDE_PID=$OLDCLI "$B" role implementer 2>&1 | head -1
sleep 2.5; LISTENER=$(cat "$WORK/listener.old")
echo "# the old session's listener runs: $(kill -0 $LISTENER 2>/dev/null && echo yes || echo no); who listens, by .handoff/presence: $(ls .handoff/presence 2>/dev/null | tr '\n' ' ')"
echo "# /background: the old CLI's process ends, its transcript names s-new"
transcript s-old s-new; transcript s-new
kill -9 $OLDCLI; while kill -0 $OLDCLI 2>/dev/null; do sleep 0.05; done
( input s-new SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook brief 2>&1 | cut -c1-60 | head -1 | sed 's/^/# s-new, SessionStart, brief: /' )
input s-new SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook listen --timeout 20s > "$WORK/listen.new" 2>&1 &
NEWL=$!
sleep 3
echo "# three seconds on, the old listener runs: $(kill -0 $LISTENER 2>/dev/null && echo yes || echo no); what it printed: $(wc -c < "$WORK/listen.old" | tr -d ' ') bytes"
env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$NEWPID "$B" note --as reviewer --to implementer --item W1 --title "for the implementer" --body "$(body)" 2>&1 | sed 's/^/# the reviewer: /'
wait $NEWL; echo "# s-new's listener: exit $?"; short < "$WORK/listen.new" | sed 's/^/#   /' | cut -c1-140
who

###############################################################################
echo; echo "## M9. An fyi from the operator, around the move"
newrepo; OLDPID=$(sleeper); RPID=$(sleeper)
give s-old $OLDPID implementer; give s-rev $RPID reviewer
term -- fyi "First: sent while the old session has the role."
ended $OLDPID; transcript s-old s-new; transcript s-new
echo "# the old session never read it. /background; its process has ended."
hook s-new $NEWPID SessionStart fork brief
bindings | perl -pe 's/"since":"[^"]+"/"since":"<ts>"/g'
echo; echo "\$ [session s-new, at SessionStart (fork)] handoff hook listen"
input s-new SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook listen --timeout 3s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[1]}"
echo; echo "\$ [session s-new, at Stop] handoff hook listen      (it is not brought twice)"
input s-new Stop | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook listen --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[1]}"
echo; echo "# (b) The same, but the operator gives the role to a third session before anything listens: that one must not get the fyi"
newrepo; OLDPID=$(sleeper); XPID=$(sleeper)
give s-old $OLDPID implementer
term -- fyi "For the old session only."
ended $OLDPID; transcript s-old s-new; transcript s-new
hook s-new $NEWPID SessionStart fork brief
give s-x $XPID implementer
bindings | perl -pe 's/"since":"[^"]+"/"since":"<ts>"/g'
echo; echo "\$ [session s-x, at Stop] handoff hook listen"
input s-x Stop | env CLAUDE_CODE_SESSION_ID=s-x CLAUDE_PID=$XPID "$B" hook listen --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[1]}"
echo; echo "# (c) Two moves, each session with a binding in turn; then an fyi named for the first, read by a plain handoff wait in the third"
newrepo; OLDPID=$(sleeper); P2=$(sleeper)
give s-1 $OLDPID implementer
term -- fyi "Named for the first session."
ended $OLDPID; transcript s-1 s-2; transcript s-2
hook s-2 $P2 SessionStart fork brief > /dev/null
ended $P2; transcript s-2 s-3; transcript s-3
hook s-3 $NEWPID SessionStart fork brief > /dev/null
bindings | perl -pe 's/"since":"[^"]+"/"since":"<ts>"/g'
echo; echo "\$ [session s-3] handoff wait --timeout 2s"
env CLAUDE_CODE_SESSION_ID=s-3 CLAUDE_PID=$NEWPID "$B" wait --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[0]}"
echo; echo "# (e) The old session, resumed, runs handoff wait --as implementer by hand before the new one listens"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer
term -- fyi "Named for the old session."
ended $OLDPID; transcript s-old s-new; transcript s-new
hook s-new $NEWPID SessionStart fork brief > /dev/null
RP=$(sleeper); echo "\$ [session s-old, resumed, with no role] handoff wait --as implementer --timeout 2s"
env CLAUDE_CODE_SESSION_ID=s-old CLAUDE_PID=$RP "$B" wait --as implementer --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[0]}"
echo "\$ [session s-new, at Stop] handoff hook listen"
input s-new Stop | env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$B" hook listen --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[1]}"
echo; echo "# (d) After /clear the rule of row 93 stands: an fyi for the cleared conversation is passed over"
newrepo; CPID=$(sleeper)
give s-c1 $CPID implementer
term -- fyi "For the conversation that is cleared."
printf '{"session_id":"s-c2","hook_event_name":"SessionStart","source":"clear","transcript_path":"%s"}' "$TR/s-c2.jsonl" | env CLAUDE_CODE_SESSION_ID=s-c2 CLAUDE_PID=$CPID "$B" hook brief 2>&1 | head -1 | cut -c1-70
bindings | perl -pe 's/"since":"[^"]+"/"since":"<ts>"/g'
printf '{"session_id":"s-c2","hook_event_name":"Stop","transcript_path":"%s"}' "$TR/s-c2.jsonl" | env CLAUDE_CODE_SESSION_ID=s-c2 CLAUDE_PID=$CPID "$B" hook listen --timeout 2s 2>&1 | short | cut -c1-150; echo "# exit ${PIPESTATUS[1]}"

###############################################################################
echo; echo "## M10. A session with no role, in a project where no session has one: what its hooks leave behind"
newrepo; transcript s-b
echo "# .handoff/sessions before: $(ls -A .handoff/sessions 2>/dev/null | tr '\n' ' ')"
hook s-b $NEWPID SessionStart startup brief
hook s-b $NEWPID Stop "" listen
echo "# .handoff/sessions after: $(ls -A .handoff/sessions 2>/dev/null | tr '\n' ' ')"
echo "# git status --short in the project: [$(git status --short | tr '\n' ' ')]"
echo "# the same with v0.1.4:"; newrepo > /dev/null
hook s-b $NEWPID SessionStart startup brief "$OLD" > /dev/null
echo "# .handoff/sessions after: $(ls -A .handoff/sessions 2>/dev/null | tr '\n' ' ')"

###############################################################################
echo; echo "## M11. A named pipe where the old transcript should be"
fresh; transcript s-b; rm -f "$TR/s-a.jsonl"; mkfifo "$TR/s-a.jsonl"
echo "\$ [session s-b, at SessionStart (fork)] handoff hook brief      (stopped after 5 seconds if still running; in round 1 it was)"
input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief > "$WORK/o" 2>&1 &
HP=$!; sleep 5
if kill -0 $HP 2>/dev/null; then echo "# still running after 5 seconds: the hook waits for the pipe, holding the sessions' lock"
  ( env CLAUDE_CODE_SESSION_ID=s-x CLAUDE_PID=$XPID "$B" role reviewer > "$WORK/o2" 2>&1 & RP=$!; sleep 3; if kill -0 $RP 2>/dev/null; then echo "# and another session's handoff role waits too (3 seconds, then stopped)"; kill $RP; else echo "# another session's handoff role: $(head -1 "$WORK/o2")"; fi )
  kill $HP
else wait $HP; echo "# exit $?: $(cut -c1-80 "$WORK/o" | head -1)"; fi

###############################################################################
echo; echo "## M12. Bindings of other builds"
echo "# (a) the role given by v0.1.4, whose since has whole seconds; /background a moment later, with this build"
newrepo; OLDPID=$(sleeper)
give s-old $OLDPID implementer "$OLD"; bindings
ended $OLDPID; transcript s-old s-new; transcript s-new
hook s-new $NEWPID SessionStart fork brief; bindings
echo; echo "# (b) v0.1.4 reads the moved binding, with its was and its since to the nanosecond"
echo "\$ [session s-new] handoff-v0.1.4 role"; env CLAUDE_CODE_SESSION_ID=s-new CLAUDE_PID=$NEWPID "$OLD" role 2>&1 | head -1
hook s-new $NEWPID Stop "" listen "$OLD"
XPID=$(sleeper); give s-x $XPID implementer "$OLD"; who
echo; echo "# (c) the role given by this build in the same second as the line (the line's time is cut to the millisecond)"
bad=0; for i in 1 2 3 4 5 6 7 8 9 10; do newrepo > /dev/null; OLDPID=$(sleeper); env CLAUDE_CODE_SESSION_ID=s-a CLAUDE_PID=$OLDPID "$B" role implementer > /dev/null 2>&1; transcript s-a s-b; kill $OLDPID; transcript s-b
  input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief 2>&1 | grep -q 'you are implementer' || bad=$((bad+1)); done
echo "# ten times, the line written straight after handoff role returns: not moved in $bad of 10"

###############################################################################
echo; echo "## M13. Many hooks at once, with the old process ending half a second in"
bad=0
for round in $(seq 1 10); do
  newrepo > /dev/null; OLDPID=$(sleeper); env CLAUDE_CODE_SESSION_ID=s-a CLAUDE_PID=$OLDPID "$B" role implementer > /dev/null 2>&1; transcript s-a s-b; transcript s-b
  ( sleep 0.5; kill $OLDPID ) &
  for i in 1 2 3 4; do
    ( input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook brief 2>&1 | grep -q 'you are implementer' || echo x >> "$WORK/nobrief.$round" ) &
    input s-b SessionStart fork | env CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$NEWPID "$B" hook listen --timeout 1s > /dev/null 2>&1 &
  done
  wait
  files=$(ls .handoff/sessions/*.json 2>/dev/null | wc -l | tr -d ' ')
  [ "$files" = 1 ] && grep -q '"session":"s-b"' .handoff/sessions/*.json && grep -q '"was":\["s-a"\]' .handoff/sessions/*.json && [ ! -f "$WORK/nobrief.$round" ] || { bad=$((bad+1)); echo "#   round $round: $(cat .handoff/sessions/*.json | tr '\n' ' ') briefs without the role: $(wc -l < "$WORK/nobrief.$round" 2>/dev/null)"; }
done
echo "# 10 rounds of four briefs and four listeners at once: rounds where a brief lacked the role, or the binding was not s-b's with was [s-a]: $bad"
}
main 2>&1 | clean
