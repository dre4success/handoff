#!/usr/bin/env bash
# The reviewer's H21 probes. Run from the root of a checkout of the commit
# under review:
#   OUT=<this directory> bash probes.sh
# handoff is built from the checkout into a scratch folder. Every run reads
# scratch user settings (CLAUDE_CONFIG_DIR and CODEX_HOME, or a scratch HOME),
# never the operator's; alerts and sleep holds go to scratch files.
set -u
OUT=${OUT:?}
WT=$(pwd -P)
COMMIT=$(git rev-parse HEAD)
S=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$S"' EXIT
export HANDOFF_TEST_NOTIFY="$S/alerts.txt" HANDOFF_TEST_AWAKE="$S/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export CLAUDE_CONFIG_DIR="$S/empty-claude" CODEX_HOME="$S/empty-codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { sed "s|$S|<scratch>|g; s|$WT|<worktree>|g; s|$HOME|~|g"; }
newproject() { local d="$S/$1"; mkdir -p "$d" && (cd "$d" && git init -q -b main && git commit -q --allow-empty -m base) && echo "$d"; }
# bounded runs a command with the given stdin for at most 15 s and reports
# its exit code and what it printed.
bounded() { # bounded <stdin> <cmd...>
  local in=$1; shift
  printf '%s' "$in" | "$@" > "$S/b.out" 2> "$S/b.err" & local pid=$!
  local i; for i in $(seq 150); do kill -0 $pid 2>/dev/null || break; sleep 0.1; done
  if kill -0 $pid 2>/dev/null; then kill $pid; wait $pid 2>/dev/null; echo "  STILL RUNNING after 15 s; killed"; return; fi
  wait $pid; local code=$?
  echo "  exit $code; stdout $(wc -c < "$S/b.out" | tr -d ' ') bytes; stderr $(wc -c < "$S/b.err" | tr -d ' ') bytes"
  sed 's/^/  out: /' "$S/b.out" | clean; sed 's/^/  err: /' "$S/b.err" | clean
}
# What settings --global prints, as files.
globalclaude() { CLAUDE_CONFIG_DIR=$1 "$H" settings claude --global | sed -n '/^{/,$p'; }
globalcodex() { CODEX_HOME=$1 "$H" settings codex --global | sed -n "\\|^$1/hooks.json:\$|,/^\$/p" | sed '1d;$d'; }
lines() { grep -E '^  (Claude Code|Codex):|^For each CLI' | clean; }
offerline() { grep -o 'offer to set them up for every project: .* prints them' | clean; }

# P1. Hooks from user settings run in every project: they must do nothing,
# quietly, where handoff isn't used.
{
  header "hook brief|listen|notify with no record (outside git, in a git repository, in /) and in a project with a record for a session with no role; with no role, HANDOFF_ROLE=reviewer, and a Claude Code session id"
  P1G=$(newproject p1-git); mkdir -p "$S/p1-nogit"; P1R=$(newproject p1-rec); (cd "$P1R" && "$H" init > /dev/null)
  for dir in "$S/p1-nogit" "$P1G" / "$P1R"; do
    for e in "none" "HANDOFF_ROLE=reviewer" "CLAUDE_CODE_SESSION_ID=s-p1 CLAUDE_PID=$$"; do
      for hook in brief listen notify; do
        case $hook in
          brief) in='{"session_id":"s-p1","source":"startup"}';;
          listen) in='{"session_id":"s-p1","stop_hook_active":false}';;
          notify) in='{"session_id":"s-p1","message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}';;
        esac
        echo "## in $dir, env $e: hook $hook" | clean
        if [ "$e" = none ]; then (cd "$dir" && bounded "$in" "$H" hook $hook); else (cd "$dir" && bounded "$in" env $e "$H" hook $hook); fi
      done
    done
  done
  echo "## alerts sent (to the scratch file): $(cat "$S/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"; sed 's/^/  /' "$S/alerts.txt" 2>/dev/null | clean
} > "$OUT/p1-harmless-elsewhere.log" 2>&1

