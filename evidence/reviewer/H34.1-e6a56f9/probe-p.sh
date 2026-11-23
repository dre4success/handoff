#!/bin/zsh
# Reviewer's probes P1, P3 for H34.1 round 2 (5ac3a2b): reviewer-2's #485 points 1 and 3. Usage: probe-p.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <an older handoff binary, without row 108>
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

echo "## P1. A round cut after its first request; the operator denies that request; the round is completed and its second request approved"
new p1; C=$(work w1)
HANDOFF_TEST_FAULT=first-line $H post --as implementer --item W1 --to reviewer,reviewer-2 --commit $C --title W1 --body ../body.md >/dev/null 2>&1
approve reviewer 1 $C
op decide 1 --deny | tail -2
post W1 reviewer,reviewer-2 $C
approve reviewer-2 4 $C
op decide 4 --approve | tail -3
inboxm

echo; echo "## P3. The work goes from commit A to commit B and back to A; A is then approved"
new p3; A=$(work wa); B=$(work wb); echo "back at A, with B dropped" > ../body2.md
post W1 reviewer $A
post W1 reviewer $B "--supersedes 1"
$H post --as implementer --item W1 --to reviewer --commit $A --title "W1, back at A" --body ../body2.md --supersedes 2 2>&1 | grep -E "^posted|^handoff" | tidy
approve reviewer 3 $A
op decide W1 --approve | tail -2
inboxm
echo "\$ handoff show 3 (head)"; $H show 3 2>&1 | sed -n '1,9p' | tidy
