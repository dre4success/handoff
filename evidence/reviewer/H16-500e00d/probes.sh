#!/usr/bin/env bash
# Reviewer's H16 probes (Fable 5.1, reviewer). Run from a checkout of the
# commit under review. Alerts and sleep holds go to scratch files.
set -u
OUT=${OUT:?}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
mkdir "$SCRATCH/bin" && go build -o "$SCRATCH/bin/handoff" ./cmd/handoff || exit 1
export PATH="$SCRATCH/bin:$PATH"
PIDS=()
trap 'kill "${PIDS[@]}" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
printf '# command: bash probes.sh\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (%s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git rev-parse HEAD)"
project() { mkdir "$SCRATCH/$1" && cd "$SCRATCH/$1" && git init -q -b main && echo x > f && git add f && git commit -qm base; }
start() { echo "{\"session_id\":\"$1\",\"hook_event_name\":\"$2\",\"source\":\"${3:-startup}\"}"; }
show() { echo; echo "\$ $*"; "$@" 2>&1; echo "# exit: $?"; }
roles() { python3 -c 'import json,sys;c=json.load(open(".handoff/config.json"));print("roles:",[r["name"]+("" if r["reviews"] else "(no review)") for r in c["roles"]],"main:",c["main"],"quiet_after:",c.get("quiet_after"))'; }

echo "## P1: hooks from settings made before H16 (--prefer) never fail and never bind, at every kind of start, at Stop, and at a notification"
sleep 300 >/dev/null 2>&1 & PIDS+=($!); P1=$!
(project p1 && handoff init --roles claude,codex >/dev/null
 for src in startup resume clear compact; do
   echo "# claude SessionStart source=$src, hook brief --prefer claude:"
   start c1 SessionStart $src | CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P1 handoff hook brief --prefer claude; echo "# exit $?"
 done
 echo "# claude Stop, hook listen --prefer claude --timeout 1s:"
 start c1 Stop | CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P1 handoff hook listen --prefer claude --timeout 1s; echo "# exit $?"
 echo "# claude Notification permission_prompt, hook notify:"
 echo '{"session_id":"c1","hook_event_name":"Notification","notification_type":"permission_prompt","message":"x"}' | CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P1 handoff hook notify; echo "# exit $?"
 echo "# codex SessionStart, hook brief --prefer codex:"
 start t1 SessionStart | CODEX_THREAD_ID=t1 handoff hook brief --prefer codex; echo "# exit $?"
 echo "# codex Stop, hook listen --prefer codex --timeout 1s:"
 start t1 Stop | CODEX_THREAD_ID=t1 handoff hook listen --prefer codex --timeout 1s; echo "# exit $?"
 echo "# .handoff/sessions after all of that: [$(ls -A .handoff/sessions 2>/dev/null | tr '\n' ' ')]"
 echo "# inbox from the Claude session (no role):"; CLAUDE_CODE_SESSION_ID=c1 CLAUDE_PID=$P1 handoff inbox; echo "# exit $?"
 echo "# alerts sent: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d " ")"
)

echo; echo "## P2: ten roles add at once, then five adds of one name at once"
(project p2 && handoff init >/dev/null
 for i in $(seq 1 10); do handoff roles add "r$i" >/dev/null 2>&1 & done; wait
 roles
 for i in 1 2 3 4 5; do (handoff roles add same >/dev/null 2>"$SCRATCH/same.$i"; echo "add same: exit $? $(head -c 60 "$SCRATCH/same.$i")") & done; wait
 python3 -c 'import json;c=json.load(open(".handoff/config.json"));print("same appears", [r["name"] for r in c["roles"]].count("same"), "time(s); roles:", len(c["roles"]))'
)

