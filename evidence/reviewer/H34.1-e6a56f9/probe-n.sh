#!/bin/zsh
# Reviewer's probes N1 to N5 for H34.1 round 2 (5ac3a2b). Usage: probe-n.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without row 108>
D=$1; H=$2; AT=$3; OLD=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && ${2:-$H} init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md
  python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='${QUIET:-30m}';json.dump(c,open(p,'w'),indent=2)"; : > "$HANDOFF_TEST_NOTIFY"; }
# work <branch>: a commit on a new branch from main; prints its sha
work() { git checkout -q -b $1 main && git commit -q --allow-empty -m "$1" && git rev-parse HEAD && git checkout -q main; }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g; s/since 20[0-9T:.-]+Z/since TIME/g' | cut -c1-${W:-230}; }
x() { echo "\$ ${*//$H/handoff}" | sed -E 's#(--body) [^ ]*#\1 f#' | tidy -c1-200; "$@" 2>&1 | grep -v '^$' | tidy; echo "  [exit ${pipestatus[1]}]"; }
q() { "$@" >/dev/null 2>&1; }
op() { echo "\$ (the operator, at a terminal) handoff $*"; python3 $AT $H "$@" 2>&1 | grep -v '^$' | tidy; }
sect() { awk -v t="$1:" '$0==t{f=1} f&&/^$/{exit} f{print}' | tidy; }   # one titled section of an inbox
inboxm() { echo "\$ handoff inbox --for ${1:-implementer}   (the \"approved, to merge\" part)"; r=$($H inbox --for ${1:-implementer} 2>&1 | sect "approved, to merge"); echo "${r:-  (no such section)}"; $H inbox --for ${1:-implementer} --quiet; echo "  [inbox --quiet: exit $?]"; }
alerts() { echo "  alerts to the operator: $(grep -c . "$HANDOFF_TEST_NOTIFY" 2>/dev/null)"; grep -h "merge\|approved" "$HANDOFF_TEST_NOTIFY" 2>/dev/null | tidy | sed 's/^/    /'; }
post() { $H post --as implementer --item $1 --to $2 --commit $3 --title "$1" --body ../body.md ${=4} 2>&1 | grep -E "^posted|^handoff" | tidy; }
approve() { $H reply --as $1 $2 --verdict APPROVE --commit $3 --body ../body.md 2>&1 | grep -E "^posted|^handoff" | tidy; }

echo "## N1. The go-ahead itself approves work already approved by its reviewer: what told the requester"
new n1; C=$(work w1)
post W1 reviewer $C; approve reviewer 1 $C
q $H wait --for implementer --timeout 1s
echo "-- no session has implementer when the operator goes away:"
op away "merge when approved" | tail -4
inboxm
echo; echo "-- the same, with a session holding implementer:"
new n1b; C=$(work w1); sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer >/dev/null
post W1 reviewer $C; approve reviewer 1 $C
q env CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H wait --timeout 1s
op away "merge when approved" | tail -4
inboxm
x env CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H wait --timeout 2s
inboxm | head -3
kill $PI 2>/dev/null

echo; echo "## N2. Held work reaches main all the same: one alert"
QUIET=2s new n2; C=$(work w1)
post W1 reviewer,reviewer-2 $C; q python3 $AT $H away "carry on"; approve reviewer 1 $C; approve reviewer-2 2 $C; q python3 $AT $H back
q python3 $AT $H decide W1 --deny
echo "\$ handoff show 4   (the APPROVE of a held request, head)"; $H show 4 2>&1 | sed -n '1,9p' | tidy
x $H wait --for implementer --timeout 2s
: > "$HANDOFF_TEST_NOTIFY"
git merge -q --no-ff -m "merge w1" w1
q $H wait --for reviewer-3 --timeout 3s; q $H wait --for reviewer-3 --timeout 3s
echo "  alerts after the merge: $(grep -c . "$HANDOFF_TEST_NOTIFY")"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'
echo "\$ handoff (the operator's view)"; $H 2>&1 | tidy

echo; echo "## N3. git is asked in the main worktree: from a subfolder, from a linked worktree, and by a listener whose worktree is removed"
QUIET=4s new n3; C=$(work w1)
post W1 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide W1 --approve
mkdir -p sub; git worktree add -q "$D/n3-wt" w1 2>/dev/null
( cd sub && inboxm | sed -n '2,3p' )
( cd "$D/n3-wt" && echo "-- from a linked worktree:" && $H inbox --for implementer 2>&1 | sect "approved, to merge" )
( cd "$D/n3-wt" && $H wait --for reviewer-3 --timeout 9s > "$D/n3-wait.out" 2>&1; echo "[exit $?]" >> "$D/n3-wait.out" ) & WP=$!
sleep 1; git worktree remove --force "$D/n3-wt"; echo "-- the listener's worktree is removed while it listens; 9 s later it has said:"
wait $WP; sed 's/^/  /' "$D/n3-wait.out" | tidy

echo; echo "## N4. Two requesters' work approved on one item: decide takes the id"
new n4; C=$(work w1); C2=$(work w2)
post W1 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide 1 --approve
$H post --as reviewer-3 --item W1 --to reviewer-2 --commit $C2 --title other --body ../body.md 2>&1 | grep "^posted" | tidy
approve reviewer-2 4 $C2; q python3 $AT $H decide 4 --approve
op decide W1 --deny
op decide W1 --approve
