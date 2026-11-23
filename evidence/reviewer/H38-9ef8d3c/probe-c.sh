#!/bin/zsh
# Reviewer's probe for H38 round 2 (9ef8d3c): a withdrawal and an APPROVE of the same request at once, N times.
# Never both written: either the APPROVE stands and the withdrawal is refused, or the request is withdrawn and the reply refused.
# Usage: probe-c.sh <a new, empty scratch dir> <handoff binary> <how many times>
D=$1; H=$2; N=${3:-40}
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
mkdir -p $D/r; cd $D/r; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer --main main >/dev/null; echo body > ../body.md; C=$(git rev-parse HEAD)
both=0; wfirst=0; afirst=0; neither=0
for i in $(seq 1 $N); do
  R=$($H post --as implementer --item W$i --to reviewer --commit $C --title W$i --body ../body.md 2>&1 | sed -n 's/^posted #\([0-9]*\).*/\1/p')
  ( $H withdraw --as implementer $R > $D/w$i.out 2>&1 ) & A=$!
  ( $H reply --as reviewer $R --verdict APPROVE --commit $C --body ../body.md > $D/a$i.out 2>&1 ) & B=$!
  wait $A $B
  w=$(grep -c "^posted" $D/w$i.out); a=$(grep -c "^posted" $D/a$i.out)
  case "$w$a" in 11) both=$((both+1)); echo "  run $i: both written";; 10) wfirst=$((wfirst+1));; 01) afirst=$((afirst+1));; *) neither=$((neither+1)); echo "  run $i: neither: $(head -1 $D/w$i.out | cut -c1-80) | $(head -1 $D/a$i.out | cut -c1-80)";; esac
done
echo "  $N runs: $both with both written; $wfirst withdrawn (the reply refused); $afirst approved (the withdrawal refused); $neither neither"
