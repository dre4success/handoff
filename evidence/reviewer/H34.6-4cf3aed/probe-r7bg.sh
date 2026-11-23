#!/bin/zsh
# Reviewer's race probes for H34.6 round 7 (4cf3aed): a session's start hooks at once, as the settings run them
# (hook brief and hook listen together), after /background only (R3 of probe-r7race.sh). Usage: probe-r7bg.sh <a new, empty scratch dir> <handoff binary> <how many times>
D=$1; H=$2; N=${3:-40}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1" "$D/$1-t"; T="$D/$1-t"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; }
now() { python3 -c "import datetime;print(datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.%f')[:-3]+'Z')"; }
# both <case> <i> <session> <pid> <json> [flags]: brief, brief and listen at once; prints the first line of each
both() { local c=$1 i=$2 id=$3 pid=$4 json=$5; shift 5
  ( echo "$json" | CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid $H hook brief "$@" > "$D/$c-a$i.out" 2>&1 ) & A=$!
  ( echo "$json" | CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid $H hook listen --timeout 1s "$@" > "$D/$c-l$i.out" 2>&1; echo "exit $?" >> "$D/$c-l$i.out" ) & L=$!
  ( echo "$json" | CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid $H hook brief "$@" > "$D/$c-b$i.out" 2>&1 ) & B=$!
  wait $A $L $B; }

echo "## R3. /background from a session that holds reviewer: the three hooks at once, $N times, each from the last"
new r3; sleep 900 & P=$!
CLAUDE_CODE_SESSION_ID=b0 CLAUDE_PID=$P $H role reviewer >/dev/null
bad=0
for i in $(seq 1 $N); do
  j=$((i-1)); sleep 0.02
  print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"b$i\",\"timestamp\":\"$(now)\"}" >> $T/b$j.jsonl; : >> $T/b$i.jsonl
  kill $P 2>/dev/null; sleep 0.1; sleep 900 & P=$!
  both r3 $i b$i $P "{\"session_id\":\"b$i\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/b$i.jsonl\"}"
  a=$(head -1 "$D/r3-a$i.out"); b=$(head -1 "$D/r3-b$i.out"); l=$(grep -v '^exit' "$D/r3-l$i.out" | head -1); x=$(tail -1 "$D/r3-l$i.out")
  if [[ "$a" != "handoff brief: you are reviewer"* || "$b" != "handoff brief: you are reviewer"* || "$l" != *"nothing new for reviewer"* || "$x" != "exit 2" ]]; then bad=$((bad+1)); echo "  /background $i: brief: ${a[1,60]} | brief: ${b[1,60]} | listen: ${l[1,60]} ($x)"; fi
done
echo "  $N moves: $bad where a hook did not have the role"
echo "  bindings with a role now: $(grep -l '^{"v":1,"role":"reviewer"' .handoff/sessions/*.json | wc -l | tr -d ' '); marked cut: $(grep -l '"cut":true' .handoff/sessions/*.json 2>/dev/null | wc -l | tr -d ' ')"
kill $P 2>/dev/null
