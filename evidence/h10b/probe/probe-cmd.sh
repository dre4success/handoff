#!/bin/sh
echo "CODEX_SANDBOX=${CODEX_SANDBOX:-unset}"
if touch "$HOME/.handoff-sandbox-probe" 2>/dev/null; then echo "write outside the workspace: allowed"; rm -f "$HOME/.handoff-sandbox-probe"; else echo "write outside the workspace: denied"; fi
