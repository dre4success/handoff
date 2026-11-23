#!/bin/zsh
# Reviewer's probes for H35.2 round 2 (ea1ec40): the wake when the spare moves or the print fails, notes, and what away says.
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

echo "## L1. The old spare's wait is held at its delivery while the spare goes on by /clear (reviewer-2's #853 point 1)"
stuck l1
( HANDOFF_TEST_PAUSE=deliver=$PZ S sp $P $H wait --timeout 20s > $D/l1-old.out 2>&1; echo "exit $?" >> $D/l1-old.out ) & WP=$!
untilf $PZ/deliver.paused && echo "  (the old id's wait is held, about to give the wake)"; keys
print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | S sp1 $P $H hook brief 2>&1 | head -1 | tidy
touch $PZ/deliver.go; wait $WP
echo "  the old id's wait, after: $(tr '\n' '|' < $D/l1-old.out | tidy)"; keys
spw sp1 $P; keys
kill $P 2>/dev/null

echo; echo "## L2. The same, but the operator gives the spare to another session meanwhile"
stuck l2; proc P2
( HANDOFF_TEST_PAUSE=deliver=$PZ S sp $P $H wait --timeout 20s > $D/l2-old.out 2>&1; echo "exit $?" >> $D/l2-old.out ) & WP=$!
untilf $PZ/deliver.paused && echo "  (the old spare's wait is held, about to give the wake)"
S other $P2 $H role spare 2>&1 | head -1 | tidy
touch $PZ/deliver.go; wait $WP
echo "  the old spare's wait, after: $(tr '\n' '|' < $D/l2-old.out | tidy)"; keys
spw other $P2
kill $P $P2 2>/dev/null

echo; echo "## L3. The wake can't be printed (stdout closed): not counted as given"
stuck l3
echo "\$ (sp, the spare) handoff wait --timeout 2s >&-"; S sp $P $H wait --timeout 2s >&- 2> $D/l3.err; echo "  [exit $?] $(head -1 $D/l3.err | tidy)"; keys
spw sp $P; keys
kill $P 2>/dev/null

echo; echo "## L4. Notes: one answered by a reviewer that never listened, and one not answered"
new l4; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y
must $H note --as implementer --to reviewer --item P1 --title "a question" --body ../body.md; N=$(last)
S rv $PR $H note --to implementer --re $N --body ../body.md 2>&1 | grep "^posted" | tidy
echo "  reviewer's cursor: $(cat .handoff/cursors/reviewer.json 2>/dev/null || echo none)"
kill $PR; sleep 4.5
echo "-- the reviewer answered the note with note --re, never listened, and its process has ended:"; spw sp $P
kill $P 2>/dev/null

echo; echo "## L5. A fresh permission prompt on old work is no sign"
new l5; proc P; proc PR; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer
sleep 8.5; must S rv $PR $H inbox --quiet 2>/dev/null; sleep 0.1
print -r -- '{"session_id":"rv","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' | S rv $PR $H hook notify >/dev/null 2>&1
echo "-- the request is 8 s old (twice quiet_after); the prompt is new:"; spw sp $P
sleep 8.5; echo "-- the prompt has stood 8 s:"; spw sp $P
kill $P $PR 2>/dev/null

echo; echo "## L6. A Claude Code seat marked gone by a sweep (its process ended, then another role was given)"
new l6; proc P; proc PR; proc PX; must S sp $P $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer
kill $PR; sleep 0.3; must S x $PX $H role reviewer-2
python3 -c "import json;b=json.load(open('.handoff/sessions/rv.json'));print('  binding rv: role', b.get('role'), '| gone', b.get('gone', False), '| pid', 'yes' if b.get('pid') else 'no', '| cli', b.get('cli'))"
sleep 4.5; spw sp $P
kill $P $PX 2>/dev/null

echo; echo "## L7. What away says of the spare"
new l7
must env ANTIGRAVITY_CONVERSATION_ID=G $H role spare
echo "-- an Antigravity spare, listening as it should:"; op y away "one" | grep -i "spare\|cover\|note\|antigravity\|\\$"; q python3 $AT "" $H back
print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s >/dev/null 2>&1
echo "-- the same spare at its cap:"; op y away "two" | grep -i "spare\|cover\|note\|antigravity\|\\$"; cover; q python3 $AT "" $H back
new l7b; proc P; must S sp $P $H role spare
print -r -- '{"session_id":"sp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' | S sp $P $H hook notify >/dev/null 2>&1
echo "-- a Claude Code spare at a permission prompt:"; op n away "three" | grep -i "spare\|cover\|note\|\\$"; cover
kill $P 2>/dev/null
new l7c implementer,reviewer; python3 -c "
import json;p='.handoff/config.json';c=json.load(open(p));c['roles'].append({'name':'spare','reviews':True});json.dump(c,open(p,'w'),indent=2)"
echo "-- a project with a role already named spare:"; op "" away "four" | grep -i "spare\|cover\|away (\|\\$"

echo; echo "## L8. The wake by the hooks, as the spare reads it"
stuck l8
echo "\$ (sp, Stop hook) handoff hook listen --timeout 2s"; print -r -- '{"session_id":"sp","hook_event_name":"Stop"}' | S sp $P $H hook listen --timeout 2s 2>&1 | tidy; echo "  [exit ${pipestatus[2]}]"
kill $P 2>/dev/null
new l8b; proc PR; must env ANTIGRAVITY_CONVERSATION_ID=G $H role spare; must S rv $PR $H role reviewer; away y; post W1 reviewer; kill $PR; sleep 4.5
echo "\$ (G, Stop hook, stdin only, executionNum 2) handoff hook listen --timeout 2s"; print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":2,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 2s 2>&1 | python3 -c "
import sys,json
t=sys.stdin.read().strip()
try: d=json.loads(t); print('   ', d.get('decision'), '|', (d.get('reason') or '').replace('\n',' / ')[:400])
except Exception: print('   ', t[:300])" | tidy
echo "\$ handoff   (the \"sessions\" part, the spare's process... still there)"; $H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy
