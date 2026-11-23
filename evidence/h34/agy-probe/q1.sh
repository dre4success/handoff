#!/bin/zsh
# Q1: post N notes to implementer one at a time; each after the previous is handled
# (wakes.log has a line for it) and a wait listens again. Nobody types.
P=$1; N=$2; LOG=$3
export HANDOFF_TEST_NOTIFY=$P/notify.log
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_ROLE
cd $P/proj
lock=.handoff/presence/implementer.json.lock
calls() { grep -c 'streamGenerateContent' $P/agy-A.log }
wl() { [ -f wakes.log ] && wc -l < wakes.log | tr -d ' ' || echo 0 }
base=$(wl)
for k in $(seq 1 $N); do
  # wait for a listener
  for i in $(seq 1 120); do python3 $P/locked.py $lock >/dev/null && break; sleep 1; done
  echo "$(date -u +%H:%M:%S) step $k: listener $(python3 $P/locked.py $lock) pid $(python3 -c "import json;print(json.load(open('.handoff/presence/implementer.json'))['pid'])"), model calls so far $(calls)" >> $LOG
  echo "probe note $k at $(date -u +%H:%M:%S.%N)" > $P/body-$k.md
  handoff note --as reviewer --to implementer --item P1 --body $P/body-$k.md >> $LOG 2>&1
  t0=$(date +%s)
  until [ $(wl) -ge $((base+k)) ]; do
    if [ $(( $(date +%s) - t0 )) -gt 240 ]; then echo "$(date -u +%H:%M:%S) step $k: NOT HANDLED in 240 s; cursor $(cat .handoff/cursors/implementer.json 2>/dev/null); listener $(python3 $P/locked.py $lock)" >> $LOG; exit 1; fi
    sleep 1
  done
  echo "$(date -u +%H:%M:%S) step $k: handled after $(( $(date +%s) - t0 )) s; wakes.log: $(tail -1 wakes.log); model calls $(calls)" >> $LOG
done
sleep 20
echo "$(date -u +%H:%M:%S) done; listener $(python3 $P/locked.py $lock); model calls $(calls)" >> $LOG
