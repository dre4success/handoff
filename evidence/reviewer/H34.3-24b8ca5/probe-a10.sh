#!/bin/zsh
# Reviewer's probe A10 for H34.3 round 3 (e92ef74): reviewer-2's #563. Usage: probe-a10.sh <a new, empty scratch dir> <handoff binary> <atterm_y.py> <the same build with HoldFor cut to 6 s>
D=$1; H=$2; AT=$3; H6=$4
export HANDOFF_TEST_NOTIFY="$D/notify.log"
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID ANTIGRAVITY_CONVERSATION_ID HANDOFF_ROLE
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
new() { mkdir -p "$D/$1"; cd "$D/$1"; git init -q -b main && git commit -q --allow-empty -m base && $H init --roles implementer,reviewer,reviewer-2,reviewer-3 --main main >/dev/null; echo body > ../body.md; echo two > ../body2.md; : > "$HANDOFF_TEST_NOTIFY"; C=$(git rev-parse HEAD); git commit -q --allow-empty -m other; C2=$(git rev-parse HEAD); }
tidy() { sed -E 's/at 20[0-9T:.-]+Z/at TIME/g' | cut -c1-${W:-200}; }
q() { "$@" >/dev/null 2>&1; }
post() { ${B:-$H} post --as implementer --item $1 --to $2 --commit $3 --title "$1" --body ../${5:-body}.md ${=4} 2>&1 | grep -E "^posted|^handoff: [^n]" | tidy; }
reply() { ${B:-$H} reply --as $1 $2 --verdict $3 --commit $4 --body ../body.md 2>&1 | grep -E "^posted" | tidy; }
w() { echo "\$ handoff wait --for implementer --timeout ${1:-2}s $2"; ${B:-$H} wait --for implementer --timeout ${1:-2}s ${=2} 2>&1 | tidy | sed 's/^/    /'; echo "    [exit ${pipestatus[1]}]"; }

hook() { echo "\$ (I, Stop hook) handoff hook listen --timeout ${1:-2}s"; echo '{"session_id":"I","hook_event_name":"Stop"}' | CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H hook listen --timeout ${1:-2}s 2>&1 | tidy | sed 's/^/    /'; echo "    [exit ${pipestatus[2]}; cursor $(cat .handoff/cursors/implementer.json 2>/dev/null || find .handoff -name '*cursor*' | head -1 | xargs cat 2>/dev/null)]"; }
echo "## A10. The cursor can't be written (its folder is read-only); a note waits; the Stop hook, three times"
new a10; sleep 300 & PI=$!
CLAUDE_CODE_SESSION_ID=I CLAUDE_PID=$PI $H role implementer >/dev/null
hook 1 | grep -v "nothing new" | tail -1
$H note --as reviewer --to implementer --item P1 --title "one note" --body ../body.md 2>&1 | grep "^posted" | tidy
mkdir -p .handoff/cursors; chmod 555 .handoff/cursors
hook 2; hook 2; hook 2
chmod 755 .handoff/cursors
echo "-- the folder is writable again:"
hook 2; hook 2 | grep -v "nothing new"
echo "  alerts to the operator: $(grep -c . "$HANDOFF_TEST_NOTIFY")"
kill $PI 2>/dev/null
