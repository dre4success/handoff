#!/usr/bin/env bash
# The reviewer's H17 evidence. Run from a checkout of the commit under review:
#   OUT=<this directory> bash collect.sh <h14-scrub.py> <h14-patterns.txt>
# The script and the patterns file are H14's, kept outside the repository.
# Alerts and sleep holds go to scratch files. The race suite is collected
# separately (go-test-race.log), since it runs for eight minutes.
set -u
OUT=${OUT:?}
SCRUBPY=$(cd "$(dirname "${1:?h14-scrub.py}")" && pwd -P)/$(basename "$1")
PATTERNS=$(cd "$(dirname "${2:?h14-patterns.txt}")" && pwd -P)/$(basename "$2")
COMMIT=$(git rev-parse HEAD)
BASE=f1fcd1738894e9dcd28c7ac4b2fed5bf7aa87b04
SCRUB=d9dd08e
TAGGED_AT=9eae9b94e223a5657306adbf54a9b73a52769bf7 # HEAD^: the commit before the evidence commit
REPO=$(cd "$(git rev-parse --git-common-dir)/.." && pwd -P)
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
header() { printf '# command: %s\n# date: %s\n# by: %s\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"; }
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
trap 'git worktree remove --force "$SCRATCH/base" 2>/dev/null; rm -rf "$SCRATCH"' EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
REGEX='^handoff (\(devel\)|v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?(\+dirty)?)$'
check() { printf '%-22s %s  -> ' "$1" "$2"; echo "$2" | grep -Eq "$REGEX" && echo "matches the test's regex" || echo "DOES NOT MATCH the test's regex"; }

# 1. Build, vet, gofmt, no net, no dependencies; what H17 is.
{
  header "go build ./...; go vet ./...; gofmt -l cmd internal; go list -deps ./... | grep net; go.mod; git log/diff --stat"
  go build ./... && echo "## go build: ok"
  go vet ./... && echo "## go vet: clean"
  echo "## gofmt -l cmd internal: [$(gofmt -l cmd internal)]"
  echo "## go list -deps ./... matching net: [$(go list -deps ./... | grep -E '(^|/)net(/|$)' | tr '\n' ' ')]"
  echo "## go.mod: $(grep -c '^require' go.mod) require lines; $(grep '^go ' go.mod)"
  echo "## commits on top of the base $BASE: $(git rev-list --count $BASE..HEAD)"
  git log --oneline $BASE..HEAD
  echo "## files changed after $TAGGED_AT, where the implementer's evidence was collected:"
  git diff --stat $TAGGED_AT HEAD
} > "$OUT/build-vet-deps.log" 2>&1

# 2. handoff version: here, in a plain clone at a tag, untagged, after a tag, dirty; by go install from a file proxy; outside a project; and the test.
{
  header "go build and handoff version in this worktree and in a plain clone (tagged, untagged, after a tag, dirty); go install …@v0.1.0 via a file:// module proxy; the version test in both"
  echo "## built in this checkout (a linked worktree):"
  go build -o "$SCRATCH/here" ./cmd/handoff && check "worktree" "$("$SCRATCH/here" version)"
  go version -m "$SCRATCH/here" | grep -E '^\s+(mod|vcs)' || echo "(no vcs lines)"
  git clone -q --no-hardlinks "$REPO" "$SCRATCH/clone" && cd "$SCRATCH/clone" && git checkout -q "$COMMIT"
  echo "## a plain clone, same commit, no tag:"
  go build -o "$SCRATCH/untagged" ./cmd/handoff && check "clone, untagged" "$("$SCRATCH/untagged" version)"
  go version -m "$SCRATCH/untagged" | grep -E '^\s+(mod|vcs\.(revision|modified))'
  echo "## the clone with v0.1.0 on this commit:"
  git tag v0.1.0 && go build -o "$SCRATCH/tagged" ./cmd/handoff && check "clone, at the tag" "$("$SCRATCH/tagged" version)"
  echo "## the clone with v0.1.0 on this commit and a dirty tree:"
  echo x >> README.md && go build -o "$SCRATCH/dirty" ./cmd/handoff && check "clone, tag, dirty" "$("$SCRATCH/dirty" version)"; git checkout -q -- README.md
  echo "## the clone one commit after the tag (v0.1.0 moved to $TAGGED_AT, built at this commit):"
  git tag -d v0.1.0 >/dev/null && git tag v0.1.0 "$TAGGED_AT" && go build -o "$SCRATCH/after" ./cmd/handoff && check "clone, after the tag" "$("$SCRATCH/after" version)"
  echo "## go install github.com/dre4success/handoff/cmd/handoff@v0.1.0 through a file:// module proxy made from the tag (on this commit):"
  git tag -d v0.1.0 >/dev/null && git tag v0.1.0
  P="$SCRATCH/proxy/github.com/dre4success/handoff/@v" && mkdir -p "$P"
  git archive --format=zip --prefix=github.com/dre4success/handoff@v0.1.0/ -o "$P/v0.1.0.zip" v0.1.0
  git show v0.1.0:go.mod > "$P/v0.1.0.mod"; echo '{"Version":"v0.1.0","Time":"2026-10-24T21:00:00Z"}' > "$P/v0.1.0.info"; echo v0.1.0 > "$P/list"
  (cd / && GOBIN="$SCRATCH/gobin" GOPROXY="file://$SCRATCH/proxy" GOSUMDB=off GOFLAGS= go install github.com/dre4success/handoff/cmd/handoff@v0.1.0 2>&1)
  check "go install @v0.1.0" "$("$SCRATCH/gobin/handoff" version)"; check "--version" "$("$SCRATCH/gobin/handoff" --version)"
  go version -m "$SCRATCH/gobin/handoff" | grep -E '^\s+(path|mod)'
  echo "## version from /, outside any project or repository: $(cd / && "$SCRATCH/gobin/handoff" version); exit $?"
  echo "## TestH17VersionNamesTheBuild in the plain clone, no tag:"
  git tag -d v0.1.0 >/dev/null
  go test -count=1 -run TestH17VersionNamesTheBuild ./cmd/handoff 2>&1 | grep -E 'version_test|^(ok|FAIL|---)' | cut -c1-200
  echo "# go test exit: ${PIPESTATUS[0]}"
  cd "$OLDPWD"
  echo "## the same test in this worktree:"
  go test -count=1 -run TestH17VersionNamesTheBuild ./cmd/handoff 2>&1 | grep -E '^(ok|FAIL|---)'
} > "$OUT/version.log" 2>&1

# 3. The renaming reproduces from H14's script on the base.
{
  header "python3 -I h14-scrub.py <worktree at $BASE>; git diff against $SCRUB; tree ids"
  git worktree add -q --detach "$SCRATCH/base" "$BASE"
  python3 -I "$SCRUBPY" "$SCRATCH/base"
  (cd "$SCRATCH/base" && git add -A && git diff --cached --stat | tail -3 && git diff --cached | grep -v '^index ' > "$SCRATCH/reproduced.diff")
  git diff "$SCRUB^" "$SCRUB" | grep -v '^index ' > "$SCRATCH/committed.diff"
  if cmp -s "$SCRATCH/reproduced.diff" "$SCRATCH/committed.diff"; then echo "## the reproduced diff is identical to $SCRUB's"; else echo "## DIFFERS:"; diff "$SCRATCH/reproduced.diff" "$SCRATCH/committed.diff"; fi
  echo "## tree the script makes: $(cd "$SCRATCH/base" && git write-tree)"
  echo "## tree of $SCRUB:         $(git rev-parse "$SCRUB^{tree}")"
  git worktree remove --force "$SCRATCH/base"
} > "$OUT/scrub.log" 2>&1

# 4. The sweep, counts only, at the base and here; the stale-hash sweep.
cat > "$SCRATCH/sweep.py" <<'PY'
import re, sys
pats = [l.rstrip("\n").split("\t", 1) for l in open(sys.argv[1]) if l.strip()]
files = [f for f in sys.stdin.read().split("\0") if f]
print(f"{len(files)} files")
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
PY
{
  header "H14's sweep (the patterns file outside the repository) over git archive of $BASE and of this commit; python3 -I evidence/h14/stale-hashes.py $SCRUB^ $SCRUB $SCRUB^ $COMMIT"
  for rev in "$BASE" "$COMMIT"; do
    echo "## $rev"; mkdir "$SCRATCH/t" && git archive "$rev" | tar -x -C "$SCRATCH/t" && (cd "$SCRATCH/t" && find . -type f | sed 's|^\./||' | tr '\n' '\0' | python3 -I "$SCRATCH/sweep.py" "$PATTERNS"); rm -rf "$SCRATCH/t"
  done
  echo "## hashes"; python3 -I evidence/h14/stale-hashes.py "$SCRUB^" "$SCRUB" "$SCRUB^" "$COMMIT"
} > "$OUT/sweep.log" 2>&1

# 5. A dry run of the merge onto main, objects to a temporary directory.
{
  header "git merge-tree --write-tree --name-only main $COMMIT, GIT_OBJECT_DIRECTORY in a temporary directory"
  MAIN=$(git rev-parse main); OBJ=$(cd "$(git rev-parse --git-common-dir)" && pwd -P)/objects
  echo "## main is $MAIN; since the base it changed: $(git diff --name-only $BASE $MAIN | tr '\n' ' ')"
  before=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  mkdir "$SCRATCH/obj"; GIT_OBJECT_DIRECTORY="$SCRATCH/obj" GIT_ALTERNATE_OBJECT_DIRECTORIES="$OBJ" git merge-tree --write-tree --name-only "$MAIN" "$COMMIT" 2>/dev/null
  echo "# merge-tree exit: $? (0: clean, no conflicts)"
  after=$(git count-objects -v 2>/dev/null | awk '/^count:/{c=$2} /^in-pack:/{p=$2} END{print c+p}')
  echo "## objects in the repository before $before, after $after; in the temporary directory: $(find "$SCRATCH/obj" -type f | wc -l | tr -d ' ')"
} > "$OUT/merge-dry-run.log" 2>&1

# 6. The README against the tool: its length; what it dropped that handoff help, init, settings or the brief say; which CLI gets the permission-prompt hook.
{
  header "wc -w README.md at $BASE and here; handoff help; handoff settings claude|codex and init in a scratch project; grep"
  echo "## words at $BASE: $(git show "$BASE:README.md" | wc -w | tr -d ' '); here: $(wc -w < README.md | tr -d ' ')"
  H="$SCRATCH/here"
  echo "## handoff help says: exit codes: $("$H" help 2>&1 | grep -c 'Exit codes'); HANDOFF_ROLE: $("$H" help 2>&1 | grep -c HANDOFF_ROLE); --supersedes: $("$H" help 2>&1 | grep -c -- --supersedes); --main: $("$H" help 2>&1 | grep -c -- --main); which commands need no role: $("$H" help 2>&1 | grep -c 'need no role')"
  mkdir "$SCRATCH/p" && cd "$SCRATCH/p" && git init -q -b main && echo x > f && git add f && GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t git commit -qm base
  "$H" init > "$SCRATCH/init.txt" 2>&1; "$H" settings claude > "$SCRATCH/sc.txt" 2>&1; "$H" settings codex > "$SCRATCH/sx.txt" 2>&1
  CLAUDE_CODE_SESSION_ID=s1 "$H" role implementer > "$SCRATCH/brief.txt" 2>&1
  echo "## 'worktree' or 'one record' in: init output: $(grep -ci 'worktree\|one record' "$SCRATCH/init.txt"); settings claude: $(grep -ci 'worktree\|one record' "$SCRATCH/sc.txt"); settings codex: $(grep -ci 'worktree\|one record' "$SCRATCH/sx.txt"); the brief: $(grep -ci 'worktree\|one record' "$SCRATCH/brief.txt"); help: $("$H" help 2>&1 | grep -ci 'worktree\|one record')"
  echo "## hooks in settings claude: $(grep -oE '"(SessionStart|Stop|Notification)"' "$SCRATCH/sc.txt" | sort -u | tr '\n' ' ')"
  echo "## hooks in settings codex:  $(grep -oE '"(SessionStart|Stop|Notification)"' "$SCRATCH/sx.txt" | sort -u | tr '\n' ' ')"
  echo "## 'permission' in settings codex's text: $(grep -ci permission "$SCRATCH/sx.txt"); in settings claude's: $(grep -ci permission "$SCRATCH/sc.txt")"
  cd "$OLDPWD"
} > "$OUT/readme.log" 2>&1
echo "# alerts sent to the scratch file during this collection: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')" >> "$OUT/readme.log"
