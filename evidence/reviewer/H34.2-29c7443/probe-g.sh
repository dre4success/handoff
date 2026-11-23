#!/bin/zsh
# Reviewer's probe G for H34.2 round 4 (29c7443) on H34.1 round 6: the other project's night again. Usage: probe-g.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
mkdir -p "$D/g"; cd "$D/g"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md
python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='3s';json.dump(c,open(p,'w'),indent=2)"
git checkout -q -b w1 && git commit -q --allow-empty -m w1 && C=$(git rev-parse HEAD) && git checkout -q main
tidy() { sed -E 's/since [0-9]{2}:[0-9]{2}/since HH:MM/g; s/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-260; }
alerts() { echo "  alerts to the operator ($(grep -c . "$HANDOFF_TEST_NOTIFY")):"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'; : > "$HANDOFF_TEST_NOTIFY"; }
agy() { echo "\$ (Antigravity A, Stop hook, executionNum $1) handoff hook listen --timeout 2s"; echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":$1}" | ANTIGRAVITY_CONVERSATION_ID=A $H hook listen --timeout 2s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try:
    d=json.loads(t); print('   ', d.get('decision','(no wake)'), '|', (d.get('reason') or '').replace('\n',' / ')[:420])
except Exception: print('   ', t[:300])" | tidy; }
echo "## G. An Antigravity implementer at its cap, the operator away, three reviewers approve"
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
ANTIGRAVITY_CONVERSATION_ID=A $H post --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
python3 $AT $H away "merge once the reviewers approve" >/dev/null 2>&1; : > "$HANDOFF_TEST_NOTIFY"
agy 10
alerts
for r in reviewer reviewer-2 reviewer-3; do n=$((n+1)); $H reply --as $r $n --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "^posted|^handoff: (told|impl)" | tidy; done
alerts
echo "\$ handoff inbox --for implementer"; $H inbox --for implementer 2>&1 | sed -n '1,16p' | tidy
echo "\$ handoff   (the operator's view)"; $H 2>&1 | tidy
echo "-- three seconds later, any listener's pass:"
sleep 4; $H wait --for reviewer-3 --timeout 2s >/dev/null 2>&1; alerts
echo "-- the operator types in the session; its next Stop hook:"
agy 0
