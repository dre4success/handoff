#!/bin/sh
in=$(cat)
printf '{"t":"%s","pwd":"%s","pdir":"%s","in":%s}\n' "$(date -u +%H:%M:%S)" "$(pwd -P)" "${CLAUDE_PROJECT_DIR:-}" "$in" >> "$HOOKLOG"
