#!/bin/zsh
# Reviewer's probes S1 to S3 for H34.1 round 3 (3388ed7): reviewers asked in separate posts. Usage: probe-s.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without row 108>
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

echo "## S1. Two reviewers asked in separate posts at one commit; one asks for changes, the operator approves the other; the fix goes on at another commit"
QUIET=2s new s1; C=$(work w1); C2=$(work w1b)
post W1 reviewer-2 $C; post W1 reviewer $C
$H reply --as reviewer-2 1 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
approve reviewer 2 $C
q python3 $AT $H decide 2 --approve
inboxm
echo "-- the fix, to the reviewer who asked for it, at another commit:"
post W1 reviewer-2 $C2 "--supersedes 1"
inboxm
sleep 3; q $H wait --for reviewer-3 --timeout 2s; alerts

echo; echo "## S1b. The same, the two posts in the other order"
new s1b; C=$(work w1); C2=$(work w1b)
post W1 reviewer $C; post W1 reviewer-2 $C
$H reply --as reviewer-2 2 --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
approve reviewer 1 $C
q python3 $AT $H decide 1 --approve
post W1 reviewer-2 $C2 "--supersedes 2"
inboxm

echo; echo "## S2. Two reviewers asked in separate posts, both approve; the operator approves one, then denies the other"
new s2; C=$(work w1)
post W1 reviewer-2 $C; post W1 reviewer $C
approve reviewer-2 1 $C; approve reviewer 2 $C
q python3 $AT $H decide 2 --approve
op decide 1 --deny | tail -1
inboxm

echo; echo "## S3. The same two posts as one round, for comparison"
new s3; C=$(work w1)
post W1 reviewer-2,reviewer $C
approve reviewer-2 1 $C; approve reviewer 2 $C
q python3 $AT $H decide 2 --approve
op decide 1 --deny | tail -1
inboxm
