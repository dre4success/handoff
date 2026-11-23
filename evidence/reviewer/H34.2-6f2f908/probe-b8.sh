#!/bin/zsh
# Reviewer's probe B8 for H34.2 round 6 (a1f5644). Usage: probe-b8.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; echo three > ../body3.md; : > "$HANDOFF_TEST_NOTIFY"; }
tidy() { sed -E 's/since [0-9]{2}:[0-9]{2}/since HH:MM/g; s/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-240}; }
x() { echo "\$ ${*//$H/handoff}" | sed -E 's#(--body) [^ ]*#\1 f#' | cut -c1-200; "$@" 2>&1 | grep -v '^$' | tidy; echo "  [exit ${pipestatus[1]}]"; }
bind() { python3 -c "
import json,sys
b=json.load(open('.handoff/sessions/$1.json'))
print('  binding $1: role', b.get('role'), '| block:', (b.get('block') or {}).get('reason','none'), ((b.get('block') or {}).get('detail') or ''), '| listened:', 'yes' if b.get('listened') else 'no')"; }
alerts() { echo "  alerts to the operator ($(grep -c . "$HANDOFF_TEST_NOTIFY")):"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'; : > "$HANDOFF_TEST_NOTIFY"; }
# agy <conversation> <executionNum> [fullyIdle]: Antigravity's Stop hook
agy() { local idle=""; [[ -n "$3" ]] && idle=",\"fullyIdle\":$3"; echo "\$ (Antigravity $1, Stop hook, executionNum $2${3:+, fullyIdle $3}) handoff hook listen --timeout 2s"
  echo "{\"conversationId\":\"$1\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":$2$idle}" | ANTIGRAVITY_CONVERSATION_ID=$1 $H hook listen --timeout 2s 2>&1 | tidy; echo "  [exit ${pipestatus[2]}]"; }
note() { $H note --as $1 --to $2 --item P1 --title "$3" --body ../${4:-body}.md 2>&1 | tidy; }
home() { echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy; }

echo "## B8. A permission prompt while work already waits: one notice, naming the work; then new work"
new b8; C=$(git rev-parse HEAD); sleep 300 & PI=$!
I() { CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI "$@"; }
prompt() { print -r -- "{\"session_id\":\"I\",\"hook_event_name\":\"Notification\",\"notification_type\":\"permission_prompt\",\"message\":\"$1\"}" | I $H hook notify 2>&1 | tidy; }
I $H role implementer | head -1
note reviewer implementer "one"
: > "$HANDOFF_TEST_NOTIFY"
prompt "allow A"; bind I; alerts
echo "-- a listener's alert pass, with nothing new:"
$H wait --for reviewer-3 --timeout 2s >/dev/null 2>&1; alerts
echo "-- a second note arrives:"
note reviewer implementer "two" body2; alerts
echo "-- the session answers and runs a command; an older prompt's hook can't block it again (the fence, by answered):"
I $H inbox --quiet >/dev/null 2>&1; bind I
python3 -c "import json;b=json.load(open('.handoff/sessions/I.json'));print('  answered recorded:', bool(b.get('answered')))"
kill $PI 2>/dev/null
