#!/usr/bin/env bash
# The reviewer's H21 live check, after the implementer's live-claude.sh: the
# global rule `Edit(//**/.handoff/**)` where the project sits under a hidden
# folder (a dot in a parent's name), which a glob's ** may or may not cross;
# and whether Claude Code runs one hook twice when two settings files hold
# it, as the user's and a project's both would after `--global`. Throwaway
# sessions (claude -p --model haiku) get the settings with --settings; the
# operator's own settings are not changed. From the root of a checkout:
#   bash live-hidden.sh <out file>
set -u
OUT=$(cd "$(dirname "$1")" && pwd -P)/$(basename "$1")
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P)
T="$HOME/.claude/projects/$(printf %s "$S" | sed 's/[^A-Za-z0-9]/-/g')"
trap 'rm -rf "$S" "$T"-*' EXIT
export HANDOFF_TEST_NOTIFY="$S/alerts.txt" HANDOFF_TEST_AWAKE="$S/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
g() { git -c user.name=t -c user.email=t@t "$@"; }
P=$S/.hidden/project && mkdir -p "$P" && (cd "$P" && git init -q -b main && g commit -q --allow-empty -m base && "$H" init >/dev/null) || exit 1
CLAUDE_CONFIG_DIR="$S/claude-home" "$H" settings claude --global | sed -n '/^{/,$p' > "$S/global.json" || exit 1
python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); d.pop("hooks"); json.dump(d, open(sys.argv[2], "w"), indent=2)' "$S/global.json" "$S/rules.json"
session() { # $1 = directory, $2 = tools, $3 = prompt, then claude's flags
  (cd "$1" && claude -p "$3" --model haiku --tools "$2" --output-format stream-json --verbose "${@:4}" </dev/null 2>&1) |
    python3 -I -c '
import json, sys
for line in sys.stdin:
    try:
        m = json.loads(line)
    except ValueError:
        continue
    if m.get("type") == "user":
        for c in m["message"]["content"]:
            if c.get("type") == "tool_result":
                t = c["content"] if isinstance(c["content"], str) else " ".join(x.get("text", "") for x in c["content"])
                print("  tool result:", " ".join(t.split())[:300])
    if m.get("type") == "result":
        print("  last message:", " ".join(str(m.get("result", "")).split())[:400])
' | sed "s|$S|<scratch>|g"
}
{
  printf '# command: throwaway Claude Code sessions (claude -p --model haiku) given the rules `handoff settings claude --global` prints, with --settings, in a project under a hidden folder; then one hook in two settings files\n# date: %s\n# by: Fable 5.1 (reviewer), on the operator'"'"'s Mac, %s\n# commit: %s\n' \
    "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(claude --version 2>/dev/null)" "$COMMIT"
  echo; echo "rules: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["permissions"])' "$S/rules.json" | sed "s|$S|<scratch>|g")"
  n=0
  for c in "$P/.handoff/probe.txt|into the record of a project under a hidden folder" "$P/probe.txt|beside that record (the control)"; do
    IFS='|' read -r file what <<< "$c"; n=$((n+1))
    echo; echo "## 1.$n: Write $what, permission checks bypassed" | sed "s|$S|<scratch>|g"
    session "$P" Write "Use the Write tool to create the file $file containing the word probe. Use no other tool, and try once. Then say WRITTEN, or DENIED and why." --dangerously-skip-permissions --settings "$S/rules.json"
    if [ -e "$file" ]; then echo "  result: $file exists"; else echo "  result: $file does not exist"; fi | sed "s|$S|<scratch>|g"
  done
  echo; echo "## 2: one SessionStart hook in the project's .claude/settings.json and, word for word, in a --settings file: how many times it runs"
  D=$S/dup && mkdir -p "$D/.claude" && (cd "$D" && git init -q -b main && g commit -q --allow-empty -m base)
  printf '{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"sh -c '"'"'echo ran >> %s/marker.txt'"'"'"}]}]}}\n' "$S" > "$D/.claude/settings.json"
  cp "$D/.claude/settings.json" "$S/dup.json"
  echo "hook: $(sed "s|$S|<scratch>|g" "$S/dup.json")"
  session "$D" "" "Say OK and nothing else. Use no tool." --settings "$S/dup.json"
  echo "  marker lines: $(cat "$S/marker.txt" 2>/dev/null | wc -l | tr -d ' ')"
  echo; echo "## notifications (sent to a file)"; cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
} > "$OUT"
