#!/bin/zsh
# mixed.sh NEW OLD: a listener built at OLD is already waiting for the
# implementer when NEW's commands withdraw a request under the go-ahead and
# the reviewer then approves it. Prints each command and what it printed.
NEW=$1; OLD=$2
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID CLAUDE_PROJECT_DIR HANDOFF_ROLE
S=$(mktemp -d "${TMPDIR:-/tmp}/h38-mixed.XXXX")
export HANDOFF_TEST_NOTIFY=$S/notified
mkdir $S/repo && cd $S/repo
git init -q -b main && git -c user.name=t -c user.email=t@example.com commit -q --allow-empty -m base
git switch -q -c w1 && echo a > a && git add a && git -c user.name=t -c user.email=t@example.com commit -q -m A
A=$(git rev-parse HEAD)
echo "please review" > $S/body.md
run() { echo "\$ $*"; "$@" 2>&1; echo "exit $?"; }
run $NEW init --roles implementer,reviewer --reviewers reviewer --main main >/dev/null
echo '$ HANDOFF_ROLE=implementer new post --item W1 --to reviewer --commit A'
HANDOFF_ROLE=implementer $NEW post --item W1 --to reviewer --commit $A --title W1 --body $S/body.md 2>&1; echo "exit $?"
echo '$ (at a terminal) new away "Merge when approved."'
script -q /dev/null $NEW away "Merge when approved." </dev/null 2>&1 | tr -d '\r' | sed 's/^\^D//'; echo "exit $?"
echo '$ HANDOFF_ROLE=implementer old wait --timeout 8s &   (the listener built at the old commit, already waiting)'
HANDOFF_ROLE=implementer $OLD wait --timeout 8s > $S/old.out 2>&1 &
L=$!
sleep 2
echo '$ HANDOFF_ROLE=implementer new withdraw 1'
HANDOFF_ROLE=implementer $NEW withdraw 1 2>&1; echo "exit $?"
echo '$ HANDOFF_ROLE=reviewer new reply 1 --verdict APPROVE --commit A'
HANDOFF_ROLE=reviewer $NEW reply 1 --verdict APPROVE --commit $A --body $S/body.md 2>&1; echo "exit $?"
wait $L; echo "the old listener: exit $? (3 is a timeout), and it printed:"
cat $S/old.out
echo "(end of what it printed)"
rm -rf $S
