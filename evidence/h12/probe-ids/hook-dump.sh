#!/bin/sh
{ printf '%s %s ppid=%s ' "$(date +%T)" "$1" "$PPID"; head -c 600; echo; env | grep -E '^(CLAUDE_PID|CLAUDE_CODE_SESSION_ID|CODEX_THREAD_ID)=' | sed 's/^/  env: /'; } >> "$(dirname "$0")/rt-hook.log"
