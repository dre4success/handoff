#!/bin/zsh
# H34 probe: a session whose role was taken by another can still act as that role with --as.
# Usage: shutout.sh <scratch dir> [handoff binary]
export HANDOFF_TEST_NOTIFY="$1/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
H=${2:-handoff}; P="$1/proj"; rm -rf "$P"; mkdir -p "$P"; cd "$P"
git init -q && git commit -q --allow-empty -m init && git switch -q -c work && git commit -q --allow-empty -m "work"
echo "\$ $H init"; $H init >/dev/null && echo ok
sleep 30 & OLD=$!; sleep 30 & NEW=$!
echo "\$ (session old, pid $OLD) $H role implementer"; CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$OLD $H role implementer | head -1
echo "\$ (session new, pid $NEW) $H role implementer"; CLAUDE_CODE_SESSION_ID=new CLAUDE_PID=$NEW $H role implementer | head -1
echo "\$ ls .handoff/sessions"; ls .handoff/sessions
echo "body" > ../body.md
echo "\$ (session old) $H post --as implementer --item P1 --to reviewer --commit work ..."
CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$OLD $H post --as implementer --item P1 --to reviewer --commit work --title "from the old session" --body ../body.md; echo "exit $?"
echo "\$ (session old) $H role"; CLAUDE_CODE_SESSION_ID=old CLAUDE_PID=$OLD $H role; echo "exit $?"
kill $OLD $NEW 2>/dev/null