# P2. Hook detection: what init says, and whether the brief offers, for each
# way the hooks can be present or absent.
UC=$S/uc; UX=$S/ux
fresh() { rm -rf "$UC" "$UX"; mkdir -p "$UC" "$UX/rules"; }
initin() { (cd "$1" && CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX "$H" init > "$S/init.out" 2>&1; echo "  exit $?"; lines < "$S/init.out"); }
offerin() { (cd "$1" && CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX HANDOFF_ROLE=reviewer "$H" brief > "$S/brief.out" 2>&1; echo "  brief exit $?; $(offerline < "$S/brief.out" || echo 'no offer')"); }
{
  header "init and the brief's offer (outside a session) with the hooks present or absent in each file handoff reads: user settings, the project's, a linked worktree's; other hooks, a wrong hook, malformed JSON"
  echo "## a. nothing set up"; fresh; P=$(newproject p2-a); initin "$P"; offerin "$P"
  echo "## b. the user's Claude Code settings have another tool's Stop hook only"; fresh
  echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/usr/bin/true"}]}]},"permissions":{"allow":["Bash(ls *)"]}}' > "$UC/settings.json"
  P=$(newproject p2-b); initin "$P"; offerin "$P"
  echo "## c. handoff's hooks in SessionStart only, none in Stop"; fresh
  globalclaude "$UC" | python3 -I -c 'import json,sys; d=json.load(sys.stdin); del d["hooks"]["Stop"]; json.dump(d, sys.stdout, indent=2)' > "$UC/settings.json"
  P=$(newproject p2-c); initin "$P"; offerin "$P"
  echo "## d. malformed user settings for both CLIs"; fresh; echo '{oops' > "$UC/settings.json"; echo '{oops' > "$UX/hooks.json"
  P=$(newproject p2-d); initin "$P"; offerin "$P"
  echo "## e. a line comment in otherwise good user settings"; fresh; { echo '// handoff'; globalclaude "$UC"; } > "$UC/settings.json"
  P=$(newproject p2-e); initin "$P"; offerin "$P"
  echo "## f. the project's .claude/settings.json alone"; fresh; P=$(newproject p2-f); mkdir "$P/.claude"; globalclaude "$UC" > "$P/.claude/settings.json"; initin "$P"; offerin "$P"
  echo "## g. the project's .claude/settings.local.json alone, and .codex/hooks.json alone"; fresh; P=$(newproject p2-g); mkdir "$P/.claude" "$P/.codex"; globalclaude "$UC" > "$P/.claude/settings.local.json"; globalcodex "$UX" > "$P/.codex/hooks.json"; initin "$P"; offerin "$P"
  echo "## h. a Stop hook naming another handoff binary, command and arguments in one string"; fresh
  echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/opt/elsewhere/handoff hook listen","timeout":86400}]}]}}' > "$UC/settings.json"
  P=$(newproject p2-h); initin "$P"; offerin "$P"
  echo "## i. a Stop hook running handoff's notify, not listen"; fresh
  echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"handoff","args":["hook","notify"]}]}]}}' > "$UC/settings.json"
  P=$(newproject p2-i); initin "$P"; offerin "$P"
  echo "## j. the user's and the project's both set up: which init names"; fresh; globalclaude "$UC" > "$UC/settings.json"; globalcodex "$UX" > "$UX/hooks.json"
  P=$(newproject p2-j); mkdir "$P/.claude" "$P/.codex"; cp "$UC/settings.json" "$P/.claude/settings.json"; cp "$UX/hooks.json" "$P/.codex/hooks.json"; initin "$P"; offerin "$P"
  echo "## k. a linked worktree: the main worktree's .claude/settings.json is set up, the linked one has none; then from a subfolder of the main worktree; then the linked worktree's own"; fresh
  P=$(newproject p2-k); mkdir "$P/.claude" "$P/sub"; globalclaude "$UC" > "$P/.claude/settings.json"; git -C "$P" worktree add -q --detach "$S/p2-k-linked" HEAD
  echo "  from the linked worktree:"; initin "$S/p2-k-linked"; echo "  from a subfolder of the main worktree (init refused: the record exists; so brief and role):"
  (cd "$P/sub" && CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-k CLAUDE_PID=$$ "$H" role reviewer > "$S/role.out" 2>&1; echo "  role exit $?; $(offerline < "$S/role.out" || echo 'no offer')")
  mkdir "$S/p2-k-linked/.claude"; cp "$P/.claude/settings.json" "$S/p2-k-linked/.claude/"
  (cd "$S/p2-k-linked" && CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-k2 CLAUDE_PID=$$ "$H" role reviewer-2 > "$S/role.out" 2>&1; echo "  linked worktree with its own settings: role exit $?; $(offerline < "$S/role.out" || echo 'no offer')")
  echo "## l. no CLAUDE_CONFIG_DIR or CODEX_HOME: the home folder's .claude/settings.json and .codex/hooks.json"
  HOMEX=$S/home; mkdir -p "$HOMEX/.claude" "$HOMEX/.codex/rules"; globalclaude "$HOMEX/.claude" > "$HOMEX/.claude/settings.json"; globalcodex "$HOMEX/.codex" > "$HOMEX/.codex/hooks.json"
  P=$(newproject p2-l); (cd "$P" && env -u CLAUDE_CONFIG_DIR -u CODEX_HOME HOME="$HOMEX" "$H" init | lines)
  echo "## m. settings --global, printed outside any project, from /"; (cd / && CLAUDE_CONFIG_DIR=$UC "$H" settings claude --global | head -2 | clean; echo "  exit ${PIPESTATUS[0]}"; CODEX_HOME=$UX "$H" settings codex --global | head -2 | clean; echo "  exit ${PIPESTATUS[0]}")
  echo "## n. the Claude Code deny rule, per project and global"; P=$(newproject p2-n); (cd "$P" && "$H" init >/dev/null && "$H" settings claude | grep -o '"Edit([^"]*"' | clean; CLAUDE_CONFIG_DIR=$UC "$H" settings claude --global | grep -o '"Edit([^"]*"')
  echo "## alerts sent: $(cat "$S/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} > "$OUT/p2-detection.log" 2>&1

