#!/usr/bin/env bash
# The reviewer's probes for H29 round 2 (b8e1c7e): the shorter README, claim by claim. Run as
#   S=<scratchpad> B=<handoff built at the commit> WT=<worktree at the commit> bash probes.sh > probes.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; WT=${WT:?worktree}
mkdir -p "$S/h29"
WORK=$(mktemp -d "$S/h29/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { sed -E "s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s|$WT|<worktree>|g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
runin() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ env ${envs[*]} $*"; env "${envs[@]}" "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
term() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ [terminal] ${envs[*]:+env ${envs[*]} }$*"; env ${envs[@]+"${envs[@]}"} script -q /dev/null "$@" < /dev/null > "$WORK/o" 2>&1; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }
hook() { local json=$1; shift; local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ echo '$json' | env ${envs[*]} handoff hook $*"; printf '%s' "$json" | env "${envs[@]}" "$B" hook "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
stop() { printf '{"session_id":"%s","hook_event_name":"Stop"}' "$1"; }
start() { printf '{"session_id":"%s","hook_event_name":"SessionStart","source":"%s"}' "$1" "$2"; }
sleeper() { sleep 600 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
body() { printf '%s\n' "$1" > "$WORK/body.md"; echo "$WORK/body.md"; }
readme() { sed -n "$1p" "$WT/README.md"; }
sha() { sed -E 's/@ [0-9a-f]{12}/@ <sha12>/g; s/--commit [0-9a-f]{40}/--commit <sha>/g' "$@"; }
main() {
printf '# command: the reviewer'"'"'s probes for H29, the README at this commit, claim by claim; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"
echo "# Each heading quotes the README's line (its number first), then runs it."

###############################################################################
echo; echo "## G1. Install"
echo "13: $(readme 13)"
echo "# go.mod: $(grep '^go ' "$WT/go.mod")"
mkdir "$WORK/nogit"; cd "$WORK/nogit"
run "$B" init
echo "# and inside a git repository with no commit yet:"
git init -q -b main "$WORK/empty"; cd "$WORK/empty"
run "$B" init

###############################################################################
echo; echo "## G2. Quick start, as written"
echo "17: $(readme 17)"
git init -q -b main "$WORK/quick"; git -C "$WORK/quick" commit -q --allow-empty -m base; cd "$WORK/quick"
echo; echo "\$ handoff settings claude --global   (the lines before the settings themselves)"
"$B" settings claude --global 2>&1 | awk '/^[{[]/{exit} {print}'
echo; echo "\$ handoff settings codex --global   (the lines that are not the files' contents)"
"$B" settings codex --global 2>&1 | grep -v -E '^[][{}" ]|^$|^(\[|#)' | sed -n 1,14p
echo; echo "# step 1 done here: the JSON it prints, saved as the scratch ~/.claude/settings.json"
"$B" settings claude --global 2>/dev/null | awk '/^{/{p=1} p' > "$CLAUDE_CONFIG_DIR/settings.json"
python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); print("# it parses as JSON; hooks for:", ", ".join(sorted(d.get("hooks",{}))))' "$CLAUDE_CONFIG_DIR/settings.json"
echo; echo "18: $(readme 18)"
run "$B" init
echo "# git status --short --ignored: $(git status --short --ignored | tr '\n' ' ')"
echo "# roles in .handoff/config.json: $(grep -o '"name": *"[^"]*"' .handoff/config.json | tr '\n' ' '); quiet_after: $(grep -o '"quiet_after": *"[^"]*"' .handoff/config.json)"
echo; echo "19: $(readme 19)"
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
echo; echo "20: $(readme 20)"
git checkout -q -b w1; git commit -q --allow-empty -m "feat: the task"; SHA=$(git rev-parse HEAD); git checkout -q main
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" post --item W1 --to reviewer --commit $SHA --title "the task" --body "$(body "Please review.")" | sha
hook "$(stop s-rev)" CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- listen --timeout 3s | sha
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" claim 1
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" reply 1 --verdict APPROVE --commit $SHA --body "$(body "Fine.")" | sha

###############################################################################
echo; echo "## G3. Deciding"
echo "29: $(readme 29)"
echo "# the alerts sent so far (to the scratch file), one for the approval that waits:"
sha "$WORK/alerts.txt" 2>/dev/null
echo "# the pop-up's buttons, from popup.go:"; grep -n -o 'show(title, popupText(t), \[\]string{[^}]*}' "$WT/cmd/handoff/popup.go"
echo; echo "\$ handoff   (in the project, no terminal: it lists)"
"$B" < /dev/null 2>&1 | sha
echo; echo "\$ [terminal] handoff   (answering a, for approve)"
(sleep 1; printf 'a\n'; sleep 1) | script -q /dev/null "$B" 2>&1 | tr -d '\r' | sed 's/^\^D//' | sha
run "$B" show 4 | sha
echo; echo "# a Claude Code session stuck at a permission prompt (hook notify):"
: > "$WORK/alerts.txt"
hook '{"session_id":"s-impl","hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission to use Bash"}' CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- notify
echo "# alerts sent by it:"; cat "$WORK/alerts.txt"
echo; echo "# an agent that has owed a move for quiet_after (set to 1s here; the default above is 30m):"
: > "$WORK/alerts.txt"
git checkout -q -b w2; git commit -q --allow-empty -m "feat: another"; SHA2=$(git rev-parse HEAD); git checkout -q main
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" post --item W2 --to reviewer --commit $SHA2 --title "another" --body "$(body "Please review.")" | sha
python3 -I -c 'import json,sys; p=".handoff/config.json"; c=json.load(open(p)); c["quiet_after"]="1s"; json.dump(c,open(p,"w"),indent=2)'
sleep 2
runin HANDOFF_ROLE=implementer -- "$B" wait --for implementer --timeout 2s | sha
echo "# alerts sent:"; sha "$WORK/alerts.txt"

###############################################################################
echo; echo "## G4. While you're away"
echo "39: $(readme 39)"
term -- "$B" fyi "Carry on while I'm away."
hook "$(stop s-impl)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- listen --timeout 3s | grep -E 'fyi|exit'
echo "# the brief's line for it:"; env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" brief 2>&1 | grep -w 'fyi'

###############################################################################
echo; echo "## G5. Good to know"
echo "43: $(readme 43)"
echo "# the brief a set-up session gets when it starts (the SessionStart hook of step 1's settings), its lines on listening and its rules:"
printf '%s' "$(start s-rev startup)" | env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C "$B" hook brief 2>&1 | grep -E '^(Listening|Rules):'
echo "# and handoff brief, typed in that session, shows: $(env CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C "$B" brief 2>&1 | sed -n 1p)"
echo; echo "44: $(readme 44)"
echo "# a resume: the same id, a new process"
kill $C; wait $C 2>/dev/null; C=$(sleeper)
hook "$(start s-rev resume)" CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- brief | sed -n 1,3p
echo "# /clear in Claude Code: a new id, the same process"
hook "$(start s-rev-2 clear)" CLAUDE_CODE_SESSION_ID=s-rev-2 CLAUDE_PID=$C -- brief | sed -n 1,3p
echo "# a new Codex thread, after one that had the role:"
runin CODEX_THREAD_ID=t-one -- "$B" role reviewer-2 | sed -n 1,3p
runin CODEX_THREAD_ID=t-two -- "$B" role | sed -n 1,4p
echo "# giving the role to another session takes it from the first:"
D=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-other CLAUDE_PID=$D -- "$B" role reviewer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev-2 CLAUDE_PID=$C -- "$B" role | sed -n 1,4p
echo; echo "45: $(readme 45)"
env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" brief 2>&1 | grep -o -E 'commit your work on a branch of its own, not on main; you don.t need to ask the operator first|Never merge, push or deploy without the operator.s decision'
echo; echo "46: $(readme 46)"
run "$B" roles add reviewer-3
git init -q -b main "$WORK/ab"; git -C "$WORK/ab" commit -q --allow-empty -m base
(cd "$WORK/ab" && "$B" init --roles a,b 2>&1 | sed -n 1,3p)
echo; echo "47: $(readme 47)"
echo "# what settings codex --global says of the hooks and the sandbox:"
"$B" settings codex --global 2>&1 | grep -i -E 'trust all|sandbox' | sed -n 1,6p
echo "# a Codex thread with a role and no hooks, its brief's line on listening:"
env CODEX_THREAD_ID=t-one "$B" brief 2>&1 | grep -E '^Listening:'
echo; echo "48: $(readme 48)"
echo "# the limit, from the source: $(grep -n 'const AwakeWithin' "$WT/internal/wait/wait.go")"
echo "# sleep holds taken in this run (the scratch file): $( [ -f "$WORK/awake.txt" ] && wc -l < "$WORK/awake.txt" | tr -d ' ' || echo 0)"

###############################################################################
echo; echo "## G6. Commands"
echo "# the README's table against handoff help, both ways"
tbl=$(sed -n '/^## Commands/,/^`handoff help`/p' "$WT/README.md" | grep '^| `' | cut -d'|' -f2 | grep -o '`[a-z]*' | tr -d '`' | sort -u | tr '\n' ' ')
hlp=$("$B" help 2>&1 | grep -E '^  [a-z]+( |$)' | awk '{print $1}' | sort -u | tr '\n' ' ')
echo "# in the table: $tbl"
echo "# in help:      $hlp"
for c in $tbl; do [ "$c" = handoff ] && continue; case " $hlp " in *" $c "*) ;; *) echo "# in the table, not in help: $c";; esac; done
for c in $hlp; do case " $tbl " in *" $c "*) ;; *) echo "# in help, not in the table: $c";; esac; done
echo; echo "# what the README dropped and help or the tool still says:"
"$B" help 2>&1 | grep -n -E -e '--reviewers|--as <role>\]|--global'
run "$B" roles remove reviewer-3
run "$B" roles rename reviewer-3 r3
echo; echo "\$ handoff settings codex   (one project alone: what it says of git)"
"$B" settings codex 2>&1 | grep -i -E 'git|exclude|trust' | sed -n 1,6p
echo; echo "\$ handoff version"; "$B" version

###############################################################################
echo; echo "## G7. More"
echo "70: $(readme 70)"
for p in docs/proposals docs/CHANNEL.md docs/prompts LICENSE; do [ -e "$WT/$p" ] && echo "# $p: there" || echo "# $p: MISSING"; done
echo "# docs/prompts holds: $(ls "$WT/docs/prompts" | tr '\n' ' ')"
echo "# LICENSE's first line: $(sed -n 1p "$WT/LICENSE")"
kill $A $C $D 2>/dev/null
}
main 2>&1 | clean
