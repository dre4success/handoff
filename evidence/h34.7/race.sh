#!/bin/zsh
# H34.7 evidence: under a go-ahead, separate posts to three reviewers let the
# first approval approve the item before the other requests exist; one post
# to all three is one round, approved only when all three approve.
# Usage: race.sh <scratch dir> <handoff binary>
D=$1; H=$2; HERE=${0:A:h}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
P="$D/proj"; rm -rf "$P"; mkdir -p "$P"; cd "$P"
git init -q -b main && git commit -q --allow-empty -m base && git switch -q -c w1 && git commit -q --allow-empty -m work
$H init --roles implementer,reviewer,reviewer-2,reviewer-3 --reviewers reviewer,reviewer-2,reviewer-3 --main main >/dev/null
echo body > ../body.md
echo "\$ handoff away (at a terminal)"; python3 "$HERE/atterm.py" $H away "Merge when the reviewers approve." | head -1
run() { echo "\$ $*"; "$@" 2>&1 | grep -v "no session"; }
idof() { sed -n -E "s/^posted #([0-9]+) (request|round) .*/\\1/p" | head -1; }
if [ "$3" = round ]; then
  run $H post --as implementer --item W1 --to reviewer,reviewer-2,reviewer-3 --commit w1 --title W1 --body ../body.md | tee ../out
  first=$(idof < ../out)
  run $H reply $first --as reviewer --verdict APPROVE --commit w1 --body ../body.md
else
  run $H post --as implementer --item W1 --to reviewer --commit w1 --title W1 --body ../body.md | tee ../out
  first=$(idof < ../out)
  run $H reply $first --as reviewer --verdict APPROVE --commit w1 --body ../body.md
  run $H post --as implementer --item W1 --to reviewer-2 --commit w1 --title W1 --body ../body.md
  run $H post --as implementer --item W1 --to reviewer-3 --commit w1 --title W1 --body ../body.md
fi
echo "\$ handoff show $first | grep -E '^state|approved under'"; $H show $first | grep -E "^state|approved under"
