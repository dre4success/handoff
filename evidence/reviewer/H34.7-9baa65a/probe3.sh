#!/bin/zsh
# Reviewer's probes for H34.7 round 2 (9baa65a). Usage: probe3.sh <scratch dir> <handoff binary> <atterm.py> <atterm_y.py>
D=$1; H=$2; AT=$3; ATY=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE HANDOFF_TEST_FAULT
new() { P="$D/$1"; rm -rf "$P"; mkdir -p "$P"; cd "$P"; : > "$HANDOFF_TEST_NOTIFY"
  git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m x && git commit -q --allow-empty -m y
  X=$(git rev-parse HEAD~1); Y=$(git rev-parse HEAD)
  $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo other > ../body2.md }
run() { echo "\$ ${*//$H/handoff}"; "$@" 2>&1 | grep -v "no session has\|advisory: .*head.json is behind"; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
states() { echo "  states: $(for i in $@; do printf '#%s=%s ' $i "$($H show $i 2>/dev/null | sed -n 's/^state: //p')"; done)"; }
told() { echo "  what the operator was told:"; grep -v "go-ahead is recorded" $HANDOFF_TEST_NOTIFY | sed 's/^/    /'; : > $HANDOFF_TEST_NOTIFY; }
P3=(--as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title W1 --body ../body.md)

echo "## R1. Completing a cut round with another body, or another key, is refused; as it was, it completes"
new r1
q env HANDOFF_TEST_FAULT=first-line $H post $P3 --commit $X
run $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title W1 --body ../body2.md --commit $X
run $H post $P3 --commit $X --key k9
run $H post $P3 --commit $X
echo "  kinds on W1: $(grep '"item":"W1"' .handoff/channel.jsonl | sed -n 's/.*"kind":"\([a-z]*\)".*/\1/p' | tr '\n' ' ')"

echo; echo "## R2. A complete round; the operator denies one approved request; under a go-ahead the other two approve"
new r2
q $H post $P3 --commit $X
q $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
echo "\$ handoff decide 1 --deny --note no   (the operator)"; python3 $ATY $H decide 1 --deny --note no 2>&1 | grep -v advisory | tail -1
python3 $AT $H away "Merge when the reviewers approve." | head -1; : > $HANDOFF_TEST_NOTIFY
q $H reply 2 --as reviewer-2 --verdict APPROVE --commit $X --body ../body.md
run $H reply 3 --as reviewer-3 --verdict APPROVE --commit $X --body ../body.md
states 1 2 3; told

echo; echo "## R3. A cut first round whose one request the operator approves: what is left for the requester"
new r3
q env HANDOFF_TEST_FAULT=first-line $H post $P3 --commit $X
q $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
told
echo "\$ handoff decide 1 --approve   (the operator; what decide shows first)"; python3 $ATY $H decide 1 --approve 2>&1 | grep -v advisory | tail -5
states 1
run $H inbox --for implementer
run $H inbox --for implementer --quiet
run $H post --as implementer --item W1 --to reviewer --title next --body ../body.md --commit $Y
run $H post $P3 --commit $X
states 1 2 3
run $H inbox --for reviewer-2

echo; echo "## R4. A round's request through the usual moves: what each role reads, claim, CHANGES, QUESTION and its answer, a single post replacing one, a round replacing a single"
new r4
run $H post $P3 --commit $X
run $H inbox --for reviewer
run $H wait --for reviewer --timeout 1s
run $H show 1
run $H claim --as reviewer 1
run $H reply 1 --as reviewer --verdict QUESTION --commit $X --body ../body.md
run $H note --as implementer --to reviewer --re 4 --body ../body2.md
states 1
run $H reply 2 --as reviewer-2 --verdict CHANGES --commit $X --body ../body.md
run $H post --as implementer --item W1 --to reviewer-2 --title fix --body ../body.md --commit $Y --supersedes 2
states 1 2 3 8
run $H post $P3 --commit $Y --supersedes 1,8,3
states 1 2 3 8 9 10 11
run $H
