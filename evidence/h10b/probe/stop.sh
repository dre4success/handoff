#!/bin/sh
input=$(cat)
d=<probe>
echo "$(date -u +%T) stop: $input" >> $d/hook.log
env | grep -E '^(CODEX|HANDOFF)' | sed 's/^/  hook env: /' >> $d/hook.log
case "$input" in *'"stop_hook_active":true'*) echo "$(date -u +%T) second stop: let it end" >> $d/hook.log; exit 0;; esac
i=0; while [ ! -e $d/trigger ] && [ $i -lt 90 ]; do sleep 1; i=$((i+1)); done
echo "$(date -u +%T) first stop: waking with exit 2" >> $d/hook.log
echo "PROBE: a new entry arrived. Reply with exactly the word PONG." >&2
exit 2
