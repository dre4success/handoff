#!/bin/zsh
# Reviewer's probes for H35.1 round 2 (ad210fc): the spare's listener in its real paths, and role spare in races.
# Usage: probe-c.sh <a new, empty scratch dir> <handoff binary> <an older handoff binary, without H35> <how many times>
D=$1; H=$2; OLD=$3; N=${4:-40}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && ${2:-$H} init --roles ${3:-implementer,reviewer,reviewer-2} --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g" | cut -c1-${W:-230}; }
as() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}" | cut -c1-200; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -v '^$' | head -${L:-1} | tidy; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
gen() { python3 -c "
import json,os
p='.handoff/sessions/$1.json'
if not os.path.exists(p): print('  binding $1: (no file)')
else:
    b=json.load(open(p)); l=b.get('lost') or {}
    print('  binding $1: role', b.get('role') or '-', '| gen', str(b.get('gen'))[-6:], '| gone', b.get('gone', False), '| lost', (l.get('role') or '-'), ('to '+l['to']) if l.get('to') else '', l.get('went') or '')"; }
role() { python3 -c "import json;print(json.load(open('.handoff/sessions/$1.json')).get('role') or '-')" 2>/dev/null || echo none; }

echo "## U1. The spare's own listening, each way a session listens"
new u1; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare
as sp $P $H wait --timeout 1s
as sp $P $H wait --for spare --timeout 1s
echo "\$ (sp, Stop hook) handoff hook listen --timeout 1s"; print -r -- '{"session_id":"sp","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H hook listen --timeout 1s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[2]}]"
echo "-- after /clear, the new id listens as the spare:"
print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | CLAUDE_CODE_SESSION_ID=sp1 CLAUDE_PID=$P $H hook brief >/dev/null 2>&1
as sp1 $P $H wait --for spare --timeout 1s
echo "-- the spare moves to another session; the old one may not wait for it:"
sleep 900 & P2=$!; q env CLAUDE_CODE_SESSION_ID=other CLAUDE_PID=$P2 $H role spare
as sp1 $P $H wait --for spare --timeout 1s
echo "-- a session with another role, and one with none, waiting for the spare; the spare waiting for a seat:"
sleep 900 & PR=$!; q env CLAUDE_CODE_SESSION_ID=rv CLAUDE_PID=$PR $H role reviewer
as rv $PR $H wait --for spare --timeout 1s
echo "\$ handoff wait --for spare --timeout 1s   (no session)"; $H wait --for spare --timeout 1s 2>&1 | head -1 | tidy
as other $P2 $H wait --for reviewer --timeout 1s
echo "-- reading stays open:"
as other $P2 $H inbox --for reviewer
as rv $PR $H inbox --for spare
kill $P $P2 $PR 2>/dev/null

echo; echo "## U2. Antigravity as the spare: its hook with the id on stdin only, and its own background wait"
new u2
ANTIGRAVITY_CONVERSATION_ID=G $H role spare | head -1
echo "\$ (Stop hook, stdin only, executionNum 3) handoff hook listen --timeout 1s"; print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":3,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try: d=json.loads(t); print('   ', d.get('decision'), '|', (d.get('reason') or '').replace('\n',' / ')[:150])
except Exception: print('   ', t[:200])"
echo "\$ (G's own command) handoff wait --for spare --timeout 1s"; ANTIGRAVITY_CONVERSATION_ID=G $H wait --for spare --timeout 1s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[1]}]"

echo; echo "## U3. A project with a role already named spare: anyone may wait for it, as for any role"
new u3 $OLD implementer,reviewer,spare
echo "\$ handoff wait --for spare --timeout 1s   (no session)"; $H wait --for spare --timeout 1s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[1]}]"

