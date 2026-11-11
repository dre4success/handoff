#!/usr/bin/env bash
# The reviewer's probe 1 for H25 (ba23d94): every claim in README.md that can be
# run, run with handoff built from a detached worktree at the commit. Run as
#   S=<scratchpad> B=<handoff built at the commit> WT=<worktree> bash probes.sh > p1-readme-claims.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; WT=${WT:?worktree}
mkdir -p "$S/h25"
WORK=$(mktemp -d "$S/h25/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
clean() { sed -E "s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/[0-9a-f]{40}/<sha>/g; s/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/<time>/g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g; s/pid [0-9]+/pid <pid>/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# runin <env...> -- <cmd...>: a command in a session's environment
runin() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ env ${envs[*]} $*"; env "${envs[@]}" "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# hook <json> <env...> -- <hook args...>: a hook, fed what the CLI writes to its stdin
hook() { local json=$1; shift; local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ echo '$json' | env ${envs[*]} handoff hook $*"; printf '%s' "$json" | env "${envs[@]}" "$B" hook "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
hookin() { if [ "$2" = SessionStart ]; then printf '{"session_id":"%s","hook_event_name":"%s","source":"%s"}' "$1" "$2" "${3:-startup}"; else printf '{"session_id":"%s","hook_event_name":"%s"}' "$1" "$2"; fi; }
sleeper() { sleep 300 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
N=0
newrepo() { N=$((N+1)); REPO="$WORK/repo$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; }

main() {
printf '# command: every README claim that can be run, run; see each heading\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"
echo "Line numbers are README.md's at this commit."

echo; echo "## L14, L17: the module path, the main package, go.mod's go line, handoff version"
grep -n '^module\|^go ' "$WT/go.mod"
(cd "$WT" && go list ./cmd/handoff)
run "$B" version

echo; echo "## L31-36: claude --model takes an alias (claude --help)"
echo "claude $(claude --version 2>&1 | head -1)"
claude --help 2>&1 | grep -A3 -- '--model <model>'

echo; echo "## L19: handoff init outside a git repository"
mkdir -p "$WORK/nogit"; cd "$WORK/nogit"; run "$B" init

echo; echo "## L23, L48, L66, L70, L73, L85: every settings form, in a project; the one the README named before, first"
newrepo; run "$B" init
run "$B" settings --global
run "$B" settings claude --global
run "$B" settings codex --global
run "$B" settings claude --as reviewer
run "$B" settings claude
run "$B" settings codex
run "$B" settings codex --as reviewer
echo; echo "## L72: 'sandbox' in what settings codex prints, and in settings.go"
"$B" settings codex --global 2>&1 | grep -n -i sandbox
grep -n -i "sandbox" "$WT/cmd/handoff/settings.go"

echo; echo "## L23: the hooks do nothing where handoff isn't used: a git repository with no .handoff/, with a session and with HANDOFF_ROLE"
mkdir -p "$WORK/plain"; git init -q -b main "$WORK/plain"; cd "$WORK/plain"
P0=$(sleeper)
hook "$(hookin x Stop)" CLAUDE_CODE_SESSION_ID=x CLAUDE_PID=$P0 -- listen --timeout 1s
hook "$(hookin x SessionStart)" CLAUDE_CODE_SESSION_ID=x CLAUDE_PID=$P0 -- brief
hook '{"session_id":"x","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' CLAUDE_CODE_SESSION_ID=x CLAUDE_PID=$P0 -- notify
hook "$(hookin x Stop)" HANDOFF_ROLE=reviewer -- listen --timeout 1s
hook "$(hookin x SessionStart)" HANDOFF_ROLE=reviewer -- brief
hook "$(hookin t Stop)" CODEX_THREAD_ID=t -- listen --timeout 1s
echo "alerts so far: $( [ -f "$WORK/alerts.txt" ] && wc -l < "$WORK/alerts.txt" | tr -d ' ' || echo 0)"

echo; echo "## L27: init makes .handoff/, ignored by git, with three roles; says whether each CLI is set up; every worktree shares it"
newrepo; run "$B" init
run git status --short --ignored
run git check-ignore -v .handoff/
run ls -A .handoff
echo ".handoff/.gitignore:"; cat .handoff/.gitignore
echo "config.json:"; cat .handoff/config.json
run git worktree add -q "$WORK/side$N" -b side
echo "a note posted from the main worktree:"; echo "hello" > "$WORK/b0.md"
run "$B" note --as implementer --to reviewer --item W0 --title "from the main worktree" --body "$WORK/b0.md"
cd "$WORK/side$N"; echo "(now in the linked worktree $(pwd | sed "s|$WORK|<scratch>|"))"
run ls -A
run "$B" inbox --for reviewer
run "$B" init
run "$B" show 1
cd "$REPO"

echo; echo "## L54: quiet_after in config.json after init"
grep -n quiet .handoff/config.json

echo; echo "## L58, L42, L60: no role until given; the brief; a second session takes the role"
newrepo; "$B" init > /dev/null
P1=$(sleeper); P2=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" inbox
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" role
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" brief
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" role reviewer
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" role
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" brief
runin CLAUDE_CODE_SESSION_ID=s2 CLAUDE_PID=$P2 -- "$B" role reviewer
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" role
runin CLAUDE_CODE_SESSION_ID=s1 CLAUDE_PID=$P1 -- "$B" inbox
runin CLAUDE_CODE_SESSION_ID=s2 CLAUDE_PID=$P2 -- "$B" role

echo; echo "## L59: the role survives a resume (same id, new process) and /clear (new id, same process); a new Codex thread needs telling again"
P3=$(sleeper)
hook "$(hookin s2 SessionStart resume)" CLAUDE_CODE_SESSION_ID=s2 CLAUDE_PID=$P3 -- brief
echo "s2's binding now:"; cat .handoff/sessions/s2.json
hook "$(hookin s2c SessionStart clear)" CLAUDE_CODE_SESSION_ID=s2c CLAUDE_PID=$P3 -- brief
runin CLAUDE_CODE_SESSION_ID=s2c CLAUDE_PID=$P3 -- "$B" role
echo "bindings now: $(ls .handoff/sessions | grep -v '^\.' | tr '\n' ' ')"
runin CODEX_THREAD_ID=t1 -- "$B" role implementer
hook "$(hookin t2 SessionStart)" CODEX_THREAD_ID=t2 -- brief
runin CODEX_THREAD_ID=t2 -- "$B" role
runin CODEX_THREAD_ID=t1 -- "$B" role

echo; echo "## L61, L62, L63: the implementer's brief, with one reviewer session (s2c, process alive); a post to reviewer-2, which has none"
runin CODEX_THREAD_ID=t1 -- "$B" brief
echo "req" > "$WORK/req.md"
git checkout -q -b w1 && git commit -q --allow-empty -m "W1 work" && SHA=$(git rev-parse HEAD)
runin CODEX_THREAD_ID=t1 -- "$B" post --item W1 --to reviewer-2 --commit "$SHA" --title "to a reviewer with no session" --body "$WORK/req.md"

echo; echo "## L64, L65: init --roles, --reviewers; roles add; no rename or remove"
newrepo; run "$B" init --roles a,b
cat .handoff/config.json
newrepo; run "$B" init --roles implementer,b,c --reviewers b
cat .handoff/config.json
run "$B" roles add reviewer-3
cat .handoff/config.json
run "$B" roles rename b x
run "$B" roles remove c
run "$B" roles

echo; echo "## L71: a Codex session's brief against a Claude Code session's, same role, neither with hooks: the lines that differ"
newrepo; "$B" init > /dev/null
P4=$(sleeper)
env CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P4 "$B" role reviewer > /dev/null
env CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P4 "$B" brief > "$WORK/brief-claude.txt" 2>&1
env CODEX_THREAD_ID=x1 "$B" role reviewer > /dev/null
env CODEX_THREAD_ID=x1 "$B" brief > "$WORK/brief-codex.txt" 2>&1
diff "$WORK/brief-claude.txt" "$WORK/brief-codex.txt"; echo "# diff exit $?"

echo; echo "## L46: a Claude Code session on duty, stuck at a permission prompt: hook notify (alerts go to the scratch file)"
env CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P4 "$B" role reviewer > /dev/null
: > "$WORK/alerts.txt"
hook '{"session_id":"c1","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P4 -- notify
hook '{"session_id":"c1","hook_event_name":"Notification","notification_type":"idle_prompt","message":"Claude is waiting for your input"}' CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P4 -- notify
echo "alerts file:"; cat "$WORK/alerts.txt"

echo; echo "## L7, L52: the pop-up's buttons, in popup.go"
grep -n '"Later", "Deny", "Approve"' "$WT/cmd/handoff/popup.go"

echo; echo "## L77: what keeps the Mac awake, and for how long (awake.go, wait.go)"
grep -n 'exec.Command(caffeinate' "$WT/internal/awake/awake.go"
grep -n 'AwakeWithin = ' "$WT/internal/wait/wait.go"

echo; echo "## L101-114: the By hand sequence, as the operator at a terminal with no session, in a fresh project"
newrepo; "$B" init > /dev/null
git checkout -q -b w1 && echo x > x && git add x && git commit -q -m "W1 work" && SHA=$(git rev-parse HEAD)
echo "request" > "$WORK/req.md"; echo "looks right" > "$WORK/verdict.md"
: > "$WORK/alerts.txt"
echo; echo "\$ handoff wait --for reviewer   (started first, in the background, with --timeout 60s so the probe cannot hang)"
"$B" wait --for reviewer --timeout 60s > "$WORK/wait.out" 2>&1 &
WPID=$!; echo $WPID >> "$WORK/pids"
sleep 1
run "$B" post --as implementer --item W1 --to reviewer --commit "$SHA" --title "W1: a change" --body "$WORK/req.md"
for i in $(seq 1 100); do kill -0 $WPID 2>/dev/null || break; sleep 0.2; done
wait $WPID; wc=$?
echo "the wait ended with exit $wc, $((i / 5)) s or so after the post; it printed:"; cat "$WORK/wait.out"
run "$B" claim --as reviewer 1
run "$B" reply --as reviewer 1 --verdict APPROVE --commit "$SHA" --body "$WORK/verdict.md"
echo; echo "## L53: handoff with no arguments, not at a terminal: what waits"
run "$B"
echo; echo "## L53: handoff with no arguments at a terminal (script(1) gives it one; its stdin is kept open, since script ends the terminal's input when its own closes), answering n"
(sleep 1; echo n; sleep 3) | script -q "$WORK/script.out" "$B" > /dev/null 2>&1; echo "exit $?"; tr -d '\r' < "$WORK/script.out"
echo; echo "## L113: handoff decide W1 --approve, not at a terminal"
run "$B" decide W1 --approve
echo; echo "## L113: handoff decide W1 --approve at a terminal, answering y to its [y/N]"
(sleep 1; echo y; sleep 3) | script -q "$WORK/script.out" "$B" decide W1 --approve > /dev/null 2>&1; echo "exit $?"; tr -d '\r' < "$WORK/script.out"
run "$B"
run "$B" inbox
run "$B" show 4
echo; echo "## L94: repair on a sound record"
run "$B" repair
echo; echo "## L54: the alerts that went to the operator during the sequence"
cat "$WORK/alerts.txt"

echo; echo "## L118-120, L126: the files the README points to"
(cd "$WT" && ls -d docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md docs/CHANNEL.md docs/prompts/*.md LICENSE && head -1 LICENSE)

echo; echo "## L97: flags defined in the source against the flags handoff help names"
(cd "$WT" && grep -o 'fs\.\(String\|Bool\|Duration\)("[a-z]*"' cmd/handoff/*.go | grep -v _test | sed 's/.*("\(.*\)"/\1/' | sort -u | tr '\n' ' '); echo
"$B" help 2>&1 | grep -o -- '--[a-z]*' | sort -u | tr '\n' ' '; echo
echo "(prefer is ignored, as hook.go says; hook's --timeout, --as and --prefer are the hooks', set by what settings prints)"

echo; echo "## sleep holds asked for during the probes (none expected: nothing was owed for long):"; cat "$WORK/awake.txt" 2>/dev/null | sort | uniq -c
echo "## done"
}
main 2>&1 | clean
