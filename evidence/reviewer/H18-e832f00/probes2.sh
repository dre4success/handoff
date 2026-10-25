#!/usr/bin/env bash
# The reviewer's H18 round-2 probes, on top of round 1's eleven (P1 to P11,
# evidence/reviewer/H18-1c392de/probes.sh, run again from this commit by
# collect.sh). Run from a checkout of the commit under review:
#   OUT=<this directory> bash probes2.sh
# Everything happens in a scratch folder. Alerts and sleep holds go to scratch
# files. Git speaks English (LC_ALL=C) and reads no config of the operator's.
set -u
OUT=${OUT:?}
COMMIT=$(git rev-parse HEAD)
ROUND1=1c392de29d81a8c196bbdd0baf32faa37090288c
WT=$(pwd -P)
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
cleanup() { chmod -R u+rwx "$SCRATCH" 2>/dev/null; rm -rf "$SCRATCH"; }
trap cleanup EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID GIT_DIR GIT_CEILING_DIRECTORIES GIT_DISCOVERY_ACROSS_FILESYSTEM
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 LC_ALL=C LANG=C
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
H="$SCRATCH/handoff"

# say <label> <dir> [VAR=value...]: the git call handoff makes, in that
# folder and environment, then `handoff init` there.
say() {
  local label=$1 dir=$2; shift 2
  echo "## $label"
  local gt; gt=$(cd "$dir" && env "$@" git rev-parse --path-format=absolute --git-common-dir 2>&1; echo "[exit $?]")
  echo "$gt" | sed 's/^/  git: /'
  case "$gt" in
    *"not a git repository (or any"*) echo "  git's words contain \"not a git repository (or any\": yes";;
    *"not a git repository"*) echo "  git's words contain \"not a git repository\": yes, but not \"(or any\"";;
    *) echo "  git's words contain \"not a git repository\": no";;
  esac
  (cd "$dir" && env "$@" "$H" init </dev/null 2>&1; echo "[exit $?]") | sed 's/^/  handoff: /'
}
# then <dir> [VAR=value...]: git init as the hint says, then handoff init again.
then_init() {
  local dir=$1; shift
  echo "  then, as the hint says: git init -q -b main"
  (cd "$dir" && env "$@" git init -q -b main >"$SCRATCH/o" 2>&1; echo "  git init: [exit $?]"; sed 's/^/  git init: /' "$SCRATCH/o"
   env "$@" "$H" init </dev/null >"$SCRATCH/o" 2>&1; echo "  handoff init again: [exit $?]"; head -1 "$SCRATCH/o" | sed 's/^/  handoff init again: /')
}

{
printf '# command: bash probes2.sh (round-2 probes: P12 to P16; built from HEAD)\n# date: %s\n# by: %s\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"

mkdir "$SCRATCH/p12" && (cd "$SCRATCH/p12" && git init -q -b main && chmod 000 .git)
say "P12 a repository whose .git directory cannot be read (mode 000)" "$SCRATCH/p12"
then_init "$SCRATCH/p12"
chmod 700 "$SCRATCH/p12/.git"

mkdir "$SCRATCH/p13" && echo garbage > "$SCRATCH/p13/.git"
say "P13 a .git file that is not a gitdir pointer" "$SCRATCH/p13"
then_init "$SCRATCH/p13"

mkdir -p "$SCRATCH/p14/.git"
say "P14 an empty .git directory (mkdir .git)" "$SCRATCH/p14"
then_init "$SCRATCH/p14"

mkdir "$SCRATCH/p15" && (cd "$SCRATCH/p15" && git init -q -b main && mkdir sub)
say "P15 inside a repository that GIT_CEILING_DIRECTORIES hides (set to the repository's own folder; from sub/)" "$SCRATCH/p15/sub" GIT_CEILING_DIRECTORIES="$SCRATCH/p15"

echo "## P16 TestH18AMissingRepositoryGetsNoHint as written here, run on round 1's tree ($ROUND1)"
mkdir "$SCRATCH/r1" && git archive "$ROUND1" | tar -x -C "$SCRATCH/r1" && cp cmd/handoff/pickup_test.go "$SCRATCH/r1/cmd/handoff/pickup_test.go"
(cd "$SCRATCH/r1" && go test -count=1 -run 'TestH18AMissingRepositoryGetsNoHint$' ./cmd/handoff 2>&1 | grep -E '^(--- |ok|FAIL|[[:space:]]+pickup_test)' | cut -c1-220 | sed 's/^/  /'; echo "  [go test exit ${PIPESTATUS[0]}]")
echo "## P16 (continued) the two new tests here"
go test -count=1 -v -run 'TestH18AMissingRepositoryGetsNoHint$|TestH18NoRepositoryMeansGitInitFixesIt$' ./cmd/handoff ./internal/gitx 2>&1 | grep -E '^(--- |ok|FAIL)' | sed 's/^/  /'

echo
echo "# alerts sent to the scratch file during these probes: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} 2>&1 | sed "s|$SCRATCH|<scratch>|g; s|$WT|<worktree>|g; s|$HOME|~|g" > "$OUT/probes2.log"
