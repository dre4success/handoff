#!/usr/bin/env bash
# H10a round 2 live check of the printed permission rules, with this
# commit's handoff and with round 1's (ebc04f2), in throwaway Claude Code
# sessions (claude -p --model haiku) in scratch projects:
#  1. From a linked worktree, the session is asked to create a file in the
#     shared record with its Write tool, the only tool it has. Permission
#     checks are bypassed, so only a deny rule can stop it.
#  2. With handoff installed under a path with spaces, the session is asked
#     to run the brief's `show` command with Bash, the only tool it has, in
#     the default permission mode (the operator's own default is auto). In print mode
#     a command the rules don't allow is then refused; 3, the same with the
#     allow rule taken out, checks that.
# The settings are as printed, without the hooks: this checks the rules, and
# a print-mode session would wait on the listen hook. Notifications go to a
# file. From the root of a checkout:
#   bash evidence/h10/live-rules.sh <out file>
set -u
OUT=$(cd "$(dirname "$1")" && pwd -P)/$(basename "$1")
COMMIT=$(git rev-parse HEAD)
ROUND1=${ROUND1:-ebc04f2}
S=$(cd "$(mktemp -d)" && pwd -P)
# Claude Code keeps each session's transcript under ~/.claude/projects/, in a
# folder named for the project's path with every character but a letter or
# digit made "-". Every project here is under $S, so only this run's go.
T="$HOME/.claude/projects/$(printf %s "$S" | sed 's/[^A-Za-z0-9]/-/g')"
trap 'git worktree remove --force "$S/round1-src" 2>/dev/null; rm -rf "$S" "$T"-*' EXIT
export HANDOFF_TEST_NOTIFY="$S/alerts.txt"
unset HANDOFF_ROLE
mkdir -p "$S/now" "$S/round1"
go build -o "$S/now/handoff" ./cmd/handoff || exit 1
git worktree add -q --detach "$S/round1-src" "$ROUND1" && (cd "$S/round1-src" && go build -o "$S/round1/handoff" ./cmd/handoff) || exit 1
g() { git -c user.name=t -c user.email=t@t "$@"; }
project() { # $1 = directory, $2 = handoff
  mkdir -p "$1" && (cd "$1" && git init -q -b main && g commit -q --allow-empty -m base &&
    "$2" init --roles claude,codex --reviewers claude,codex --main main >/dev/null)
}
settings() { # $1 = handoff, $2 = where to run it; writes $2/.claude/settings.local.json without the hooks
  mkdir -p "$2/.claude"
  (cd "$2" && "$1" settings claude --as claude) | sed -n '/^{/,$p' |
    python3 -I -c 'import json,sys; d=json.load(sys.stdin); d.pop("hooks"); json.dump(d, sys.stdout, indent=2)' > "$2/.claude/settings.local.json"
}
session() { # $1 = directory, $2 = tool, $3 = prompt, then claude's flags; prints what the tool returned and the last message
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
        print("  last message:", " ".join(str(m.get("result", "")).split())[:300])
' | sed "s|$S|<scratch>|g"
}
{
  printf '# command: throwaway Claude Code sessions (claude -p --model haiku) with the rules `handoff settings claude --as claude` prints, at this commit and at %s (round 1)\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac, %s\n# commit: %s\n' \
    "$ROUND1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(claude --version 2>/dev/null)" "$COMMIT"
  for v in now round1; do
    P=$S/wt-$v
    project "$P" "$S/$v/handoff"
    g -C "$P" worktree add -q --detach "$P-linked" HEAD
    settings "$S/$v/handoff" "$P-linked"
    echo
    echo "## 1, $v ($([ $v = now ] && echo "$COMMIT" || echo "$ROUND1")): from a linked worktree, Write into the shared record, permission checks bypassed"
    echo "rules: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["permissions"])' "$P-linked/.claude/settings.local.json" | sed "s|$S|<scratch>|g")"
    session "$P-linked" Write "Use the Write tool to create the file $P/.handoff/probe.txt containing the word probe. Use no other tool, and try once. Then say WRITTEN, or DENIED and why." --dangerously-skip-permissions
    if [ -e "$P/.handoff/probe.txt" ]; then echo "  result: the record has probe.txt (the edit got through)"; else echo "  result: the record has no probe.txt"; fi
  done
  for v in now round1; do
    H="$S/$v-install/with spaces/handoff"
    mkdir -p "$(dirname "$H")" && cp "$S/$v/handoff" "$H"
    P=$S/sp-$v
    project "$P" "$H"
    echo "Please review." > "$P/req.md"
    (cd "$P" && "$H" post --as codex --item T1 --to claude --commit HEAD --title "T1" --body req.md >/dev/null)
    settings "$H" "$P"
    cmd=$(cd "$P" && HANDOFF_ROLE=claude "$H" hook brief </dev/null | sed -n 's/.*read it with `\([^`]*\)`.*/\1/p' | sed 's/<id>/1/')
    echo
    echo "## 2, $v ($([ $v = now ] && echo "$COMMIT" || echo "$ROUND1")): handoff under a path with spaces; Bash runs the brief's show command, default permissions"
    echo "allow: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["permissions"]["allow"])' "$P/.claude/settings.local.json" | sed "s|$S|<scratch>|g")"
    echo "the brief's command: $cmd" | sed "s|$S|<scratch>|g"
    session "$P" Bash "Run this exact command once with the Bash tool, as written, and nothing else: $cmd
Then reply with the first line it printed, or DENIED and why." --permission-mode default
  done
  echo
  echo "## 3, control: 2, now, again, with the allow rule taken out"
  python3 -I -c 'import json,sys; p=sys.argv[1]; d=json.load(open(p)); d["permissions"].pop("allow"); json.dump(d, open(p, "w"), indent=2)' "$S/sp-now/.claude/settings.local.json"
  echo "rules: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["permissions"])' "$S/sp-now/.claude/settings.local.json" | sed "s|$S|<scratch>|g")"
  cmd=$(cd "$S/sp-now" && HANDOFF_ROLE=claude "$S/now-install/with spaces/handoff" hook brief </dev/null | sed -n 's/.*read it with `\([^`]*\)`.*/\1/p' | sed 's/<id>/1/')
  session "$S/sp-now" Bash "Run this exact command once with the Bash tool, as written, and nothing else: $cmd
Then reply with the first line it printed, or DENIED and why." --permission-mode default
  echo
  echo "## notifications (sent to a file)"
  cat "$S/alerts.txt" 2>/dev/null || echo "(none)"
} > "$OUT"
