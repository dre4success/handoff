#!/bin/zsh
# Reviewer's probes for H35.1 (d6aa334): the spare. Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without H35>
D=$1; H=$2; AT=$3; OLD=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1" "$D/$1-t"; T="$D/$1-t"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && ${2:-$H} init --roles ${3:-implementer,reviewer,reviewer-2} --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g" | cut -c1-${W:-260}; }
x() { echo "\$ ${*//$H/handoff}" | sed -E 's#(--body) [^ ]*#\1 f#' | cut -c1-200; "$@" 2>&1 | grep -v '^$' | head -${N:-3} | tidy; echo "  [exit ${pipestatus[1]}]"; }
as() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}" | sed -E 's#(--body) [^ ]*#\1 f#' | cut -c1-200; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -v '^$' | head -${N:-2} | tidy; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
bind() { python3 -c "
import json,sys,os
p='.handoff/sessions/$1.json'
if not os.path.exists(p): print('  binding $1: (no file)'); sys.exit()
b=json.load(open(p)); l=b.get('lost') or {}
print('  binding $1: role', b.get('role') or '-', '| gen', str(b.get('gen'))[-6:], '| gone', b.get('gone', False), '| pid', 'yes' if b.get('pid') else 'no', '| lost', (l.get('role') or '-'), ('to '+l['to']) if l.get('to') else '', l.get('went') or '')"; }
now() { python3 -c "import datetime;print(datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.%f')[:-3]+'Z')"; }

echo "## S1. The spare, through /clear and /background: does the role move, and does its generation stay?"
new s1; sleep 900 & P=$!
as sp0 $P $H role spare; bind sp0
echo "\$ (sp1, SessionStart hook, source clear, same process) handoff hook brief"; print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | CLAUDE_CODE_SESSION_ID=sp1 CLAUDE_PID=$P $H hook brief 2>&1 | head -1 | tidy
bind sp0; bind sp1
as sp1 $P $H role spare; bind sp1
sleep 1.2; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"sp2\",\"timestamp\":\"$(now)\"}" >> $T/sp1.jsonl; : >> $T/sp2.jsonl
kill $P 2>/dev/null; sleep 0.2; sleep 900 & P=$!
echo "\$ (sp2, SessionStart hook, source fork: /background from sp1) handoff hook brief"; print -r -- "{\"session_id\":\"sp2\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/sp2.jsonl\"}" | CLAUDE_CODE_SESSION_ID=sp2 CLAUDE_PID=$P $H hook brief 2>&1 | head -1 | tidy
bind sp1; bind sp2
kill $P 2>/dev/null

echo; echo "## S2. History, in sequences: who may be the spare"
new s2; sleep 900 & P=$!; sleep 900 & PZ=$!
echo "-- a: the spare is given a seat by the operator's word, loses it, and asks to be the spare again"
as a $P $H role spare; as a $P $H role reviewer; as z $PZ $H role reviewer; as a $P $H role spare
echo "-- b: /clear in the process of a session that lost implementer, then role spare in the new id"
sleep 900 & PB=$!; q env CLAUDE_CODE_SESSION_ID=b0 CLAUDE_PID=$PB $H role implementer; q env CLAUDE_CODE_SESSION_ID=z2 CLAUDE_PID=$PZ $H role implementer
print -r -- '{"session_id":"b1","hook_event_name":"SessionStart","source":"clear"}' | CLAUDE_CODE_SESSION_ID=b1 CLAUDE_PID=$PB $H hook brief 2>&1 | head -1 | tidy
as b1 $PB $H role spare
echo "-- c: the old id of a /background move (it held reviewer-2), resumed in a new process"
sleep 900 & PC=$!; q env CLAUDE_CODE_SESSION_ID=c0 CLAUDE_PID=$PC $H role reviewer-2
sleep 1.2; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"c1\",\"timestamp\":\"$(now)\"}" >> $T/c0.jsonl; : >> $T/c1.jsonl
kill $PC; sleep 0.2; sleep 900 & PC=$!
print -r -- "{\"session_id\":\"c1\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/c1.jsonl\"}" | CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$PC $H hook brief 2>&1 | head -1 | tidy
sleep 900 & PC2=$!; as c0 $PC2 $H role spare
echo "-- d: a session that only ever used --as (no binding): the gap row 115 names"
sleep 900 & PD=$!; q env CLAUDE_CODE_SESSION_ID=d CLAUDE_PID=$PD $H note --as reviewer --to implementer --item P1 --title t --body ../body.md
as d $PD $H role spare
kill $P $PZ $PB $PC $PC2 $PD 2>/dev/null

echo; echo "## S3. What reaches the spare: an fyi, away and back"
new s3; sleep 900 & P=$!; sleep 900 & PI=$!
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare; q env CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$PI $H role implementer
echo "\$ (the operator, at a terminal) handoff fyi \"word for all\""; python3 $AT $H fyi "word for all" 2>&1 | grep -v '^$' | tidy | sed 's/^/    /'
N=4 as sp $P $H wait --timeout 2s
echo "\$ (the operator, at a terminal) handoff away \"carry on\""; python3 $AT $H away "carry on" 2>&1 | grep -v '^$' | tidy | sed 's/^/    /'
N=4 as sp $P $H wait --timeout 2s
echo "  entries addressed to the spare: $(grep -c '"to":"spare"' .handoff/channel.jsonl)"
kill $P $PI 2>/dev/null

echo; echo "## S4. Listening as the spare, and as another role from the spare's session (reviewer-2's #803)"
new s4; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare
q $H post --as implementer --item W1 --to reviewer --commit $C --title W1 --body ../body.md
x $H wait --for spare --timeout 1s
as sp $P $H wait --for reviewer --timeout 1s
echo "  reviewer's cursor after: $(cat .handoff/cursors/reviewer 2>/dev/null || echo none)"
echo "\$ (a session with no role, fixed by its hook) handoff hook listen --as spare --timeout 1s"; print -r -- '{"session_id":"q","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=q CLAUDE_PID=$P $H hook listen --as spare --timeout 1s 2>&1 | head -2 | tidy; echo "  [exit ${pipestatus[2]}]"
kill $P 2>/dev/null

echo; echo "## S5. Antigravity as the spare, its hooks with the id on stdin only"
new s5
ANTIGRAVITY_CONVERSATION_ID=G $H role spare | head -1
echo "\$ (SessionStart hook, stdin only) handoff hook brief   (the Listening line)"; print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"SessionStart\",\"cwd\":\"$PWD\"}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook brief 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    m=json.loads(t)['injectSteps'][0]['systemMessage']['systemMessage']; print('   ', m.split('\n')[0][:200]); print('   ', [l for l in m.split('\n') if l.startswith('Listening')][0][:150])
except Exception as e: print('(not a brief)', t[:200])"
echo "\$ (Stop hook, stdin only, executionNum 10) handoff hook listen"; print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s 2>&1 | tidy
python3 -c "import json;b=json.load(open('.handoff/sessions/G.json'));print('  binding G: role', b.get('role'), '| block', (b.get('block') or {}).get('reason','none'))"
echo "  alerts to the operator: $(grep -c . "$HANDOFF_TEST_NOTIFY")"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'
echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy

echo; echo "## S6. A project that already has a role named spare (made with the older build)"
new s6 $OLD implementer,reviewer,spare; sleep 900 & P=$!
x $H post --as implementer --item W1 --to spare --commit $C --title W1 --body ../body.md
x $H claim --as spare 1
as old $P $H role implementer; as old $P $H role spare
kill $P 2>/dev/null
echo "-- and the new build's init and roles add:"
mkdir -p $D/s6b; cd $D/s6b; git init -q -b main; git commit -q --allow-empty -m base
x $H init --roles implementer,reviewer,spare --main main
q $H init --roles implementer,reviewer --main main; x $H roles add spare
