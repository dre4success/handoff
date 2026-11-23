#!/bin/zsh
# Reviewer's probes H and I for H34.6 round 2 (cd71bcf). Usage: probe-hi.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-3 --main main >/dev/null; echo body > ../body.md; }
show() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json;b=json.load(open('$f'));print(('role '+b['role'] if b.get('role') else 'lost '+b['lost']['role'])+(', gone' if b.get('gone') else '')+(', pid' if b.get('pid') else ', no pid'))")"; done)"; }
r() { local id=$1 pid=$2; shift 2; echo "\$ ($id) ${*//$H/handoff}"; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@" 2>&1 | grep -E '^(handoff|posted|You are|#[0-9])' | grep -v "^handoff brief" | sed -E 's/at 20[0-9T:-]+Z/at TIME/' | cut -c1-250; echo "  [exit ${pipestatus[1]}]"; }

echo "## H. A session that lost its role runs wait, with and without --for"
new h
sleep 90 & PO=$!; sleep 90 & PN=$!
r old $PO $H role reviewer; r new $PN $H role reviewer
$H note --as implementer --to reviewer --item P1 --title "for the reviewer" --body ../body.md 2>&1 | grep -v "no session"
r old $PO $H wait --timeout 2s
r old $PO $H wait --for reviewer --timeout 2s
r old $PO $H wait --for implementer --timeout 2s
r new $PN $H wait --timeout 2s
r old $PO $H inbox --for reviewer --quiet
kill $PO $PN 2>/dev/null

echo; echo "## I. A session whose process ended, kept and not given on: it is resumed under its id and still has its role"
new i
sleep 90 & PA=$!; sleep 90 & PB=$!; sleep 90 & PA2=$!
r A $PA $H role reviewer-3
kill $PA; sleep 0.3
r B $PB $H role implementer
show
echo "-- while A is gone, a sender is told so:"
r B $PB $H note --to reviewer-3 --item P1 --title one --body ../body.md
echo "-- A is resumed, with a new process:"
r A $PA2 $H role
r A $PA2 $H note --to implementer --item P1 --title "from A, resumed" --body ../body.md
show
echo "-- its first hook notes the new process:"
echo "\$ (A, Stop hook) handoff hook listen --timeout 2s"; echo '{"session_id":"A","hook_event_name":"Stop"}' | CLAUDE_PID=$PA2 $H hook listen --timeout 2s 2>&1 | cut -c1-200; echo "  [exit ${pipestatus[2]}]"
show
r B $PB $H note --to reviewer-3 --item P1 --title two --body ../body.md
kill $PB $PA2 2>/dev/null
