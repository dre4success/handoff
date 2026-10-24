#!/usr/bin/env bash
# H13 live check: a real `wait` and a real `hook listen` hold the Mac awake
# through the real caffeinate while an agent owes a move, and let go when
# nothing is owed, when a hook is woken, and when a wait is killed.
#   bash evidence/h13/live-awake.sh <handoff binary> > evidence/h13/live-awake.txt
# Alerts go to a file (HANDOFF_TEST_NOTIFY). The holds are real: for a few
# seconds each, this Mac does not idle-sleep.
set -u
H=${1:?usage: live-awake.sh <handoff binary>}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'kill -9 $(jobs -p) 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE HANDOFF_TEST_AWAKE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
P="$SCRATCH/p"
mkdir "$P" && cd "$P" || exit 1
git init -q -b main && echo base > f && git add f && git commit -qm base
git switch -qc w1 && echo a > f && git commit -qam A && A=$(git rev-parse HEAD)
"$H" init --roles implementer,reviewer,reviewer-2 --reviewers reviewer,reviewer-2 --main main > /dev/null || exit 1
echo "please review" > "$SCRATCH/req.md"; echo ok > "$SCRATCH/ok.md"

stamp() { echo; echo "## $(date -u +%H:%M:%SZ) $*"; }
caf() { pgrep -f "^/usr/bin/caffeinate -i -w $1\$"; }
# held <pid> <yes|no>: waits up to 10 s for a caffeinate tied to pid to be
# there (yes) or gone (no), then shows it.
held() {
  for _ in $(seq 50); do
    c=$(caf "$1")
    { [ "$2" = yes ] && [ -n "$c" ]; } || { [ "$2" = no ] && [ -z "$c" ]; } && break
    sleep 0.2
  done
  if [ -n "$c" ]; then
    echo "caffeinate for pid $1:"; ps -o pid=,ppid=,args= -p "$c" | sed 's/^/  ps: /'
    pmset -g assertions | grep -E "pid $c\(caffeinate\)" | sed -E 's/\[0x[0-9a-f]+\] //; s/^ */  pmset: /'
    lsof -p "$c" -a -d 0-9 -Fn 2>/dev/null | grep '^n' | sort | uniq -c | sed 's/^ */  open: /'
  else
    echo "no caffeinate for pid $1"
  fi
}

stamp "a wait for reviewer-2, with nothing owed"
"$H" wait --for reviewer-2 --timeout 300s > "$SCRATCH/w.out" 2> "$SCRATCH/w.err" &
W=$!
sleep 3
held $W no

stamp "the implementer posts a request to the reviewer: the reviewer owes a verdict"
"$H" post --as implementer --item W1 --to reviewer --commit "$A" --title "W1 work" --body "$SCRATCH/req.md"
held $W yes

stamp "a hook listens for the implementer, as Claude Code's Stop hook runs it"
echo '{"hook_event_name":"Stop"}' | HANDOFF_ROLE=implementer "$H" hook listen --timeout 300s > /dev/null 2> "$SCRATCH/l.err" &
L=$!
held $L yes

stamp "the reviewer approves: only the operator owes a move now"
"$H" reply --as reviewer 1 --verdict APPROVE --commit "$A" --body "$SCRATCH/ok.md"
wait $L; echo "the hook exited $? (2 wakes the session); it told the session:"; sed -e 's/^/  /' -e "s|$H|<binary>|g" "$SCRATCH/l.err"
held $L no
held $W no
kill -0 $W && echo "the wait for reviewer-2 still runs, pid $W"

stamp "a new request, W2, while W1 waits for the operator: the wait holds again"
git switch -qc w2 main && echo b > f && git commit -qam B && B=$(git rev-parse HEAD)
"$H" post --as implementer --item W2 --to reviewer --commit "$B" --title "W2 work" --body "$SCRATCH/req.md"
held $W yes

stamp "the wait is killed with SIGKILL: caffeinate -w ends with it"
kill -9 $W; wait $W 2>/dev/null
held $W no
echo; echo "the wait's stderr, all of it:"; sed 's/^/  /' "$SCRATCH/w.err"; echo "(end)"
echo "alerts sent to the file, not the screen: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
