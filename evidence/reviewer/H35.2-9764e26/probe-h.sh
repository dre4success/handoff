#!/bin/zsh
# Reviewer's probes for H35.2 round 2 (ea1ec40): H35.2 round 4 (9764e26): the spare takes a stuck seat with handoff role. Is the old session shut out, and is anything lost?
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


seats() { python3 -c "
import json,glob
for p in sorted(glob.glob('.handoff/sessions/*.json')):
    b=json.load(open(p)); l=b.get('lost') or {}
    print('  session', b.get('session'), ': role', b.get('role') or '-', ('| lost '+l['role']+((' to '+l['to']) if l.get('to') else '')) if l.get('role') else '')"; }
alerts() { echo "  alerts to the operator that name the spare: $(grep -ci 'the spare took' "$HANDOFF_TEST_NOTIFY")"; grep -i "the spare took" "$HANDOFF_TEST_NOTIFY" | tidy | sed 's/^/    /'; }
one() { "$@" 2>&1 | grep -v '^$' | head -1 | tidy; }

echo "## P1. A silent reviewer, alive: the spare is woken and takes the seat"
new p1; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer; R=$(last)
sleep 8.5; spw sp $P
( S rv $PR $H wait --timeout 30s > $D/p1-old.out 2>&1; echo "exit $?" >> $D/p1-old.out ) & WO=$!; sleep 0.5
echo "  (the old reviewer's session comes to life: its own wait starts, and takes what waited)"; wait $WO; echo "  the old reviewer's wait: $(head -1 $D/p1-old.out | tidy) | $(tail -1 $D/p1-old.out)"
( S rv $PR $H wait --timeout 30s > $D/p1-old2.out 2>&1; echo "exit $?" >> $D/p1-old2.out ) & WO=$!; sleep 0.5
echo "\$ (sp, the spare) handoff role reviewer"; S sp $P $H role reviewer 2>&1 | head -1 | tidy
wait $WO; echo "  the old reviewer's second wait, running when the seat moved: $(tr '\n' '|' < $D/p1-old2.out | tidy)"
seats; alerts
echo "\$ (rv, the old session) handoff claim $R"; one S rv $PR $H claim $R
echo "\$ (rv, the old session) handoff reply $R --verdict APPROVE"; one S rv $PR $H reply $R --verdict APPROVE --commit $C --body ../body.md
echo "\$ (sp, now reviewer) handoff inbox   (needs my review)"; S sp $P $H inbox 2>&1 | grep -A1 "needs my review" | tail -1 | tidy
echo "\$ (sp, now reviewer) handoff claim $R; handoff reply $R --verdict APPROVE"; one S sp $P $H claim $R; S sp $P $H reply $R --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep "^posted\|approved under" | tidy
echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy
kill $P $PR 2>/dev/null

echo; echo "## P2. The old reviewer's reply is held after its role was resolved; the spare takes the seat; the reply is released"
new p2; mkdir -p $D/p2-p; PZ=$D/p2-p; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer; R=$(last)
( HANDOFF_TEST_PAUSE=write=$PZ S rv $PR $H reply $R --verdict APPROVE --commit $C --body ../body.md > $D/p2-reply.out 2>&1; echo "exit $?" >> $D/p2-reply.out ) & WR=$!
untilf $PZ/write.paused && echo "  (the old reviewer's reply is held, its role already resolved)"
echo "\$ (sp, the spare) handoff role reviewer"; S sp $P $H role reviewer 2>&1 | head -1 | tidy
touch $PZ/write.go; wait $WR
echo "  the old reviewer's reply, after: $(tr '\n' '|' < $D/p2-reply.out | tidy)"
echo "  verdicts in the record on #$R: $(python3 -c "
import json
print(sum(1 for l in open('.handoff/channel.jsonl') if json.loads(l).get('kind')=='reply'))")"
kill $P $PR 2>/dev/null

echo; echo "## P3. The implementer's process has ended with a CHANGES owed: the spare takes the implementer seat"
new p3; proc P; proc PI; must S sp $P $H role spare; must S im $PI $H role implementer; away y
S im $PI $H post --item W1 --to reviewer --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy; R=$(last)
must $H reply --as reviewer $R --verdict CHANGES --commit $C --body ../body.md
kill $PI; sleep 4.5; spw sp $P
echo "\$ (sp, the spare) handoff role implementer"; S sp $P $H role implementer 2>&1 | head -1 | tidy
alerts
echo "\$ (sp, now implementer) handoff inbox   (needs my response)"; S sp $P $H inbox 2>&1 | grep -A1 "needs my response" | tail -1 | tidy
proc PI2; echo "-- the old implementer's session is resumed in a new process:"
echo "\$ (im) handoff inbox"; one S im $PI2 $H inbox
echo "\$ (im) handoff post … --supersedes $R"; one S im $PI2 $H post --item W1 --to reviewer --commit $C --title again --body ../body.md --supersedes $R
kill $P $PI2 2>/dev/null

echo; echo "## P4. handoff role <seat> by the spare when nothing is stuck, and when the operator isn't away"
new p4; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer
echo "-- the operator is here:"; echo "\$ (sp, the spare) handoff role reviewer-2"; S sp $P $H role reviewer-2 2>&1 | head -1 | tidy; alerts
proc P2; must S sp2 $P2 $H role spare; away y
echo "-- away with cover, no seat stuck:"; echo "\$ (sp2, the spare) handoff role reviewer"; S sp2 $P2 $H role reviewer 2>&1 | head -1 | tidy; alerts; seats
kill $P $P2 $PR 2>/dev/null
