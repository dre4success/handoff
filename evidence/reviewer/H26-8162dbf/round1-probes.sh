#!/usr/bin/env bash
# The reviewer's round-1 probes for H26, run again at round 2 (8162dbf), unchanged but for this line. Run as
#   S=<scratchpad> B=<handoff built at the commit> BASE=<handoff built at 1af34c7> OLD=<handoff v0.1.2> WT=<worktree> bash probes.sh > probes.log
# HOME and both CLIs' settings folders are scratch folders; alerts and sleep
# holds go to scratch files; nothing of the operator's is read or sent.
set -u
S=${S:?scratchpad}; B=${B:?handoff}; BASE=${BASE:?handoff at the base}; OLD=${OLD:?handoff v0.1.2}; WT=${WT:?worktree}
mkdir -p "$S/h26"
WORK=$(mktemp -d "$S/h26/work.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
: > "$WORK/pids"
cleanup() { while read -r p; do kill "$p" 2>/dev/null; done < "$WORK/pids"; }
trap cleanup EXIT
export HOME="$WORK/home" CLAUDE_CONFIG_DIR="$WORK/home/.claude" CODEX_HOME="$WORK/home/.codex"
mkdir -p "$CLAUDE_CONFIG_DIR" "$CODEX_HOME"
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_TEST_FAULT
clean() { sed -E "s|$BASE|handoff-base|g; s|$OLD|handoff-v0.1.2|g; s|$B|handoff|g; s|$HOME|~|g; s|$WORK|<scratch>|g; s/CLAUDE_PID=[0-9]+/CLAUDE_PID=<pid>/g; s/\"(pid|host)\":[0-9]+/\"\1\":<pid>/g; s/\"prev\":\"[0-9a-f]{64}\"/\"prev\":\"<hash>\"/g"; }
run() { echo; echo "\$ $*"; "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# runin <env...> -- <cmd...>: a command in a session's environment
runin() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ env ${envs[*]} $*"; env "${envs[@]}" "$@" > "$WORK/o" 2>&1 < /dev/null; c=$?; cat "$WORK/o"; echo "# exit $c"; }
# term <env...> -- <cmd...>: a command at a terminal (script(1)), as the operator's own
term() { local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ [terminal] ${envs[*]:+env ${envs[*]} }$*"; env ${envs[@]+"${envs[@]}"} script -q /dev/null "$@" < /dev/null > "$WORK/o" 2>&1; c=$?; tr -d '\r' < "$WORK/o" | sed 's/^\^D//'; echo "# exit $c"; }
# hook <json> <env...> -- <hook args...>: a hook, fed what the CLI writes to its stdin
hook() { local json=$1; shift; local envs=(); while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift; echo; echo "\$ echo '$json' | env ${envs[*]} handoff hook $*"; printf '%s' "$json" | env "${envs[@]}" "$B" hook "$@" > "$WORK/o" 2>&1; c=$?; cat "$WORK/o"; echo "# exit $c"; }
stop() { printf '{"session_id":"%s","hook_event_name":"Stop"}' "$1"; }
sleeper() { sleep 600 </dev/null >/dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
N=0
newrepo() { N=$((N+1)); REPO="$WORK/repo$N"; git init -q -b main "$REPO"; git -C "$REPO" commit -q --allow-empty -m base; cd "$REPO"; "$B" init > /dev/null; }
body() { printf '%s\n' "$1" > "$WORK/body.md"; echo "$WORK/body.md"; }
lines() { wc -l < .handoff/channel.jsonl | tr -d ' '; }

main() {
printf '# command: the reviewer'"'"'s probes for handoff tell; see each heading\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$(git -C "$WT" rev-parse HEAD)"

###############################################################################
echo "## P1. Does an old tell reach a session started later? (README: \"A session started later doesn't get it.\"; row 92 (b): \"a session given a role later never gets an old tell\")"

echo; echo "### P1a. The session that was told ends before its turn does; a new session takes the role later (3 s later here; the next morning, in life)"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-night CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
echo "# s-night is in the middle of a turn: it has the role and no listener yet"
term -- "$B" tell "Carry on while I'm away. Merge when the reviewer approves."
kill $A; wait $A 2>/dev/null
echo; echo "# s-night's process has ended (a crash, a usage limit, a closed window). It never listened, so it never read the tell."
sleep 3
C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-morning CLAUDE_PID=$C -- "$B" role implementer | sed -n 1,3p
echo; echo "# the bindings now, and the tell's time:"
cat .handoff/sessions/*.json | clean
grep -o '"ts":"[^"]*","kind":"tell"' .handoff/channel.jsonl
hook "$(stop s-morning)" CLAUDE_CODE_SESSION_ID=s-morning CLAUDE_PID=$C -- listen --timeout 5s
echo "# ^ the new session's first listen: exit 2 wakes it with the old tell. The line names no time."
kill $C; wait $C 2>/dev/null

echo; echo "### P1b. The operator moves the role to another session before the first one's turn ends"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-one CLAUDE_PID=$A -- "$B" role reviewer | sed -n 1,3p
term -- "$B" tell "Carry on while I'm away."
runin CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
hook "$(stop s-two)" CLAUDE_CODE_SESSION_ID=s-two CLAUDE_PID=$C -- listen --timeout 5s
kill $A $C; wait $A $C 2>/dev/null

echo; echo "### P1c. A Codex thread that took its role by a command and ran no hook: its binding has no process, so it counts as a session for good"
newrepo
runin CODEX_THREAD_ID=t-gone -- "$B" role reviewer | sed -n 1,3p
cat .handoff/sessions/*.json | clean
echo "# no such thread runs. The tell still says it told reviewer:"
term -- "$B" tell "Carry on while I'm away."
runin CODEX_THREAD_ID=t-new -- "$B" role reviewer | sed -n 1,3p
run "$B" wait --for reviewer --timeout 2s

echo; echo "### P1d. Control: a role whose Claude Code session has ended is not told, and a later session gets nothing"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
kill $C; wait $C 2>/dev/null
term -- "$B" tell "Carry on while I'm away."
D=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-rev-new CLAUDE_PID=$D -- "$B" role reviewer | sed -n 1,3p
hook "$(stop s-rev-new)" CLAUDE_CODE_SESSION_ID=s-rev-new CLAUDE_PID=$D -- listen --timeout 3s
kill $A $D; wait $A $D 2>/dev/null

###############################################################################
echo; echo "## P2. A tell cut short or killed"

echo; echo "### P2a. Three roles, a record that already has entries, killed halfway through the write (HANDOFF_TEST_FAULT=torn)"
newrepo
A=$(sleeper); C=$(sleeper); D=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev2 CLAUDE_PID=$D -- "$B" role reviewer-2 | sed -n 1,3p
runin HANDOFF_ROLE=implementer -- "$B" note --item W1 --to reviewer --title before --body "$(body before)"
cp .handoff/channel.jsonl "$WORK/before.jsonl"
term HANDOFF_TEST_FAULT=torn -- "$B" tell "Carry on."
echo "# the record: $(lines) whole lines, $(wc -c < .handoff/channel.jsonl | tr -d ' ') bytes; the bytes after the last newline: $(tail -c +$(( $(head -n "$(lines)" .handoff/channel.jsonl | wc -c) + 1 )) .handoff/channel.jsonl | wc -c | tr -d ' ')"
run "$B" wait --for implementer --timeout 1s
run "$B" wait --for reviewer --timeout 1s
run "$B" wait --for reviewer-2 --timeout 1s
term -- "$B" tell "Carry on."
echo "# lines after the refused tell: $(lines)"
cp .handoff/channel.jsonl "$WORK/torn.jsonl"
runin HANDOFF_ROLE=reviewer -- "$B" repair
head -c "$(wc -c < "$WORK/torn.jsonl" | tr -d ' ')" .handoff/channel.jsonl | cmp - "$WORK/torn.jsonl" && echo "# repair kept every byte of the torn record ($(wc -c < "$WORK/torn.jsonl" | tr -d ' ') bytes)"
term -- "$B" tell "Carry on."
run "$B" wait --for implementer --timeout 1s
run "$B" wait --for reviewer --timeout 1s
run "$B" wait --for reviewer-2 --timeout 1s
run "$B" inbox --for implementer
kill $A $C $D; wait $A $C $D 2>/dev/null

echo; echo "### P2b. Killed after the write is synced and before head.json (HANDOFF_TEST_FAULT=after-sync)"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
term HANDOFF_TEST_FAULT=after-sync -- "$B" tell "Carry on."
echo "# the record: $(lines) whole lines; head.json: $(cat .handoff/head.json)"
run "$B" wait --for implementer --timeout 1s
run "$B" wait --for reviewer --timeout 1s
echo "# the operator saw nothing printed, so tells again:"
term -- "$B" tell "Carry on."
run "$B" wait --for implementer --timeout 1s
runin HANDOFF_ROLE=reviewer -- "$B" repair
run "$B" inbox --for reviewer
kill $A $C; wait $A $C 2>/dev/null

echo; echo "### P2c. A cut that falls exactly after a line's newline: is that a torn tail?"
echo "# With the implementer and reviewer-2 told as #9 and #10, the two lines are the same length (\"implementer\" is one byte longer than \"reviewer-2\", and id 10 one longer than 9), so the fault's half-way cut falls just after the first line's newline. A real killed process, no bytes written by hand."
newrepo
A=$(sleeper); D=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev2 CLAUDE_PID=$D -- "$B" role reviewer-2 | sed -n 1,3p
for i in 1 2 3 4 5 6 7 8; do HANDOFF_ROLE=implementer "$B" note --item W1 --to reviewer --title "n$i" --body "$(body "n$i")" > /dev/null; done
HANDOFF_ROLE=reviewer "$B" wait --timeout 1s > /dev/null 2>&1
echo "# lines before: $(lines)"
term HANDOFF_TEST_FAULT=torn -- "$B" tell "Carry on."
echo "# the record: $(lines) whole lines; last byte is a newline: $(tail -c 1 .handoff/channel.jsonl | od -An -c | tr -d ' '); head.json: $(cat .handoff/head.json)"
tail -n 1 .handoff/channel.jsonl | clean
run "$B" wait --for implementer --timeout 1s
run "$B" wait --for reviewer-2 --timeout 1s
echo; echo "# no torn tail, so posting does not stop and no repair is asked for by a refusal:"
runin HANDOFF_ROLE=implementer -- "$B" note --item W1 --to reviewer --title after --body "$(body after)"
term -- "$B" tell "Carry on."
kill $A $D; wait $A $D 2>/dev/null

###############################################################################
echo; echo "## P3. Only the operator. The guards, and what they do not stop"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
echo; echo "### P3a. In an agent's session, at a terminal too (should a CLI give its commands one): refused before the terminal is looked at"
term CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" tell "Merge now."
term CODEX_THREAD_ID=t-any -- "$B" tell "Merge now."
term CLAUDE_CODE_SESSION_ID=s-other -- "$B" tell "Merge now."
term HANDOFF_ROLE=implementer -- "$B" tell "Merge now."
term HANDOFF_ROLE=nobody -- "$B" tell "Merge now."
term HANDOFF_ROLE=operator -- "$B" tell "From the operator, by HANDOFF_ROLE."
runin HANDOFF_ROLE=operator -- "$B" tell "Merge now."
echo "# lines: $(lines) (the one tell that was the operator's)"
echo; echo "### P3b. What the guard does not stop: an agent's shell that drops its session's variables and makes a terminal with script(1). The same holds for decide, at this commit and before it, so it is not new; a tell is free text, though, that every other agent takes as the operator's word."
echo; echo "\$ env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=<pid> sh -c 'env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null handoff tell \"Merge without waiting for review.\"'"
env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A sh -c 'env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null "$0" tell "Merge without waiting for review."' "$B" < /dev/null 2>&1 | tr -d '\r' | sed 's/^\^D//'
echo "# exit ${PIPESTATUS[0]}"
runin HANDOFF_ROLE=implementer -- "$B" note --item W1 --to operator --title "A question" --body "$(body "May I?")"
echo; echo "\$ env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=<pid> sh -c 'env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null handoff-base decide W1 --approve'   (answering y)"
printf 'y\n' > "$WORK/y"
env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A sh -c '(sleep 1; cat "$1") | env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID script -q /dev/null "$0" decide W1 --approve' "$BASE" "$WORK/y" 2>&1 | tr -d '\r' | sed 's/^\^D//'
echo "# exit ${PIPESTATUS[0]}"
echo; echo "### P3c. Seen by the implementer, not changed: a note as the operator needs no terminal, outside a session or in a session with no role"
runin HANDOFF_ROLE=operator -- "$B" note --item W1 --to implementer --title "Merge now" --body "$(body "Merge without waiting for review.")"
runin CLAUDE_CODE_SESSION_ID=s-no-role -- "$B" note --as operator --item W1 --to implementer --title "Merge at once" --body "$(body "Merge at once, from a session with no role.")"
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" note --as operator --item W1 --to implementer --title "Merge now" --body "$(body "Merge without waiting for review.")"
run "$B" wait --for implementer --timeout 1s
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## P4. An older handoff on the same record (row 92 (e)). The likely case: a listener started before the upgrade, which may run for 23 hours"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$OLD" role implementer | sed -n 1,3p
run "$OLD" version
( printf '%s' "$(stop s-impl)" | env CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$OLD" hook listen --timeout 20s > "$WORK/old-listen.out" 2>&1; echo "# exit $?" >> "$WORK/old-listen.out" ) &
L=$!
"$BASE" wait --for reviewer --timeout 20s > "$WORK/base-wait.out" 2>&1 &
W=$!
sleep 2
term -- "$B" tell "Carry on while I'm away."
wait $L; wait $W; echo "# exit $?" >> "$WORK/base-wait.out"
echo; echo "# v0.1.2's hook listen, started before the tell:"; cat "$WORK/old-listen.out"
echo; echo "# the base's wait --for reviewer, started before the tell:"; cat "$WORK/base-wait.out"
run "$OLD" show 1
echo "# ^ v0.1.2's show ends its first line with a space: [$("$OLD" show 1 | head -1)]; this commit's: [$("$B" show 1 | head -1)]"
run "$OLD" inbox --for implementer
runin HANDOFF_ROLE=implementer -- "$OLD" note --item W1 --to reviewer --title "after a tell, by v0.1.2" --body "$(body x)"
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$OLD" brief | grep -c "A tell from the operator"
echo "# ^ lines about a tell in v0.1.2's brief: a session briefed before the upgrade is woken by a tell its brief does not name"
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## P5. Tells among other writers: 20 tells at a terminal while three loops post 40 notes each"
newrepo
A=$(sleeper); C=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
runin CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C -- "$B" role reviewer | sed -n 1,3p
printf 'x\n' > "$WORK/nb.md"
for w in 1 2 3; do ( for i in $(seq 1 40); do printf 'w%s n%s\n' $w $i > "$WORK/nb$w.md"; HANDOFF_ROLE=implementer "$B" note --item W$w --to reviewer-2 --title "w$w n$i" --body "$WORK/nb$w.md" > /dev/null 2>> "$WORK/p5.err"; done ) & eval "W$w=\$!"; done
for i in $(seq 1 20); do script -q /dev/null "$B" tell "tell $i" < /dev/null >> "$WORK/p5.tell" 2>&1; done
wait $W1 $W2 $W3
echo "# told lines printed: $(tr -d '\r' < "$WORK/p5.tell" | grep -c 'told implementer and reviewer (#')"; echo "# notes posted: $(grep -c '"kind":"note"' .handoff/channel.jsonl); writers' stderr lines other than the no-session notice: $(grep -vc 'no session has reviewer-2 now' "$WORK/p5.err")"
python3 -I - .handoff/channel.jsonl <<'PY'
import json, sys, hashlib
raw = open(sys.argv[1], 'rb').read()
lines = raw.split(b'\n')[:-1]
es = [json.loads(l) for l in lines]
prev = 'genesis'; ok = True
for l, e in zip(lines, es):
    if e['prev'] != prev: ok = False
    prev = hashlib.sha256(l).hexdigest()
tells = [(i, e) for i, e in enumerate(es) if e['kind'] == 'tell']
pairs = 0; bad = 0
for k in range(0, len(tells), 2):
    (i, a), (j, b) = tells[k], tells[k + 1]
    if j == i + 1 and b['id'] == a['id'] + 1 and a['ts'] == b['ts'] and a['body'] == b['body'] and (a['to'], b['to']) == ('implementer', 'reviewer') and 'item' not in a:
        pairs += 1
    else:
        bad += 1
ids = [e['id'] for e in es]
print(f"# {len(es)} entries, ids 1..{ids[-1]} in order: {ids == list(range(1, len(es) + 1))}; chain verifies: {ok}; tells: {len(tells)}; pairs on adjacent lines with consecutive ids, one time, one body: {pairs}; other: {bad}")
PY
run "$B" inbox --for reviewer --quiet
echo "# head.json: $(cat .handoff/head.json | clean)"
echo "# the reviewer's wait gets its 20, in order, and none of the implementer's:"
"$B" wait --for reviewer --timeout 1s 2>&1 | awk '{print $1, $2, $3, $4, $5, $6, $7, $8}' | tr '\n' ';' ; echo
kill $A $C; wait $A $C 2>/dev/null

###############################################################################
echo; echo "## P6. The message"
newrepo
A=$(sleeper)
runin CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A -- "$B" role implementer | sed -n 1,3p
term -- "$B" tell Carry on, "two  spaces" kept
term -- "$B" tell "Carry on" --as reviewer
term -- "$B" tell "- no merges tonight"
term -- "$B" tell -- "- no merges tonight"
term -- "$B" tell -h
term -- "$B" tell -h now
term -- "$B" tell --help
term -- "$B" tell ""
term -- "$B" tell "$(printf 'First line.\nSecond line, with "quotes", a tab\tand caf\303\251.')"
run "$B" wait --for implementer --timeout 1s
run "$B" show 3
echo; echo "# from a subfolder, and from a linked worktree of the project:"
mkdir -p sub/dir; cd sub/dir; term -- "$B" tell "From a subfolder."; cd "$REPO"
git worktree add -q "$WORK/linked" -b side; cd "$WORK/linked"; term -- "$B" tell "From a linked worktree."; cd "$REPO"
echo "# outside any project:"
mkdir -p "$WORK/nowhere"; cd "$WORK/nowhere"; term -- "$B" tell "Anyone?"; cd "$REPO"
echo "# lines: $(lines)"
kill $A; wait $A 2>/dev/null

###############################################################################
echo; echo "## P7. A tell asks nothing: the same thread with and without tells between its entries"
thread() { # $1: yes to tell between the steps
  newrepo
  A=$(sleeper); C=$(sleeper)
  CLAUDE_CODE_SESSION_ID=s-impl CLAUDE_PID=$A "$B" role implementer > /dev/null
  CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$C "$B" role reviewer > /dev/null
  t() { if [ "$1" = yes ]; then script -q /dev/null "$B" tell "Carry on." < /dev/null > /dev/null 2>&1; fi; }
  sha=$(git rev-parse HEAD)
  t "$1"
  HANDOFF_ROLE=implementer "$B" post --item W1 --to reviewer --commit "$sha" --title T --body "$(body req)" > "$WORK/post.out" 2>&1
  req=$(sed -n 's/^posted #\([0-9]*\).*/\1/p' "$WORK/post.out")
  t "$1"
  echo "--- after the request"; for r in implementer reviewer operator; do HANDOFF_ROLE=$r "$B" inbox 2>&1 | sed -E 's/#[0-9]+/#N/g'; done
  HANDOFF_ROLE=reviewer "$B" claim "$req" > /dev/null 2>&1
  t "$1"
  echo "--- after the claim"; for r in implementer reviewer operator; do HANDOFF_ROLE=$r "$B" inbox 2>&1 | sed -E 's/#[0-9]+/#N/g'; done
  HANDOFF_ROLE=reviewer "$B" reply "$req" --verdict APPROVE --commit "$sha" --body "$(body ok)" > /dev/null 2>&1
  t "$1"
  echo "--- after the APPROVE"; for r in implementer reviewer operator; do HANDOFF_ROLE=$r "$B" inbox 2>&1 | sed -E 's/#[0-9]+/#N/g'; done
  echo "--- alerts sent"; sed -E 's/#[0-9]+/#N/g; s/decide-[0-9-]+/decide-N/g' "$WORK/alerts.txt" 2>/dev/null; : > "$WORK/alerts.txt"
  echo "--- sleep holds"; cat "$WORK/awake.txt" 2>/dev/null | sed -E 's/[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/<time>/g'; : > "$WORK/awake.txt"
  kill $A $C; wait $A $C 2>/dev/null
}
: > "$WORK/alerts.txt"; : > "$WORK/awake.txt"
thread no  > "$WORK/t1" 2>&1; sed -E "s|repo[0-9]+|repoN|g; s/[0-9a-f]{40}/<sha>/g; s/[0-9a-f]{12}/<sha12>/g" "$WORK/t1" > "$WORK/thread-plain.txt"
thread yes > "$WORK/t2" 2>&1; sed -E "s|repo[0-9]+|repoN|g; s/[0-9a-f]{40}/<sha>/g; s/[0-9a-f]{12}/<sha12>/g" "$WORK/t2" > "$WORK/thread-tells.txt"
cd "$REPO"
echo "# the second thread has a tell before the request and after each of its three entries, to two roles each time: $(grep -c '"kind":"tell"' .handoff/channel.jsonl) tell lines in its record"
if diff "$WORK/thread-plain.txt" "$WORK/thread-tells.txt" > "$WORK/thread.diff"; then echo "# every inbox, the alerts sent and the sleep holds are the same with and without the tells (ids aside):"; cat "$WORK/thread-tells.txt"; else echo "# they differ:"; cat "$WORK/thread.diff"; fi
echo; echo "# deciding a tell, and deciding the item with tells around it:"
term -- "$B" decide 1 --approve
echo; echo "\$ [terminal] handoff decide W1 --approve   (answering y)"
(sleep 1; echo y) | script -q /dev/null "$B" decide W1 --approve 2>&1 | tr -d '\r' | sed 's/^\^D//'
run "$B" inbox --for operator
echo; echo "# alerts sent during all probes went to the scratch file only."
}
main 2>&1 | clean
