#!/usr/bin/env bash
# The reviewer's probes for H28 round 2 (b63d430): round 1's K1 to K11, unchanged, and K12 and K13 for the notifications. Run as
#   S=<scratchpad> B=<handoff built at the commit> OLD=<handoff built at 9315eb4, the code of v0.1.3> WT=<worktree> bash probes.sh > probes.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; OLD=${OLD:?older handoff}; WT=${WT:?worktree}
mkdir -p "$S/h28"
WORK=$(mktemp -d "$S/h28/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { sed -E "s|$OLD|handoff-v0.1.3|g; s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g; s/@ [0-9a-f]{12}/@ <sha12>/g; s/--commit [0-9a-f]{40}/--commit <sha>/g; s/^(commit|base): [0-9a-f]{40}/\1: <sha>/; s/^prev: [0-9a-f]{64}/prev: <hash>/; s/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/<ts>/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# as <role> <cmd...>: a command in that role's session
as() { local r=$1; shift; echo; echo "\$ [$r's session] handoff $*"; env CLAUDE_CODE_SESSION_ID=s-$r CLAUDE_PID=$(pid $r) "$B" "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# term [answer] -- <cmd...>: a command at a terminal (script(1)), as the operator's own; the answer, if any, is typed a second after it starts
term() { local ans=""; if [ "$1" != "--" ]; then ans=$1; shift; fi; shift; echo; echo "\$ [terminal] handoff $*${ans:+   (answering $ans)}"; if [ -n "$ans" ]; then (sleep 1; printf '%s\n' "$ans"; sleep 1) | script -q /dev/null "$B" "$@" > "$WORK/o" 2>&1; else script -q /dev/null "$B" "$@" < /dev/null > "$WORK/o" 2>&1; fi; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }
# listen <role>: that role's hook at the end of its turn
listen() { local r=$1; echo; echo "\$ [$r's session, at the end of its turn] handoff hook listen"; printf '{"session_id":"s-%s","hook_event_name":"Stop"}' "$r" | env CLAUDE_CODE_SESSION_ID=s-$r CLAUDE_PID=$(pid $r) "$B" hook listen --timeout 2s > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
sleeper() { sleep 900 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
body() { printf '%s\n' "${1:-ok}" > "$WORK/body.md"; echo "$WORK/body.md"; }
alerts() { echo "# alerts sent so far: $( [ -f "$WORK/alerts.txt" ] && wc -l < "$WORK/alerts.txt" | tr -d ' ' || echo 0)"; [ -f "$WORK/alerts.txt" ] && sed 's/^/#   /' "$WORK/alerts.txt"; }
pid() { cat "$WORK/pid.$1"; }
N=0
# newrepo <roles with a session...>: a scratch project, with a session for each role named
newrepo() { N=$((N+1)); REPO="$WORK/k$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; : > "$WORK/alerts.txt"; for r in "$@"; do sleeper > "$WORK/pid.$r"; env CLAUDE_CODE_SESSION_ID=s-$r CLAUDE_PID=$(pid $r) "$B" role $r > /dev/null 2>&1; done; echo "# a scratch project; sessions: ${*:-none}"; }
# commit <name>: a commit on a branch of its own; prints its sha
commit() { git checkout -q -b "$1" main 2>/dev/null || git checkout -q "$1"; git commit -q --allow-empty -m "$1"; git rev-parse HEAD; git checkout -q main; }
post() { local item=$1 to=$2 sha=$3; shift 3; as implementer post --item "$item" --to "$to" --commit "$sha" --title "$item work" --body "$(body)" "$@"; }
reply() { local r=$1 id=$2 v=$3 sha=$4; as "$r" reply "$id" --verdict "$v" --commit "$sha" --body "$(body)"; }
state() { echo; echo "# each request's state, from the inboxes:"; for r in implementer reviewer reviewer-2 operator; do "$B" inbox --for $r 2>&1 | awk -v r=$r '/^[a-z].*:$/{h=$0} /^  #/{print "#   " r ": " h " " $0}'; done | sort -u; }
main() {
printf '# command: the reviewer'"'"'s probes for H28 round 2, handoff away and back; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"

###############################################################################
echo "## K1. The README's own go-ahead, \"Merge when both reviewers approve.\", and an item sent to one reviewer"
newrepo implementer reviewer reviewer-2
term -- away "Merge when both reviewers approve."
echo; echo "# the implementer's brief on whom to ask, with both reviewers in session:"
env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" brief 2>&1 | grep -E '^To ask for a review'
A=$(commit w1)
post W1 reviewer $A
reply reviewer 5 APPROVE $A
alerts
listen implementer
run "$B" show 6
state

###############################################################################
echo; echo "## K2. The same go-ahead; the second reviewer is asked after the first has approved"
echo "# (W1 above is approved. Now the implementer sends W1, the same commit, to reviewer-2.)"
post W1 reviewer-2 $A
state
reply reviewer-2 7 CHANGES $A
alerts
listen implementer
state
term -- back

###############################################################################
echo; echo "## K3. A reviewer takes its approval back before the item is complete"
newrepo implementer reviewer reviewer-2
term -- away "Merge when both reviewers approve."
A=$(commit w3)
post W3 reviewer $A; post W3 reviewer-2 $A
reply reviewer 5 APPROVE $A
reply reviewer 5 CHANGES $A
reply reviewer-2 6 APPROVE $A
alerts
state
listen implementer
term -- back

###############################################################################
echo; echo "## K4. A fix for one reviewer: approvals at two commits, then each reviewer asked again"
newrepo implementer reviewer reviewer-2
term -- away "Merge when both reviewers approve."
A=$(commit w4)
post W4 reviewer $A; post W4 reviewer-2 $A
reply reviewer 5 APPROVE $A
reply reviewer-2 6 CHANGES $A
alerts
Bc=$(commit w4)
post W4 reviewer-2 $Bc --supersedes 6
reply reviewer-2 9 APPROVE $Bc
alerts
echo; echo "\$ handoff   (no terminal: what the operator would see)"; "$B" < /dev/null 2>&1
echo; echo "# the implementer now does as its brief says, and asks the first reviewer again at the fix:"
post W4 reviewer $Bc --supersedes 5
reply reviewer 11 APPROVE $Bc
alerts
listen implementer
echo; echo "\$ handoff   (no terminal)"; "$B" < /dev/null 2>&1
state
echo; echo "# the two pop-ups raised above are for #5 and #9. A click on one of them now:"
term -- decide 9 --deny
term -- back

###############################################################################
echo; echo "## K5. away when something is approved already, and the set changes while the operator is asked"
newrepo implementer reviewer
A=$(commit w5); A6=$(commit w6)
post W5 reviewer $A; post W6 reviewer $A6
reply reviewer 1 APPROVE $A
alerts
echo; echo "# away is started; while its question is up (the answer comes a second later), W6 is approved too:"
( sleep 0.4; env CLAUDE_CODE_SESSION_ID=s-reviewer CLAUDE_PID=$(pid reviewer) "$B" reply 2 --verdict APPROVE --commit $A6 --body "$(body)" > "$WORK/late.out" 2>&1 ) & LATE=$!
term y -- away "Merge when the reviewer approves."
wait $LATE; echo "# the late approval said: $(tr '\n' ' ' < "$WORK/late.out")"
echo "# lines in the record: $(wc -l < .handoff/channel.jsonl | tr -d ' '); of kind away: $(grep -c '"kind":"away"' .handoff/channel.jsonl)"
term n -- away "Merge when the reviewer approves."
echo "# of kind away: $(grep -c '"kind":"away"' .handoff/channel.jsonl)"
term y -- away "Merge when the reviewer approves."
echo; echo "# what the implementer is given: the two APPROVEs came before the go-ahead, so they carry no line; the fyi's first line; and the fyi in full"
listen implementer
run "$B" show 6
run "$B" show 3
echo; echo "# a session that starts now, as implementer, with no fyi to it: what its brief says of W5 and W6"
sleeper > "$WORK/pid.implementer"; env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" role implementer > /dev/null 2>&1
env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" brief 2>&1 | grep -n -E 'away|go-ahead|W5|W6|awaiting the operator|\(none\)' | sed -n 1,14p
term -- back

###############################################################################
echo; echo "## K6. back while an approval waits quietly for the other reviewer; away twice; back twice"
newrepo implementer reviewer reviewer-2
term -- away "First words."
A=$(commit w7)
post W7 reviewer $A; post W7 reviewer-2 $A
reply reviewer 5 APPROVE $A
alerts
term -- away "Second words, on a second line too." "More."
echo; echo "\$ handoff   (no terminal)"; "$B" < /dev/null 2>&1
term -- back
alerts
term -- back
echo; echo "# reviewer-2 approves after back:"
reply reviewer-2 6 APPROVE $A
alerts
state

###############################################################################
echo; echo "## K7. No session when the go-ahead is given; sessions that start later"
newrepo
term -- away "Merge when the reviewer approves." "Don't push."
for r in implementer reviewer; do sleeper > "$WORK/pid.$r"; env CLAUDE_CODE_SESSION_ID=s-$r CLAUDE_PID=$(pid $r) "$B" role $r > /dev/null 2>&1; done
echo; echo "# the brief each new session gets (its second line), and what its first listen brings:"
printf '{"session_id":"s-implementer","hook_event_name":"SessionStart","source":"startup"}' | env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" hook brief 2>&1 | sed -n 2p
listen implementer
A=$(commit w8)
post W8 reviewer $A
reply reviewer 2 APPROVE $A
alerts
listen implementer
term -- back

###############################################################################
echo; echo "## K8. A go-ahead whose words say not to merge"
newrepo implementer reviewer
term -- away "Do not merge anything while I'm away. Review only."
A=$(commit w9)
post W9 reviewer $A
reply reviewer 4 APPROVE $A
alerts
listen implementer
term -- back

###############################################################################
echo; echo "## K9. Who can give the go-ahead"
newrepo implementer reviewer
as implementer away "Merge everything."
run "$B" away "Merge everything."
run env HANDOFF_ROLE=operator "$B" away "Merge everything."
run env HANDOFF_ROLE=reviewer "$B" back
echo "# lines in the record after these: $( [ -f .handoff/channel.jsonl ] && wc -l < .handoff/channel.jsonl | tr -d ' ' || echo 0)"
echo; echo "# the way round the guard that decide and fyi have (H26, P3b): a shell in the implementer's session drops its session's variables and wraps the command in script(1)"
echo "\$ [implementer's session] env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null handoff away \"Merge everything.\""
env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) sh -c 'env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null "$0" away "Merge everything."' "$B" < /dev/null 2>&1 | tr -d '\r' | sed 's/^\^D//'
A=$(commit w10)
post W10 reviewer $A
reply reviewer 4 APPROVE $A
alerts
listen implementer

###############################################################################
echo; echo "## K10. An older build (the code of v0.1.3) on the same record, while the go-ahead stands"
echo "# (the record of K9: W10 is approved under the go-ahead)"
echo; echo "\$ handoff-v0.1.3   (no terminal)"; "$OLD" < /dev/null 2>&1
echo; echo "\$ [terminal] handoff-v0.1.3 decide W10 --deny   (answering y)"; (sleep 1; printf 'y\n'; sleep 1) | script -q /dev/null "$OLD" decide W10 --deny 2>&1 | tr -d '\r' | sed 's/^\^D//'
echo; echo "# and this build, after that:"
echo "\$ handoff   (no terminal)"; "$B" < /dev/null 2>&1
run "$B" show 5
state
term -- back
echo; echo "# alerts sent in K10's project: "; alerts

###############################################################################
echo; echo "## K11. A question to the operator while away: the go-ahead decides no note"
newrepo implementer reviewer
term -- away "Merge when the reviewer approves."
as implementer note --to operator --item W11 --title "May I add a dependency?" --body "$(body "Yes or no.")"
alerts
echo; echo "\$ handoff   (no terminal)"; "$B" < /dev/null 2>&1
term y -- decide 4 --deny --note "No."
listen implementer
term -- back

###############################################################################
echo; echo "## K12. The notifications: plain, never a pop-up, and once each"
echo "# Here pop-ups go to a second scratch file, as a click-less pop-up would go to the screen, so the two kinds can be told apart."
newrepo implementer reviewer reviewer-2
export HANDOFF_TEST_POPUP="$WORK/popups.txt"; : > "$WORK/popups.txt"
both() { alerts; echo "# pop-ups raised so far: $(grep -c . "$WORK/popups.txt")"; sed 's/^/#   /' "$WORK/popups.txt"; }
term -- away "Merge when the reviewers approve."
both
A=$(commit w12); A2=$(commit w13)
post W12 reviewer $A; post W12 reviewer-2 $A; post W13 reviewer $A2
reply reviewer 5 APPROVE $A
echo "# one of two has approved W12:"; both
reply reviewer-2 6 APPROVE $A
reply reviewer 7 APPROVE $A2
both
echo; echo "# every command and listener that looks for alerts, run again and again:"
for i in 1 2 3; do "$B" inbox --for operator > /dev/null 2>&1; "$B" < /dev/null > /dev/null 2>&1; env HANDOFF_ROLE=reviewer "$B" wait --for reviewer --timeout 2s > /dev/null 2>&1; printf '{"session_id":"s-implementer","hook_event_name":"Stop"}' | env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" hook listen --timeout 2s > /dev/null 2>&1; done
as reviewer-2 note --item W13 --to implementer --title "a note, which looks for alerts too" --body "$(body)"
both
echo "# the markers that record what was sent: $(find .handoff -type f \( -name 'away-*' -o -name 'go-ahead-*' \) | sed 's|.*/||' | sort | tr '\n' ' ')"
echo; echo "\$ handoff   (no terminal)"; "$B" < /dev/null 2>&1
echo; echo "# a second go-ahead in the same spell away: its own notice, and none again for W12 or W13"
term -- away "Second words."
both
term -- back
echo; echo "# after back, an approval asks as before, in a pop-up:"
A3=$(commit w14); post W14 reviewer $A3
reply reviewer 20 APPROVE $A3
sleep 1; both
touch "$WORK/popups.txt.release"; sleep 1; unset HANDOFF_TEST_POPUP

###############################################################################
echo; echo "## K13. An item approved by the away entry itself: what each agent is given, then and at back"
newrepo implementer reviewer
A=$(commit w15)
post W15 reviewer $A
reply reviewer 1 APPROVE $A
listen implementer
term y -- away "Merge when the reviewer approves."
alerts
listen implementer
listen reviewer
run "$B" show 4
echo; echo "# a session that starts now, as implementer: its brief's line on the go-ahead"
sleeper > "$WORK/pid.implementer"; env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" role implementer > /dev/null 2>&1
env CLAUDE_CODE_SESSION_ID=s-implementer CLAUDE_PID=$(pid implementer) "$B" brief 2>&1 | grep -E '^The operator is away'
term -- back
listen implementer
alerts
}
main 2>&1 | clean
