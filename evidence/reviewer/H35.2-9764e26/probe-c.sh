#!/bin/zsh
# Reviewer's probes for H35.2 (ad2285d), third set: a seat something listens for is not stuck.
# Usage: probe.sh <a new, empty scratch dir> <handoff binary> <atterm.py> <an older handoff binary, without H35>
# quiet_after is 4 s in these projects, so "twice quiet_after" is 8 s.
D=$1; H=$2; AT=$3; OLD=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles ${2:-implementer,reviewer,reviewer-2} --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"
  python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='${QUIET:-4s}';json.dump(c,open(p,'w'),indent=2)"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g; s/since [0-9]{2}:[0-9]{2}/since HH:MM/g; s/for [0-9]+ seconds?/for N seconds/g" | cut -c1-${W:-260}; }
q() { "$@" >/dev/null 2>&1; }
S() { local id=$1 pid=$2; shift 2; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@"; }       # a command of a Claude Code session
proc() { sleep 900 & eval "$1=$!"; }
post() { $H post --as implementer --item $1 --to $2 --commit $C --title $1 --body ../body.md 2>&1 | grep "^posted" | tidy; }
op() { local a=$1; shift; echo "\$ (the operator, at a terminal, answering ${a:-nothing}) handoff $*"; python3 $AT "$a" $H "$@" 2>&1 | grep -v '^$' | grep -v "^told \|^not told\|isn.t listening now\|doesn.t read your words\|^Under it" | tidy | sed 's/^/    /'; }
away() { q python3 $AT "${1:-y}" $H away "carry on"; }
# spw <id> <pid>: the spare's listener, for 2 s: it checks at once when it starts
spw() { echo "\$ ($1, the spare) handoff wait --timeout 2s"; r=$(S $1 $2 $H wait --timeout 2s 2>&1); x=$?; echo "${r:-  (nothing)}" | tidy; echo "  [exit $x]"; }
echo "## J1. The reviewer's process has ended and a request is owed, past quiet_after: stuck only when nothing listens for reviewer"
new j1; proc P; proc PR; q S sp $P $H role spare; q S rv $PR $H role reviewer
away y; post W1 reviewer
q $H wait --for reviewer --timeout 2s     # a listener takes the request and the fyi, and exits: the request is still owed (not claimed)
kill $PR; sleep 4.5
( $H wait --for reviewer --timeout 20s >/dev/null 2>&1 ) & WR=$!; sleep 0.5
echo "  listening for reviewer now: $([[ -n $(pgrep -f 'wait --for reviewer --timeout 20s') ]] && echo yes || echo no)"
echo "-- while a wait for reviewer runs (it has nothing new to take, so it stays):"; spw sp $P
pkill -f "wait --for reviewer --timeout 20s"; sleep 0.5
echo "  listening for reviewer now: $([[ -n $(pgrep -f 'wait --for reviewer --timeout 20s') ]] && echo yes || echo no)"
echo "-- that wait gone:"; spw sp $P
kill $P 2>/dev/null
