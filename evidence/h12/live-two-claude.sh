#!/usr/bin/env bash
# H12 live check: two real, throwaway Claude Code sessions (Haiku) in one
# scratch project, set up by `handoff settings claude` with no --as. Each is
# told its role in its first prompt, as the operator would in chat; nobody types after.
# Notifications go to a file. From the root of a checkout:
#   bash evidence/h12/live-two-claude.sh <out file>
set -u
OUT=$1
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P) # the real path, as the sessions see it
trap 'rm -rf "$S"' EXIT
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
R=$S/live-two
export HANDOFF_TEST_NOTIFY="$S/alerts.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
mkdir -p "$R/.claude" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
  echo hello > a.txt && git add a.txt && git -c user.name=t -c user.email=t@t commit -q -m "T1: add a.txt" || exit 1
"$H" init --roles implementer,reviewer >/dev/null || exit 1
"$H" settings claude | sed -n '/^{/,$p' > .claude/settings.local.json
echo "Please review T1: it adds a.txt with one line." > req.md
cd - >/dev/null
python3 -I evidence/h12/live-two-claude.py "$R" "$H" "$S/alerts.txt" "$S/driver.log"
{
  printf '# command: two throwaway interactive Claude Code sessions (claude --model haiku) in one scratch project, set up by `handoff settings claude` with no --as; each told its role in its first prompt; driven through pseudo-terminals, nobody typing after\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n\n' \
    "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(claude --version 2>/dev/null)" "$COMMIT"
  echo "## the settings (no role fixed: no env, no --as)"
  sed "s|$S|<scratch>|g" "$R/.claude/settings.local.json"
  echo
  echo "## what happened (driver log; times UTC)"
  sed "s|$S|<scratch>|g" "$S/driver.log"
  echo
  echo "## notifications (sent to a file)"
  cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
} > "$OUT"
