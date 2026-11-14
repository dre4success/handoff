#!/bin/sh
# replay.sh <handoff binary> <hooks log>: a session's role, then the hook input
# Claude Code gave the session it went on as, from the probe.
bin=$1; log=$2
input() { grep "\"session_id\":\"$1" "$log" | grep '"hook_event_name":"SessionStart"' | head -1; }
pidof() { grep -A3 "\"session_id\":\"$1" "$log" | grep CLAUDE_PID | head -1 | sed 's/.*=//'; }
run() { # old new label
  old=$1; new=$2
  CLAUDE_CODE_SESSION_ID=$old CLAUDE_PID=$(pidof $old) "$bin" role implementer >/dev/null
  printf '%s: implementer given to %s (pid %s, ended: %s)\n' "$3" "${old%%-*}" "$(pidof $old)" "$(kill -0 $(pidof $old) 2>/dev/null && echo no || echo yes)"
  printf '  the old transcript names: %s\n' "$(grep -o '"continuedInSessionId":"[^"]*"' "$(dirname "$(input $new | sed 's/.*"transcript_path":"\([^"]*\)".*/\1/')")/$old.jsonl" | cut -d'"' -f4 | cut -c1-8)"
  input $new | CLAUDE_CODE_SESSION_ID=$new CLAUDE_PID=$(pidof $new) "$bin" hook brief | head -c 160 | sed 's/^/  brief of '"${new%%-*}"' (SessionStart, source fork): /'; echo
  printf '  bindings: %s\n' "$(cat .handoff/sessions/*.json | sed 's/,"since":"[^"]*"//' | tr '\n' ' ')"
}
run 8d47e001-f495-43b9-a180-84183d947c32 e05f1c64-264b-427e-be69-d710dba403e0 /background
run 175512d7-eed2-46b5-8ae7-25776adc008a e3cd0611-012a-4861-8787-1f5b7bd44acd /fork
