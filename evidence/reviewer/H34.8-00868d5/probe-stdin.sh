#!/bin/zsh
# Reviewer's probe for H34.8 round 3 (00868d5): Antigravity hooks with the session named on stdin only, no ANTIGRAVITY_CONVERSATION_ID.
# Usage: probe-stdin.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p "$D/s"; cd "$D/s"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; echo body > ../body.md
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`]*/handoff#handoff#g" | cut -c1-${W:-520}; }
listening() { python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    d=json.loads(t); m=d['injectSteps'][0]['systemMessage']['systemMessage']; print([l for l in m.split('\n') if l.startswith('Listening')][0])
except Exception as e: print('(not a brief)', t[:200])"; }
echo "## Y1. An Antigravity session's hooks, with its id on stdin only (env has no ANTIGRAVITY_CONVERSATION_ID)"
echo "-- the agent's own command has the variable, as Antigravity gives it to commands:"
ANTIGRAVITY_CONVERSATION_ID=A $H role reviewer | head -1
python3 -c "import json;print('  binding A: cli', json.load(open('.handoff/sessions/A.json')).get('cli'))"
echo "\$ (SessionStart hook, stdin only) handoff hook brief"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"SessionStart\",\"cwd\":\"$PWD\"}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook brief 2>&1 | listening | tidy
echo "\$ (Stop hook, stdin only, executionNum 3) handoff hook listen --timeout 1s"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":3,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s 2>&1 | python3 -c "
import sys,json
d=json.loads(sys.stdin.read()); print('   ', d.get('decision'), '|', (d.get('reason') or '').replace('\n',' / '))" | tidy
echo "\$ (Stop hook, stdin only, executionNum 10) handoff hook listen"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s 2>&1 | tidy
python3 -c "import json;b=json.load(open('.handoff/sessions/A.json'));print('  binding A: block', (b.get('block') or {}).get('reason','none'), '| cli', b.get('cli'))"
echo; echo "## Y2. A session that takes its role with its id on stdin only can't: role is a command, and needs the variable"
echo "\$ handoff role reviewer   (no session variable at all)"; $H role reviewer 2>&1 | head -1 | tidy
echo; echo "## Y3. away's line for an Antigravity session"
echo "\$ (the operator, at a terminal) handoff away \"carry on\""; python3 $AT $H away "carry on" 2>&1 | grep -i "antigravity" | tidy | sed 's/^/    /'
