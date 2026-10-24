#!/usr/bin/env bash
# H10b live check: a real, throwaway Codex session (codex exec, gpt-6-luna,
# low effort) in a scratch project, set up only by the files
# `handoff settings codex` prints. It answers one prompt, running handoff on
# its own to show the role its commands get, then its Stop hook listens. It sits idle for DELAY seconds (default 660, past Codex's default
# hook timeout of 600), then the other agent posts a request; nobody types.
# Notifications go to a file. From the root of a checkout:
#   bash evidence/h10b/live-codex.sh <out file>
# codex exec cannot show the hook-trust prompt, so the run passes
# --dangerously-bypass-hook-trust and marks the scratch project trusted with
# -c. In the operator's own sessions, the operator trusts the hooks once, in Codex's prompt.
set -u
OUT=$(cd "$(dirname "$1")" && pwd -P)/$(basename "$1") # the script moves into the scratch project
DELAY=${DELAY:-660}
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P) # the real path, as Codex names the project
CX=
cleanup() { [ -n "$CX" ] && kill "$CX" 2>/dev/null; rm -rf "$S"; }
trap cleanup EXIT
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
R=$S/live-codex
export HANDOFF_TEST_NOTIFY="$S/alerts.txt"
unset HANDOFF_ROLE
mkdir -p "$R/.codex/rules" && cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base &&
  echo hello > a.txt && git add a.txt && git -c user.name=t -c user.email=t@t commit -q -m "T1: add a.txt" || exit 1
"$H" init --roles claude,codex --reviewers claude,codex --main main >/dev/null || exit 1
"$H" settings codex --as codex > "$S/settings.txt" || exit 1
sed -n '/^\.codex\/config\.toml:$/,/^$/p' "$S/settings.txt" | sed '1d;$d' > .codex/config.toml
sed -n '/^\.codex\/hooks\.json:$/,/^$/p' "$S/settings.txt" | sed '1d;$d' > .codex/hooks.json
sed -n '/^\.codex\/rules\/handoff\.rules:$/,$p' "$S/settings.txt" | sed '1d' > .codex/rules/handoff.rules
echo "Please review T1: it adds a.txt with one line." > req.md
log() { echo "$(date -u +%H:%M:%SZ) $*" | sed 's/ *$//' >> "$S/log"; }
show() { "$H" show "$1" 2>/dev/null; }

log "codex exec starts in $R; .codex/config.toml sets: $(grep HANDOFF_ROLE .codex/config.toml)"
codex exec -m gpt-6-luna -c model_reasoning_effort=low --dangerously-bypass-hook-trust \
  -c "projects.\"$R\".trust_level=\"trusted\"" --sandbox workspace-write \
  "Say READY and the role your handoff brief gives you. Then run this command once, exactly as written, with no arguments: $H
Give the first line it printed. Nothing else." </dev/null > "$S/codex.out" 2> "$S/codex.err" &
CX=$!
for _ in $(seq 240); do grep -q '"how":"hook"' .handoff/presence/codex.json 2>/dev/null && break; sleep 0.5; done
log "presence: $(cat .handoff/presence/codex.json 2>/dev/null || echo none)"
"$H" wait --for codex --timeout 3s > /dev/null 2> "$S/w.err"; log "a second wait for codex: exit $? $(cat "$S/w.err")"
log "idle for $DELAY s"
sleep "$DELAY"
"$H" wait --for codex --timeout 3s > /dev/null 2> "$S/w.err"; log "after $DELAY s, a wait for codex: exit $? $(cat "$S/w.err")"
log "claude posts: $("$H" post --as claude --item T1 --to codex --commit HEAD --title "T1: add a.txt" --body req.md 2>&1)"
for _ in $(seq 300); do show 2 | grep -q '^#2 claim' && break; sleep 1; done
log "entry 2: $(show 2 | head -1)"
for _ in $(seq 300); do show 3 | grep -q '^#3 reply' && break; sleep 1; done
log "entry 3: $(show 3 | head -1)"
for _ in $(seq 60); do grep -q '"how":"hook"' .handoff/presence/codex.json 2>/dev/null && [ "$(sed -E 's/.*"since":"([^"]*)".*/\1/' .handoff/presence/codex.json)" \> "$(show 3 | sed -n 's/^ts: //p')" ] && break; sleep 1; done
log "listening again after the reply: $(cat .handoff/presence/codex.json)"
if kill -0 "$CX" 2>/dev/null; then log "codex exec still running; stopping it"; kill "$CX"; else log "codex exec had already ended"; fi
wait "$CX"; CXEXIT=$?; CX=
sleep 3
"$H" wait --for codex --timeout 2s > /dev/null 2> "$S/w.err"; log "after codex was stopped, a wait for codex: exit $? $(cat "$S/w.err")"
log "hook processes left: $(pgrep -fl "$H hook" || echo '(none)')"
{
  printf '# command: a throwaway Codex session (codex exec -m gpt-6-luna, low effort) in a scratch project, set up by `handoff settings codex --as codex`; idle %s s, then a request from claude; nobody typing\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n\n' \
    "$DELAY" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(codex --version 2>/dev/null)" "$COMMIT"
  echo "## what happened (times UTC)"; sed "s|$S|<scratch>|g" "$S/log"
  echo; echo "## notifications (sent to a file)"; cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
  echo; echo "## the record afterwards"; for i in 1 2 3 4 5 6; do show $i && echo; done | sed "s|$S|<scratch>|g"
  echo "## codex exec's own log (stderr, whole), and how it ended"; sed "s|$S|<scratch>|g" "$S/codex.err" | cut -c1-300
  echo "## codex exec's stdout (its last message)"; sed "s|$S|<scratch>|g" "$S/codex.out"
  echo "# codex exec exit: $CXEXIT"
} > "$OUT"
