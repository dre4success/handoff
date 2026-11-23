#!/bin/zsh
# Reviewer's probes W1b, W3b for H34.5 (b9dbd6b). Usage: probe-b.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"; }
quiet() { python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='$1';json.dump(c,open(p,'w'),indent=2)"; }
tidy() { sed -E 's/since [0-9]{2}:[0-9]{2}/since HH:MM/g; s/at 20[0-9T:.-]+Z/at TIME/g; s/\(([0-9]+) s\)/(N s)/g; s/for [0-9]+ s/for N s/g' | cut -c1-${W:-300}; }
alerts() { echo "  alerts to the operator ($(grep -c . "$HANDOFF_TEST_NOTIFY")):"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'; : > "$HANDOFF_TEST_NOTIFY"; }
pass() { $H wait --for ${1:-reviewer-3} --timeout 2s >/dev/null 2>&1; }

echo "## W1b. A role whose claim has gone quiet while its session is listening: no clause"
new w1b; sleep 300 & P1=$!
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H role reviewer-2 >/dev/null
$H post --as implementer --item W1 --to reviewer-2 --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H wait --timeout 2s >/dev/null 2>&1
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H claim 1 >/dev/null 2>&1
: > "$HANDOFF_TEST_NOTIFY"; quiet 2s
( CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H wait --timeout 7s >/dev/null 2>&1 ) & WP=$!
sleep 4; alerts; wait $WP
echo "-- the same claim, a moment after that listener ended:"
: > "$HANDOFF_TEST_NOTIFY"; rm -f .handoff/notified/* 2>/dev/null; $H inbox --for implementer 2>&1 | awk '/^gone quiet:/{f=1} f&&/^$/{exit} f{print}' | tidy
kill $P1 2>/dev/null

echo; echo "## W3b. The decision on an APPROVE with no other request on the item"
new w3b
$H post --as implementer --item W2 --to reviewer --commit $C --title W2 --body ../body.md 2>&1 | grep "^posted" | tidy
$H reply --as reviewer 1 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "told the operator" | tidy
echo "-- and a round of two where the other has approved already:"
$H post --as implementer --item W3 --to reviewer,reviewer-2 --commit $C --title W3 --body ../body2.md 2>&1 | grep "^posted" | head -1 | tidy
$H reply --as reviewer-2 4 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "told the operator" | tidy
$H reply --as reviewer 3 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "told the operator" | tidy
