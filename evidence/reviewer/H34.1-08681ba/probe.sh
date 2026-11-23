#!/bin/zsh
# Reviewer's probes for H34.1 (6a918e8). Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without row 108>
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

echo "## M1. One reviewer, the operator approves: listed until the commit is in main"
new m1; C=$(work w1)
post W1 reviewer $C; approve reviewer 1 $C
inboxm
op decide W1 --approve
inboxm
x $H wait --for implementer --timeout 2s
inboxm
echo "\$ handoff   (the operator's view, the part)"; $H 2>&1 | sect "approved, to merge"
git merge -q --no-ff -m "merge w1" w1
inboxm

echo; echo "## M2. A round to two reviewers under a go-ahead: one line, one alert; then gone quiet"
QUIET=3s new m2; C=$(work w2)
q python3 $AT $H away "carry on"
post W2 reviewer,reviewer-2 $C
approve reviewer 2 $C; inboxm
approve reviewer-2 3 $C; inboxm
sleep 4; x $H wait --for implementer --timeout 2s
alerts
git merge -q --no-ff -m "merge w2" w2; inboxm

echo; echo "## M3. Three reviewers, no go-ahead: the operator approves one while another's CHANGES stands; then a new round"
new m3; C=$(work w3)
post W3 reviewer,reviewer-2,reviewer-3 $C
approve reviewer 1 $C; approve reviewer-3 3 $C
$H reply --as reviewer-2 2 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep -E "^posted" | tidy
op decide 1 --approve
inboxm
echo "-- the fix, as a new round, approved by all three under a go-ahead:"
C2=$(work w3b)
post W3 reviewer,reviewer-2,reviewer-3 $C2 "--supersedes 1,2,3"
q python3 $AT $H away "carry on"
approve reviewer 6 $C2; approve reviewer-2 7 $C2; approve reviewer-3 8 $C2
inboxm

echo; echo "## M4. A squash merge, and handoff merged"
new m4; C=$(work w4)
post W4 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide W4 --approve
git merge -q --squash w4 2>/dev/null; git commit -q --allow-empty -m "squash w4"; SQ=$(git rev-parse HEAD)
inboxm
x $H note --as implementer --to operator --item W4 --title "merged it" --body ../body.md
inboxm | head -3
x $H merged --as reviewer W4 $SQ
x $H merged --as implementer W4 $C
x $H merged --as implementer W9 $SQ
x $H merged --as implementer W4 $SQ
inboxm
x $H merged --as implementer W4 $SQ
echo "\$ handoff   (the operator's view, after)"; $H 2>&1 | grep -A2 "recorded as merged" | tidy

echo; echo "## M5. The operator holds approved work"
new m5; C=$(work w5)
post W5 reviewer,reviewer-2 $C; q python3 $AT $H away "carry on"
approve reviewer 1 $C; approve reviewer-2 2 $C; q python3 $AT $H back
q $H wait --for implementer --timeout 1s
inboxm
op decide W5 --approve
op decide W5 --deny
inboxm
x $H wait --for implementer --timeout 2s
op decide W5 --deny
x $H merged --as implementer W5 main

echo; echo "## M6. git can't tell"
QUIET=2s new m6; C=$(work w6)
post W6 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide W6 --approve
python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['main']='trunk';json.dump(c,open(p,'w'),indent=2)"
echo "-- the config now names a main branch that isn't there:"
inboxm
sleep 3; x $H wait --for implementer --timeout 2s
alerts
python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['main']='main';json.dump(c,open(p,'w'),indent=2)"
echo "-- the branch is back in the config; the approved commit's branch is deleted and pruned after a squash:"
git merge -q --squash w6 2>/dev/null; git commit -q --allow-empty -m "squash w6"; git branch -q -D w6; git reflog expire --expire=now --all; git gc -q --prune=now 2>/dev/null
inboxm
x $H merged --as implementer W6 main
inboxm

echo; echo "## M7. The upgrade: approvals before tracking began are history; a request posted before and approved after is tracked"
new m7 $OLD; C=$(work w7); C8=$(work w8)
echo "-- with the older build: W7 approved and not merged, W8 posted and approved by its reviewer, not yet decided"
$OLD post --as implementer --item W7 --to reviewer --commit $C --title W7 --body ../body.md | tidy; q $OLD reply --as reviewer 1 --verdict APPROVE --commit $C --body ../body.md; q python3 $AT $OLD decide W7 --approve
$OLD post --as implementer --item W8 --to reviewer --commit $C8 --title W8 --body ../body.md | tidy; q $OLD reply --as reviewer 4 --verdict APPROVE --commit $C8 --body ../body.md
ls .handoff/tracking.json 2>&1 | sed 's/^/  /'
echo "-- the new build opens the project:"
inboxm
echo "  tracking.json: $(cat .handoff/tracking.json)"
op decide W8 --approve
inboxm
echo "-- the older build still reads the record, with a hold and a merged entry in it:"
q python3 $AT $H decide W8 --deny
x $OLD inbox --for implementer --quiet
$OLD show 7 2>&1 | head -3 | tidy

echo; echo "## M8. Approved and not merged, then the requester posts again on the item"
new m8; C=$(work w9); C2=$(work w9b)
post W9 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide W9 --approve
inboxm | head -3
post W9 reviewer $C2
inboxm | head -3
$H reply --as reviewer 4 --verdict CHANGES --commit $C2 --body ../body.md 2>&1 | grep -E "^posted" | tidy
inboxm | head -3
