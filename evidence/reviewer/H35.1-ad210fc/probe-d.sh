#!/bin/zsh
# Reviewer's probe for H35.1 round 2 (ad210fc): U7 again beside what any other command does after the same cut move.
# Usage: probe-d.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
gen() { python3 -c "
import json,os
p='.handoff/sessions/$1.json'
if not os.path.exists(p): print('  binding $1: (no file)')
else:
    b=json.load(open(p)); l=b.get('lost') or {}
    print('  binding $1: role', b.get('role') or '-', '| gen', str(b.get('gen'))[-6:], '| lost', (l.get('role') or '-'), ('to '+l['to']) if l.get('to') else '', l.get('went') or '')"; }
for first in "inbox --quiet" "role" "role spare"; do
  d="$D/${first// /_}"; mkdir -p $d; cd $d; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; sleep 900 & P=$!
  CLAUDE_CODE_SESSION_ID=sp0 CLAUDE_PID=$P $H role spare >/dev/null 2>&1
  echo "## the spare's /clear is cut between its two writes; the new id's first command is: handoff $first"; gen sp0
  print -r -- '{"session_id":"sp1","hook_event_name":"SessionStart","source":"clear"}' | HANDOFF_TEST_FAULT=move-between CLAUDE_CODE_SESSION_ID=sp1 CLAUDE_PID=$P $H hook brief >/dev/null 2>&1
  echo "\$ (sp1) handoff $first"; CLAUDE_CODE_SESSION_ID=sp1 CLAUDE_PID=$P $H ${=first} 2>&1 | head -1 | cut -c1-160; echo "  [exit ${pipestatus[1]}]"
  gen sp1; kill $P 2>/dev/null; echo
done
