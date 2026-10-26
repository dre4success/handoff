#!/usr/bin/env bash
# The reviewer's probes for H23 (b24daa1): the implementer's line on asking
# for a review, and post, note and reply, before (f773529, H22) and after, as
# reviewers' sessions come and go. Run as
#   S=<scratch> OUT=<this dir> bash probes.sh
# with $S/h23/before/handoff built from f773529 and $S/h23/after/handoff from
# b24daa1. A sleep stands in for Claude Code's process; a Codex thread has
# none. Alerts go to a scratch file. The CLIs' user settings are empty
# scratch folders, never the operator's.
set -u
S=${S:?scratch}; OUT=${OUT:?out}
BEFORE=$S/h23/before/handoff; AFTER=$S/h23/after/handoff
COMMIT=b24daa1367ce6978a5844027e1d11825a20f78e8
WORK=$(mktemp -d "$S/h23/probes.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
mkdir -p "$WORK/none/claude" "$WORK/none/codex"; export CLAUDE_CONFIG_DIR="$WORK/none/claude" CODEX_HOME="$WORK/none/codex"
echo "please review" > "$WORK/body.md"
echo "another note" > "$WORK/body2.md"
header() { printf '# command: %s\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$COMMIT"; }
clean() { sed "s|$BEFORE|handoff|g; s|$AFTER|handoff|g; s|$WORK|<scratch>|g; s/pid [0-9][0-9]*/pid <pid>/g; s/\"pid\":[0-9]*/\"pid\":<pid>/g; s/\"host\":[0-9]*/\"host\":<pid>/g; s/@ [0-9a-f]\{12\}/@ <sha>/g; s/\"since\":\"[^\"]*\"/\"since\":\"<time>\"/g; s/since [0-9TZ:-]*/since <time>/g"; }
: > "$WORK/pids"
trap 'kill $(cat "$WORK/pids") 2>/dev/null; rm -rf "$WORK"' EXIT

n=0; P=""; BIN=$AFTER
newproj() { # [init args...]
  n=$((n+1)); P="$WORK/p$n"; mkdir -p "$P"
  git -C "$P" init -q -b main && git -C "$P" commit -q --allow-empty -m base && git -C "$P" switch -q -c w1 && git -C "$P" commit -q --allow-empty -m work
  (cd "$P" && "$BIN" init "$@" > /dev/null) || echo "init failed"
}
asks() { # label [role]
  local role=${2:-implementer}
  echo "## $1"; (cd "$P" && env HANDOFF_ROLE=$role "$BIN" brief 2>&1 | grep '^To ask for a review' || echo "  (no line on asking for a review)")
}
sendas() { # role label args...
  local role=$1 label=$2; shift 2
  echo "## $label"; (cd "$P" && env HANDOFF_ROLE=$role "$BIN" "$@" > "$WORK/o" 2> "$WORK/e"; echo "# exit $?" >> "$WORK/o")
  sed 's/^/  out: /' "$WORK/o"; [ -s "$WORK/e" ] && sed 's/^/  err: /' "$WORK/e"; return 0
}
sends() { local label=$1; shift; sendas implementer "$label" "$@"; }
give() { # role env...
  local role=$1; shift; (cd "$P" && env "$@" "$BIN" role "$role" > /dev/null 2>&1) || echo "  (role $role refused)"
}
# A process standing in for Claude Code. It must not hold the command substitution's pipe.
sleeper() { sleep 600 < /dev/null > /dev/null 2>&1 & echo $! >> "$WORK/pids"; echo $!; }
ended() { kill "$@" 2>/dev/null; for i in 1 2 3 4 5 6 7 8 9 10; do kill -0 "$@" 2>/dev/null || break; sleep 0.1; done; }
binding() { echo "  binding $1: $(cat "$P/.handoff/sessions/$1.json" 2>/dev/null || echo none)"; }

{
  header "the implementer's brief line, post, note and reply, built from f773529 (before) and b24daa1 (after), as reviewers' sessions come and go"

  echo "# Part 1: init's three roles, before and after"
  for bin in before after; do
    BIN=$S/h23/$bin/handoff; newproj
    echo; echo "# $bin"
    asks "a. no reviewer has a session yet (no sessions folder, no lock file)"
    rev=$(sleeper); give reviewer CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$rev
    asks "b. reviewer has a session (Claude Code, a live process); reviewer-2 none"
    sends "c. post to reviewer, from an implementer with no binding (HANDOFF_ROLE)" post --item W1 --to reviewer --commit HEAD --title "W1 work" --body "$WORK/body.md"
    sends "d. post to reviewer-2" post --item W2 --to reviewer-2 --commit HEAD --title "W2 work" --body "$WORK/body.md"
    sends "e. note to the operator" note --item W1 --to operator --title "A question" --body "$WORK/body.md"
    give reviewer-2 CODEX_THREAD_ID=t-rev-2
    asks "f. reviewer-2 has a session too (Codex, given by command: no host)"
    ended $rev
    asks "g. reviewer's process has ended; its binding stays"
    sends "h. note to reviewer" note --item W1 --to reviewer --title "A note" --body "$WORK/body.md"
    sends "i. post to reviewer, a new item" post --item W3 --to reviewer --commit HEAD --title "W3 work" --body "$WORK/body.md"
  done
  BIN=$AFTER

  echo; echo "# Part 2: after only"
  echo; echo "# 2a. a Codex binding given by command has no host, so it counts until a hook records one"
  newproj; give reviewer-2 CODEX_THREAD_ID=t-rev-2; binding t-rev-2
  asks "with reviewer-2's binding alone (reviewer has none)"
  printf '{"session_id":"t-rev-2","hook_event_name":"SessionStart","source":"startup"}' > "$WORK/hook.json"
  sh -c "cd '$P' && env CODEX_THREAD_ID=t-rev-2 '$BIN' hook brief < '$WORK/hook.json' > /dev/null 2>&1; :"
  binding t-rev-2
  asks "after a SessionStart hook for t-rev-2 whose parent shell has exited (host recorded, now dead)"
  sh -c "cd '$P' && env CODEX_THREAD_ID=t-rev-2 '$BIN' hook brief < '$WORK/hook.json' > /dev/null 2>&1; sleep 2" & host=$!; sleep 0.5
  asks "while a hook's parent for t-rev-2 is alive (host alive)"
  wait $host 2>/dev/null

  echo; echo "# 2b. a listener holding the role's lock counts as a session, with no binding: a real \`wait --for reviewer\`"
  newproj
  asks "nothing listens, no binding"
  (cd "$P" && "$BIN" wait --for reviewer --timeout 15s > "$WORK/wait.out" 2> "$WORK/wait.err") & w=$!; sleep 1
  echo "  presence: $(cat "$P/.handoff/presence/reviewer.json" 2>/dev/null || echo none)"
  asks "while wait --for reviewer runs"
  sends "note to reviewer while it listens" note --item W1 --to reviewer --title "A note" --body "$WORK/body.md"
  echo "  wait said: $(cat "$WORK/wait.err" "$WORK/wait.out" | tr '\n' ' ')"
  wait $w 2>/dev/null; echo "  wait exited $?"
  asks "once the wait has ended (lock file still there, nobody holds it)"
  sends "note to reviewer once nothing listens" note --item W1 --to reviewer --title "Another note" --body "$WORK/body2.md"

  echo; echo "# 2c. three reviewers: init --roles implementer,r1,r2,r3"
  newproj --roles implementer,r1,r2,r3
  asks "none has a session"
  p1=$(sleeper); give r1 CLAUDE_CODE_SESSION_ID=s-r1 CLAUDE_PID=$p1
  asks "r1 has a session"
  p2=$(sleeper); give r2 CLAUDE_CODE_SESSION_ID=s-r2 CLAUDE_PID=$p2
  asks "r1 and r2 have sessions"
  give r3 CODEX_THREAD_ID=t-r3
  asks "all three have sessions"
  ended $p1 $p2
  asks "r1's and r2's processes have ended"

  echo; echo "# 2d. one reviewer: init --roles implementer,reviewer"
  newproj --roles implementer,reviewer
  asks "reviewer has no session"
  sends "post to reviewer with no session" post --item W1 --to reviewer --commit HEAD --title "W1 work" --body "$WORK/body.md"
  r=$(sleeper); give reviewer CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$r
  asks "reviewer has a session"
  sends "post to reviewer with a session" post --item W2 --to reviewer --commit HEAD --title "W2 work" --body "$WORK/body.md"

  echo; echo "# 2e. every role reviews: init --roles a,b,c"
  newproj --roles a,b,c
  asks "as a, none has a session" a
  pb=$(sleeper); give b CLAUDE_CODE_SESSION_ID=s-b CLAUDE_PID=$pb
  asks "as a, b has a session" a
  asks "as b, neither a nor c has one" b
  asks "as c, b has a session" c

  echo; echo "# 2f. the brief from a SessionStart hook says the same as \`brief\`"
  newproj; pi=$(sleeper); give implementer CLAUDE_CODE_SESSION_ID=s-imp CLAUDE_PID=$pi
  rv=$(sleeper); give reviewer CLAUDE_CODE_SESSION_ID=s-rev CLAUDE_PID=$rv
  printf '{"session_id":"s-imp","hook_event_name":"SessionStart","source":"startup"}' > "$WORK/hook.json"
  echo "## hook brief for the implementer's session"
  (cd "$P" && env CLAUDE_CODE_SESSION_ID=s-imp CLAUDE_PID=$pi "$BIN" hook brief < "$WORK/hook.json" 2>&1 | grep '^To ask for a review' || echo "  (no line)")
  asks "brief for the implementer's session"

  echo; echo "# 2g. a reply to an implementer whose process has ended: no line (post and note alone say it, row 89 (b))"
  sends "post W1 to reviewer" post --item W1 --to reviewer --commit HEAD --title "W1 work" --body "$WORK/body.md"
  ended $pi
  sendas reviewer "claim #1 as reviewer" claim 1
  sendas reviewer "reply CHANGES to #1, the implementer's process ended" reply 1 --verdict CHANGES --commit HEAD --body "$WORK/body.md"
  sendas reviewer "note from reviewer to the implementer, whose process ended" note --item W1 --to implementer --title "A note" --body "$WORK/body.md"

  echo; echo "# alerts sent: $(cat "$WORK/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the scratch file only)"
} 2>&1 | clean > "$OUT/p1-sessions.log"
echo "probes done"
