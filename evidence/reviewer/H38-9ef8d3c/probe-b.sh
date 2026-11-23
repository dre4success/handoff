#!/bin/zsh
# Reviewer's probe for H38 (dc109a9): reviewer-2's #906, run again. A listener of the build before H38 is already waiting
# for implementer when the request is withdrawn and a late APPROVE is posted, both with the new build.
# Usage: probe-b.sh <a new, empty scratch dir> <handoff binary> <atterm.py> <the build of main it is based on (22791ca)>
D=$1; H=$2; AT=$3; OLD=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
tidy() { sed -E "s#/private/tmp/claude-501/[^ \`']*/handoff[-a-z]*#handoff#g" | cut -c1-260; }
last() { python3 -c "
import json
for l in open('.handoff/channel.jsonl'): e=json.loads(l)
print(e['id'])"; }
for which in old new; do
  [[ $which == old ]] && L=$OLD || L=$H
  mkdir -p $D/$which; cd $D/$which; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; echo body > ../body.md; : > "$HANDOFF_TEST_NOTIFY"
  git checkout -q -b work && git commit -q --allow-empty -m work && C=$(git rev-parse HEAD) && git checkout -q main
  echo "## The implementer's listener is the $which build"
  $H post --as implementer --item W1 --to reviewer --commit $C --title W1 --body ../body.md 2>&1 | grep "^posted" | tidy; R=$(last)
  python3 $AT "" $H away "carry on" 2>&1 | grep "^away" | tidy
  $L wait --for implementer --timeout 3s >/dev/null 2>&1   # takes the away's fyi, so the next wait starts clean
  ( $L wait --for implementer --timeout 8s > $D/$which-wait.out 2>&1; echo "exit $?" >> $D/$which-wait.out ) & WP=$!; sleep 1
  $H withdraw --as implementer $R 2>&1 | grep "^posted" | tidy
  $H reply --as reviewer $R --verdict APPROVE --commit $C --body ../body.md 2>&1 | grep "^posted\|withdrawn" | tidy
  wait $WP; echo "  what the listener gave the implementer:"; grep -v "told the operator" $D/$which-wait.out | tidy | sed 's/^/    /'
  echo "  alerts to the operator: $(grep -v 'go-ahead is recorded' "$HANDOFF_TEST_NOTIFY" | tidy | tr '\n' '|')"; echo
done
