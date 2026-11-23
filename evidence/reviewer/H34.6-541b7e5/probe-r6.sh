#!/bin/zsh
# Reviewer's probe Z for H34.6 round 6 (541b7e5). Usage: probe-r6.sh <a new, empty scratch dir> <handoff binary>
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

echo "## Z. In a cut move's window, before any hook: what the new id may do, and the operator's way back"
new z; pids A
r X $PA $H role reviewer
clrcut Y $PA
show
r Y $PA $H role
r Y $PA $H
r Y $PA $H wait --for reviewer --timeout 1s
r Y $PA $H inbox --for reviewer --quiet
echo "-- the operator says \"you're reviewer\" in Y, before any hook of Y has run:"
r Y $PA $H role reviewer
show
note Y $PA reviewer
stop Y $PA
kill $PA 2>/dev/null
