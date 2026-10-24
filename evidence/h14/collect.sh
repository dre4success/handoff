#!/usr/bin/env bash
# Collects the H14 evidence. Run from the root of a checkout of the commit
# under review:
#   bash evidence/h14/collect.sh <patterns file>
# The patterns file stays outside the repository, since it names what was
# removed: one line each, a label, a tab, a Python regular expression.
# SCRUB is the commit that renamed; alerts go to a file.
set -u
PATTERNS=${1:?usage: collect.sh <patterns file>}
OUT=${OUT:-evidence/h14}
COMMIT=$(git rev-parse HEAD)
SCRUB=${SCRUB:-$(git log --format=%h -1 --grep='^refactor: name no person')}
BY="Opus 5.5 (implementer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() {
  printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"
}
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
[ -n "$SCRUB" ] || { echo "no scrub commit found"; exit 1; }
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1

# 1. The sweep: every tracked file, its contents and its name, for each pattern.
{
  header "every tracked file's contents and name, searched for each pattern of a file kept outside the repository"
  git ls-files -z | python3 -I -c '
import re, sys
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
files = [f for f in sys.stdin.read().split("\0") if f]
print(f"{len(files)} tracked files")
for label, rx in pats:
    r = re.compile(rx)
    lines = names = 0
    for f in files:
        names += bool(r.search(f))
        try:
            text = open(f, encoding="utf-8", errors="surrogateescape").read()
        except OSError:
            continue
        lines += sum(1 for l in text.split("\n") if r.search(l))
    print(f"{label}: {lines} lines, {names} file names")
' "$PATTERNS"
} > "$OUT/sweep.log" 2>&1

# 2. The renaming commit changes words only: as many lines in as out, and
# these are the words it put in (what it took out is not repeated here).
{
  header "git diff --numstat and --word-diff=porcelain $SCRUB^ $SCRUB"
  git diff --numstat "$SCRUB^" "$SCRUB" | awk '{a+=$1; d+=$2; n++} END {print n " files; " a " lines in, " d " lines out"}'
  git diff -M --name-status "$SCRUB^" "$SCRUB" | awk '$1 ~ /^R/ {print "renamed: " $3}'
  echo "## each word or phrase put in, with how often:"
  git diff --word-diff=porcelain "$SCRUB^" "$SCRUB" | grep '^+' | grep -v '^+++' | sed 's/^+//' | sort | uniq -c | sort -rn
} > "$OUT/scrub-diff.log" 2>&1

# 3. What an agent now reads: the brief, in a project made with defaults.
{
  header "handoff init, then handoff brief --for claude, in a scratch repository"
  (mkdir "$SCRATCH/p" && cd "$SCRATCH/p" && git init -q -b main && echo x > f && git add f && git commit -qm base &&
    "$SCRATCH/handoff" init > /dev/null && "$SCRATCH/handoff" brief --for claude | sed "s|$SCRATCH|<scratch>|g")
} > "$OUT/brief.txt" 2>&1

# 4. Every test under the race detector.
{
  header "go test -count=1 -race -v ./... (one line per test)"
  go test -count=1 -race -v ./... 2>&1 | grep -E '^(--- |ok|FAIL|PASS)'
  echo "# go test exit: ${PIPESTATUS[0]}"
} > "$OUT/go-test-race.log"

# 5. No dependencies, vet and gofmt.
{
  header "go.mod requires; go vet ./...; gofmt -l cmd internal"
  echo "## go.mod require lines: $(grep -c '^require' go.mod)"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
} > "$OUT/deps-vet-fmt.log" 2>&1

# 6. The trial's record (§17 row 78): Fable's chain check from
# evidence/trial/collect.sh, reporting each link, on the record before the
# renaming and on the redacted copy here; which lines differ, by number only,
# since their text names the operator; and the tool reading the copy.
{
  header "Fable's chain check on evidence/trial/record before ($SCRUB^) and after ($COMMIT); handoff inbox and show in a project holding the copy"
  for rev in "$SCRUB^" "$COMMIT"; do
    echo "## at $(git rev-parse --short "$rev")"
    git show "$rev:evidence/trial/record/channel.jsonl" > "$SCRATCH/channel-$rev.jsonl"
    git show "$rev:evidence/trial/record/head.json" > "$SCRATCH/head-$rev.json"
    python3 -I - "$SCRATCH/channel-$rev.jsonl" "$SCRATCH/head-$rev.json" <<'PY'
import sys, json, hashlib
lines = open(sys.argv[1],'rb').read().split(b'\n')[:-1]; prev='genesis'; ok=True
for i,l in enumerate(lines,1):
    e=json.loads(l)
    match = e['prev']==prev; ok = ok and match
    print(f"line {i}: prev {'matches' if match else 'MISMATCH'}")
    prev=hashlib.sha256(l).hexdigest()
h=json.load(open(sys.argv[2]))
print("chain verifies:",ok,"| head.lines==lines:",h['lines']==len(lines),"| head.last==sha256(last line):",h['last']==prev)
PY
  done
  echo "## lines that differ, by number"
  python3 -I -c '
import sys
a, b = (open(f, "rb").read().split(b"\n") for f in sys.argv[1:3])
print("lines:", len(a) - 1, "and", len(b) - 1, "| differ:", [i for i, (x, y) in enumerate(zip(a, b), 1) if x != y])
' "$SCRATCH/channel-$SCRUB^.jsonl" "$SCRATCH/channel-$COMMIT.jsonl"
  for rev in "$SCRUB^" "$COMMIT"; do
    cmp -s "$SCRATCH/head-$rev.json" evidence/trial/record/head.json && echo "## head.json at $(git rev-parse --short "$rev"): the same as here"
  done
  git diff --quiet "$SCRUB^" "$COMMIT" -- evidence/trial/record/config.json && echo "## config.json: unchanged"
  echo "## the tool, in a scratch project whose .handoff/ holds the copy"
  (mkdir -p "$SCRATCH/t/.handoff" && cp evidence/trial/record/* "$SCRATCH/t/.handoff/" && cd "$SCRATCH/t" && git init -q -b main &&
    "$SCRATCH/handoff" inbox --for operator; echo "# inbox exit: $?"
    "$SCRATCH/handoff" show 5 | sed -n '1,5p')
} > "$OUT/trial-record.log" 2>&1

# 7. No other hash in the tree is of bytes the renaming changed: searched in
# the tree before it and here.
{
  header "python3 -I evidence/h14/stale-hashes.py $SCRUB^ $SCRUB $SCRUB^ $COMMIT"
  python3 -I evidence/h14/stale-hashes.py "$SCRUB^" "$SCRUB" "$SCRUB^" "$COMMIT"
  echo "# exit: $?"
} > "$OUT/stale-hashes.log" 2>&1
