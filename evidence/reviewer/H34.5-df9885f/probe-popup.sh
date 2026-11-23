#!/bin/zsh
# Reviewer's probe for H34.5 round 4 (df9885f): the real pop-up path, its dialog sent to a file (HANDOFF_TEST_POPUP), and decide at a terminal.
# Usage: probe-popup.sh <a new, empty scratch dir> <handoff binary> <atterm_n.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log" HANDOFF_TEST_POPUP="$D/popup.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_POPUP"; }
tidy() { cut -c1-${W:-330}; }
popups() { sleep 1; echo "  what the pop-up's dialog was given (title | text | buttons):"; sed 's/^/    /' "$HANDOFF_TEST_POPUP" | tidy; : > "$HANDOFF_TEST_POPUP"; touch "$HANDOFF_TEST_POPUP.release"; sleep 1; rm -f "$HANDOFF_TEST_POPUP.release"; }

echo "## PP1. Three reviewers: one asks for changes, one approves, one hasn't picked up. The pop-up for the APPROVE"
new pp1
$H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $C --title "the title of W1" --body ../body.md 2>&1 | grep "^posted" | head -1 | tidy
$H reply --as reviewer-2 2 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
$H reply --as reviewer 1 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
popups
echo "\$ (the operator, at a terminal) handoff decide 1 --approve   (answered n)"; python3 $AT $H decide 1 --approve 2>&1 | grep -v '^$' | tidy | sed 's/^/    /'

echo; echo "## PP2. One reviewer only: the pop-up as before"
new pp2
$H post --as implementer --item W2 --to reviewer --commit $C --title "the title of W2" --body ../body.md 2>&1 | grep "^posted" | tidy
$H reply --as reviewer 1 --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
popups
