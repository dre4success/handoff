#!/usr/bin/env bash
# A hand-run of the step-1 slice in a scratch repository, so the reviewer can
# read the real output of each command. Run from the repository root:
#   bash evidence/step1/demo.sh > evidence/step1/demo.log 2>&1
set -u
ROOT=$(pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example
unset HANDOFF_ROLE

printf '# command: bash evidence/step1/demo.sh\n# date: %s\n# by: Opus 5.5 (implementer), Claude Code 2.1.295, on the operator'"'"'s Mac\n# commit: %s\n\n' \
  "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(git -C "$ROOT" rev-parse HEAD)"

go build -o "$WORK/handoff" "$ROOT/cmd/handoff" || exit 1
H="$WORK/handoff"

# show runs one command as a role, printing it, its output and its exit code.
show() {
  local role=$1; shift
  echo "\$ HANDOFF_ROLE=$role handoff $*"
  HANDOFF_ROLE=$role "$H" "$@" 2>&1
  echo "[exit $?]"
  echo
}

mkdir "$WORK/repo" && cd "$WORK/repo"
git init -q -b main
echo base > f.txt && git add f.txt && git commit -qm base
git switch -qc w31
echo "french labels" > f.txt && git commit -qam "W31: French labels"
A=$(git rev-parse HEAD)
echo "french labels, fixed" > f.txt && git commit -qam "W31: fix the accents"
B=$(git rev-parse HEAD)
printf 'Please review the French labels.\n' > "$WORK/req.md"
printf 'The accents on line 2 are wrong.\n' > "$WORK/changes.md"
printf 'Accents fixed in %s.\n' "$B" > "$WORK/r2.md"
printf 'Correct at this commit.\n' > "$WORK/ok.md"

show "" init --roles implementer,reviewer --main main

echo "# The reviewer waits in the background before anything is posted."
echo "\$ HANDOFF_ROLE=reviewer handoff wait --for reviewer --timeout 30s   (background)"
HANDOFF_ROLE=reviewer "$H" wait --for reviewer --timeout 30s > "$WORK/wait.out" 2>&1 &
WAITPID=$!
sleep 1.5
t0=$(perl -MTime::HiRes=time -e 'printf "%.3f", time')
show implementer post --item W31 --to reviewer --commit w31~1 --title "W31: French labels" --body "$WORK/req.md" --key w31-r1
wait $WAITPID; code=$?
t1=$(perl -MTime::HiRes=time -e 'printf "%.3f", time')
echo "background wait exited $code, $(perl -e "printf '%.2f', $t1 - $t0")s after the post began; it printed:"
cat "$WORK/wait.out"
echo

echo "# The same post, retried with the same key, returns the first receipt."
show implementer post --item W31 --to reviewer --commit w31~1 --title "W31: French labels" --body "$WORK/req.md" --key w31-r1

show reviewer claim 1
echo "# The first reply names a different commit from the request's and is refused; the second is accepted."
show reviewer reply 1 --verdict CHANGES --commit "$B" --body "$WORK/changes.md"
show reviewer reply 1 --verdict CHANGES --commit "$A" --body "$WORK/changes.md"
show implementer inbox

echo "# A new revision without --supersedes is refused; with it, accepted."
show implementer post --item W31 --to reviewer --commit "$B" --title "W31: French labels, r2" --body "$WORK/r2.md"
show implementer post --item W31 --to reviewer --commit "$B" --title "W31: French labels, r2" --body "$WORK/r2.md" --supersedes 1
show reviewer wait --for reviewer --timeout 5s
show reviewer reply 4 --verdict APPROVE --commit "$B" --body "$WORK/ok.md"
echo "# The implementer's role does not review, so its verdict is refused."
show implementer reply 4 --verdict APPROVE --commit "$B" --body "$WORK/ok.md"
show operator inbox
show "" show 1
show "" show 4
echo "# This script's stdin is not a terminal, so decide refuses, as it must for an agent's shell."
show operator decide 4 --approve --body "$WORK/ok.md"

echo "\$ head -c 600 .handoff/channel.jsonl; cat .handoff/head.json"
head -c 600 .handoff/channel.jsonl; echo " …"
cat .handoff/head.json
echo
echo "\$ git status --short   (the record is inside the repository and untracked)"
git status --short
