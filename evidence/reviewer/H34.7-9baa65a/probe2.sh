#!/bin/zsh
# Reviewer's second probes for H34.7, after reading #308 and #314: do their failures reproduce?
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE HANDOFF_TEST_FAULT
new() { P="$D/$1"; rm -rf "$P"; mkdir -p "$P"; cd "$P"; : > "$HANDOFF_TEST_NOTIFY"
  git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m x && git commit -q --allow-empty -m y
  X=$(git rev-parse HEAD~1); Y=$(git rev-parse HEAD)
  $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo other > ../body2.md }
run() { echo "\$ ${*//$H/handoff}"; "$@" 2>&1 | grep -v "no session has\|advisory: .*head.json is behind"; echo "  [exit ${pipestatus[1]}]"; }
states() { echo "  states: $(for i in $@; do printf '#%s=%s ' $i "$($H show $i 2>/dev/null | sed -n 's/^state: //p')"; done)"; }
P3=(--as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title W1 --body ../body.md)

echo "## Q1. (#308 point 1) A replacement round cut after one request; the operator decides that one request; the old round is then approved"
new q1
run $H post $P3 --commit $X
python3 $AT $H away "Merge when the reviewers approve." | head -1
echo "\$ HANDOFF_TEST_FAULT=first-line handoff post ... --commit Y --supersedes 1,2,3"; HANDOFF_TEST_FAULT=first-line $H post $P3 --commit $Y --supersedes 1,2,3 >/dev/null 2>&1; echo "  [exit $?]"
states 1 2 3 5
run $H reply 5 --as reviewer --verdict APPROVE --commit $Y --body ../body.md
echo "\$ handoff decide 5 --deny --note probe   (the operator, at a terminal)"; python3 ${0:A:h}/atterm_y.py $H decide 5 --deny --note probe 2>&1 | grep -v advisory | tail -2
states 1 2 3 5
run $H inbox --for implementer
run $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
run $H reply 2 --as reviewer-2 --verdict APPROVE --commit $X --body ../body.md
run $H reply 3 --as reviewer-3 --verdict APPROVE --commit $X --body ../body.md
states 1 2 3 5
echo "\$ handoff show 1 | grep 'approved under'"; $H show 1 2>/dev/null | grep "approved under"

echo; echo "## Q2. (#314 point 1) A keyed round, approved under the go-ahead, then the very same post again"
new q2
python3 $AT $H away "Merge when the reviewers approve." | head -1
run $H post $P3 --commit $X --key k1
for i in 2 3 4; do r=reviewer; [ $i = 3 ] && r=reviewer-2; [ $i = 4 ] && r=reviewer-3; $H reply $i --as $r --verdict APPROVE --commit $X --body ../body.md >/dev/null 2>&1; done
states 2 3 4
run $H post $P3 --commit $X --key k1

echo; echo "## Q3. (#314 point 1) A keyed round, complete and open, posted again with the same key and another title and body"
new q3
run $H post $P3 --commit $X --key k1
run $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title OTHER --body ../body2.md --commit $X --key k1
echo "-- for comparison, one request with a key, then the same key with another body:"
run $H post --as implementer --item W2 --to reviewer --title W2 --body ../body.md --commit $X --key k2
run $H post --as implementer --item W2 --to reviewer --title W2 --body ../body2.md --commit $X --key k2

echo; echo "## Q4. (#308 point 3) An unkeyed round, approved under the go-ahead, then the same post again"
new q4
python3 $AT $H away "Merge when the reviewers approve." | head -1
run $H post $P3 --commit $X
for i in 2 3 4; do r=reviewer; [ $i = 3 ] && r=reviewer-2; [ $i = 4 ] && r=reviewer-3; $H reply $i --as $r --verdict APPROVE --commit $X --body ../body.md >/dev/null 2>&1; done
states 2 3 4
run $H post $P3 --commit $X
echo "  requests on W1 in the record: $(grep -c '"kind":"request","item":"W1"' .handoff/channel.jsonl)"
echo "-- for comparison, one request, approved under the go-ahead, then the same post again:"
run $H post --as implementer --item W2 --to reviewer --title W2 --body ../body.md --commit $X
N=$($H inbox --for reviewer 2>/dev/null | sed -n 's/^  #\([0-9]*\) request W2.*/\1/p' | head -1); $H reply $N --as reviewer --verdict APPROVE --commit $X --body ../body.md >/dev/null 2>&1; states $N
run $H post --as implementer --item W2 --to reviewer --title W2 --body ../body.md --commit $X
echo "  requests on W2 in the record: $(grep -c '"kind":"request","item":"W2"' .handoff/channel.jsonl)"
