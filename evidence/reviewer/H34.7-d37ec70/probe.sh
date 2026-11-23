#!/bin/zsh
# Reviewer's probes for H34.7. Usage: probe.sh <scratch dir> <new handoff binary> <atterm.py>
D=$1; H=$2; AT=$3; OLD=$(whence -p handoff)
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE HANDOFF_TEST_FAULT
new() { P="$D/$1"; rm -rf "$P"; mkdir -p "$P"; cd "$P"; : > "$HANDOFF_TEST_NOTIFY"
  git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m x && git commit -q --allow-empty -m y
  X=$(git rev-parse HEAD~1); Y=$(git rev-parse HEAD)
  $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo other > ../body2.md }
run() { echo "\$ ${${*//$H/handoff}//$OLD/handoff-installed}"; "$@" 2>&1 | grep -v "no session has"; echo "  [exit ${pipestatus[1]}]"; }
states() { echo "  states: $($H inbox --for operator >/dev/null 2>&1; for i in $@; do printf '#%s=%s ' $i "$($H show $i | sed -n 's/^state: //p')"; done)"; }
post3() { run $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title W1 --body ../body.md "$@"; }

echo "## P1. A round cut after its first request, under a go-ahead; the member is approved; the same post completes it; then all approve"
new p1
python3 $AT $H away "Merge when the reviewers approve." | head -1
echo "\$ HANDOFF_TEST_FAULT=first-line handoff post ... --to reviewer,reviewer-2,reviewer-3 --commit X"; HANDOFF_TEST_FAULT=first-line $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $X --title W1 --body ../body.md >/dev/null 2>&1; echo "  [exit $?]"
run $H reply 2 --as reviewer --verdict APPROVE --commit $X --body ../body.md
states 2
run $H inbox --for implementer
echo "  what the operator was told:"; sed 's/^/    /' $HANDOFF_TEST_NOTIFY
echo "-- refused while pending:"
run $H post --as implementer --item W1 --to reviewer-2 --commit $X --title W1 --body ../body.md
post3 --commit $Y
run $H post --as implementer --item W1 --to reviewer-2,reviewer,reviewer-3 --commit $X --title W1 --body ../body.md
run $H post --as implementer --item W1 --to reviewer,reviewer-2 --commit $X --title W1 --body ../body.md
echo "-- another item is not held up:"
run $H post --as implementer --item W2 --to reviewer --commit $X --title W2 --body ../body.md
echo "-- two of the same post at once, then once more:"
( $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $X --title W1 --body ../body.md > ../a.out 2>&1 & $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $X --title W1 --body ../body.md > ../b.out 2>&1 & wait )
echo "  first:";  grep -v "no session has" ../a.out | sed 's/^/    /'; echo "  second:"; grep -v "no session has" ../b.out | sed 's/^/    /'
post3 --commit $X
echo "  requests on W1 in the record: $(grep -c '"kind":"request","item":"W1"' .handoff/channel.jsonl)"
states 2
N2=$($H inbox --for reviewer-2 | sed -n 's/^  #\([0-9]*\) request W1.*/\1/p' | head -1); N3=$($H inbox --for reviewer-3 | sed -n 's/^  #\([0-9]*\) request W1.*/\1/p' | head -1)
run $H reply $N2 --as reviewer-2 --verdict APPROVE --commit $X --body ../body.md
states 2 $N2 $N3
run $H reply $N3 --as reviewer-3 --verdict APPROVE --commit $X --body ../body.md
states 2 $N2 $N3
echo "-- the installed, older handoff reads this record:"
run $OLD version
run $OLD show 2
run $OLD inbox --for implementer

echo; echo "## P2. A new round cut after one member: the old round stays whole and live; neither can be approved under a go-ahead"
new p2
post3 --commit $X
run $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
run $H reply 2 --as reviewer-2 --verdict APPROVE --commit $X --body ../body.md
run $H reply 3 --as reviewer-3 --verdict CHANGES --commit $X --body ../body.md
echo "\$ HANDOFF_TEST_FAULT=first-line handoff post ... --commit Y --supersedes 1,2,3"; HANDOFF_TEST_FAULT=first-line $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $Y --title W1 --body ../body.md --supersedes 1,2,3 >/dev/null 2>&1; echo "  [exit $?]"
states 1 2 3 7
python3 $AT $H away "Merge when the reviewers approve." | head -1
run $H reply 3 --as reviewer-3 --verdict APPROVE --commit $X --body ../body2.md
states 1 2 3 7
echo "  what the operator was told:"; sed 's/^/    /' $HANDOFF_TEST_NOTIFY
echo "-- the same post with other supersedes is refused; as it was, with another body, completes it:"
post3 --commit $Y --supersedes 1,2
run $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $Y --title W1 --body ../body2.md --supersedes 1,2,3
states 1 2 3 7
run $H inbox --for implementer

echo; echo "## P3. Odd inputs"
new p3
run $H post --as implementer --item W1 --to reviewer,reviewer --commit $X --title W1 --body ../body.md
run $H post --as implementer --item W1 --to reviewer,implementer --commit $X --title W1 --body ../body.md
run $H post --as implementer --item W1 --to reviewer,nobody --commit $X --title W1 --body ../body.md
run $H post --as implementer --item W1 --to "reviewer," --commit $X --title W1 --body ../body.md
run $H post --as implementer --item W1 --to reviewer --commit $X --title W1 --body ../body.md --supersedes 1,2
post3 --commit $X --key k1
post3 --commit $X --key k1
post3 --commit $X
post3 --commit $Y
post3 --commit $Y --supersedes 1,2
post3 --commit $Y --supersedes 1,2,2
post3 --commit $Y --supersedes 1,2,3,9
run $H post --as implementer --item W1 --to reviewer,reviewer-2 --commit $Y --title W1 --body ../body.md --supersedes 1,2
states 1 2 3
echo "  (reviewer-3's request at X is still live beside the new round at Y: the go-ahead can't cover mixed commits)"

echo; echo "## P4. No go-ahead: what the operator is asked when a member of a cut round is approved"
new p4
HANDOFF_TEST_FAULT=first-line $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit $X --title W1 --body ../body.md >/dev/null 2>&1
run $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
echo "  what the operator was told:"; sed 's/^/    /' $HANDOFF_TEST_NOTIFY
run $H
