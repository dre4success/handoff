#!/bin/zsh
# Reviewer's probes for H38 (dc109a9): can a withdrawal make an item approved without the operator, and what a withdrawn request leaves behind.
# Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm.py> <the build of main it is based on (22791ca)>
D=$1; H=$2; AT=$3; OLD=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2 --main main >/dev/null; echo body > ../body.md; : > "$HANDOFF_TEST_NOTIFY"
  git checkout -q -b work && git commit -q --allow-empty -m work && C=$(git rev-parse HEAD) && git checkout -q main; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff[-a-z]*#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g" | cut -c1-${W:-250}; }
last() { python3 -c "
import json
for l in open('.handoff/channel.jsonl'): e=json.loads(l)
print(e['id'])"; }
# run <label...>: a command whose result matters: its first lines and exit
run() { echo "\$ ${*//$H/handoff}" | sed -E 's#(--body) [^ ]*#\1 f#' | cut -c1-210; "$@" 2>&1 | grep -v '^$' | head -${N:-3} | tidy; echo "  [exit ${pipestatus[1]}]"; }
# set up, loudly: prints the posted line or the refusal
do_() { "$@" 2>&1 | grep -E "^posted|^handoff" | grep -v "told the operator\|asking the operator" | head -2 | tidy; }
round() { do_ $H post --as implementer --item $1 --to $2 --commit $C --title $1 --body ../body.md ${=3}; }
op() { local a=$1; shift; echo "\$ (the operator, at a terminal, answering ${a:-nothing}) handoff $*"; python3 $AT "$a" $H "$@" 2>&1 | grep -v '^$' | grep -v "^told \|^not told\|isn.t listening now\|doesn.t read your words\|^Under it\|No spare\|no agent has a session" | tidy | sed 's/^/    /'; }
alerts() { echo "  alerts to the operator:"; grep -v "Your go-ahead is recorded\|hasn't picked up\|hasn't been given" "$HANDOFF_TEST_NOTIFY" | tidy | sed 's/^/    /'; : > "$HANDOFF_TEST_NOTIFY"; }
sect() { awk -v t="$1:" '$0==t{f=1} f&&/^$/{exit} f{print}' | tidy; }
tomerge() { r=$($H inbox --for implementer 2>&1 | sect "approved, to merge"); echo "  implementer's \"approved, to merge\": ${r:-(none)}" | tr '\n' ' '; echo; }

echo "## A. Away. A round to two reviewers; reviewer-2 asks for changes; its request is withdrawn; the reviewer approves"
new a; op "" away "carry on" >/dev/null
round W1 reviewer,reviewer-2; R1=$(( $(last) - 1 )); R2=$(last); echo "  (requests #$R1 to reviewer, #$R2 to reviewer-2)"
do_ $H reply --as reviewer-2 $R2 --verdict CHANGES --commit $C --body ../body.md
N=4 run $H withdraw --as implementer $R2 --note "reviewer-2 is out"
do_ $H reply --as reviewer $R1 --verdict APPROVE --commit $C --body ../body.md
alerts; tomerge
echo "-- the operator decides it:"; op y decide $R1 --approve; tomerge
echo "\$ handoff show $R2   (its state)"; $H show $R2 2>&1 | grep -i "^state\|withdrawn" | tidy

echo; echo "## B. Away. The reviewer approves first; then reviewer-2's open request is withdrawn"
new b; op "" away "carry on" >/dev/null
round W1 reviewer,reviewer-2; R1=$(( $(last) - 1 )); R2=$(last)
do_ $H reply --as reviewer $R1 --verdict APPROVE --commit $C --body ../body.md
N=4 run $H withdraw --as implementer $R2
alerts; tomerge
echo "-- reviewer-2 is asked again on the item, and approves:"
round W1 reviewer-2; R3=$(last); do_ $H reply --as reviewer-2 $R3 --verdict APPROVE --commit $C --body ../body.md
alerts; tomerge

echo; echo "## C. Away. Two separate requests on one item (no round); reviewer-2's is withdrawn; the reviewer approves"
new c; op "" away "carry on" >/dev/null
round W1 reviewer; R1=$(last); round W1 reviewer-2; R2=$(last)
do_ $H withdraw --as implementer $R2
do_ $H reply --as reviewer $R1 --verdict APPROVE --commit $C --body ../body.md
alerts; tomerge

echo; echo "## D. Away. After a withdrawal, a new round to the reviewer alone replaces its request; it approves"
new d; op "" away "carry on" >/dev/null
round W1 reviewer,reviewer-2; R1=$(( $(last) - 1 )); R2=$(last)
do_ $H reply --as reviewer-2 $R2 --verdict CHANGES --commit $C --body ../body.md
do_ $H withdraw --as implementer $R2
round W1 reviewer "--supersedes $R1"; R3=$(last)
do_ $H reply --as reviewer $R3 --verdict APPROVE --commit $C --body ../body.md
alerts; tomerge

echo; echo "## E. Not away. The withdrawal comes first, the approval next, and then the operator runs away"
new e
round W1 reviewer,reviewer-2; R1=$(( $(last) - 1 )); R2=$(last)
do_ $H reply --as reviewer-2 $R2 --verdict CHANGES --commit $C --body ../body.md
do_ $H withdraw --as implementer $R2
do_ $H reply --as reviewer $R1 --verdict APPROVE --commit $C --body ../body.md
alerts
op n away "carry on"; tomerge

echo; echo "## F. A verdict on a withdrawn request, and who may withdraw"
new f
round W1 reviewer; R1=$(last)
run $H withdraw --as reviewer $R1
run $H withdraw --as implementer $R1
run $H withdraw --as implementer $R1
run $H reply --as reviewer $R1 --verdict APPROVE --commit $C --body ../body.md
alerts
echo "\$ (the operator, at a terminal) handoff decide $R1 --approve"; python3 $AT y $H decide $R1 --approve 2>&1 | grep -v '^$' | head -2 | tidy | sed 's/^/    /'
tomerge
echo "\$ handoff inbox --for reviewer --quiet"; $H inbox --for reviewer --quiet; echo "  [exit $?]"
round W2 reviewer; R2=$(last); do_ $H reply --as reviewer $R2 --verdict APPROVE --commit $C --body ../body.md
run $H withdraw --as implementer $R2
echo "\$ (the build of main, without H38) handoff inbox --for reviewer   (needs my review)"; $OLD inbox --for reviewer 2>&1 | grep -A1 "needs my review" | tail -1 | tidy; $OLD inbox --for implementer --quiet; echo "  [the older build reads the record: exit $?]"
