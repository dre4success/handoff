#!/bin/zsh
# Reviewer's probe for H35.1 round 2 (ad210fc): reviewer-2's #817 point 2, run again. Usage: probe-e.sh <a new, empty scratch dir> <handoff binary>
D=$1; H=$2
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p $D/v; cd $D/v; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; sleep 900 & P=$!
CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H role spare >/dev/null 2>&1
echo "## V1. In the spare's own session, wait --for spare with --as spare or HANDOFF_ROLE=spare"
echo "\$ (sp) handoff wait --for spare --as spare --timeout 1s"; CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --for spare --as spare --timeout 1s 2>&1 | head -1 | cut -c1-200; echo "  [exit ${pipestatus[1]}]"
echo "\$ (sp) HANDOFF_ROLE=spare handoff wait --for spare --timeout 1s"; HANDOFF_ROLE=spare CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --for spare --timeout 1s 2>&1 | head -1 | cut -c1-200; echo "  [exit ${pipestatus[1]}]"
echo "\$ (sp) handoff wait --as spare --timeout 1s   (no --for)"; CLAUDE_CODE_SESSION_ID=sp CLAUDE_PID=$P $H wait --as spare --timeout 1s 2>&1 | head -1 | sed -E "s#/private/tmp/claude-501/[^ \`']*/handoff#handoff#g" | cut -c1-200; echo "  [exit ${pipestatus[1]}]"
kill $P 2>/dev/null