echo; echo "## P3: roles add while a reviewer's wait runs; the new role is addressable at once and can hold, claim and reply"
sleep 300 >/dev/null 2>&1 & PIDS+=($!); P3=$!
(project p3 && handoff init --roles implementer,reviewer >/dev/null
 CLAUDE_CODE_SESSION_ID=c3 CLAUDE_PID=$P3 handoff role reviewer >/dev/null
 CLAUDE_CODE_SESSION_ID=c3 CLAUDE_PID=$P3 handoff wait --timeout 30s > "$SCRATCH/wait3.txt" 2>&1 & W=$!
 sleep 1.5
 echo "# before add: post --to reviewer-2:"; handoff post --as implementer --item W1 --to reviewer-2 --commit HEAD --title W1 --body f; echo "# exit $?"
 show handoff roles add reviewer-2
 echo "# after add: post --to reviewer-2:"; handoff post --as implementer --item W1 --to reviewer-2 --commit HEAD --title W1 --body f; echo "# exit $?"
 echo "# a Codex thread takes reviewer-2:"; CODEX_THREAD_ID=t3 handoff role reviewer-2 | head -1
 CODEX_THREAD_ID=t3 handoff inbox | sed -n '3,4p'
 printf 'ok\n' > "$SCRATCH/v.md"
 CODEX_THREAD_ID=t3 handoff claim 1; CODEX_THREAD_ID=t3 handoff reply 1 --verdict APPROVE --commit HEAD --body "$SCRATCH/v.md"; echo "# reply exit $?"
 echo -n "# the reviewer's wait still runs: "; kill -0 $W 2>/dev/null && echo yes || echo no
 handoff post --as implementer --item W2 --to reviewer --commit HEAD --title W2 --body f >/dev/null
 wait $W; echo "# the reviewer's wait returned: exit $?"; head -3 "$SCRATCH/wait3.txt"
 roles
)

echo; echo "## P4: handoff role <new> racing roles add <new>, 20 times: never a corrupt config, never a role that is not in it"
(project p4 && handoff init >/dev/null
 okc=0; refc=0; other=0
 for i in $(seq 1 20); do
   n="x$i"
   handoff roles add "$n" >/dev/null 2>&1 &
   out=$(CODEX_THREAD_ID="t$i" handoff role "$n" 2>&1); rc=$?
   wait
   case "$rc:$out" in
     0:*"You are now $n"*) okc=$((okc+1));;
     2:*"unknown role"*) refc=$((refc+1));;
     *) other=$((other+1)); echo "# odd: $rc $out";;
   esac
   python3 -c 'import json,sys;c=json.load(open(".handoff/config.json"));sys.exit(0 if any(r["name"]==sys.argv[1] for r in c["roles"]) else 1)' "$n" || echo "# $n missing from config"
   if [ -f ".handoff/sessions/t$i.json" ] && [ $rc -ne 0 ]; then echo "# t$i bound though role was refused"; fi
 done
 echo "# role succeeded: $okc, refused as unknown: $refc, other: $other"
 python3 -c 'import json;c=json.load(open(".handoff/config.json"));print("config parses;", len(c["roles"]), "roles")'
)

echo; echo "## P5: roles add keeps quiet_after and main, and every byte of the record"
(project p5 && handoff init --roles claude,codex --main main >/dev/null
 handoff post --as claude --item W1 --to codex --commit HEAD --title W1 --body f >/dev/null
 python3 - <<'PY'
import json;p=".handoff/config.json";c=json.load(open(p));c["quiet_after"]="45m";json.dump(c,open(p,"w"),indent=2)
PY
 shasum -a 256 .handoff/channel.jsonl .handoff/head.json > "$SCRATCH/before.sum"
 show handoff roles add implementer reviewer
 shasum -a 256 -c "$SCRATCH/before.sum"
 roles
 handoff inbox --for codex | sed -n '3,4p'
)

echo; echo "## P6: refusals write nothing"
(project p6 && handoff init >/dev/null && cp .handoff/config.json "$SCRATCH/cfg6"
 for args in "add reviewer" "add implementer,reviewer-9" "add operator" "add Bad" "add a..b" "remove reviewer" "add"; do
   echo -n "\$ handoff roles $args -> "; handoff roles $args 2>&1 | head -1; 
 done
 cmp -s .handoff/config.json "$SCRATCH/cfg6" && echo "# config.json unchanged" || echo "# config.json CHANGED"
)
echo; echo "# alerts across all probes: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d " "); awake holds: $(cat "$SCRATCH/awake.txt" 2>/dev/null | wc -l | tr -d " ")"
