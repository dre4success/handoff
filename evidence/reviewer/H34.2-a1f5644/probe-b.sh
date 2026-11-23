#!/bin/zsh
# Reviewer's probes B6, B3 for H34.2 (4522906). Usage: probe-b.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
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

echo "## B6. What counts as work waiting for a blocked role: a request, a note, a decision"
new b6; C=$(git rev-parse HEAD)
ANTIGRAVITY_CONVERSATION_ID=A $H role reviewer | head -1
agy A 10 >/dev/null; bind A; alerts
echo "-- a note to it:"
note implementer reviewer "please hold"; alerts
echo "-- a request to it:"
$H post --as implementer --item W1 --to reviewer --commit $C --title W1 --body ../body.md 2>&1 | tidy; alerts
echo "-- another note, with the request still waiting:"
note implementer reviewer "and this" body2; alerts
echo "-- the same notes to a blocked role with nothing else waiting, then the operator's view:"
new b6b
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
agy A 10 >/dev/null; : > "$HANDOFF_TEST_NOTIFY"
note reviewer implementer "hold the merge"; $H wait --for reviewer-3 --timeout 2s >/dev/null 2>&1; alerts
echo "  the implementer's cursor: $(cat .handoff/cursor-implementer 2>/dev/null || ls .handoff | tr '\n' ' ')"
echo; echo "## B3. A permission prompt: the block, a reply's sender, and what clears it"
new b3; C=$(git rev-parse HEAD); sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer | head -1
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H post --item W1 --to reviewer --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
echo "\$ (I, Notification hook, permission_prompt) handoff hook notify"
print -r -- '{"session_id":"I","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash\nmore"}' | CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H hook notify 2>&1 | tidy
bind I; alerts
x $H reply --as reviewer 1 --verdict CHANGES --commit $C --body ../body.md
alerts
home
echo "-- the operator answers the prompt, and the session runs a handoff command:"
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H inbox --quiet; echo "  [inbox --quiet: exit $?]"
bind I
note reviewer implementer "after" body2
kill $PI 2>/dev/null

