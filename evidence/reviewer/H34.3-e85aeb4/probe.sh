#!/bin/zsh
# Reviewer's probes for H34.3 (5bef5e5). Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <the same build with HoldFor cut to 6 s>
D=$1; H=$2; AT=$3; H6=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; : > "$HANDOFF_TEST_NOTIFY"; C=$(git rev-parse HEAD); git commit -q --allow-empty -m other; C2=$(git rev-parse HEAD); }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-200}; }
q() { "$@" >/dev/null 2>&1; }
post() { ${B:-$H} post --as implementer --item $1 --to $2 --commit $3 --title "$1" --body ../${5:-body}.md ${=4} 2>&1 | grep -E "^posted|^handoff: [^n]" | tidy; }
reply() { ${B:-$H} reply --as $1 $2 --verdict $3 --commit $4 --body ../body.md 2>&1 | grep -E "^posted" | tidy; }
w() { echo "\$ handoff wait --for implementer --timeout ${1:-2}s $2"; ${B:-$H} wait --for implementer --timeout ${1:-2}s ${=2} 2>&1 | tidy | sed 's/^/    /'; echo "    [exit ${pipestatus[1]}]"; }

echo "## A1. A round to three reviewers: one wake, with all three"
new a1
post W1 reviewer,reviewer-2,reviewer-3 $C
reply reviewer 1 APPROVE $C; w
reply reviewer-2 2 APPROVE $C; w
echo "  alerts to the operator so far: $(grep -c . $HANDOFF_TEST_NOTIFY)"
reply reviewer-3 3 APPROVE $C; w

echo; echo "## A2. A held APPROVE is carried by a CHANGES; a later APPROVE is held again"
new a2
post W1 reviewer,reviewer-2,reviewer-3 $C
reply reviewer 1 APPROVE $C; w
reply reviewer-2 2 CHANGES $C; w
reply reviewer-3 3 APPROVE $C; w
echo "-- the fix is posted, replacing all three; the held APPROVE is on a replaced request now:"
post W1 reviewer,reviewer-2,reviewer-3 $C2 "--supersedes 1,2,3" body2
w

echo; echo "## A3. --emit holds nothing; inbox shows the held APPROVE meanwhile"
new a3
post W1 reviewer,reviewer-2 $C
reply reviewer 1 APPROVE $C
echo "\$ handoff inbox --for implementer (awaiting the operator)"; $H inbox --for implementer 2>&1 | awk '/^awaiting the operator:/{f=1} f&&/^$/{exit} f{print}' | tidy
w 2 --emit

echo; echo "## A4. One reviewer only, and two reviewers at different commits: nothing is held"
new a4
post W1 reviewer $C; reply reviewer 1 APPROVE $C; w
post W2 reviewer $C; post W2 reviewer-2 $C2 "" body2
reply reviewer 3 APPROVE $C; w

echo; echo "## A5. The operator's fyi to the session carries a held APPROVE"
new a5; sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer >/dev/null
post W1 reviewer,reviewer-2 $C
reply reviewer 1 APPROVE $C
echo "\$ (I) handoff wait --timeout 2s"; CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H wait --timeout 2s 2>&1 | tidy | sed 's/^/    /'
q python3 $AT $H fyi "carry on"
echo "\$ (I) handoff wait --timeout 2s   (after the operator's fyi)"; CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H wait --timeout 2s 2>&1 | tidy | sed 's/^/    /'
kill $PI 2>/dev/null

echo; echo "## A6. The hold's end, with the build whose hold is 6 s: no new write, and a restart keeps it"
B=$H6
new a6
post W1 reviewer,reviewer-2 $C
reply reviewer 1 APPROVE $C
w 2
echo "-- a second listener, started at once, for 8 s:"
t0=$(python3 -c 'import time;print(time.time())'); w 8; echo "    [returned after $(python3 -c "import time;print(round(time.time()-$t0))") s]"
echo; echo "## A7. Two held APPROVEs go in one wake when the first's time is up (6 s build)"
new a7
post W1 reviewer,reviewer-2,reviewer-3 $C
reply reviewer 1 APPROVE $C; sleep 3; reply reviewer-2 2 APPROVE $C
t0=$(python3 -c 'import time;print(time.time())'); w 9; echo "    [returned after $(python3 -c "import time;print(round(time.time()-$t0))") s; the first APPROVE was 3 s old at the start]"
unset B