# P3. The tests never read the operator's settings: with HOME a scratch home
# whose settings hold handoff's hooks, and no CLAUDE_CONFIG_DIR or CODEX_HOME,
# a test process that fell back to the home folder would see hooks where the
# tests expect none, and fail.
{
  header "go test -run 'H21|H18|H11|H12|H10' ./cmd/handoff with HOME=<scratch>/home, whose .claude/settings.json and .codex/hooks.json hold handoff's hooks, and CLAUDE_CONFIG_DIR, CODEX_HOME unset; the control is init under the same environment"
  HOMEX=$S/home
  P=$(newproject p3); echo "## control: init"; (cd "$P" && env -u CLAUDE_CONFIG_DIR -u CODEX_HOME HOME="$HOMEX" "$H" init | lines)
  before=$(cd "$HOMEX" && find .claude .codex -type f -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -c1-16)
  echo "## the tests:"
  (cd "$WT" && env -u CLAUDE_CONFIG_DIR -u CODEX_HOME HOME="$HOMEX" GOCACHE="$(go env GOCACHE)" GOPATH="$(go env GOPATH)" GOMODCACHE="$(go env GOMODCACHE)" GOTOOLCHAIN=local go test -count=1 -run 'H21|H18|H11|H12|H10' -v ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL|PASS)' | clean)
  after=$(cd "$HOMEX" && find .claude .codex -type f -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -c1-16)
  echo "## the scratch home's .claude and .codex before and after the tests: $([ "$before" = "$after" ] && echo unchanged || echo CHANGED); their files now: $(cd "$HOMEX" && find .claude .codex -type f | sort | tr '\n' ' '); other folders go made under that home: $(cd "$HOMEX" && find . -mindepth 1 -maxdepth 1 -type d ! -name .claude ! -name .codex | tr '\n' ' ')"
  echo "## Source: file writes in settings.go and setup.go: [$(grep -n 'WriteFile\|os\.Create\|OpenFile\|MkdirAll\|os\.Mkdir' cmd/handoff/settings.go cmd/handoff/setup.go | tr '\n' ' ')]"
  echo "## Source: TestMain sets, for every test process: $(grep -n 'CLAUDE_CONFIG_DIR\|CODEX_HOME' cmd/handoff/main_test.go | tr '\n' ' ')"
  echo "## Source: env() keeps the parent's variables but HANDOFF_*, GIT_* and the session's: $(grep -n 'HasPrefix(kv' cmd/handoff/main_test.go | tr '\n' ' ')"
} > "$OUT/p3-tests-read-no-user-settings.log" 2>&1

# P4. Two listen hooks for one session and role at once, as user and project
# settings both holding the hook would start on one Stop; then a request.
{
  header "two hook listen processes for one Claude Code session and role at once; then the implementer posts a request to that role"
  P4=$(newproject p4); cd "$P4" && "$H" init > /dev/null
  echo "## $(CLAUDE_CODE_SESSION_ID=s-4 CLAUDE_PID=$$ "$H" role reviewer | head -1 | clean)"
  for i in 1 2; do (printf '{"session_id":"s-4","stop_hook_active":false}' | CLAUDE_CODE_SESSION_ID=s-4 CLAUDE_PID=$$ "$H" hook listen --timeout 60s > "$S/l$i.out" 2> "$S/l$i.err"; echo $? > "$S/l$i.code") & done
  for _ in $(seq 100); do [ -f .handoff/presence/reviewer.json ] && break; sleep 0.1; done; sleep 2
  echo "## presence: $(cat .handoff/presence/reviewer.json 2>/dev/null | clean)"
  echo "## listen processes running: $(pgrep -f "$H hook listen" | wc -l | tr -d ' '); exit codes so far: [$(cat "$S/l1.code" 2>/dev/null)] [$(cat "$S/l2.code" 2>/dev/null)]"
  git switch -q -c t1 && echo hi > a.txt && git add a.txt && git commit -q -m "T1: a"; echo 'review please' > "$S/req.md"
  echo "## implementer posts: $(HANDOFF_ROLE=implementer "$H" post --item T1 --to reviewer --commit HEAD --title "T1: a" --body "$S/req.md" 2>&1 | clean)"
  wait
  for i in 1 2; do echo "## listener $i: exit $(cat "$S/l$i.code"); stdout $(wc -c < "$S/l$i.out" | tr -d ' ') bytes; stderr:"; sed 's/^/  /' "$S/l$i.err" | clean; done
  echo "## presence after: $(cat .handoff/presence/reviewer.json 2>/dev/null | clean || echo none)"
  echo "## alerts sent: $(cat "$S/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
  cd "$WT"
} > "$OUT/p4-two-listeners.log" 2>&1

