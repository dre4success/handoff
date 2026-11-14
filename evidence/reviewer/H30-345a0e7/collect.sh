#!/usr/bin/env bash
# The reviewer's build, vet and test logs for H30 (345a0e7). Run as
#   W=<detached worktree at the commit> WB=<the base's tree, 2336ecd, from git archive> WM=<a scratch copy of W's tree, for the changes tried below> OUT=<this dir> ALL=<a saved go test -race -count=1 -json ./... output from W> bash collect.sh
# The Go tests send alerts to /dev/null by themselves (TestMain).
set -u
W=${W:?worktree}; WB=${WB:?tree at the base}; WM=${WM:?scratch copy}; OUT=${OUT:?out}; ALL=${ALL:?saved test output}
BASE=2336ecd97bb887cfce119980ce382eb28b1f37d5
COMMIT=$(git -C "$W" rev-parse HEAD)
BY="Fable 5.2 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
clean() { perl -pe "s|\Q$WM\E|<scratch copy>|g; s|\Q$WB\E|<the base's tree>|g; s|\Q$W\E|<worktree>|g; s|\Q$HOME\E|~|g"; }
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
cd "$W"
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go.mod requires; go list -deps for net; the commits and diffstat from the base; what the later commits touch"
  go build ./... && echo "## go build ./...: ok"
  go vet ./... && echo "## go vet ./...: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  echo "## packages in go list -deps ./... named net or under net/: $(go list -deps ./... | grep -cE '^net(/|$)')"
  echo "## commits on the base (2336ecd):"; git log --oneline --no-decorate $BASE..HEAD
  echo "## diffstat:"; git diff --stat $BASE..HEAD
  echo "## the implementer's test log and replay name 3a5d686; what the commit under review adds to it:"; git diff --stat 3a5d686..HEAD
  echo "## files under cmd or internal that differ from the base:"; git diff --stat $BASE..HEAD -- cmd internal | sed '$d'
  echo "## the worktree is clean: [$(git status --short)]"
} 2>&1 | clean > "$OUT/build-vet-deps.log"
{
  header "go test -race -count=1 -json ./... (no -timeout but go's own); the test names at the base and here; the H30 tests, -count=5; the H30 tests with one condition of the change taken out at a time"
  echo "# (the full run was started at $(stat -f '%SB' -t '%Y-%m-%d %H:%M:%S' "$ALL" 2>/dev/null) local time, in this worktree, with a scratch HOME, and its output is read here)"
  python3 -I - "$ALL" <<'EOF'
import json, sys, collections
tests = collections.Counter(); sub = collections.Counter(); races = 0; pk = []
for l in open(sys.argv[1]):
    try: e = json.loads(l)
    except ValueError: continue
    a = e.get("Action")
    if a in ("pass", "fail", "skip"):
        if e.get("Test"): (sub if "/" in e["Test"] else tests)[a] += 1
        else: pk.append((a, e["Package"], e.get("Elapsed")))
    if "DATA RACE" in e.get("Output", ""): races += 1
for a, p, s in sorted(pk, key=lambda x: x[1]): print("%-5s %s  %ss" % (a, p, s))
print("# tests: passed %d, failed %d, skipped %d; subtests: passed %d, failed %d; data races: %d" % (tests["pass"], tests["fail"], tests["skip"], sub["pass"], sub["fail"], races))
EOF
  echo; echo "## the tests' names at the base that are not here, and here that are not at the base"
  (cd "$WB" && go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort) > "$T/base.names"
  go test -count=1 -list '.*' ./... 2>/dev/null | grep '^Test' | sort > "$T/here.names"
  diff "$T/base.names" "$T/here.names" | grep -E '^[<>]' || echo "(none)"
  echo "# tests at the base: $(wc -l < "$T/base.names" | tr -d ' '); here: $(wc -l < "$T/here.names" | tr -d ' ')"
  echo; echo "## the H30 tests, 5 times each under the race detector"
  go test -race -count=5 -run '^TestH30' -v ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|--- |panic|WARNING: DATA RACE)' | sed -E 's/ \([0-9.]+s\)//; s/\t[0-9.]+s$//' | sort | uniq -c
  echo; echo "## the H30 tests with one condition taken out of cmd/handoff/session.go at a time, in a scratch copy of the tree"
  echo "# (which tests notice each condition; a condition no test notices is not wrong, only untested)"
  try() { # label, perl edit
    cp "$W/cmd/handoff/session.go" "$WM/cmd/handoff/session.go"
    perl -0pi -e "$2" "$WM/cmd/handoff/session.go"
    if cmp -s "$W/cmd/handoff/session.go" "$WM/cmd/handoff/session.go"; then echo "$1: THE EDIT CHANGED NOTHING"; return; fi
    (cd "$WM" && go test -count=1 -run '^TestH30' -v ./cmd/handoff 2>&1 | grep -E '^\s*--- FAIL|^(ok|FAIL)\s|cannot|undefined|declared and not used|imported and not used' | sed -E 's/ \([0-9.]+s\)//; s/\t[0-9.]+s$//' | tr '\n' ';' | sed "s|^|$1: |"; echo)
  }
  try "0. nothing taken out" 's/(\/\/ moveContinued moves a role)/\/\/ (unchanged)\n$1/'
  try "1. the move itself (sessionRole never calls moveContinued)" 's/if in\.Transcript != "" \{/if false \&\& in.Transcript != "" {/'
  try "2. \"its process has ended\" (!b.running())" 's/if !b\.running\(\) && wentOnAs/if wentOnAs/'
  try "3. the hook's transcript is named after its session" 's/ \|\| filepath\.Base\(transcript\) != s\.ID\+"\.jsonl"//'
  try "4. the old id is a plain name (in wentOnAs)" 's/\t\tif !plainID\.MatchString\(id\) \{\n\t\t\treturn false\n\t\t\}\n//'
  try "5. exactly one binding qualifies" 's/if len\(from\) != 1 \{/if len(from) == 0 {/'
  try "6. the line's type is continued-in" 's/ && e\.Type == "continued-in"//'
  try "7. the last such line wins (first one taken instead)" 's/\t\t\tnext = e\.Next\n/\t\t\tif next == "" {\n\t\t\t\tnext = e.Next\n\t\t\t}\n/'
  cp "$W/cmd/handoff/session.go" "$WM/cmd/handoff/session.go"
} 2>&1 | clean > "$OUT/go-test-race.log"
