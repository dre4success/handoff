#!/bin/zsh
# Reviewer's probes for H35.2 (ad2285d), second set: the parts of probe.sh that used a wrong entry id, done again, and more.
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
cover() { python3 -c "
import json
for l in open('.handoff/channel.jsonl'):
    e=json.loads(l)
    if e.get('kind')=='away': print('  away entry #%d: cover'%e['id'], ('session %s' % e['cover']['session']) if e.get('cover') else 'none')"; }
last() { python3 -c "
import json
for l in open('.handoff/channel.jsonl'): e=json.loads(l)
print(e['id'])"; }
# reqto <item> <role>: post a request, print its line, and set R to its id
reqto() { post $1 $2; R=$(last); }

echo "## H1. The implementer seat (G6 again, with the CHANGES really posted)"
new h1; proc P; proc PR; proc PI; q S sp $P $H role spare; q S rv $PR $H role reviewer; q S im $PI $H role implementer
away y; reqto W1 reviewer; $H reply --as reviewer $R --verdict CHANGES --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
( $H wait --for reviewer --timeout 30s >/dev/null 2>&1 ) & WR=$!   # reviewer listens, so only the implementer's seat is in question
sleep 9; echo "-- the CHANGES has waited past twice quiet_after; the implementer's session is alive, silent, not listening:"; spw sp $P
print -r -- '{"session_id":"im","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' | S im $PI $H hook notify >/dev/null 2>&1
echo "-- and now at a permission prompt:"; spw sp $P
( $H wait --for implementer --timeout 30s >/dev/null 2>&1 ) & WI=$!; sleep 0.3
kill $PI; sleep 0.3; echo "-- its process ends, but something listens for implementer (my probe's own wait, which took the CHANGES):"; spw sp $P
kill $WI 2>/dev/null; pkill -f "wait --for implementer --timeout 30s"; sleep 0.3
q $H note --as reviewer --to implementer --item W1 --title more --body ../body.md; sleep 4.5
echo "-- a note has waited past quiet_after for the implementer, whose process has ended:"; spw sp $P
pkill -f "wait --for reviewer --timeout 30s"; kill $P $PR 2>/dev/null

echo; echo "## H2. away's two questions, in order (G1 again, with the APPROVE really posted)"
new h2; proc P; q S sp $P $H role spare
reqto W1 reviewer; $H reply --as reviewer $R --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep "^posted" | tidy
echo "-- no to the first:"; op n away "one" ; cover
echo "-- yes, then no:"; op yn away "two"; cover; q python3 $AT "" $H back
reqto W2 reviewer; q $H reply --as reviewer $R --verdict APPROVE --commit $C --body ../body.md
echo "-- yes, then yes:"; op yy away "three"; cover
kill $P 2>/dev/null

echo; echo "## H3. Where every role reviews: silence takes no seat, an exact sign does"
new h3 a,b,c; python3 -c "
import json;p='.handoff/config.json';c=json.load(open(p))
for r in c['roles']: r['reviews']=True
json.dump(c,open(p,'w'),indent=2)"
proc P; proc PB; q S sp $P $H role spare; q S sb $PB $H role b
away y; $H post --as a --item W1 --to b --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy
sleep 9; echo "-- b alive and silent past twice quiet_after:"; spw sp $P
kill $PB; sleep 0.3; echo "-- b's process ends:"; spw sp $P
kill $P 2>/dev/null

echo; echo "## H4. The timer alone: the spare's wait is already running when a seat becomes stuck, and nothing is written meanwhile"
new h4; proc P; proc PR; q S sp $P $H role spare; q S rv $PR $H role reviewer
away y; reqto W1 reviewer; kill $PR
T0=$(python3 -c "import time;print(time.time())")
echo "\$ (sp, the spare) handoff wait --timeout 100s   (started at once; the request has waited 0 s)"
r=$(S sp $P $H wait --timeout 100s 2>&1); x=$?; echo "$r" | grep -v "told the operator" | tidy; echo "  [exit $x, $(python3 -c "import time;print(round(time.time()-$T0))") s after the request; quiet_after is 4 s and the check runs once a minute]"
kill $P 2>/dev/null

echo; echo "## H5. A project with a role already named spare (older build's init), at away"
new h5 implementer,reviewer; python3 -c "
import json;p='.handoff/config.json';c=json.load(open(p));c['roles'].append({'name':'spare','reviews':True});json.dump(c,open(p,'w'),indent=2)"
proc P; S x $P $H role spare 2>&1 | head -1 | tidy
op "" away "one"; cover
kill $P 2>/dev/null
