#!/bin/zsh
# Reviewer's probes for H35.1 round 3 (741cc66): a wait whose session becomes the spare, the spare in handoff's own view, and fyi's line.
# Usage: probe-f.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1" "$D/$1-p"; PZ="$D/$1-p"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2 --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD); : > "$HANDOFF_TEST_NOTIFY"
  python3 -c "import json;p='.handoff/config.json';c=json.load(open(p));c['quiet_after']='${QUIET:-30m}';json.dump(c,open(p,'w'),indent=2)"; }
tidy() { sed -E "s#$H#handoff#g; s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g; s/20[0-9]{2}-[0-9T:.-]+Z/TIME/g; s/since [0-9]{2}:[0-9]{2}/since HH:MM/g" | cut -c1-${W:-250}; }
q() { "$@" >/dev/null 2>&1; }
post() { $H post --as implementer --item $1 --to reviewer --commit $C --title $1 --body ../body.md 2>&1 | grep "^posted" | tidy; }
cursor() { echo "  reviewer's cursor: $(cat .handoff/cursors/reviewer.json 2>/dev/null || echo none)"; }
sessions() { echo "\$ handoff   (the \"sessions\" part)"; r=$($H 2>&1 | awk '/^sessions:/{f=1} f&&/^$/{exit} f{print}' | tidy); echo "${r:-  (no such part)}"; }
untilf() { local i=0; until [[ -e $1 ]] || (( i++ > 200 )); do sleep 0.05; done; [[ -e $1 ]]; }

echo "## F1. reviewer-2's #817 point 1: a wait of a session with no role, held before a delivery; the session becomes the spare"
new f1; sleep 900 & P=$!
( HANDOFF_TEST_PAUSE=deliver=$PZ CLAUDE_CODE_SESSION_ID=q CLAUDE_PID=$P $H wait --for reviewer --timeout 20s > $D/f1-wait.out 2>&1; echo "exit $?" >> $D/f1-wait.out ) & WP=$!
sleep 0.5; post W1
untilf $PZ/deliver.paused && echo "  (the wait is held at its delivery)"
CLAUDE_CODE_SESSION_ID=q CLAUDE_PID=$P $H role spare 2>&1 | head -1 | tidy
touch $PZ/deliver.go; wait $WP
echo "  the wait, after: $(tr '\n' '|' < $D/f1-wait.out | tidy)"; cursor
echo "\$ handoff wait --for reviewer --timeout 2s   (another listener)"; $H wait --for reviewer --timeout 2s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[1]}]"
kill $P 2>/dev/null

echo; echo "## F2. The same with nothing held: the wait is idle when its session becomes the spare"
new f2; sleep 900 & P=$!
( CLAUDE_CODE_SESSION_ID=q CLAUDE_PID=$P $H wait --for reviewer --timeout 30s > $D/f2-wait.out 2>&1; echo "exit $?" >> $D/f2-wait.out ) & WP=$!
sleep 1; T0=$(python3 -c "import time;print(time.time())")
CLAUDE_CODE_SESSION_ID=q CLAUDE_PID=$P $H role spare 2>&1 | head -1 | tidy
wait $WP; echo "  the wait ended $(python3 -c "import time;print(round(time.time()-$T0,1))") s after role spare: $(tr '\n' '|' < $D/f2-wait.out | tidy)"
post W2
echo "\$ handoff wait --for reviewer --timeout 2s   (another listener)"; $H wait --for reviewer --timeout 2s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[1]}]"
kill $P 2>/dev/null

echo; echo "## F3. The spare's own wait, when the operator gives its session a seat (role reviewer)"
new f3; sleep 900 & P=$!
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare
( CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --timeout 30s > $D/f3-wait.out 2>&1; echo "exit $?" >> $D/f3-wait.out ) & WP=$!
sleep 1; T0=$(python3 -c "import time;print(time.time())")
CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role reviewer 2>&1 | head -1 | tidy
wait $WP; echo "  the spare's wait ended $(python3 -c "import time;print(round(time.time()-$T0,1))") s after: $(tr '\n' '|' < $D/f3-wait.out | tidy)"
post W3
echo "\$ (sp, now reviewer) handoff wait --timeout 2s"; CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --timeout 2s 2>&1 | head -1 | tidy; echo "  [exit ${pipestatus[1]}]"
kill $P 2>/dev/null

echo; echo "## F4. The spare in handoff's own view"
QUIET=2s new f4; sleep 900 & P=$!
echo "-- no spare:"; sessions
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare
echo "-- a spare that has just been named:"; sessions
sleep 3; echo "-- after quiet_after (2 s here) with no listener:"; sessions
( CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --timeout 4s >/dev/null 2>&1 ) & WP=$!; sleep 1
echo "-- while its wait runs:"; sessions; wait $WP
echo "-- a permission prompt in it (Notification hook):"
print -r -- '{"session_id":"sp","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' | CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H hook notify 2>&1 | tidy
sessions
echo "-- its process ends:"; kill $P; sleep 0.3; sessions
echo "-- an Antigravity spare at its cap:"
q env ANTIGRAVITY_CONVERSATION_ID=G $H role spare
print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"Stop\",\"cwd\":\"$PWD\",\"executionNum\":10,\"fullyIdle\":true}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook listen --timeout 1s >/dev/null 2>&1
sessions
echo "  alerts to the operator ($(grep -c . "$HANDOFF_TEST_NOTIFY")):"; tidy < "$HANDOFF_TEST_NOTIFY" | sed 's/^/    /'

echo; echo "## F5. What fyi, away and back say of the spare"
new f5; sleep 900 & P=$!; sleep 900 & PI=$!
q env CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$PI $H role implementer
op() { echo "\$ (the operator, at a terminal) handoff $*"; python3 $AT $H "$@" 2>&1 | grep -v '^$' | grep -i "told\|spare" | tidy | sed 's/^/    /'; }
echo "-- no spare:"; op fyi "one"
q env CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare
echo "-- with a spare:"; op fyi "two"; op away "carry on"; op back
echo "  entries addressed to the spare: $(grep -c '"to":"spare"' .handoff/channel.jsonl)"
kill $P; sleep 0.3
echo "-- the spare's process has ended:"; op fyi "three"
kill $PI 2>/dev/null
