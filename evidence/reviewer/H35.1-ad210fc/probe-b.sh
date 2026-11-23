#!/bin/zsh
# Reviewer's probes for H35.1 (d6aa334), second set: the other reviewers' findings (#803, #804) run again, and a blocked spare in `handoff`.
# Usage: probe-b.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2 --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g" | cut -c1-${W:-240}; }
as() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}" | cut -c1-200; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -v '^$' | head -${N:-1} | tidy; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
bind() { python3 -c "
import json,sys,os
p='.handoff/sessions/$1.json'
if not os.path.exists(p): print('  binding $1: (no file)'); sys.exit()
b=json.load(open(p)); l=b.get('lost') or {}
print('  binding $1: role', b.get('role') or '-', '| gone', b.get('gone', False), '| pid', 'yes' if b.get('pid') else 'no', '| lost', (l.get('role') or '-'), ('to '+l['to']) if l.get('to') else '', l.get('went') or '')"; }

echo "## T1. reviewer-3's #804, 1: a /clear move from implementer cut between its two writes, then role spare in the new id"
new t1; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P $H role implementer
echo "\$ (s2, SessionStart hook, source clear, HANDOFF_TEST_FAULT=move-between) handoff hook brief"; print -r -- '{"session_id":"s2","hook_event_name":"SessionStart","source":"clear"}' | HANDOFF_TEST_FAULT=move-between CLAUDE_CODE_SESSION_ID=s2 CLAUDE_PID=$P $H hook brief 2>&1 | head -1 | tidy
bind s1; bind s2
as s2 $P $H role spare
bind s2
kill $P 2>/dev/null

echo; echo "## T2. reviewer-3's #804, 2: the spare's process ended and was swept; resumed, it runs role spare"
new t2; sleep 900 & P0=$!; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P0 $H role spare; kill $P0; sleep 0.3
q env CLAUDE_CODE_SESSION_ID=rv CLAUDE_PID=$P $H role reviewer
bind sp
sleep 900 & P1=$!
as sp $P1 $H role spare
bind sp
echo "-- for contrast, any other command of it notes its new process:"
as sp $P1 $H inbox --quiet
bind sp
kill $P $P1 2>/dev/null

echo; echo "## T3. reviewer-3's #804, 3: HANDOFF_ROLE and --as with the spare"
new t3; sleep 900 & P=$!
echo "\$ (n) HANDOFF_ROLE=spare handoff role spare"; HANDOFF_ROLE=spare CLAUDE_CODE_SESSION_ID=n CLAUDE_PID=$P $H role spare 2>&1 | head -1 | tidy
echo "\$ (n) HANDOFF_ROLE=implementer handoff role spare"; HANDOFF_ROLE=implementer CLAUDE_CODE_SESSION_ID=n CLAUDE_PID=$P $H role spare 2>&1 | head -1 | tidy
as n $P $H inbox --as spare
kill $P 2>/dev/null

echo; echo "## T4. A blocked spare, and a blocked reviewer, in handoff on its own"
new t4
q env ANTIGRAVITY_CONVERSATION_ID=G $H role spare; q env ANTIGRAVITY_CONVERSATION_ID=R $H role reviewer
for id in G R; do print -r -- "{\"conversationId\":\"$id\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | ANTIGRAVITY_CONVERSATION_ID=$id $H hook listen --timeout 1s >/dev/null 2>&1; done
python3 -c "
import json
for s in 'GR':
    b=json.load(open('.handoff/sessions/%s.json'%s)); print('  binding', s+': role', b.get('role'), '| block', (b.get('block') or {}).get('reason','none'))"
echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy
echo "  alerts to the operator: $(grep -c . "$HANDOFF_TEST_NOTIFY")"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'
