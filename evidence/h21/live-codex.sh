#!/usr/bin/env bash
# H21 live check: a real, throwaway Codex session (codex exec, gpt-6-luna,
# low effort) in a scratch project with no .codex/ folder, set up only by
# what `handoff settings codex --global` prints, put in a scratch CODEX_HOME
# (its auth.json a link to the operator's, so it can sign in; nothing else of
# the operator's Codex settings is read or changed). The session is told to
# take a role; then its Stop hook listens, the implementer posts a request,
# and nobody types. Notifications go to a file. From the root of a checkout:
#   bash evidence/h21/live-codex.sh <out file>
# codex exec cannot show the hook-trust prompt, so the run passes
# --dangerously-bypass-hook-trust, as H10b's did. The project is not marked
# trusted: Codex loads user hooks in any project.
set -u
OUT=$(cd "$(dirname "$1")" && pwd -P)/$(basename "$1")
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P)
CX=
cleanup() { [ -n "$CX" ] && kill "$CX" 2>/dev/null; rm -rf "$S"; }
trap cleanup EXIT
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
R=$S/live
export HANDOFF_TEST_NOTIFY="$S/alerts.txt" HANDOFF_TEST_AWAKE="$S/awake.txt"
export CODEX_HOME="$S/codex-home" CLAUDE_CONFIG_DIR="$S/claude-home"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
mkdir -p "$CODEX_HOME/rules" "$CLAUDE_CONFIG_DIR" && ln -s "$HOME/.codex/auth.json" "$CODEX_HOME/auth.json" || exit 1
"$H" settings codex --global > "$S/settings.txt" || exit 1
sed -n "\\|^$CODEX_HOME/hooks.json:\$|,/^\$/p" "$S/settings.txt" | sed '1d;$d' > "$CODEX_HOME/hooks.json"
sed -n "\\|^$CODEX_HOME/rules/handoff.rules:\$|,\$p" "$S/settings.txt" | sed '1d' > "$CODEX_HOME/rules/handoff.rules"
mkdir -p "$R" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
  git switch -q -c t1 && echo hello > a.txt && git add a.txt && git -c user.name=t -c user.email=t@t commit -q -m "T1: add a.txt" || exit 1
"$H" init > "$S/init.txt" || exit 1
echo "Please review T1: it adds a.txt with one line." > "$S/req.md"
log() { echo "$(date -u +%H:%M:%SZ) $*" | sed 's/ *$//' >> "$S/log"; }
show() { "$H" show "$1" 2>/dev/null; }

log "the project has no .codex/: $(ls -d .codex 2>&1)"
log "CODEX_HOME holds: $(cd "$CODEX_HOME" && find . -type f -o -type l | sort | tr '\n' ' ')"
log "codex exec starts in $R"
codex exec -m gpt-6-luna -c model_reasoning_effort=low --dangerously-bypass-hook-trust --sandbox workspace-write \
  "You are the reviewer here. Run this command once, exactly as written: $H role reviewer
Then follow the brief it prints. Say READY and end your turn." </dev/null > "$S/codex.out" 2> "$S/codex.err" &
CX=$!
for _ in $(seq 360); do grep -q '"how":"hook"' .handoff/presence/reviewer.json 2>/dev/null && break; sleep 0.5; done
log "presence: $(cat .handoff/presence/reviewer.json 2>/dev/null || echo none)"
log "implementer posts: $(HANDOFF_ROLE=implementer "$H" post --item T1 --to reviewer --commit HEAD --title "T1: add a.txt" --body "$S/req.md" 2>&1)"
for _ in $(seq 300); do show 2 | grep -q '^#2 claim' && break; sleep 1; done
log "entry 2: $(show 2 | head -1)"
for _ in $(seq 300); do show 3 | grep -q '^#3 reply' && break; sleep 1; done
log "entry 3: $(show 3 | head -1)"
for _ in $(seq 60); do grep -q '"how":"hook"' .handoff/presence/reviewer.json 2>/dev/null && [ "$(sed -E 's/.*"since":"([^"]*)".*/\1/' .handoff/presence/reviewer.json)" \> "$(show 3 | sed -n 's/^ts: //p')" ] && break; sleep 1; done
log "listening again after the reply: $(cat .handoff/presence/reviewer.json 2>/dev/null)"
if kill -0 "$CX" 2>/dev/null; then log "codex exec still running; stopping it"; kill "$CX"; else log "codex exec had already ended"; fi
wait "$CX"; CXEXIT=$?; CX=
sleep 3
log "hook processes left: $(pgrep -fl "$H hook" || echo '(none)')"
{
  printf '# command: a throwaway Codex session (codex exec -m gpt-6-luna, low effort) in a scratch project with no .codex/, set up only by `handoff settings codex --global` in a scratch CODEX_HOME; told to take the reviewer role, then a request from the implementer; nobody typing\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n\n' \
    "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(codex --version 2>/dev/null)" "$COMMIT"
  echo "## what init printed"; sed "s|$S|<scratch>|g" "$S/init.txt"
  echo; echo "## what happened (times UTC)"; sed "s|$S|<scratch>|g" "$S/log"
  echo; echo "## notifications (sent to a file)"; cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
  echo; echo "## the record afterwards"; for i in 1 2 3 4 5; do show $i && echo; done | sed "s|$S|<scratch>|g"
  echo "## codex exec's own log (stderr), and how it ended"; sed "s|$S|<scratch>|g" "$S/codex.err" | cut -c1-300 | tail -60
  echo "## codex exec's stdout (its last message)"; sed "s|$S|<scratch>|g" "$S/codex.out"
  echo "# codex exec exit: $CXEXIT"
} > "$OUT"
