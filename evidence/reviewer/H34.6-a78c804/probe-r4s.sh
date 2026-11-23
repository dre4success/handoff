#!/bin/zsh
# Reviewer's probe S for H34.6 round 4: reviewer-2's #408 point 2. Usage: as probe-r4.sh
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p "$D/s"; cd "$D/s"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));l=b.get('lost') or {};print(('role '+b['role'] if b.get('role') else 'lost '+l['role']+' to '+l['to'])+(', gone' if b.get('gone') else ''))")"; done)"; }
r() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}"; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -E '^(handoff|posted|You are|#[0-9])' | grep -v "^handoff brief" | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-175; echo "  [exit ${pipestatus[1]}]"; }
clr() { echo "\$ (/clear: a new conversation $1 in the same process; its SessionStart hook is: handoff hook brief $3 $4)"; echo "{\"session_id\":\"$1\",\"hook_event_name\":\"SessionStart\",\"source\":\"clear\"}" | CLAUDE_CODE_SESSION_ID=$1 CLAUDE_PID=$2 $H hook brief $3 $4 2>&1 | head -1 | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-110; }
echo "## S. A marked session does /clear, where the project's hooks fix a role (--as reviewer)"
sleep 120 & PX=$!; sleep 120 & PZ=$!
r X $PX $H role reviewer; r Z $PZ $H role reviewer
clr Y $PX --as reviewer
show
r Y $PX $H note --as reviewer --to reviewer-3 --item P1 --title "from Y, after /clear of a marked session" --body ../body.md
echo "-- the same with the usual hook, no --as, in another process:"
sleep 120 & PA=$!; sleep 120 & PB=$!
r A $PA $H role implementer; r B $PB $H role implementer
clr A2 $PA
show
r A2 $PA $H note --as implementer --to reviewer-3 --item P1 --title "from A2" --body ../body.md
kill $PX $PZ $PA $PB 2>/dev/null
