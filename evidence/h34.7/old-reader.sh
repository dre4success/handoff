#!/bin/zsh
# H34.7 evidence: a round written by this branch, read by a handoff older than
# row 106 (main's build). The old one counts none of the round's requests, so
# it approves nothing on them, not even on a part of the panel.
# Usage: old-reader.sh <scratch dir> <new handoff> <old handoff>
D=$1; NEW=$2; OLD=$3; HERE=${0:A:h}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
P="$D/proj"; rm -rf "$P"; mkdir -p "$P"; cd "$P"
git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m work
$NEW init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null
echo body > ../body.md
run() { echo "\$ $*" | sed "s|$NEW|new|; s|$OLD|old|"; "$@" 2>&1 | grep -v "no session"; }
echo "\$ new away (at a terminal)"; python3 "$HERE/atterm.py" $NEW away "Merge when the reviewers approve." | head -1
echo "# A round cut after its first request, by the new build:"
HANDOFF_TEST_FAULT=first-line $NEW post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit w1 --title W1 --body ../body.md >/dev/null 2>&1
run $OLD show 2
echo "# The old build's reviewer tries to approve it:"
run $OLD reply 2 --as reviewer --verdict APPROVE --commit w1 --body ../body.md
echo "# The new build's reviewer approves it; the old build's view of it:"
run $NEW reply 2 --as reviewer --verdict APPROVE --commit w1 --body ../body.md
run $OLD inbox --for operator
run $OLD inbox --for implementer
echo "# The old build's listener for the implementer delivers the approval as a plain line, with no go-ahead line:"
run $OLD wait --for implementer --timeout 2s
echo "# The legacy case (reviewer-2, #331): a request from the old build, then a round replacing it, while an old listener runs."
git commit -q --allow-empty -m work2
run $OLD post --as implementer --item W2 --to reviewer --commit w1~1 --title W2 --body ../body.md | tee ../legacy
legacy=$(sed -n -E 's/^posted #([0-9]+) .*/\1/p' ../legacy | head -1)
$OLD wait --for reviewer-3 --timeout 15s > ../oldwait.out 2>&1 &
sleep 2
run $NEW post --as implementer --item W2 --to reviewer,reviewer-2,reviewer-3 --commit w1 --title W2 --body ../body.md --supersedes $legacy
echo "# Once the old listener has ended, the same post goes through:"
wait
run $NEW post --as implementer --item W2 --to reviewer,reviewer-2,reviewer-3 --commit w1 --title W2 --body ../body.md --supersedes $legacy
