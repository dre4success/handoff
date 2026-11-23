#!/bin/zsh
# Reviewer's probe Y for H34.6 round 5 (3043862): reviewer-2's #422. Usage: probe-r5y.sh <a new, empty scratch dir> <handoff binary>
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

hooklisten() { local id=$1 pid=$2; shift 2; echo "\$ ($id, Stop hook) handoff hook listen $* --timeout 3s   (the wait for the old process cut to 300 ms)"; echo "{\"session_id\":\"$id\",\"hook_event_name\":\"Stop\",\"transcript_path\":\"$T/$id.jsonl\"}" | HANDOFF_TEST_CONTINUED_WAIT=300ms CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid $H hook listen "$@" --timeout 3s 2>&1 | head -2 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-150; echo "  [exit ${pipestatus[2]}]"; }
echo "## Y. A marked session goes on by /background and its old process is still running when the new id's hook gives up waiting"
new y; pids X Z Y N I
r X $PX $H role reviewer; r Z $PZ $H role reviewer; r I $PI $H role implementer
sleep 0.05; print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"Y\",\"timestamp\":\"$(now)\"}" >> $T/X.jsonl; : >> $T/Y.jsonl
echo "-- X's transcript names Y; X's process still runs. A note for the reviewer (Z, not listening now) is waiting:"
r I $PI $H note --to reviewer --item P1 --title "for the reviewer, one" --body ../body.md
echo "-- with the usual hook, no fixed role:"
hooklisten Y $PY
show
echo "-- with a hook that fixes the role:"
hooklisten Y $PY --as reviewer
show
note Y $PY reviewer
echo "-- for comparison, a new session with no history, same hook:"
r I $PI $H note --to reviewer --item P1 --title "for the reviewer, two" --body ../body.md
hooklisten N $PN --as reviewer
echo "-- X's process ends; Y's next hook:"
kill $PX; sleep 0.3
hooklisten Y $PY --as reviewer
show
note Y $PY reviewer
kill $PZ $PY $PN $PI 2>/dev/null
