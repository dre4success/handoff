#!/bin/sh
# Stop hook for probe 2. $G/mode says what to do: "count" continues every stop, up to 15 times;
# "hold N" holds one stop N seconds, then continues it once; anything else lets the turn end.
in=$(cat)
n=$(cat "$HOOKLOG.n" 2>/dev/null || echo 0); n=$((n+1)); echo $n > "$HOOKLOG.n"
info=$(printf '%s' "$in" | python3 -I -c 'import json,sys
d=json.load(sys.stdin); print(d.get("hookEventName"), "continuationActive=%s executionNum=%s terminationReason=%s fullyIdle=%s lastUserInput=%r" % (d.get("continuationActive"), d.get("executionNum"), d.get("terminationReason"), d.get("fullyIdle"), (d.get("lastUserInput") or "")[:50]))')
echo "$(date -u +%H:%M:%S) stop #$n ppid=$PPID $info" >> "$HOOKLOG"
set -- $(cat "$MODE" 2>/dev/null)
case "$1" in
  count) if [ $n -le 15 ]; then echo "{\"decision\":\"continue\",\"reason\":\"Reply with the word PING$n only.\"}"; else echo '{}'; fi ;;
  hold) echo off > "$MODE"; sleep "$2"; echo "$(date -u +%H:%M:%S) stop #$n held $2 s; continues" >> "$HOOKLOG"
        echo '{"decision":"continue","reason":"Reply with the word PONG only."}' ;;
  *) echo '{}' ;;
esac
