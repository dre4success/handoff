#!/bin/zsh
# Reviewer's probe M: a session that holds a role goes on by /background, and the old id is resumed. Usage: as probe-388.sh
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p "$D/m" "$D/m-t"; cd "$D/m"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; T="$D/m-t"
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));print(('role '+b['role'] if b.get('role') else 'lost '+b['lost']['role'])+(', gone' if b.get('gone') else ''))")"; done)"; }
r() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}"; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -E '^(handoff|posted|You are|#[0-9])' | cut -c1-200; echo "  [exit ${pipestatus[1]}]"; }
echo "## M. A session with a role, not marked, goes on by /background; its old id is resumed"
sleep 60 & PX=$!; sleep 60 & PY=$!; sleep 60 & PX2=$!
r X $PX $H role reviewer
sleep 1.2 # the continued-in line must not be older than the role
print -r -- "{\"type\":\"continued-in\",\"continuedInSessionId\":\"Y\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%S.000Z)\"}" > $T/X.jsonl; : > $T/Y.jsonl
kill $PX; sleep 0.3
echo "{\"session_id\":\"Y\",\"hook_event_name\":\"SessionStart\",\"source\":\"fork\",\"transcript_path\":\"$T/Y.jsonl\"}" | CLAUDE_CODE_SESSION_ID=Y CLAUDE_PID=$PY $H hook brief 2>&1 | head -1 | cut -c1-90
show
r Y $PY $H role
echo "-- the operator resumes X by its id:"
r X $PX2 $H role
r X $PX2 $H note --as reviewer --to implementer --item P1 --title "from X, resumed; Y has its role" --body ../body.md
kill $PY $PX2 2>/dev/null
