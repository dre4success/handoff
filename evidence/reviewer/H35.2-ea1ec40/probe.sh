#!/bin/zsh
# Reviewer's probes for H35.2 (ad2285d): cover at away, the stuck check, the spare's wake.
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
    if e.get('kind')=='away': print('  away entry #%d: cover'%e['id'], ('session %s, generation …%s' % (e['cover']['session'], str(e['cover']['gen'])[-6:])) if e.get('cover') else 'none')"; }
gen() { python3 -c "import json;b=json.load(open('.handoff/sessions/$1.json'));print('  binding $1: role', b.get('role') or '-', '| generation …'+str(b.get('gen'))[-6:])"; }

echo "## G1. away and the cover question"
new g1; proc P
echo "-- no spare:"; op "" away "one"; cover; q python3 $AT "" $H back
q S sp $P $H role spare; gen sp
echo "-- a spare, and yes:"; op y away "two"; cover; q python3 $AT "" $H back
echo "-- a spare, and no:"; op n away "three"; cover
echo "-- away again while away, with yes:"; op y away "four"; cover; q python3 $AT "" $H back
echo "-- approved work waits for a decision, so away asks of that first: yes, then yes"
q $H post --as implementer --item W1 --to reviewer --commit $C --title W1 --body ../body.md; q $H reply --as reviewer 9 --verdict APPROVE --commit $C --body ../body.md
$H 2>&1 | grep -c "W1" | sed 's/^/  lines naming W1 in handoff: /'
op yy away "five"; cover; q python3 $AT "" $H back
echo "-- the spare's process has ended:"; kill $P; sleep 0.3; op "" away "six"; cover; q python3 $AT "" $H back

echo; echo "## G2. A spare that can't be woken, at away"
new g2
q env ANTIGRAVITY_CONVERSATION_ID=G $H role spare
print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s >/dev/null 2>&1
echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy
op y away "one"; cover

echo; echo "## G3. An older build reads a record with cover in it"
new g3; proc P; q S sp $P $H role spare; away y; cover
echo "\$ (older build) handoff inbox --for implementer --quiet"; $OLD inbox --for implementer --quiet 2>&1 | head -2 | tidy; echo "  [exit ${pipestatus[1]}]"
echo "\$ (older build) handoff show 1"; $OLD show 1 2>&1 | head -3 | tidy
echo "\$ (older build) handoff note …, then this build reads it"; $OLD note --as reviewer --to implementer --item P1 --title t --body ../body.md 2>&1 | head -1 | tidy; $H inbox --for implementer --quiet; echo "  [this build, inbox --quiet: exit $?]"
kill $P 2>/dev/null

echo; echo "## G4. The exact signs: a reviewer whose process ended, with a request waiting"
new g4; proc P; proc PR; q S sp $P $H role spare; q S rv $PR $H role reviewer
away y; post W1 reviewer
echo "-- at once (work has waited under quiet_after), the reviewer alive:"; spw sp $P
kill $PR; sleep 0.3
echo "-- its process ended, still under quiet_after:"; spw sp $P
sleep 4; echo "-- after quiet_after:"; spw sp $P
echo "-- again: woken once only"; spw sp $P
echo "  alerts to the operator that name a stuck seat or the spare: $(grep -ci "stuck\|spare" "$HANDOFF_TEST_NOTIFY")"
kill $P 2>/dev/null

echo; echo "## G5. What doesn't make a seat stuck"
new g5; proc P; proc PR; proc PI; q S sp $P $H role spare; q S rv $PR $H role reviewer; q S im $PI $H role implementer
away y; kill $PR; sleep 4.5
echo "-- the reviewer's process ended, and only the operator's fyi (the away entry) waits for it:"; spw sp $P
post W1 reviewer; ( $H wait --for reviewer --timeout 8s >/dev/null 2>&1 ) & WP=$!; sleep 0.5
echo "-- a request waits... but something listens for reviewer (it takes it):"; spw sp $P; wait $WP
q python3 $AT "" $H back; post W2 reviewer; sleep 4.5
echo "-- the operator is back; a request has waited past quiet_after for the dead reviewer:"; spw sp $P
away n; echo "-- away again, cover refused:"; spw sp $P
kill $P $PI 2>/dev/null

