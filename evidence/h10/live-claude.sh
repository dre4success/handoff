#!/usr/bin/env bash
# H10a live check: a real, throwaway Claude Code session (Haiku) in a scratch
# project, set up only by the settings `handoff settings claude` prints. It
# starts idle; the other agent posts a request; nobody types. Notifications
# go to a file. From the root of a checkout:
#   bash evidence/h10/live-claude.sh <out file>
set -u
OUT=$1
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P) # the real path, as the session sees it
trap 'rm -rf "$S"' EXIT
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
R=$S/live-claude
export HANDOFF_TEST_NOTIFY="$S/alerts.txt"
unset HANDOFF_ROLE
mkdir -p "$R/.claude" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
  echo hello > a.txt && git add a.txt && git -c user.name=t -c user.email=t@t commit -q -m "T1: add a.txt" || exit 1
"$H" init --roles claude,codex --reviewers claude,codex --main main >/dev/null || exit 1
"$H" settings claude --as claude | sed -n '/^{/,$p' > .claude/settings.local.json
echo "Please review T1: it adds a.txt with one line." > req.md
cd - >/dev/null
python3 -I evidence/h10/live-claude.py "$R" "$H" "$S/alerts.txt" "$S/driver.log"
{
  printf '# command: a throwaway interactive Claude Code session (claude --model haiku) in a scratch project, set up by `handoff settings claude --as claude`; driven through a pseudo-terminal, nobody typing after the first prompt\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n\n' \
    "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(claude --version 2>/dev/null)" "$COMMIT"
  echo "## what happened (driver log; times UTC)"
  sed "s|$S|<scratch>|g" "$S/driver.log"
  echo
  echo "## notifications (sent to a file)"
  cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
  echo
  echo "## the record afterwards"
  # $(...) drops the blank line after the last entry.
  printf '%s\n' "$(cd "$R" && for i in 1 2 3 4 5 6; do "$H" show $i 2>/dev/null && echo; done)" | sed "s|$S|<scratch>|g"
} > "$OUT"