echo; echo "## U4. role spare and role implementer in one session at once, $N times: it must end as implementer, whichever ran first"
new u4; bad=0; first=0
for i in $(seq 1 $N); do
  sleep 900 & P=$!
  ( CLAUDE_CODE_SESSION_ID=x$i CLAUDE_PID=$P $H role spare > $D/u4-s$i.out 2>&1 ) & A=$!
  ( CLAUDE_CODE_SESSION_ID=x$i CLAUDE_PID=$P $H role implementer > $D/u4-i$i.out 2>&1 ) & B=$!
  wait $A $B
  r=$(role x$i); s=$(head -1 $D/u4-s$i.out)
  [[ "$s" == "You are now the spare"* ]] && first=$((first+1))
  if [[ "$r" != implementer ]]; then bad=$((bad+1)); echo "  run $i: ended as $r | role spare said: ${s[1,90]}"; fi
  kill $P 2>/dev/null
done
echo "  $N runs: $bad ended as something other than implementer ($first times role spare ran first and was then replaced; the rest it was refused)"

echo; echo "## U5. role spare in two sessions at once, $N times: one spare, the other marked"
new u5; bad=0
for i in $(seq 1 $N); do
  sleep 900 & P=$!; sleep 900 & P2=$!
  ( CLAUDE_CODE_SESSION_ID=a$i CLAUDE_PID=$P $H role spare >/dev/null 2>&1 ) & A=$!
  ( CLAUDE_CODE_SESSION_ID=b$i CLAUDE_PID=$P2 $H role spare >/dev/null 2>&1 ) & B=$!
  wait $A $B
  n=$(grep -l '^{"v":1,"role":"spare"' .handoff/sessions/*.json | wc -l | tr -d ' ')
  if [[ "$n" != 1 ]]; then bad=$((bad+1)); echo "  run $i: $n bindings hold the spare"; fi
  kill $P $P2 2>/dev/null
done
echo "  $N runs: $bad where the spare was not held by exactly one session"

echo; echo "## U6. The spare's start hooks at once after /clear (brief, listen, brief), $N times: the role and its generation"
new u6; sleep 900 & P=$!
CLAUDE_CODE_SESSION_ID=s0 CLAUDE_PID=$P $H role spare >/dev/null
G0=$(python3 -c "import json;print(json.load(open('.handoff/sessions/s0.json'))['gen'])"); bad=0
for i in $(seq 1 $N); do
  j="{\"session_id\":\"s$i\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}"
  ( print -r -- "$j" | CLAUDE_CODE_SESSION_ID=s$i CLAUDE_PID=$P $H hook brief > $D/u6-a$i.out 2>&1 ) & A=$!
  ( print -r -- "$j" | CLAUDE_CODE_SESSION_ID=s$i CLAUDE_PID=$P $H hook listen --timeout 1s > $D/u6-l$i.out 2>&1 ) & L=$!
  ( print -r -- "$j" | CLAUDE_CODE_SESSION_ID=s$i CLAUDE_PID=$P $H hook brief > $D/u6-b$i.out 2>&1 ) & B=$!
  wait $A $L $B
  a=$(head -1 $D/u6-a$i.out); b=$(head -1 $D/u6-b$i.out); l=$(head -1 $D/u6-l$i.out)
  g=$(python3 -c "import json;b=json.load(open('.handoff/sessions/s$i.json'));print(b.get('gen') if b.get('role')=='spare' else 'no role')" 2>/dev/null)
  if [[ "$a" != "handoff brief: you are the spare"* || "$b" != "handoff brief: you are the spare"* || "$l" != *"nothing new for spare"* || "$g" != "$G0" ]]; then bad=$((bad+1)); echo "  /clear $i: brief: ${a[1,50]} | brief: ${b[1,50]} | listen: ${l[1,50]} | gen ${g: -6}"; fi
done
echo "  $N /clears: $bad where a hook did not have the spare, or its generation changed"
kill $P 2>/dev/null

echo; echo "## U7. A /clear of the spare cut between its two writes, then role spare in the new id"
new u7; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=sp0 CLAUDE_PID=$P $H role spare; gen sp0
print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | HANDOFF_TEST_FAULT=move-between CLAUDE_CODE_SESSION_ID=sp1 CLAUDE_PID=$P $H hook brief >/dev/null 2>&1
gen sp0; gen sp1
as sp1 $P $H role spare
gen sp1
kill $P 2>/dev/null