echo; echo "## G6. Silence: a reviewer seat at twice quiet_after, the implementer seat never"
new g6; proc P; proc PR; proc PI; q S sp $P $H role spare; q S rv $PR $H role reviewer; q S im $PI $H role implementer
away y; post W1 reviewer; q $H reply --as reviewer 5 --verdict CHANGES --commit $C --body ../body.md
echo "  (#5 waits for reviewer; the CHANGES #6 waits for implementer; both sessions alive, neither listening)"
sleep 4.5; echo "-- after quiet_after:"; spw sp $P
q S rv $PR $H inbox --quiet; sleep 4; echo "-- after twice quiet_after, but the reviewer's session ran a command 4 s ago:"; spw sp $P
sleep 4.5; echo "-- 8 s after that command:"; spw sp $P
echo "-- the implementer, as silent for as long:"; spw sp $P
kill $PI; sleep 0.3; echo "-- the implementer's process ends (an exact sign):"; spw sp $P
kill $P $PR 2>/dev/null

echo; echo "## G7. A permission prompt: a reviewer seat at twice quiet_after"
new g7; proc P; proc PR; q S sp $P $H role spare; q S rv $PR $H role reviewer
away y; post W1 reviewer
print -r -- '{"session_id":"rv","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' | S rv $PR $H hook notify >/dev/null 2>&1
sleep 4.5; echo "-- after quiet_after:"; spw sp $P
sleep 4; echo "-- after twice quiet_after:"; spw sp $P
kill $P $PR 2>/dev/null

echo; echo "## G8. Antigravity's cap: any seat at quiet_after; not while its own background wait runs"
new g8; proc P; q S sp $P $H role spare
q env ANTIGRAVITY_CONVERSATION_ID=A $H role implementer
away y; q $H post --as implementer --item W1 --to reviewer --commit $C --title W1 --body ../body.md; q $H reply --as reviewer 4 --verdict CHANGES --commit $C --body ../body.md
print -r -- "{\"conversationId\":\"A\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s >/dev/null 2>&1
sleep 4.5
( $H wait --for implementer --timeout 3s >/dev/null 2>&1 ) & WP=$!; sleep 0.3
echo "-- at the cap, past quiet_after, while a wait for implementer runs (it takes the CHANGES):"; spw sp $P; wait $WP
q $H reply --as reviewer 4 --verdict CHANGES --commit $C --body ../body.md 2>/dev/null; q $H note --as reviewer --to implementer --item W1 --title more --body ../body.md
sleep 4.5; echo "-- a note has waited past quiet_after, nothing listens, the cap's block stands:"; spw sp $P
kill $P 2>/dev/null

echo; echo "## G9. Which spare is woken: by the generation the away entry names"
new g9; proc P; proc PR; proc P2; q S sp $P $H role spare; q S rv $PR $H role reviewer
away y; cover; post W1 reviewer; kill $PR; sleep 4.5
print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | S sp1 $P $H hook brief >/dev/null 2>&1; gen sp1
echo "-- the spare after /clear, by its Stop hook:"
echo "\$ (sp1, Stop hook) handoff hook listen --timeout 2s"; print -r -- '{"session_id":"sp1","hook_event_name":"Stop"}' | S sp1 $P $H hook listen --timeout 2s 2>&1 | tidy; echo "  [exit ${pipestatus[2]}]"
echo "-- a second stuck seat: reviewer-2's process ends with a request waiting"
proc PR2; q S rv2 $PR2 $H role reviewer-2; post W2 reviewer-2; kill $PR2; sleep 4.5; spw sp1 $P
echo "-- the spare is given to another session after away:"
q S other $P2 $H role spare; gen other; rm -f .handoff/notified/spare-* 2>/dev/null; echo "  (my probe removed the wake's once-only keys here, so only the generation can stop the wake)"
spw other $P2
echo "\$ handoff   (the \"sessions\" part)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy
kill $P $P2 2>/dev/null

echo; echo "## G10. An Antigravity spare, its hook with the id on stdin only"
new g10; proc PR
q env ANTIGRAVITY_CONVERSATION_ID=G $H role spare; q S rv $PR $H role reviewer
away y; post W1 reviewer; kill $PR; sleep 4.5
echo "\$ (G, Stop hook, stdin only, executionNum 2) handoff hook listen --timeout 2s"; print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":2,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 2s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try: d=json.loads(t); print('   ', d.get('decision'), '|', (d.get('reason') or '').replace('\n',' / ')[:330])
except Exception: print('   ', t[:300])" | tidy
