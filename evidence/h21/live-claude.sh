#!/usr/bin/env bash
# H21 live check of what `handoff settings claude --global` prints, in
# throwaway Claude Code sessions (claude -p --model haiku) in scratch
# projects, given those settings with --settings, as user settings would give
# them; the operator's own settings are not changed.
#  1. The rule against edits names every .handoff/ folder: a session with
#     permission checks bypassed and only the Write tool is asked to write into
#     a project's record, into the record shared by a linked worktree, and,
#     as the control, beside the record. Only a deny rule can stop it.
#  2. The brief hook: a session started in a project with a record, with no
#     role, is told so by the SessionStart hook, and quotes it.
# The settings for 1 are without the hooks, and those for 2 without listen,
# since a print-mode session would wait on it. Notifications go to a file.
# From the root of a checkout:
#   bash evidence/h21/live-claude.sh <out file>
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
P=$S/project && mkdir -p "$P" && (cd "$P" && git init -q -b main && g commit -q --allow-empty -m base && "$H" init >/dev/null) || exit 1
g -C "$P" worktree add -q --detach "$S/linked" HEAD || exit 1
CLAUDE_CONFIG_DIR="$S/claude-home" "$H" settings claude --global | sed -n '/^{/,$p' > "$S/global.json" || exit 1
python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); d.pop("hooks"); json.dump(d, open(sys.argv[2], "w"), indent=2)' "$S/global.json" "$S/rules.json"
python3 -I -c '
import json, sys
d = json.load(open(sys.argv[1]))
d["hooks"] = {"SessionStart": [{"hooks": [h for h in d["hooks"]["SessionStart"][0]["hooks"] if h["args"] == ["hook", "brief"]]}]}
json.dump(d, open(sys.argv[2], "w"), indent=2)' "$S/global.json" "$S/brief.json"
session() { # $1 = directory, $2 = tool, $3 = prompt, then claude's flags
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
  printf '# command: throwaway Claude Code sessions (claude -p --model haiku) given what `handoff settings claude --global` prints, with --settings\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n' \
    "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(claude --version 2>/dev/null)" "$COMMIT"
  echo; echo "rules: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["permissions"])' "$S/rules.json" | sed "s|$S|<scratch>|g")"
  n=0
  for c in "$P|$P/.handoff/probe.txt|into the project's record" "$S/linked|$P/.handoff/probe-linked.txt|into the record, from a linked worktree" "$P|$P/probe.txt|beside the record (the control)"; do
    IFS='|' read -r dir file what <<< "$c"; n=$((n+1))
    echo; echo "## 1.$n: Write $what, permission checks bypassed" | sed "s|$S|<scratch>|g"
    session "$dir" Write "Use the Write tool to create the file $file containing the word probe. Use no other tool, and try once. Then say WRITTEN, or DENIED and why." --dangerously-skip-permissions --settings "$S/rules.json"
    if [ -e "$file" ]; then echo "  result: $file exists"; else echo "  result: $file does not exist"; fi | sed "s|$S|<scratch>|g"
  done
  echo; echo "## 2: the SessionStart brief hook, in a project with a record, a session with no role"
  echo "hooks: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["hooks"])' "$S/brief.json" | sed "s|$S|<scratch>|g")"
  session "$P" Read "Before this prompt, a SessionStart hook may have given you a message from handoff. Quote its first sentence exactly, or say NONE. Use no tool." --settings "$S/brief.json"
  echo; echo "## notifications (sent to a file)"; cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
} > "$OUT"
