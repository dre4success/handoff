#!/bin/sh
input=$(cat)
src=$(printf '%s' "$input" | python3 -I -c 'import json,sys; print(json.load(sys.stdin).get("source","?"))')
echo "$(date -u +%T) session start source=$src" >> <probe>/hook.log
echo "PROBE-CONTEXT: your codeword is KIWI-$src."
