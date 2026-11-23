#!/bin/zsh
# Reviewer's probes T1 to T5 for H34.1 round 4 (e6a56f9). Usage: probe-t.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without row 108>
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

echo "## T1. Separate posts, each approved by the operator: one line, both reviewers"
new t1; C=$(work w1)
post W1 reviewer-2 $C; post W1 reviewer $C
approve reviewer-2 1 $C; approve reviewer 2 $C
q python3 $AT $H decide 2 --approve; inboxm | head -3
q python3 $AT $H decide 1 --approve; inboxm

echo; echo "## T2. Separate posts approved together under a go-ahead: one line, both reviewers, one alert"
QUIET=2s new t2; C=$(work w1)
post W1 reviewer-2 $C; post W1 reviewer $C
q python3 $AT $H away "carry on"
approve reviewer-2 1 $C; approve reviewer 2 $C
inboxm; sleep 3; q $H wait --for reviewer-3 --timeout 2s; alerts

echo; echo "## T3. A second reviewer asked after the first request was approved, at the same commit"
new t3; C=$(work w1)
post W1 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide 1 --approve
inboxm | head -3
post W1 reviewer-2 $C
inboxm | head -3
$H reply --as reviewer-2 4 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
inboxm | head -3
echo "-- reviewer-2's request is withdrawn by a fix at another commit:"
C2=$(work w1b); post W1 reviewer-2 $C2 "--supersedes 4"
inboxm | head -3

echo; echo "## T4. Denied, then asked again at the same commit and approved"
new t4; C=$(work w1)
post W1 reviewer $C; approve reviewer 1 $C; q python3 $AT $H decide 1 --deny
echo two > ../body2.md
$H post --as implementer --item W1 --to reviewer --commit $C --title "W1 again" --body ../body2.md 2>&1 | grep "^posted" | tidy
approve reviewer 4 $C; q python3 $AT $H decide 4 --approve
inboxm

echo; echo "## T5. Hold one of two approvals on separate items; merged on the other"
new t5; C=$(work w1); C2=$(work w2)
post W1 reviewer $C; post W2 reviewer $C2
q python3 $AT $H away "carry on"; approve reviewer 1 $C; approve reviewer 2 $C2; q python3 $AT $H back
inboxm
q python3 $AT $H decide W1 --deny
git merge -q --no-ff -m "merge w2" w2
inboxm
