#!/bin/sh
{ printf -- '--- %s %s pid=%s ppid=%s\n' "$(date -u +%T)" "$1" "$$" "$PPID"; head -c 1200; echo; env | grep -E '^(CLAUDE_PID|CLAUDE_CODE_SESSION_ID)=' | sed 's/^/  env: /'; } >> "$(dirname "$0")/hooks.log"
