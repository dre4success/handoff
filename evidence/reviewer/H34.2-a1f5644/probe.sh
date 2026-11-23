#!/bin/zsh
# Reviewer's probes for H34.2 (4522906). Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
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

echo "## B1. Antigravity's cap: the block, who sees it, and what clears it"
new b1
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
agy A 10
bind A; alerts
echo "-- a note for the blocked role:"
note reviewer implementer "one"
alerts
echo "\$ handoff inbox --for implementer (top)"; $H inbox --for implementer 2>&1 | sed -n '1,4p' | tidy
echo "\$ handoff brief --for implementer (the Blocked line)"; $H brief --for implementer 2>&1 | grep -i "^Blocked" | tidy
home
echo "-- a read sends nothing more; a second note sends one more:"
$H inbox --for implementer >/dev/null 2>&1; alerts
note reviewer implementer "two" body2; alerts
echo "-- the operator types in the session; the count starts again, and its next Stop hook listens:"
agy A 0
bind A; home
note reviewer implementer "three" body3

echo; echo "## B2. The cap while something already listens for the role, and from a session with no binding"
new b2
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
( $H wait --for implementer --timeout 4s >/dev/null 2>&1 ) & WP=$!; sleep 1
agy A 10
bind A; wait $WP
echo "-- a fixed role, no binding (hook listen --as reviewer), at the cap:"
echo '{"conversationId":"Q","hookEventName":"Stop","cwd":"'$PWD'","executionNum":10}' | ANTIGRAVITY_CONVERSATION_ID=Q $H hook listen --as reviewer --timeout 2s 2>&1 | tidy; ls .handoff/sessions/ | tr '\n' ' '; echo; alerts

echo; echo "## B3. A permission prompt: the block, a reply's sender, and what clears it"
new b3; C=$(git rev-parse HEAD); sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer | head -1
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H post --item W1 --to reviewer --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
echo "\$ (I, Notification hook, permission_prompt) handoff hook notify"
echo '{"session_id":"I","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash\nmore"}' | CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H hook notify 2>&1 | tidy
bind I; alerts
x $H reply --as reviewer 1 --verdict CHANGES --commit $C --body ../body.md
alerts
home
echo "-- the operator answers the prompt, and the session runs a handoff command:"
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H inbox --quiet; echo "  [inbox --quiet: exit $?]"
bind I
note reviewer implementer "after" body2
kill $PI 2>/dev/null

echo; echo "## B4. Antigravity says its own task still runs (fullyIdle false)"
new b4
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
note reviewer implementer "one"
agy A 3 false
agy A 10 false
bind A
agy A 3 true

echo; echo "## B5. A blocked session loses its role: the block doesn't follow the role"
new b5
ANTIGRAVITY_CONVERSATION_ID=A $H role implementer | head -1
agy A 10 >/dev/null; bind A
ANTIGRAVITY_CONVERSATION_ID=Z $H role implementer | head -1
bind Z
note reviewer implementer "one"
home
