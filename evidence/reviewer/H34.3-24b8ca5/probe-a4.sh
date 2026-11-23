#!/bin/zsh
# Reviewer's probe A4 again for H34.3 (5bef5e5), with the right ids. Usage: probe-a4.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <the same build with HoldFor cut to 6 s>
D=$1; H=$2; AT=$3; H6=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; : > "$HANDOFF_TEST_NOTIFY"; C=$(git rev-parse HEAD); git commit -q --allow-empty -m other; C2=$(git rev-parse HEAD); }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-200}; }
q() { "$@" >/dev/null 2>&1; }
post() { ${B:-$H} post --as implementer --item $1 --to $2 --commit $3 --title "$1" --body ../${5:-body}.md ${=4} 2>&1 | grep -E "^posted|^handoff: [^n]" | tidy; }
reply() { ${B:-$H} reply --as $1 $2 --verdict $3 --commit $4 --body ../body.md 2>&1 | grep -E "^posted" | tidy; }
w() { echo "\$ handoff wait --for implementer --timeout ${1:-2}s $2"; ${B:-$H} wait --for implementer --timeout ${1:-2}s ${=2} 2>&1 | tidy | sed 's/^/    /'; echo "    [exit ${pipestatus[1]}; cursor $(cat .handoff/cursors/implementer 2>/dev/null || cat .handoff/cursor.implementer 2>/dev/null || echo '?')]"; }

echo "## A4. Two reviewers asked at different commits: nothing is held. Then the same commit: held"
new a4
post W2 reviewer $C; post W2 reviewer-2 $C2 "" body2
reply reviewer 1 APPROVE $C; w
echo "-- for comparison, both at one commit, in separate posts:"
post W3 reviewer $C; post W3 reviewer-2 $C "" body2
reply reviewer 4 APPROVE $C; w
echo "\$ handoff brief --for implementer   (the line on APPROVE)"; $H brief --for implementer 2>&1 | grep -o "An APPROVE while another reviewer[^.]*\." | tidy
