#!/usr/bin/env bash
# The reviewer's probes for H27 (9315eb4): tell is now fyi. Run as
#   S=<scratchpad> B=<handoff built at the commit> H26=<handoff built at the base, 3119db5, which has tell> OLD=<handoff v0.1.2> WT=<worktree> bash probes.sh > probes.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; H26=${H26:?handoff at the base}; OLD=${OLD:?handoff v0.1.2}; WT=${WT:?worktree}
mkdir -p "$S/h27"
WORK=$(mktemp -d "$S/h27/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { sed -E "s|$H26|handoff-h26|g; s|$OLD|handoff-v0.1.2|g; s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# runin <env...> -- <cmd...>: a command in a session's environment
runin() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ env ${envs[*]} $*"; env "${envs[@]}" "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# term <env...> -- <cmd...>: a command at a terminal (script(1)), as the operator's own
term() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ [terminal] ${envs[*]:+env ${envs[*]} }$*"; env ${envs[@]+"${envs[@]}"} script -q /dev/null "$@" < /dev/null > "$WORK/o" 2>&1; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }
# hookby <binary> <json> <env...> -- <hook args...>: a hook of that build, fed what the CLI writes to its stdin
hookby() { local bin=$1 json=$2; shift 2; local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ echo '$json' | env ${envs[*]} $bin hook $*"; printf '%s' "$json" | env "${envs[@]}" "$bin" hook "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
hook() { hookby "$B" "$@"; }
stop() { printf '{"session_id":"%s","hook_event_name":"Stop"}' "$1"; }
sleeper() { sleep 600 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
N=0
newrepo() { N=$((N+1)); REPO="$WORK/repo$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; }
body() { printf '%s\n' "$1" > "$WORK/body.md"; echo "$WORK/body.md"; }
lines() { wc -l < .handoff/channel.jsonl | tr -d ' '; }
rec() { echo; echo "# the record:"; sed -E 's/"ts":"[^"]*",//; s/"v":1,//' .handoff/channel.jsonl | clean; echo "# the cursors: $(for f in .handoff/cursors/*.json; do [ -f "$f" ] && printf '%s %s  ' "$(basename "$f" .json)" "$(sed -E 's/.*"id":([0-9]+).*/#\1/' "$f")"; done)"; }
main() {
printf '# command: the reviewer'"'"'s probes for H27, handoff fyi; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"
echo "# handoff is this commit's build; handoff-h26 is the base's (3119db5, the build installed on this Mac now, which has tell); handoff-v0.1.2 is the last release"

###############################################################################
echo; echo "## F1. fyi at a terminal: a session given its role, a role held by a listener alone, a role with no session"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
"$B" wait --for reviewer --timeout 20s > "$WORK/w.out" 2>&1 < /dev/null & W=$!; echo $W >> "$WORK/pids"
sleep 1
term -- "$B" fyi "Carry on while I'm away." "Back at 9."
wait $W; echo; echo "# the reviewer's wait, outside any session, running since before the fyi: exit $?"; cat "$WORK/w.out"
hook "$(stop s-impl)" CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- listen --timeout 3s
run "$B" show 1
run "$B" show 2
rec
for r in implementer reviewer reviewer-2 operator; do "$B" inbox --for $r --quiet > /dev/null 2>&1; echo "# inbox --for $r --quiet: exit $? (0: nothing waits)"; done
echo "# alerts sent: $( [ -f "$WORK/alerts.txt" ] && wc -l < "$WORK/alerts.txt" | tr -d ' ' || echo 0); sleep holds taken: $( [ -f "$WORK/awake.txt" ] && wc -l < "$WORK/awake.txt" | tr -d ' ' || echo 0)"
echo; echo "# the brief's lines that name fyi or tell (brief in s-impl):"
env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" brief 2>&1 | grep -n -w -E 'fyi|tell'
echo "# a decision on an fyi's id:"
term -- "$B" decide 1 --approve
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## F2. tell is gone: no alias"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
echo; echo "\$ [terminal] handoff tell \"Carry on.\"   (first two lines; the usage follows)"
script -q /dev/null "$B" tell "Carry on." < /dev/null 2>&1 | tr -d '\r' | sed 's/^\^D//' | sed -n 1,2p; echo "# exit ${PIPESTATUS[0]}"
echo; echo "\$ handoff tell \"Carry on.\"   (no terminal; first line)"
"$B" tell "Carry on." < /dev/null 2>&1 | sed -n 1p; echo "# exit ${PIPESTATUS[0]}"
echo; echo "\$ env CLAUDE_CODE_SESSION_ID=s-impl handoff tell \"Carry on.\"   (in the session; first line)"
env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" tell "Carry on." < /dev/null 2>&1 | sed -n 1p
echo "# lines in the record after these: $( [ -f .handoff/channel.jsonl ] && lines || echo 0)"
echo; echo "# handoff help: every line with the word tell or fyi"
"$B" help 2>&1 | grep -n -w -E 'tell|fyi'
echo "# handoff fyi -h prints the same usage: $("$B" fyi -h 2>&1 | grep -c -w fyi) lines with fyi, exit $("$B" fyi -h > /dev/null 2>&1; echo $?)"
echo "# v0.1.2, the last release, has neither:"
"$OLD" tell x 2>&1 | sed -n 1p; "$OLD" fyi x 2>&1 | sed -n 1p
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## F3. A tell already in a record, written by the base's build, read by this one (row 94: \"would be delivered by role, as before row 93\")"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$H26" role implementer | sed -n 1,3p
term -- "$H26" tell "Merge when the reviewer approves."
echo; echo "### F3a. The session told listens with this build: it gets its tell"
cp -R .handoff "$WORK/h3"   # the same record again for F3b
hook "$(stop s-one)" CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- listen --timeout 3s
echo; echo "### F3b. The same record, but the session told ends unread and a later session takes the role, with this build: it gets the old tell (round 1's P1a, for a tell only)"
rm -rf .handoff; cp -R "$WORK/h3" .handoff
kill $A; wait $A 2>/dev/null
runin CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
hook "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 3s
run "$B" show 1
echo "# nothing waits for anyone, and the record still takes entries:"
for r in implementer reviewer operator; do "$B" inbox --for $r --quiet > /dev/null 2>&1; echo "# inbox --for $r --quiet: exit $?"; done
runin HANDOFF_ROLE=reviewer -- "$B" note --item W1 --to implementer --title "after a tell" --body "$(body x)"
term -- "$B" fyi "And an fyi after it."
rec
kill $C; wait $C 2>/dev/null

###############################################################################
echo; echo "## F4. An fyi written by this build, met by a listener of an older build (the time between the install and each session's next start)"
echo; echo "### F4a. The session told, whose listener is the base's build and was running before the fyi (a session idle since before the install): it gets its fyi"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$H26" role implementer | sed -n 1,3p
echo "# its brief, from the base's build, names: $(env CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A "$H26" brief 2>&1 | grep -c 'A tell from the operator') line for a tell, $(env CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A "$H26" brief 2>&1 | grep -c -w 'fyi') for an fyi"
( printf '%s' "$(stop s-one)" | env CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A "$H26" hook listen --timeout 20s > "$WORK/l.out" 2>&1 ) & L=$!; echo $L >> "$WORK/pids"
sleep 1
term -- "$B" fyi "Carry on while I'm away."
wait $L; echo; echo "# s-one's hook listen, the base's build, running since before: exit $?"; cat "$WORK/l.out"
echo; echo "### F4b. A second fyi to s-one, unread; s-one ends; a later session takes the role. Its listener is this build: nothing. Then the same record with the base's build, and with v0.1.2, as the later session's listener: delivered by role"
term -- "$B" fyi "Merge when the reviewer approves."
kill $A; wait $A 2>/dev/null
runin CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
cp -R .handoff "$WORK/h4"
hook "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 3s
rm -rf .handoff; cp -R "$WORK/h4" .handoff
hookby "$H26" "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 3s
rm -rf .handoff; cp -R "$WORK/h4" .handoff
hookby "$OLD" "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 3s
echo; echo "### F4c. The older builds read a record with an fyi in it"
run "$H26" show 2
run "$OLD" show 2
for r in implementer reviewer operator; do "$H26" inbox --for $r --quiet > /dev/null 2>&1; echo "# handoff-h26 inbox --for $r --quiet: exit $?"; done
runin HANDOFF_ROLE=reviewer -- "$OLD" note --item W1 --to implementer --title "after an fyi, by v0.1.2" --body "$(body x)"
rec
kill $C; wait $C 2>/dev/null

###############################################################################
echo; echo "## F5. The message: words that are also commands, and the old name as the message"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" fyi tell everyone: no merges tonight
term -- "$B" fyi fyi
term -- "$B" fyi "-fyi"
term -- "$B" fyi --
rec
kill $A; wait $A 2>/dev/null
echo; echo "# alerts sent in every probe above: $( [ -f "$WORK/alerts.txt" ] && wc -l < "$WORK/alerts.txt" | tr -d ' ' || echo 0) (to the scratch file only)"
}
main 2>&1 | clean
