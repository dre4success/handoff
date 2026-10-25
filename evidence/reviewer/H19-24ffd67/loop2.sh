#!/usr/bin/env bash
# The reviewer's H19 round-2 probes. Run from a checkout of the commit under review:
#   OUT=<this directory> bash loop2.sh
# P1: a new project's loop with two CHANGES rounds, then a second item and
# its next round. P2: every other route to a request with no base that came
# to mind. P3: two round-2 posts at once, racing for the same request.
# P4: TestH19FirstCommitNeedsNoBase as written here, on round 1's tree.
# Everything happens in scratch folders. Alerts and sleep holds go to scratch
# files; git speaks English and reads no config of the operator's.
set -u
OUT=${OUT:?}
COMMIT=$(git rev-parse HEAD)
ROUND1=63a9d7b06ab928297b160418d786decbaf788cff
WT=$(pwd -P)
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID GIT_DIR GIT_CEILING_DIRECTORIES
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 LC_ALL=C LANG=C
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
H="$SCRATCH/handoff"
c() { echo "\$ handoff $*"; "$H" "$@" </dev/null 2>&1 | sed 's/^/  | /'; echo "  [exit ${PIPESTATUS[0]}]"; }
body() { local f="$SCRATCH/body-$(echo "$1" | tr 'A-Z' 'a-z' | tr -c 'a-z0-9' '-' | cut -c1-16).md"; printf '%s\n' "$1" > "$f"; echo "$f"; }
mk() { echo "$1" > "$1.txt" && git add -A && git commit -qm "$1" && git rev-parse HEAD; }
base_of() { echo "  base of #$1 in the record: [$(grep "\"id\":$1," .handoff/channel.jsonl | grep -o '"base":"[^"]*"' | cut -d'"' -f4)]"; }
# fresh <dir>: a new project, init done, on branch w1 with the first commit posted as #1, claimed (#2), CHANGES (#3)
fresh() {
  mkdir "$SCRATCH/$1" && cd "$SCRATCH/$1" && git init -q -b main && "$H" init >/dev/null && git switch -q -c w1 && FIRST=$(mk first)
  "$H" post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: first work" --body "$(body 'please review')" >/dev/null
  "$H" claim --as reviewer 1 >/dev/null; "$H" reply --as reviewer 1 --verdict CHANGES --commit "$FIRST" --body "$(body '1. fix')" >/dev/null
}

{
printf '# command: bash loop2.sh (built from HEAD)\n# date: %s\n# by: %s\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"

echo "## P1 the loop: first commit on w1, #1 posted with no base, claimed, CHANGES (#3); then the fix on top"
fresh p1; echo "  first $FIRST; main: $(git rev-parse --verify -q main || echo unborn)"
SECOND=$(mk second)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
c show 4
base_of 4
c claim --as reviewer 4
c reply --as reviewer 4 --verdict CHANGES --commit "$SECOND" --body "$(body '1. one more')"
THIRD=$(mk third)
echo "  -- round 3 supersedes #4, itself with no base:"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 3" --supersedes 4 --body "$(body 'fixed again')"
base_of 7
c claim --as reviewer 7
c reply --as reviewer 7 --verdict APPROVE --commit "$THIRD" --body "$(body 'fine')"
c inbox --for operator
echo "  -- a second item, W2, on the same branch, main still unborn:"
FOURTH=$(mk fourth)
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2" --body "$(body 'please review')"
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2" --base "$THIRD" --body "$(body 'please review')"
base_of 10
FIFTH=$(mk fifth)
echo "  -- W2's next round supersedes #10, which has a base:"
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2: round 2" --supersedes 10 --body "$(body 'fixed')"
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2: round 2" --supersedes 10 --base "$THIRD" --body "$(body 'fixed')"
base_of 11

echo "## P2 other routes to a request with no base"
echo "### P2a --supersedes 1 with --base given: the given base wins"
fresh p2a; SECOND=$(mk second)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --base "$FIRST" --body "$(body 'fixed')"
base_of 4
echo "### P2b --supersedes 1 on a commit unrelated to the first: an orphan branch with two commits"
fresh p2b; git checkout -q --orphan other && git rm -q -rf . && O1=$(mk other-one) && O2=$(mk other-two); echo "  orphan tip $O2, parent $O1; first was $FIRST"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
base_of 4
echo "### P2c --supersedes naming an id that does not exist (#9)"
fresh p2c; SECOND=$(mk second)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 9 --body "$(body 'fixed')"
echo "### P2d --supersedes naming the reply (#3), not the request"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 3 --body "$(body 'fixed')"
echo "### P2e --supersedes 1 from another item (W2)"
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2" --supersedes 1 --body "$(body 'please review')"
echo "### P2f main born between rounds (git branch main w1 at the first commit), then round 2"
fresh p2f; git branch main "$FIRST"; SECOND=$(mk second)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
base_of 4
echo "### P2g a misnamed main (init --main main on master): the root commit posted first, then the tip of a long branch as round 2"
mkdir "$SCRATCH/p2g" && cd "$SCRATCH/p2g" && git init -q -b master && "$H" init --main main >/dev/null && R=$(mk root) && A=$(mk a) && B=$(mk b)
c post --as implementer --item W1 --to reviewer --commit "$R" --title "W1" --body "$(body 'please review')"
"$H" claim --as reviewer 1 >/dev/null; "$H" reply --as reviewer 1 --verdict CHANGES --commit "$R" --body "$(body '1. fix')" >/dev/null
c post --as implementer --item W1 --to reviewer --commit "$B" --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
base_of 4

echo "## P3 two round-2 posts at once, both superseding #1 (read outside the lock; checked under it)"
fresh p3; SECOND=$(mk second)
"$H" post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2 (A)" --supersedes 1 --body "$(body 'fixed A')" > "$SCRATCH/A.out" 2>&1 & pa=$!
"$H" post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2 (B)" --supersedes 1 --body "$(body 'fixed B')" > "$SCRATCH/B.out" 2>&1 & pb=$!
wait $pa; ea=$?; wait $pb; eb=$?
echo "  A: $(cat "$SCRATCH/A.out") [exit $ea]"; echo "  B: $(cat "$SCRATCH/B.out") [exit $eb]"
echo "  requests on W1 now: $(grep -c '"kind":"request"' .handoff/channel.jsonl); the record verifies: $("$H" inbox --for reviewer >/dev/null 2>&1 && echo yes || echo no)"
c inbox --for reviewer

echo "## P4 TestH19FirstCommitNeedsNoBase as written here, run on round 1's tree ($ROUND1), then here"
cd "$WT"; mkdir "$SCRATCH/r1" && git archive "$ROUND1" | tar -x -C "$SCRATCH/r1" && cp cmd/handoff/pickup_test.go "$SCRATCH/r1/cmd/handoff/pickup_test.go"
(cd "$SCRATCH/r1" && go test -count=1 -run 'TestH19FirstCommitNeedsNoBase$' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL|[[:space:]]+pickup_test)' | cut -c1-200 | sed 's/^/  /'; echo "  [go test exit ${PIPESTATUS[0]}]")
go test -count=1 -v -run 'TestH19FirstCommitNeedsNoBase$' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)' | sed 's/^/  /'

echo
echo "# alerts sent to the scratch file during these probes: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} 2>&1 | sed "s|$SCRATCH|<scratch>|g; s|$WT|<worktree>|g; s|$HOME|~|g" > "$OUT/loop2.log"
