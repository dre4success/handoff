#!/bin/zsh
# Reviewer's probes for H34.6 round 8 (a78c804): the operator's fyi, away and back now read who has a role under the record's lock.
# Usage: probe-r8.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; }
op() { echo "\$ (the operator, at a terminal) handoff $*"; local t0=$(python3 -c 'import time;print(time.time())'); python3 $AT $H "$@" 2>&1 | grep -v '^$' | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-200; echo "  [took $(python3 -c "import time;print(round(time.time()-$t0,1))") s]"; }
fy() { python3 - <<PY
import json
for l in open('.handoff/channel.jsonl'):
    e=json.loads(l)
    if e.get('kind') in ('fyi','away','back'): print('  #%s %s to %s, session %s' % (e['id'], e['kind'], e.get('to') or '-', e.get('session') or '-'))
PY
}
hooklisten() { ( echo "{\"session_id\":\"$1\",\"hook_event_name\":\"Stop\"}" | CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 $H hook listen --timeout 12s > "$D/$3.out" 2>&1; echo "[exit $?]" >> "$D/$3.out" ) & }

echo "## F1. fyi, away and back while two sessions listen"
new f1; sleep 300 & PR=$!; sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=R CLAUDE_PID=$PR $H role reviewer >/dev/null; CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer >/dev/null
hooklisten R $PR f1-r; hooklisten I $PI f1-i; sleep 1
op fyi "hello, both"
sleep 2; echo "-- what the reviewer's hook said:"; head -2 "$D/f1-r.out" | cut -c1-120; tail -1 "$D/f1-r.out"
hooklisten R $PR f1-r2; sleep 1
op away "going out"
op back
sleep 2; echo "-- what the reviewer's hook said:"; head -3 "$D/f1-r2.out" | cut -c1-120; tail -1 "$D/f1-r2.out"
fy
kill $PR $PI 2>/dev/null

echo; echo "## F2. An fyi sent while a /clear's move is paused between its two writes"
new f2; sleep 300 & PR=$!
CLAUDE_CODE_SESSION_ID=X CLAUDE_PID=$PR $H role reviewer >/dev/null
echo "\$ (/clear: a new conversation Y; its SessionStart hook pauses 3 s between the move's writes)"
( echo '{"session_id":"Y","hook_event_name":"SessionStart","source":"clear"}' | HANDOFF_TEST_MOVE_DELAY=3s CLAUDE_CODE_SESSION_ID=Y CLAUDE_PID=$PR $H hook brief > "$D/f2-brief.out" 2>&1 ) & HB=$!
sleep 1
op fyi "during the move"
wait $HB; head -1 "$D/f2-brief.out" | cut -c1-80
fy
kill $PR 2>/dev/null

echo; echo "## F3. With no session at all, fyi writes nothing"
new f3
op fyi "anyone?"
echo "  entries in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"
