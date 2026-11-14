#!/bin/sh
# hookwrap.sh <brief|listen|notify|end>: keeps what Claude Code gave the hook, then runs the
# build under review on it, in a scratch record kept apart from the session's own folder.
# It does as little as it can first: the file's times say when the hook started.
D=<scratch>; BIN=<handoff built at the commit>
in=$D/in/$$.$1
cat > "$in"
[ "$1" = end ] && exit 0
cd "$D/rec" || exit 0
export HANDOFF_TEST_NOTIFY=$D/alerts.txt HANDOFF_TEST_AWAKE=$D/awake.txt
if [ "$1" = brief ]; then "$BIN" hook brief < "$in" > "$in.out"; cat "$in.out"; exit 0; fi
exec "$BIN" hook "$1" < "$in"
