#!/usr/bin/env bash
# Collects the H8 round-2 evidence: everything round 1 collects, into this
# folder, then the reviewer's three probes from the H8 CHANGES (committed on
# main) on the reviewed commit and on this one. MAIN is a checkout of main.
#   bash evidence/h8/r2/collect.sh
set -u
OUT=evidence/h8/r2 bash evidence/h8/collect.sh || exit 1
MAIN=${MAIN:-~/handoff}
P=$MAIN/evidence/reviewer/H8-8b0735d
REVIEWED=8b0735d8e22d6b673256c4e68a27d354b26a40e9
COMMIT=$(git rev-parse HEAD)
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT
probe() { # $1 = commit, $2 = worktree
  git worktree add -q --detach "$2" "$1"
  cp "$P/testdata/notify-probes_test.go" "$2/internal/notify/review_h8_test.go"
  cp "$P/testdata/claim-probe_test.go" "$2/cmd/handoff/review_h8_test.go"
  (cd "$2" && go test -count=1 -v -run TestReview ./internal/notify ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL)|review_h8_test.go:')
  git worktree remove --force "$2"
}
{
  printf '# command: GPT-6'"'"'s H8 probes (%s), copied into worktrees of each commit, run by the implementer\n# date: %s\n# by: Opus 5.5 (implementer)\n# commit: %s\n\n' "$P" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$COMMIT"
  echo "## at $REVIEWED, the reviewed commit (all three fail)"
  probe "$REVIEWED" "$SCRATCH/a"
  echo
  echo "## at $COMMIT"
  echo "## Expected: both notify probes pass. The claim probe still fails, by design (row 62 (b)):"
  echo "## claiming again writes nothing, so the silence since the first claim is reported."
  echo "## TestH8ClaimingAgainWritesNothingAndTellsOnce covers that path as intended."
  probe "$COMMIT" "$SCRATCH/b"
} > evidence/h8/r2/reviewer-probes-rerun.txt
echo "collected in evidence/h8/r2 at $COMMIT"
