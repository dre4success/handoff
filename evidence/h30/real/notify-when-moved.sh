#!/bin/sh
# notify-when-moved.sh <scratch dir> <handoff>: once the implementer's binding is a session
# other than the first one given it, posts a reviewer's note to the implementer, and logs
# what handoff said, with the time.
D=$1; BIN=$2; first=""
while :; do
  for f in "$D"/rec/.handoff/sessions/*.json; do
    [ -f "$f" ] || continue
    s=$(sed -n 's/.*"session":"\([^"]*\)".*/\1/p' "$f")
    [ -z "$first" ] && first=$s
    if [ "$s" != "$first" ]; then
      sleep 2
      t=$(python3 -c 'import time; t=time.time(); print(time.strftime("%H:%M:%S", time.gmtime(t)) + ".%03d" % (t*1000%1000))')
      out=$(cd "$D/rec" && echo "Wake up: a note while you are in the background." | HANDOFF_TEST_NOTIFY=$D/alerts.txt "$BIN" note --as reviewer --to implementer --item W1 --title "A note after the move" --body - 2>&1)
      echo "$t NOTE posted by the reviewer, the binding being ${s%%-*}: $out" >> "$D/note.log"
      exit 0
    fi
  done
  sleep 0.1
done
