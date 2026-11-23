#!/bin/zsh
# Reviewer's probes for H35.2 round 2 (ea1ec40): reviewer-2's #862 point 1, run again: the operator comes back while the wake is held.
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
last() { python3 -c "
import json
for l in open('.handoff/channel.jsonl'): e=json.loads(l)
print(e['id'])"; }
must() { "$@" >/dev/null 2>&1 || echo "  !! my probe's setup step failed: ${*//$H/handoff}" | cut -c1-200; }
untilf() { local i=0; until [[ -e $1 ]] || (( i++ > 400 )); do sleep 0.05; done; [[ -e $1 ]]; }
keys() { echo "  wake keys marked sent: $(ls .handoff/notified 2>/dev/null | grep -c '^spare-')"; }
# stuck <dir>: a project away with cover, the spare sp (process P), and reviewer's process ended with a request owed past quiet_after
stuck() { new $1; mkdir -p $D/$1-p; PZ=$D/$1-p; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer; kill $PR; sleep 4.5; }

echo "## N1. The spare's wait is held at its delivery, about to give the wake; the operator runs back; the wait is released"
stuck n1
( HANDOFF_TEST_PAUSE=deliver=$PZ S sp $P $H wait --timeout 20s > $D/n1.out 2>&1; echo "exit $?" >> $D/n1.out ) & WP=$!
untilf $PZ/deliver.paused && echo "  (the wait is held, about to give the wake)"; keys
echo "\$ (the operator, at a terminal) handoff back"; python3 $AT "" $H back 2>&1 | grep "^back\|go-ahead" | head -2 | tidy | sed 's/^/    /'
touch $PZ/deliver.go; wait $WP
echo "  the spare's wait, after: $(grep -v 'told the operator' $D/n1.out | tr '\n' '|' | tidy)"; keys
kill $P 2>/dev/null