# P6. Whose command the brief offers.
{
  header "the brief's offer by session CLI and by which CLI is set up, from brief (listens itself), role (maybe a hook) and hook brief (a hook listens)"
  P=$(newproject p6); cd "$P" && "$H" init > /dev/null
  run6() { # run6 <label> <env...> -- <args...>
    local label=$1; shift; local e=(); while [ "$1" != "--" ]; do e+=("$1"); shift; done; shift
    echo "## $label: $(env "${e[@]}" "$H" "$@" > "$S/6.out" 2>&1; echo "exit $?;") $(offerline < "$S/6.out" || echo 'no offer')"
  }
  fresh; globalcodex "$UX" > "$UX/hooks.json"
  run6 "Codex set up alone; brief in a Claude Code session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-6a CLAUDE_PID=$$ -- brief --for reviewer
  run6 "Codex set up alone; role in a Claude Code session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-6b CLAUDE_PID=$$ -- role reviewer
  run6 "Codex set up alone; brief in a Codex session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CODEX_THREAD_ID=t-6a -- brief --for reviewer
  run6 "Codex set up alone; brief outside a session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX HANDOFF_ROLE=reviewer -- brief
  fresh; globalclaude "$UC" > "$UC/settings.json"
  run6 "Claude Code set up alone; brief in a Claude Code session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-6c CLAUDE_PID=$$ -- brief --for reviewer
  run6 "Claude Code set up alone; brief in a Codex session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CODEX_THREAD_ID=t-6b -- brief --for reviewer
  run6 "Claude Code set up alone; role in a Codex session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CODEX_THREAD_ID=t-6c -- role reviewer-2
  run6 "Claude Code set up alone; brief outside a session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX HANDOFF_ROLE=reviewer -- brief
  fresh; globalclaude "$UC" > "$UC/settings.json"; globalcodex "$UX" > "$UX/hooks.json"
  run6 "both set up; brief outside a session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX HANDOFF_ROLE=reviewer -- brief
  fresh
  run6 "neither set up; brief outside a session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX HANDOFF_ROLE=reviewer -- brief
  echo "## neither set up; hook brief for a session with a role: $(printf '{"session_id":"s-6b","source":"startup"}' | CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-6b CLAUDE_PID=$$ "$H" hook brief > "$S/6.out" 2>&1; echo "exit $?;") $(offerline < "$S/6.out" || echo 'no offer'); listening line: $(grep '^Listening:' "$S/6.out" | cut -c1-60)"
  mkdir .claude .codex; globalclaude "$UC" > .claude/settings.local.json; globalcodex "$UX" > .codex/hooks.json
  run6 "neither in user settings, both in the project's; role in a Claude Code session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CLAUDE_CODE_SESSION_ID=s-6d CLAUDE_PID=$$ -- role reviewer
  run6 "neither in user settings, both in the project's; brief in a Codex session" CLAUDE_CONFIG_DIR=$UC CODEX_HOME=$UX CODEX_THREAD_ID=t-6d -- brief --for reviewer
  echo "## the Codex listening line, no hooks: $(CLAUDE_CONFIG_DIR=$S/empty-claude CODEX_HOME=$S/empty-codex CODEX_THREAD_ID=t-6e "$H" brief --for reviewer | grep '^Listening:' | clean)"
  echo "## the Claude Code listening line, no hooks: $(CLAUDE_CONFIG_DIR=$S/empty-claude CODEX_HOME=$S/empty-codex CLAUDE_CODE_SESSION_ID=s-6e CLAUDE_PID=$$ "$H" brief --for reviewer | grep '^Listening:' | clean)"
  echo "## alerts sent: $(cat "$S/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
  cd "$WT"
} > "$OUT/p6-offer.log" 2>&1
