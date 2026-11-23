#!/bin/zsh
# Reviewer's probes for H34.7 round 3 (28ca099). Usage: probe4.sh <scratch dir> <new handoff> <atterm.py> <atterm_y.py>
D=$1; H=$2; AT=$3; ATY=$4; OLD=$(whence -p handoff)
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE HANDOFF_TEST_FAULT
new() { P="$D/$1"; rm -rf "$P"; mkdir -p "$P"; cd "$P"; : > "$HANDOFF_TEST_NOTIFY"
  git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m x && git commit -q --allow-empty -m y
  X=$(git rev-parse HEAD~1); Y=$(git rev-parse HEAD)
  $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo other > ../body2.md }
run() { echo "\$ ${${*//$H/handoff}//$OLD/old}"; "$@" 2>&1 | grep -v "no session has\|advisory: .*head.json is behind" | sed -E 's/\(pid [0-9]+\)/(pid N)/'; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
states() { echo "  states: $(for i in $@; do printf '#%s=%s ' $i "$($H show $i 2>/dev/null | sed -n 's/^state: //p')"; done)"; }
P3=(--as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --title W1 --body ../body.md)

echo "## S1. How a round's request is shown"
new s1
run $H post $P3 --commit $X
run $H wait --for reviewer --timeout 1s
echo "\$ handoff inbox --for reviewer | grep '#'"; $H inbox --for reviewer | grep '#'
echo "\$ handoff show 2 | head -9"; $H show 2 | head -9
q $H reply 1 --as reviewer --verdict APPROVE --commit $X --body ../body.md
echo "\$ handoff   (the operator, not at a terminal)"; $H 2>&1 | grep -v "^$"
echo "  kinds stored: $(sed -n 's/.*"kind":"\([a-z]*\)".*/\1/p' .handoff/channel.jsonl | tr '\n' ' ')"

echo; echo "## S2. An older listener, for a role the round doesn't even ask, and how it ends"
new s2
( $OLD wait --for reviewer-3 --timeout 40s > ../oldwait.out 2>&1 & ); sleep 2
echo "  presence of reviewer-3: $(cat .handoff/presence/reviewer-3.json | sed -E 's/"pid":[0-9]+/"pid":N/; s/"since":"[^"]*"/"since":"T"/')"
run $H post --as implementer --item W1 --to reviewer,reviewer-2 --title W1 --body ../body.md --commit $X
echo "-- one request still goes through, and a note to that role ends its listener:"
run $H post --as implementer --item W2 --to reviewer --title W2 --body ../body.md --commit $X
run $H note --as implementer --to reviewer-3 --item W1 --title "handoff was upgraded" --body ../body2.md
sleep 2; echo "  the old listener printed: $(cat ../oldwait.out | grep -v advisory | tr '\n' ' ')"
run $H post --as implementer --item W1 --to reviewer,reviewer-2 --title W1 --body ../body.md --commit $X
echo "-- a listener of this build doesn't hold a round up:"
( $H wait --for reviewer-3 --timeout 6s > /dev/null 2>&1 & ); sleep 1
echo "  presence of reviewer-3: $(cat .handoff/presence/reviewer-3.json | sed -E 's/"pid":[0-9]+/"pid":N/; s/"since":"[^"]*"/"since":"T"/')"
run $H post --as implementer --item W3 --to reviewer,reviewer-2 --title W3 --body ../body.md --commit $X
sleep 6

echo; echo "## S3. Keys with a slash in them"
new s3
run $H post --as implementer --item W1 --to reviewer,reviewer-2 --title W1 --body ../body.md --commit $X --key W1/r1
echo "-- a single request on another item, with a key that was never used:"
run $H post --as implementer --item W2 --to reviewer-3 --title W2 --body ../body.md --commit $X --key W1
echo "-- one request keyed W9/notes, then a round keyed W9, never used either:"
run $H post --as implementer --item W3 --to reviewer-3 --title W3 --body ../body.md --commit $X --key W9/notes
run $H post --as implementer --item W4 --to reviewer,reviewer-2 --title W4 --body ../body.md --commit $X --key W9
echo "-- the cases the guard is for:"
run $H post --as implementer --item W1 --to reviewer-3,reviewer --title other --body ../body.md --commit $Y --key W1/r1
run $H post --as implementer --item W5 --to reviewer-3 --title W5 --body ../body.md --commit $X --key W1/r1

echo; echo "## S4. (#331 point 2) A replacement cut after one request; the operator then denies an old request it was to replace; the same post again"
new s4
q $H post $P3 --commit $X
q $H reply 2 --as reviewer-2 --verdict APPROVE --commit $X --body ../body.md
echo "\$ HANDOFF_TEST_FAULT=first-line handoff post ... --commit Y --supersedes 1,2,3"; HANDOFF_TEST_FAULT=first-line $H post $P3 --commit $Y --supersedes 1,2,3 >/dev/null 2>&1; echo "  [exit $?]"
echo "\$ handoff decide 2 --deny --note no   (the operator)"; python3 $ATY $H decide 2 --deny --note no 2>&1 | grep -v advisory | tail -1
states 1 2 3 5
run $H post $P3 --commit $Y --supersedes 1,3
run $H post $P3 --commit $Y --supersedes 1,2,3
states 1 2 3 5 7 8
echo "\$ handoff show 7 | grep -E '^(supersedes|round)'"; $H show 7 | grep -E '^(supersedes|round)'
