#!/bin/zsh
# Reviewer's probes N to R for H34.6 round 4 (6f1b853). Usage: probe-r4.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1" "$D/$1-t"; T="$D/$1-t"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; }
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));l=b.get('lost') or {};print(('role '+b['role'] if b.get('role') else 'lost '+l['role']+(' went '+l['went']+' to '+l['to'] if l.get('went') else ' to '+l['to']))+(', gone' if b.get('gone') else '')+(', was '+'+'.join(b['was']) if b.get('was') else ''))")"; done)"; }
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

echo "## O. A role goes on by /background twice; the old ids are resumed; the operator gives the role on, then back to the first id"
new o; pids X Y C Z X2 Y2
r X $PX $H role reviewer
bg X $PX Y $PY; bg Y $PY C $PC
show
note C $PC reviewer; note X $PX2 reviewer; note Y $PY2 reviewer
r Z $PZ $H role reviewer
show
note C $PC reviewer; note X $PX2 reviewer
echo "-- the operator gives X the role again, in X:"
r X $PX2 $H role reviewer
show
note X $PX2 reviewer; note Z $PZ reviewer; note Y $PY2 reviewer
kill $PX $PY $PC $PZ $PX2 $PY2 2>/dev/null

echo; echo "## P. A role goes on by /clear twice in one process; the old ids are resumed elsewhere"
new p; pids A A1 B1
r X $PA $H role reviewer
clr Y $PA; clr Y2 $PA
show
note Y2 $PA reviewer; note X $PA1 reviewer; note Y $PB1 reviewer
kill $PA $PA1 $PB1 2>/dev/null

echo; echo "## Q1. /background, then the operator gives the new id its role again (or another), then /background again"
new q1; pids X Y Y2
r X $PX $H role reviewer
bg X $PX Y $PY
echo "-- the operator says \"you're reviewer\" in Y again, and Y runs it:"
r Y $PY $H role reviewer
show
bg Y $PY Y2 $PY2
show
r Y2 $PY2 $H role
stop Y2 $PY2
kill $PX $PY $PY2 2>/dev/null

echo; echo "## Q2. A marked session goes on by /background, the operator gives the new id a role, then /background again"
new q2; pids X Z Y Y2
r X $PX $H role reviewer; r Z $PZ $H role reviewer
bg X $PX Y $PY
r Y $PY $H role implementer
show
bg Y $PY Y2 $PY2
show
r Y2 $PY2 $H role
kill $PX $PZ $PY $PY2 2>/dev/null

echo; echo "## Q0. For comparison: /background twice with no handoff role between"
new q0; pids X Y Y2
r X $PX $H role reviewer
bg X $PX Y $PY; bg Y $PY Y2 $PY2
show
r Y2 $PY2 $H role
kill $PX $PY $PY2 2>/dev/null

echo; echo "## N. A finished move, then the new id's binding is dropped (another conversation in its process is given a role); the new id is resumed"
new n; pids X Y Y3
r X $PX $H role reviewer
bg X $PX Y $PY
echo "-- a new conversation W in Y's process (as after a /clear whose hook didn't run) is given implementer:"
r W $PY $H role implementer
show
echo "-- Y is resumed by its id, in a new process; its Stop hook runs:"
r Y $PY3 $H role
stop Y $PY3
show
r Y $PY3 $H role
kill $PX $PY $PY3 2>/dev/null

echo; echo "## R. A resumed marked session does /clear in its new process"
new rr; pids X Z X2
r X $PX $H role reviewer; r Z $PZ $H role reviewer
kill $PX; sleep 0.2
echo "-- X resumed in a new process, refused; then /clear there:"
note X $PX2 reviewer
clr X9 $PX2
show
note X9 $PX2 reviewer
kill $PZ $PX2 2>/dev/null
