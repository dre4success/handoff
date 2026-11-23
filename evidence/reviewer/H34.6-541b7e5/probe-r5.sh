#!/bin/zsh
# Reviewer's probes T to X for H34.6 round 5 (3043862). Usage: probe-r5.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1" "$D/$1-t"; T="$D/$1-t"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; }
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));l=b.get('lost') or {};print(('role '+b['role'] if b.get('role') else 'lost '+l['role']+(' CUT' if l.get('cut') else '')+(' went '+l['went']+' to '+l['to'] if l.get('went') else ' to '+l['to']))+(', gone' if b.get('gone') else '')+(', was '+'+'.join(b['was']) if b.get('was') else ''))")"; done)"; }
r() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}"; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -E '^(handoff|posted|You are|#[0-9])' | grep -v "^handoff brief" | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-175; echo "  [exit ${pipestatus[1]}]"; }
now() { python3 -c "import datetime;print(datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.%f')[:-3]+'Z')"; }
# bg <from> <its pid> <to> <its pid>: /background. The old transcript names the new id, the old process ends, the new session's SessionStart hook runs.
bg() { sleep 0.05; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"$3\",\"timestamp\":\"$(now)\"}" >> $T/$1.jsonl; : >> $T/$3.jsonl; kill $2 2>/dev/null; sleep 0.3
  echo "\$ (/background: $1 goes on as $3; $3's SessionStart hook)"; echo "{\"session_id\":\"$3\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/$3.jsonl\"}" | CLAUDE_CODE_SESSION_ID=$3 CLAUDE_PID=$4 $H hook brief 2>&1 | head -1 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-110; }
# clr <new id> <pid>: /clear in a process. The new conversation's SessionStart hook runs.
clr() { echo "\$ (/clear: a new conversation $1 in the same process; its SessionStart hook)"; echo "{\"session_id\":\"$1\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}" | CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 $H hook brief 2>&1 | head -1 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-110; }
stop() { echo "\$ ($1, Stop hook) handoff hook listen --timeout 1s"; echo "{\"session_id\":\"$1\",\"hook_event_name\":\"Stop\",\"transcript_path\":\"$T/$1.jsonl\"}" | CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 $H hook listen --timeout 1s 2>&1 | head -2 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-150; echo "  [exit ${pipestatus[2]}]"; }
note() { r $1 $2 $H note --as $3 --to reviewer-3 --item P1 --title "from $1 as $3" --body ../body.md; }
pids() { for n in "$@"; do sleep 300 & eval "P$n=$!"; done; }

# as bg and clr, but the move is killed between its two writes
bgcut() { sleep 0.05; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"$3\",\"timestamp\":\"$(now)\"}" >> $T/$1.jsonl; : >> $T/$3.jsonl; kill $2 2>/dev/null; sleep 0.3
  echo "\$ (/background: $1 goes on as $3; $3's SessionStart hook is killed between the move's two writes)"; echo "{\"session_id\":\"$3\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/$3.jsonl\"}" | HANDOFF_TEST_FAULT=move-between CLAUDE_CODE_SESSION_ID=$3 CLAUDE_PID=$4 $H hook brief >/dev/null 2>&1; echo "  [exit $?]"; }
clrcut() { echo "\$ (/clear: a new conversation $1; its SessionStart hook is killed between the move's two writes)"; echo "{\"session_id\":\"$1\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}" | HANDOFF_TEST_FAULT=move-between CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 $H hook brief >/dev/null 2>&1; echo "  [exit $?]"; }

echo "## T1. A role's move by /background is cut short"
new t1; pids X Y Z X2
r X $PX $H role reviewer
bgcut X $PX Y $PY
show
echo "-- before any hook of Y runs again, Y has no file:"
note Y $PY reviewer
stop Y $PY
show
note Y $PY reviewer
note X $PX2 reviewer
echo "-- a sender is told the role is free:"
r Z $PZ $H role implementer
r Z $PZ $H note --to reviewer --item P1 --title "for the reviewer" --body ../body.md
echo "-- the operator gives Y the role again:"
r Y $PY $H role reviewer
show
note Y $PY reviewer
kill $PX $PY $PZ $PX2 2>/dev/null

echo; echo "## T2. A role's move by /clear is cut short; the operator gives the role elsewhere, and it falls free; the new id's hooks run"
new t2; pids A Z
r X $PA $H role reviewer
clrcut Y $PA
show
r Z $PZ $H role reviewer
r Z $PZ $H role implementer
show
stop Y $PA
clr Y $PA
show
note Y $PA reviewer
kill $PA $PZ 2>/dev/null

echo; echo "## T3. A mark's move by /background is cut short"
new t3; pids X Z Y X2
r X $PX $H role reviewer; r Z $PZ $H role reviewer
bgcut X $PX Y $PY
show
note Y $PY reviewer; note X $PX2 reviewer
kill $PX $PZ $PY $PX2 2>/dev/null

echo; echo "## W. The old id is resumed and goes on by /background again; then the new id goes on too"
new w; pids X Y X2 X3 Y2
r X $PX $H role reviewer
bg X $PX Y $PY
echo "-- X is resumed (marked), and goes on as X3:"
bg X $PX2 X3 $PX3
show
note X3 $PX3 reviewer
echo "-- Y, which holds the role, goes on as Y2:"
bg Y $PY Y2 $PY2
show
note Y2 $PY2 reviewer
kill $PX $PY $PX2 $PX3 $PY2 2>/dev/null

echo; echo "## X. Three moves by /background, the role given again in each new id"
new x; pids A B C E
r A $PA $H role reviewer
bg A $PA B $PB; r B $PB $H role reviewer
bg B $PB C $PC; r C $PC $H role reviewer
bg C $PC E $PE
show
note E $PE reviewer; note C $PC reviewer
kill $PA $PB $PC $PE 2>/dev/null

echo; echo "## V. A hook with a fixed role (--as reviewer): the role, not marked, goes on by /clear and by /background"
new v; pids A B
r X $PA $H role reviewer
echo "\$ (/clear: a new conversation Y; its SessionStart hook is: handoff hook brief --as reviewer)"; echo "{\"session_id\":\"Y\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}" | CLAUDE_CODE_SESSION_ID=Y CLAUDE_PID=$PA $H hook brief --as reviewer 2>&1 | head -1 | cut -c1-80
show
sleep 0.05; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"Y2\",\"timestamp\":\"$(now)\"}" >> $T/Y.jsonl; : >> $T/Y2.jsonl; kill $PA; sleep 0.3
echo "\$ (/background: Y goes on as Y2; its SessionStart hook is: handoff hook brief --as reviewer)"; echo "{\"session_id\":\"Y2\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/Y2.jsonl\"}" | CLAUDE_CODE_SESSION_ID=Y2 CLAUDE_PID=$PB $H hook brief --as reviewer 2>&1 | head -1 | cut -c1-80
show
echo "-- a session with no binding and no mark still gets the fixed role:"
echo "\$ (N, SessionStart) handoff hook brief --as reviewer"; echo "{\"session_id\":\"N\",\"hook_event_name\":\"SessionStart\",\"source\":\"startup\"}" | CLAUDE_CODE_SESSION_ID=N CLAUDE_PID=$$ $H hook brief --as reviewer 2>&1 | head -1 | cut -c1-80
show
kill $PA $PB 2>/dev/null
