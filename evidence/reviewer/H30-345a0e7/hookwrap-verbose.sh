#!/bin/sh
# hookwrap.sh <brief|listen|notify|end>: logs what Claude Code gave the hook, then runs the
# build under review on it, in a scratch record kept apart from the session's own folder.
D=<scratch>; BIN=<handoff built at the commit>; kind=$1
in=$(mktemp $D/in.XXXXXX); cat > "$in"
{
  printf -- '--- %s %s hook-pid=%s parent=%s CLAUDE_PID=%s\n' "$(perl -MTime::HiRes=time -MPOSIX -e '$t=time; printf "%s.%03d", strftime("%H:%M:%S", gmtime($t)), ($t*1000)%1000')" "$kind" "$$" "$PPID" "$CLAUDE_PID"
  head -c 1500 "$in" | perl -pe 's/"(last_assistant_message|message)":"(?:[^"\\]|\\.)*"/"$1":"..."/g'; echo
  for f in $D/rec/.handoff/sessions/*.json; do [ -f "$f" ] || continue
    p=$(perl -ne 'print $1 if /"pid":(\d+)/' "$f"); s=$(perl -ne 'print $1 if /"session":"([^"]+)"/' "$f"); r=$(perl -ne 'print $1 if /"role":"([^"]+)"/' "$f")
    printf '  binding as this hook starts: %s has %s; its process %s %s\n' "$s" "$r" "$p" "$(kill -0 "$p" 2>/dev/null && echo RUNS || echo has-ended)"
  done
  printf '  listeners running: %s\n' "$(pgrep -f 'bin30/handoff hook listen' | tr '\n' ' ')"
  printf '  env: HANDOFF_TEST_NOTIFY=%s\n' "${HANDOFF_TEST_NOTIFY:-unset}"
} >> $D/hooks.log
[ "$kind" = end ] && exit 0
cd $D/rec || exit 0
export HANDOFF_TEST_NOTIFY=$D/alerts.txt HANDOFF_TEST_AWAKE=$D/awake.txt
exec "$BIN" hook "$kind" < "$in"
