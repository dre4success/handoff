#!/bin/zsh
# Reviewer's probes for H34.6 (458a36b). Usage: probe.sh <scratch dir> <handoff binary> <atterm.py>
D=$1; H=$2; AT=$3
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE HANDOFF_TEST_FAULT HANDOFF_TEST_WRITE_DELAY HANDOFF_TEST_DELIVER_DELAY
PIDS=()
new() { P="$D/$1"; rm -rf "$P"; mkdir -p "$P"; cd "$P"; : > "$HANDOFF_TEST_NOTIFY"
  git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m x
  $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo other > ../body2.md }
proc() { sleep 120 & PIDS+=($!); eval "$1=$!"; }            # a stand-in for a session's process
as() { local id=$1 pid=$2; shift 2; CLAUDE_CODE_SESSION_ID=$id CLAUDE_PID=$pid "$@"; }
run() { local who=$1 pid=$2; shift 2; echo "\$ ($who) ${*//$H/handoff}"; as $who $pid "$@" 2>&1 | grep -v "no session has" | sed -E 's/at 20[0-9T:-]+Z/at TIME/'; echo "  [exit ${pipestatus[1]}]"; }
files() { echo "  .handoff/sessions: $(for f in .handoff/sessions/*.json(N); do printf '%s[%s] ' ${f:t:r} "$(python3 -c "import json,sys;b=json.load(open('$f'));print('role '+b['role'] if b.get('role') else 'lost '+b['lost']['role'])")"; done)"; }

echo "## A. The role moves from one running session to another"
new a; proc PO; proc PN
run old $PO $H role implementer
run new $PN $H role implementer
files
run old $PO $H post --as implementer --item P1 --to reviewer --commit w1 --title t --body ../body.md
run old $PO $H note --as reviewer-2 --to implementer --item P1 --body ../body.md
run old $PO $H role
run old $PO $H
run old $PO $H inbox --for implementer --quiet
echo "\$ (old) handoff show / inbox --for stay open: $(as old $PO $H inbox --for reviewer >/dev/null 2>&1; echo "inbox --for exit $?")"
run old $PO $H role reviewer-2
files
run old $PO $H note --to implementer --item P1 --body ../body.md

echo; echo "## B. The old session's process has ended when the role moves; it is resumed under the same id"
new b; proc PA; proc PC; proc PA2
run A $PA $H role reviewer-3
kill $PA; sleep 0.3
run C $PC $H role reviewer-3
files
run A $PA2 $H note --as reviewer-3 --to implementer --item P1 --body ../body.md

echo; echo "## C. As B, but another session takes another role in between"
new c; proc PA; proc PB; proc PC; proc PA2
run A $PA $H role reviewer-3
kill $PA; sleep 0.3
run B $PB $H role implementer
files
run C $PC $H role reviewer-3
files
echo "-- A is resumed under the same id, with the context it had:"
run A $PA2 $H note --as reviewer-3 --to implementer --item P1 --body ../body.md
run A $PA2 $H role

echo; echo "## D. A write that began before the role moved"
new d; proc PO; proc PN
run old $PO $H role implementer
echo "\$ (old) HANDOFF_TEST_WRITE_DELAY=3s handoff post ... &   then, one second later:"
( HANDOFF_TEST_WRITE_DELAY=3s CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$PO $H post --as implementer --item P1 --to reviewer --commit w1 --title t --body ../body.md > ../d.out 2>&1; echo "  [exit $?]" >> ../d.out ) &
sleep 1; run new $PN $H role implementer; wait %1 2>/dev/null; sleep 2.5
echo "  the old session's post said:"; sed 's/^/    /' ../d.out | grep -v "no session has" | sed -E 's/at 20[0-9T:-]+Z/at TIME/'
echo "  entries in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' ')"

echo; echo "## E. A delivery that began before the role moved"
new e; proc PO; proc PN
run old $PO $H role implementer
echo "\$ (old) HANDOFF_TEST_DELIVER_DELAY=3s handoff wait --for implementer --timeout 20s &"
( HANDOFF_TEST_DELIVER_DELAY=3s CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$PO $H wait --timeout 20s > ../e.out 2>&1; echo "  [exit $?]" >> ../e.out ) &
sleep 1
$H note --as reviewer --to implementer --item P1 --title "for the implementer" --body ../body.md 2>&1 | grep -v "no session has"
sleep 1; run new $PN $H role implementer
sleep 4; echo "  the old session's wait said:"; sed 's/^/    /' ../e.out | sed -E 's/at 20[0-9T:-]+Z/at TIME/'
run new $PN $H wait --timeout 2s

echo; echo "## F. The fence refuses at a skipped fyi, after a delivery in the same pass"
new f; proc PZ; proc PO; proc PN
run Z $PZ $H role implementer
$H note --as reviewer --to implementer --item P1 --title "first, not yet delivered" --body ../body.md 2>&1 | grep -v "no session has"
echo "\$ handoff fyi \"hello\"   (the operator, at a terminal; it names Z, who holds implementer)"; python3 $AT $H fyi "hello" | head -2
run old $PO $H role implementer
echo "\$ (old) HANDOFF_TEST_DELIVER_DELAY=3s handoff wait --timeout 20s &   (#1 is delivered at about 3 s; the fyi for Z is reached then; the role moves at about 4.5 s)"
( HANDOFF_TEST_DELIVER_DELAY=3s CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$PO $H wait --timeout 20s > ../f.out 2>&1; echo "  [exit $?]" >> ../f.out ) &
sleep 4.5; run new $PN $H role implementer
sleep 3; echo "  the old session's wait said:"; sed 's/^/    /' ../f.out | sed -E 's/at 20[0-9T:-]+Z/at TIME/'
echo "  the role's cursor: $(cat .handoff/cursors/implementer 2>/dev/null || ls .handoff/cursors)"
run new $PN $H wait --timeout 2s
kill $PIDS 2>/dev/null
