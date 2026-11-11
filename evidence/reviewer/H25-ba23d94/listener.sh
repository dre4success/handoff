#!/usr/bin/env bash
# The reviewer's probe 2 for H25 (ba23d94): row 91's account of why
# TestH12ACodexThreadKeepsItsRoleAlone is safe in parallel. The test's shell
# line, verbatim but for the timeout, run by hand with handoff built at the
# commit; then with the listener's binding sampled while it runs, with the
# listener killed, under CPU load, and with the listener refused the lock.
#   S=<scratchpad> B=<handoff> WT=<worktree> bash listener.sh > p2-listener.log
set -u
S=${S:?}; B=${B:?}; WT=${WT:?}
WORK=$(mktemp -d "$S/h25/lwork.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
clean() { sed -E "s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/<time>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g"; }
now() { perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
printf '{"session_id":"t-1","hook_event_name":"Stop"}' > "$WORK/stop"
printf '{"session_id":"t-2","hook_event_name":"SessionStart","source":"startup"}' > "$WORK/startup"
printf '{"session_id":"t-3","hook_event_name":"SessionStart","source":"clear"}' > "$WORK/clear"
printf '{"session_id":"t-4","hook_event_name":"SessionStart","source":"startup"}' > "$WORK/t4"
N=0
newrepo() { N=$((N+1)); REPO="$WORK/repo$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; CODEX_THREAD_ID=t-1 "$B" role implementer > /dev/null; }
# the test's shell line, verbatim but for the timeout
SH='"$0" hook listen --timeout '"'"'$4'"'"' <"$1" 2>/dev/null; for f in "$2" "$3"; do "$0" hook brief --prefer implementer <"$f"; done; true'
SH='"$0" hook listen --timeout "$4" <"$1" 2>/dev/null; for f in "$2" "$3"; do "$0" hook brief --prefer implementer <"$f"; done; true'
runsh() { # runsh <timeout> : runs the shell line in the current repo, output to $WORK/out
  local t0; t0=$(now)
  CODEX_THREAD_ID=t-1 /bin/sh -c "$SH" "$B" "$WORK/stop" "$WORK/startup" "$WORK/clear" "$1" > "$WORK/out" 2>&1; SHEXIT=$?
  SHSECS=$(perl -e "printf '%.2f', $(now) - $t0")
}
checks() { # the test's four checks, on $WORK/out and the repo
  echo "sh: exit $SHEXIT, took $SHSECS s; 'this session has no role in it' x$(grep -c 'this session has no role in it' "$WORK/out") (want 2); 'you are implementer' x$(grep -ci 'you are implementer' "$WORK/out") (want 0)"
  echo "t-1 role: $(CODEX_THREAD_ID=t-1 "$B" role 2>&1 | head -1)"
  CODEX_THREAD_ID=t-2 "$B" inbox > "$WORK/o" 2>&1; echo "t-2 inbox: exit $?: $(head -1 "$WORK/o")"
  CODEX_THREAD_ID=t-4 "$B" hook brief --prefer implementer < "$WORK/t4" > "$WORK/o" 2>&1; echo "t-4 hook brief after the parent ended: exit $?: $(head -1 "$WORK/o")"
  echo "t-1's binding: $(cat .handoff/sessions/t-1.json)"
}

main() {
printf '# command: the shell line of TestH12ACodexThreadKeepsItsRoleAlone by hand, in five ways; see each heading\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"
echo "## the order in hook listen (hook.go) and where wait's clock starts (wait.go)"
grep -n 'a.hookRole(\*as, in)\|startupDelay()\|a.listen(role, "hook"\|wait.Run(a.store' "$WT/cmd/handoff/hook.go"
grep -n 'a.update(s, b.Role)\|b.PID, b.Host = s.PID, s.Host' "$WT/cmd/handoff/session.go"
grep -n 'start := o.Clock.Now()' "$WT/internal/wait/wait.go"

echo; echo "## (a) as the test runs it: --timeout 1s"
newrepo; echo "t-1's binding before: $(cat .handoff/sessions/t-1.json)"
runsh 1s; checks
echo "the listener's own stderr, run once more on its own:"; CODEX_THREAD_ID=t-1 /bin/sh -c '"$0" hook listen --timeout 1s <"$1"' "$B" "$WORK/stop" 2>&1; echo "# exit $?"

echo; echo "## (b) --timeout 5s, t-1's binding sampled at 0.2 s, 1 s and 3 s while the listener runs"
newrepo
CODEX_THREAD_ID=t-1 /bin/sh -c "$SH" "$B" "$WORK/stop" "$WORK/startup" "$WORK/clear" 5s > "$WORK/out" 2>&1 &
SHPID=$!; echo $SHPID >> "$WORK/pids"; t0=$(now)
for d in 0.2 0.8 2; do sleep $d; L=$(pgrep -P $SHPID -f "hook listen" | head -1); echo "at $(perl -e "printf '%.1f', $(now) - $t0") s: listener pid alive: $([ -n "$L" ] && kill -0 "$L" 2>/dev/null && echo yes || echo no); its parent is the sh: $([ -n "$L" ] && [ "$(ps -o ppid= -p "$L" | tr -d ' ')" = "$SHPID" ] && echo yes || echo no); binding host is the sh's pid: $(grep -q "\"host\":$SHPID" .handoff/sessions/t-1.json && echo yes || echo no); presence files: $(ls .handoff/presence/ 2>/dev/null | tr '\n' ' ')"; done
wait $SHPID; SHEXIT=$?; SHSECS=$(perl -e "printf '%.2f', $(now) - $t0"); checks

echo; echo "## (c) --timeout 5s, the listener killed (SIGKILL) 0.3 s in: the briefs after it see the same binding"
newrepo
CODEX_THREAD_ID=t-1 /bin/sh -c "$SH" "$B" "$WORK/stop" "$WORK/startup" "$WORK/clear" 5s > "$WORK/out" 2>&1 &
SHPID=$!; echo $SHPID >> "$WORK/pids"; t0=$(now)
sleep 0.3; L=$(pgrep -P $SHPID -f "hook listen" | head -1); kill -9 "$L" 2>/dev/null && echo "killed the listener at $(perl -e "printf '%.1f', $(now) - $t0") s"
wait $SHPID; SHEXIT=$?; SHSECS=$(perl -e "printf '%.2f', $(now) - $t0"); checks

echo; echo "## (d) --timeout 1s under CPU load: 16 busy loops for the run (the race suite also runs on this machine now)"
newrepo
for i in $(seq 1 16); do yes > /dev/null 2>&1 & echo $! >> "$WORK/pids"; disown; done
sleep 0.5; echo "load average: $(sysctl -n vm.loadavg)"
runsh 1s; checks
while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; : > "$WORK/pids"

echo; echo "## (e) the listener refused its lock: another wait for implementer already listens in this project (the test never meets this: each test has its own repository)"
newrepo
CODEX_THREAD_ID=t-1 "$B" wait --timeout 30s > "$WORK/wait.out" 2>&1 &
WPID=$!; echo $WPID >> "$WORK/pids"; sleep 1; echo "presence: $(ls .handoff/presence/ | tr '\n' ' ')"
runsh 5s; checks
echo "(the sh took $SHSECS s: the listener retried the lock for about 6.5 s, as listen in session.go does for a role the operator gave (12 tries 250 ms apart, each of wait.Listen's 5 tries 50 ms apart), then gave up with exit 0, and the briefs ran)"
kill $WPID 2>/dev/null; wait $WPID 2>/dev/null

echo; echo "## (f) ten runs of (a) in a row, timed"
newrepo; for i in $(seq 1 10); do runsh 1s; printf '%s ' "$SHSECS"; grep -c 'this session has no role in it' "$WORK/out" | tr '\n' ' '; grep -ci 'you are implementer' "$WORK/out"; done | awk '{print "run " NR ": " $1 " s, no-role x" $2 ", implementer x" $3}'
echo "## alerts sent to the scratch file during the probes: $(cat "$WORK/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
echo "## done"
}
main 2>&1 | clean
