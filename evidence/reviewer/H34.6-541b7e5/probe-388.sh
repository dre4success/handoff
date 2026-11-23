#!/bin/zsh
# Reviewer's probes J, K, L for reviewer-2's #388 on H34.6 round 3 (05a184f).
# Usage: probe-388.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; }
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));print(('role '+b['role'] if b.get('role') else 'lost '+b['lost']['role'])+(', gone' if b.get('gone') else '')+(', pid' if b.get('pid') else ', no pid'))")"; done)"; }
r() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}"; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -E '^(handoff|posted|You are|#[0-9])' | grep -v "^handoff brief" | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-250; echo "  [exit ${pipestatus[1]}]"; }
hook() { local id=$1 pid=$2 json=$3; shift 3; echo "\$ ($id, hook, ${json//$D/D}) handoff hook $*"; echo "$json" | CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid $H hook "$@" 2>&1 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-220 | head -3; echo "  [exit ${pipestatus[2]}]"; }

echo "## J. A marked session does /clear, and is later resumed under its old id (#388 point 1)"
new j
sleep 120 & PX=$!; sleep 120 & PZ=$!; sleep 120 & PX2=$!
r X $PX $H role reviewer; r Z $PZ $H role reviewer
show
echo "-- X is refused, as it should be:"
r X $PX $H note --as reviewer --to implementer --item P1 --title "from X, marked" --body ../body.md
echo "-- /clear in X's process: a SessionStart with source clear and a new id, Y"
hook Y $PX '{"session_id":"Y","hook_event_name":"SessionStart","source":"clear"}' brief
show
r Y $PX $H note --as reviewer --to implementer --item P1 --title "from Y" --body ../body.md
echo "-- the operator resumes X by its id, in a new process:"
r X $PX2 $H role
r X $PX2 $H note --as reviewer --to implementer --item P1 --title "from X, resumed after its /clear" --body ../body.md
kill $PX $PZ $PX2 2>/dev/null

echo; echo "## K. A marked session goes on by /background, and is later resumed under its old id (#388 point 1)"
new k
T="$D/k-transcripts"; mkdir -p $T
sleep 120 & PX=$!; sleep 120 & PZ=$!; sleep 120 & PY=$!; sleep 120 & PX2=$!
r X $PX $H role reviewer; r Z $PZ $H role reviewer
echo "-- X goes on as Y: X's transcript ends with a continued-in line, and X's process ends"
sleep 1.2 # the continued-in line, stamped to the second, must not be older than the role
print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"Y\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%S.000Z)\"}" > $T/X.jsonl; : > $T/Y.jsonl
kill $PX; sleep 0.3
hook Y $PY "{\"session_id\":\"Y\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/Y.jsonl\"}" brief
show
r Y $PY $H note --as reviewer --to implementer --item P1 --title "from Y" --body ../body.md
echo "-- the operator resumes X by its id, in a new process:"
r X $PX2 $H role
r X $PX2 $H note --as reviewer --to implementer --item P1 --title "from X, resumed after /background" --body ../body.md
kill $PZ $PY $PX2 2>/dev/null

echo; echo "## L. A hook with a fixed role (--as), started before its session had a binding (#388 point 3)"
new l
sleep 120 & PS=$!; sleep 120 & PZ=$!
echo "\$ (S, Stop hook, in the background) handoff hook listen --as reviewer --timeout 25s"
( echo '{"session_id":"S","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=S CLAUDE_PID=$PS $H hook listen --as reviewer --timeout 25s > "$D/l-hook.out" 2>&1; echo "  [exit $?]" >> "$D/l-hook.out" ) &
HP=$!
sleep 1
r S $PS $H role reviewer
r Z $PZ $H role reviewer
show
sleep 2
echo "-- S's commands are refused:"
r S $PS $H inbox --quiet
echo "-- a note for the reviewer, which is Z now:"
$H note --as implementer --to reviewer --item P1 --title "for the reviewer" --body ../body.md 2>&1 | grep -v "no session"
sleep 3
echo "-- what S's old hook said:"; sed -E 's/^/  /' "$D/l-hook.out" | cut -c1-200
echo "-- what Z, the reviewer, gets:"
r Z $PZ $H wait --timeout 3s
kill $PS $PZ $HP 2>/dev/null

echo; echo "## L2. The same, with the hook started after the binding (the fenced path, for comparison)"
new l2
sleep 120 & PS=$!; sleep 120 & PZ=$!
r S $PS $H role reviewer
echo "\$ (S, Stop hook, in the background) handoff hook listen --timeout 25s"
( echo '{"session_id":"S","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=S CLAUDE_PID=$PS $H hook listen --timeout 25s > "$D/l2-hook.out" 2>&1; echo "  [exit $?]" >> "$D/l2-hook.out" ) &
HP=$!
sleep 1
r Z $PZ $H role reviewer
sleep 2
$H note --as implementer --to reviewer --item P1 --title "for the reviewer" --body ../body.md 2>&1 | grep -v "no session"
sleep 2
echo "-- what S's old hook said:"; sed -E 's/^/  /' "$D/l2-hook.out" | cut -c1-200
echo "-- what Z, the reviewer, gets:"
r Z $PZ $H wait --timeout 3s
kill $PS $PZ $HP 2>/dev/null
