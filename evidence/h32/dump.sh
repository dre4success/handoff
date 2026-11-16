#!/bin/sh
# Logs what a hook is given; answers SessionStart with a word to remember and the first Stop with a continuation.
in=$(cat)
ev=$(printf '%s' "$in" | python3 -I -c 'import json,sys
try: d=json.load(sys.stdin)
except Exception: d={}
print(d.get("hookEventName") or "?")')
envs=$(env | grep -E '^(ANTIGRAVITY|AGY|GEMINI|CLAUDE|CODEX)' | sed 's/=.*//' | tr '\n' ' ')
printf '{"t":"%s","arg":"%s","pid":%s,"ppid":%s,"pwd":"%s","env":"%s","conv_env":"%s","in":%s}\n' "$(date -u +%H:%M:%S)" "$1" "$$" "$PPID" "$(pwd -P)" "$envs" "${ANTIGRAVITY_CONVERSATION_ID:-}" "${in:-null}" >> "$HOOKLOG"
case "$1" in
  start) echo '{"injectSteps":[{"systemMessage":{"systemMessage":"The secret word is KIWI."}}]}' ;;
  stop)
    if [ ! -e "$HOOKLOG.stopped" ]; then
      touch "$HOOKLOG.stopped"; sleep 70
      echo "## stop hook continues after 70 s at $(date -u +%H:%M:%S)" >> "$HOOKLOG"
      echo '{"decision":"continue","reason":"Reply with the word PONG only."}'
    else echo '{}'; fi ;;
  *) echo '{}' ;;
esac
