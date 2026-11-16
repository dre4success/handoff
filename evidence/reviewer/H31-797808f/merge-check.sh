#!/usr/bin/env bash
# merge-check.sh <repository> <commit> <empty scratch folder>: main has moved since the request (H32 is in).
# A scratch clone, main checked out, the commit merged: the conflicts; then the plain way to settle them
# (both variables in the test's list; the rows in number order), a build, and the H12, H30, H31 and H32 tests.
set -u
R=$1; C=$2; T=$3
printf '# command: bash merge-check.sh <repository> <commit> <an empty scratch folder>\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the Mac of the operator (darwin arm64, %s, git %s)\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$C"
git clone -q --no-hardlinks "$R" "$T/clone" && cd "$T/clone" || exit 1
MAIN=$(git rev-parse main); git checkout -q --detach "$MAIN"
echo "## main is $MAIN; commits on main that the commit doesn't have:"; git log --oneline --no-decorate "$C..$MAIN" | sed 's/^/   /'
echo "## git merge --no-commit <commit>:"; git -c user.name=probe -c user.email=probe@example.invalid merge --no-commit "$C" 2>&1 | sed 's/^/   /'
echo "## files in conflict:"; git diff --name-only --diff-filter=U | sed 's/^/   /'
echo "## cmd/handoff/main_test.go:"; sed -n '/<<<<<<</,/>>>>>>>/p' cmd/handoff/main_test.go | cut -c1-150 | sed 's/^/   /'
echo "## the proposal: the rows on each side:"; awk '/^<<<<<<</,/^>>>>>>>/' docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md | cut -c1-48 | sed 's/^/   /'
python3 -I - <<'PY'
import re
p = "cmd/handoff/main_test.go"; s = open(p).read()
s = re.sub(r"<<<<<<< HEAD\n.*?\n=======\n.*?\n>>>>>>> [0-9a-f]+\n", 'var sessionVars = []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_PID", "CODEX_THREAD_ID", "ANTIGRAVITY_CONVERSATION_ID", "CLAUDE_PROJECT_DIR"}\n', s, flags=re.S)
open(p, "w").write(s)
p = "docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md"; s = open(p).read()
s = re.sub(r"<<<<<<< HEAD\n(.*?\n)=======\n(.*?\n)>>>>>>> [0-9a-f]+\n", lambda m: m.group(2).split("\n")[0] + "\n" + m.group(1) + "\n".join(m.group(2).split("\n")[1:]), s, flags=re.S)
open(p, "w").write(s)
PY
echo "## settled the plain way; the rows now end: $(grep -o '^| 10[0-3] |' docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md | tr -d '| ' | tr '\n' ' ')"
go build -o "$T/handoff-merged" ./cmd/handoff && go vet ./... && echo "## go build, go vet: ok"
echo "## go test -count=1 -run 'TestH31|TestH32|TestH30|TestH12' ./cmd/handoff:"; HOME=$T/home go test -count=1 -run 'TestH31|TestH32|TestH30|TestH12' ./cmd/handoff 2>&1 | sed -E 's/\t[0-9.]+s$//' | sed 's/^/   /'
mkdir -p "$T/p" "$T/out" "$T/home" && git -C "$T/p" init -q -b main && git -C "$T/p" -c user.name=probe -c user.email=probe@example.invalid commit -q --allow-empty -m s && cd "$T/p" && HOME=$T/home HANDOFF_TEST_NOTIFY=$T/alerts.txt "$T/handoff-merged" init > /dev/null
echo "## an Antigravity session takes a role in a scratch project, with the merged build: exit $(env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID -u CODEX_THREAD_ID HOME=$T/home CODEX_HOME=$T/codex HANDOFF_TEST_NOTIFY=$T/alerts.txt ANTIGRAVITY_CONVERSATION_ID=c-1 "$T/handoff-merged" role reviewer > "$T/o.txt" 2>&1; echo $?), $(grep -c '^handoff: warning' "$T/o.txt") warning (it reads no transcript of Antigravity's, as row 102 says)"
