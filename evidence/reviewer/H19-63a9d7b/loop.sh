#!/usr/bin/env bash
# The reviewer's H19 probes. Run from a checkout of the commit under review:
#   OUT=<this directory> bash loop.sh
# P1: a new project's loop, run to the end, with a CHANGES round in it.
# P1b: the same round 2 by amending the first commit instead.
# P2: a misnamed main branch. P3: the first commit made on the unborn main
# itself. P4: the briefs' new lines for three role sets.
# Everything happens in scratch folders. Alerts and sleep holds go to scratch
# files; git speaks English and reads no config of the operator's.
set -u
OUT=${OUT:?}
COMMIT=$(git rev-parse HEAD)
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
body() { printf '%s\n' "$1" > "$SCRATCH/body-$(echo "$1" | tr -c 'a-z0-9' '-' | cut -c1-20).md"; echo "$SCRATCH/body-$(echo "$1" | tr -c 'a-z0-9' '-' | cut -c1-20).md"; }
mk() { echo "$1" > "$1.txt" && git add -A && git commit -qm "$1" && git rev-parse HEAD; }
line() { echo "  record line of #$1 (body and prev shortened):"; grep "\"id\":$1," .handoff/channel.jsonl | sed 's/"body":"[^"]*"/"body":"…"/; s/"prev":"[^"]*"/"prev":"…"/; s/^/    /'; }

{
printf '# command: bash loop.sh (built from HEAD)\n# date: %s\n# by: %s\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"

echo "## P1 a new project's loop: git init -b main; handoff init; a branch; the first commit; post, claim, CHANGES; a second commit; post again"
mkdir "$SCRATCH/p1" && cd "$SCRATCH/p1" && git init -q -b main
c init
git switch -q -c w1; FIRST=$(mk first); echo "  first commit $FIRST on w1; main has none: $(git rev-parse --verify -q main || echo unborn)"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: first work" --body "$(body 'please review')"
c show 1
line 1
c claim --as reviewer 1
c reply --as reviewer 1 --verdict CHANGES --commit "$FIRST" --body "$(body '1. one thing to fix')"
SECOND=$(mk second); echo "  second commit $SECOND on w1, the fix"
echo "  -- round 2, as the brief says (post again with --supersedes):"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
echo "  -- the only base that exists, the first commit:"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --base "$FIRST" --body "$(body 'fixed')"
c show 4
echo "  what that base gives a reviewer (git diff --stat $FIRST $SECOND):"; git diff --stat "$FIRST" "$SECOND" | sed 's/^/    /'
echo "  what the tree holds (git ls-tree -r --name-only $SECOND):"; git ls-tree -r --name-only "$SECOND" | sed 's/^/    /'
c claim --as reviewer 4
c reply --as reviewer 4 --verdict APPROVE --commit "$SECOND" --body "$(body 'fine')"
c inbox --for operator
c decide W1 --approve --note "merge it"
echo "  -- the implementer makes main from w1 (git branch main w1), then a second item on a new branch:"
git branch main w1 && git switch -q -c w2 && THIRD=$(mk third)
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2: more" --body "$(body 'please review')"
c show 7
echo "  files in .handoff: $(ls .handoff | tr '\n' ' ')"

echo "## P1b round 2 by amending the first commit instead (git commit --amend): the new root has no parent"
mkdir "$SCRATCH/p1b" && cd "$SCRATCH/p1b" && git init -q -b main && "$H" init >/dev/null && git switch -q -c w1 && FIRST=$(mk first)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: first work" --body "$(body 'please review')"
c claim --as reviewer 1
c reply --as reviewer 1 --verdict CHANGES --commit "$FIRST" --body "$(body '1. one thing to fix')"
echo fix > first.txt && git add -A && git commit -q --amend -m "first, fixed" && FIRST2=$(git rev-parse HEAD); echo "  amended root $FIRST2"
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1: round 2" --supersedes 1 --body "$(body 'fixed')"
c show 4

echo "## P2 a misnamed main branch: git init -b master; handoff init --main main"
mkdir "$SCRATCH/p2" && cd "$SCRATCH/p2" && git init -q -b master
c init --main main
git switch -q -c w1; FIRST=$(mk first)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1" --body "$(body 'please review')"
SECOND=$(mk second)
c post --as implementer --item W2 --to reviewer --commit HEAD --title "W2" --body "$(body 'please review')"

echo "## P3 the first commit made on the unborn main itself, no branch"
mkdir "$SCRATCH/p3" && cd "$SCRATCH/p3" && git init -q -b main && "$H" init >/dev/null && FIRST=$(mk first)
c post --as implementer --item W1 --to reviewer --commit HEAD --title "W1" --body "$(body 'please review')"
c show 1

echo "## P4 the briefs' new lines: init with no flags; --roles claude,codex; --roles a,b,c --reviewers c"
mkdir "$SCRATCH/p4a" && cd "$SCRATCH/p4a" && git init -q -b main && "$H" init >/dev/null
echo "  implementer:"; "$H" brief --for implementer 2>&1 | grep -n 'commit your work\|findings on work\|no request' | sed 's/^/    /'
echo "  reviewer:"; "$H" brief --for reviewer 2>&1 | grep -n 'commit your work\|findings on work\|no request' | sed 's/^/    /'
mkdir "$SCRATCH/p4b" && cd "$SCRATCH/p4b" && git init -q -b main && "$H" init --roles claude,codex >/dev/null
echo "  claude (both implement and review):"; "$H" brief --for claude 2>&1 | grep -n 'commit your work\|findings on work\|no request' | sed 's/^/    /'
mkdir "$SCRATCH/p4c" && cd "$SCRATCH/p4c" && git init -q -b main && "$H" init --roles a,b,c --reviewers c >/dev/null
echo "  c (the one reviewer, two implementers):"; "$H" brief --for c 2>&1 | grep -n 'no request' | sed 's/^/    /'
echo "  a (implements; one reviewer):"; "$H" brief --for a 2>&1 | grep -n 'commit your work' | sed 's/^/    /'

echo
echo "# alerts sent to the scratch file during these probes: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} 2>&1 | sed "s|$SCRATCH|<scratch>|g; s|$WT|<worktree>|g; s|$HOME|~|g" > "$OUT/loop.log"
