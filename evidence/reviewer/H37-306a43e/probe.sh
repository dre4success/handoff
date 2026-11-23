#!/bin/zsh
# Reviewer's probe for H37 (306a43e): who gets the new line in the brief, by each way a brief is given.
# Usage: probe.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles $2 --main main >/dev/null; }
has() { local n; n=$(grep -c "^- With two or more reviewers on one commit, wait for every verdict" ); echo "  $1: ${${n/0/no}/1/yes}"; }
echo "## B1. handoff brief --for <role>, by project"
for roles in implementer,reviewer implementer,reviewer,reviewer-2 implementer,reviewer,reviewer-2,reviewer-3; do
  new "p-${roles//,/-}" $roles; echo "-- roles: $roles"
  for r in ${(s:,:)roles}; do $H brief --for $r 2>&1 | has "$r"; done
done
echo "-- a second reviewer added later (roles add reviewer-2), in the one-reviewer project:"
cd $D/p-implementer-reviewer; $H roles add reviewer-2 >/dev/null 2>&1; $H brief --for implementer 2>&1 | has implementer
echo; echo "## B2. Where the line sits (the three-reviewer project, the implementer's brief)"
cd $D/p-implementer-reviewer-reviewer-2-reviewer-3; $H brief --for implementer 2>&1 | grep -n "^- " | cut -c1-110
echo; echo "## B3. The other ways a brief is given, to the implementer of the three-reviewer project"
sleep 900 & P=$!
CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$P $H role implementer 2>&1 | has "handoff role implementer"
print -r -- '{"session_id":"im","hook_event_name":"SessionStart","source":"startup"}' | CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$P $H hook brief 2>&1 | has "its SessionStart hook (Claude Code)"
print -r -- '{"session_id":"im","hook_event_name":"SessionStart","source":"compact"}' | CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$P $H hook brief 2>&1 | has "its SessionStart hook after a compaction"
CLAUDE_CODE_SESSION_ID=im CLAUDE_PID=$P $H brief 2>&1 | has "handoff brief, in its session"
kill $P 2>/dev/null
ANTIGRAVITY_CONVERSATION_ID=G $H role reviewer-3 >/dev/null 2>&1
print -r -- "{\"conversationId\":\"G\",\"hookEventName\":\"SessionStart\",\"cwd\":\"$PWD\"}" | env -u ANTIGRAVITY_CONVERSATION_ID $H hook brief 2>&1 | python3 -c "
import sys,json
print(json.loads(sys.stdin.read())['injectSteps'][0]['systemMessage']['systemMessage'])" | has "an Antigravity reviewer-3's start hook"
echo; echo "## B4. A project where every role reviews (made by editing the scratch config)"
for roles in a,b a,b,c; do new "all-${roles//,/-}" $roles
  python3 -c "
import json;p='.handoff/config.json';c=json.load(open(p))
for r in c['roles']: r['reviews']=True
json.dump(c,open(p,'w'),indent=2)"
  echo "-- roles: $roles, all reviewing"; for r in ${(s:,:)roles}; do $H brief --for $r 2>&1 | has "$r"; done
done
