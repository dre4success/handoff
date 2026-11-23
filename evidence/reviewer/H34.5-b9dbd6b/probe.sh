#!/bin/zsh
# Reviewer's probes for H34.5 (b9dbd6b). Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"; }
quiet() { python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='$1';json.dump(c,open(p,'w'),indent=2)"; }
tidy() { sed -E 's/since [0-9]{2}:[0-9]{2}/since HH:MM/g; s/at 20[0-9T:.-]+Z/at TIME/g; s/\(([0-9]+) s\)/(N s)/g; s/for [0-9]+ s/for N s/g' | cut -c1-${W:-300}; }
alerts() { echo "  alerts to the operator ($(grep -c . "$HANDOFF_TEST_NOTIFY")):"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'; : > "$HANDOFF_TEST_NOTIFY"; }
pass() { $H wait --for ${1:-reviewer-3} --timeout 2s >/dev/null 2>&1; }

echo "## W1. The quiet alert says why: a blocked role, one that never listened, one with no session, one listening"
new w1; sleep 300 & P1=$!; sleep 300 & P2=$!
ANTIGRAVITY_CONVERSATION_ID=A $H role reviewer >/dev/null
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H role reviewer-2 >/dev/null
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$P2 $H role implementer >/dev/null
echo "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10}" | ANTIGRAVITY_CONVERSATION_ID=A $H hook listen --timeout 2s >/dev/null 2>&1
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$P2 $H post --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
: > "$HANDOFF_TEST_NOTIFY"; quiet 2s; sleep 3
echo "-- reviewer: Antigravity at its cap. reviewer-2: has a session, never listened. reviewer-3: no session."
pass implementer; alerts
echo "\$ handoff   (gone quiet)"; $H 2>&1 | awk '/^gone quiet:/{f=1} f&&/^$/{exit} f{print}' | tidy
echo "\$ handoff inbox --for implementer   (gone quiet)"; $H inbox --for implementer 2>&1 | awk '/^gone quiet:/{f=1} f&&/^$/{exit} f{print}' | tidy
echo "-- reviewer-2 claims, and is then listening while its claim goes quiet:"
CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H claim 2 >/dev/null 2>&1
( CLAUDE_CODE_SESSION_ID=R2 CLAUDE_PID=$P1 $H wait --timeout 6s >/dev/null 2>&1 ) & WP=$!
sleep 4; pass implementer; alerts; wait $WP
kill $P1 $P2 2>/dev/null

echo; echo "## W2. A note left undelivered past quiet_after; then delivered"
new w2; sleep 300 & P1=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$P1 $H role implementer >/dev/null
$H note --as reviewer --to implementer --item W1 --title "hold the merge" --body ../body.md 2>&1 | grep "^posted" | tidy
$H note --as reviewer --to implementer --item W1 --title "a second note" --body ../body2.md 2>&1 | grep "^posted" | tidy
: > "$HANDOFF_TEST_NOTIFY"; quiet 2s; sleep 3
pass; alerts
echo "-- a second pass sends nothing more:"; pass; alerts
echo "-- the implementer's wait delivers both; then a pass:"
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$P1 $H wait --timeout 2s 2>&1 | tidy | sed 's/^/    /'
sleep 3; pass; alerts
echo "-- the operator's fyi to that session, left undelivered:"
quiet 30m; python3 $AT $H fyi "carry on" >/dev/null 2>&1; : > "$HANDOFF_TEST_NOTIFY"; quiet 2s; sleep 3; pass; alerts
kill $P1 2>/dev/null

echo; echo "## W3. The decision on one APPROVE names the item's other requests"
new w3
$H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | head -1 | tidy
$H reply --as reviewer-2 2 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
$H claim --as reviewer-3 3 >/dev/null 2>&1
: > "$HANDOFF_TEST_NOTIFY"
$H reply --as reviewer 1 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "^posted|told the operator" | tidy
alerts
echo "-- for comparison, one reviewer only:"
$H post --as implementer --item W2 --to reviewer --commit $C --title W2 --body ../body2.md 2>&1 | grep "^posted" | tidy
$H reply --as reviewer 6 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep -E "told the operator" | tidy
