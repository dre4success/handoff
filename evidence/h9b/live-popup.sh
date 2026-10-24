#!/usr/bin/env bash
# H9b live check: real pop-ups on the operator's screen, from a scratch project, with
# the operator clicking. It sets no HANDOFF_TEST_NOTIFY on purpose, so run it only
# with the operator present and agreed. From the root of a checkout:
#   bash evidence/h9b/live-popup.sh <out file>
set -u
OUT=$1
COMMIT=$(git rev-parse HEAD)
S=$(mktemp -d)
trap 'rm -rf "$S"' EXIT
go build -o "$S/handoff" ./cmd/handoff || exit 1
H=$S/handoff
R=$S/popup-demo
mkdir -p "$R" && (cd "$R" && git init -q -b main && git -c user.name=t -c user.email=t@t commit -q --allow-empty -m base)
(cd "$R" && "$H" init --roles claude,codex --reviewers claude,codex --main main >/dev/null)
SHA=$(cd "$R" && git rev-parse HEAD)
echo "please review" > "$S/req.md"; echo "looks right" > "$S/ok.md"; echo "Only to staging, tonight." > "$S/ask.md"
id() { sed -n 's/^posted #\([0-9]*\) .*/\1/p' | head -1; } # the entry a post, reply or note wrote
until_marked() { for _ in $(seq 600); do [ -e "$R/.handoff/notified/$1" ] && return 0; sleep 0.5; done; return 1; }
{
  printf '# command: live pop-ups from a scratch project, through this commit'"'"'s binary, with the operator clicking\n# date: %s\n# by: Opus 5.5 (implementer), on the operator'"'"'s Mac; the clicks are the operator'"'"'s\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$COMMIT"
  cd "$R"
  echo '$ handoff post --as claude --item H1 --to codex ... ; handoff reply --as codex <it> --verdict APPROVE ...'
  "$H" post --as claude --item H1 --to codex --commit "$SHA" --title "H1: the login page" --body "$S/req.md" | tee "$S/p1"
  REQ=$(id < "$S/p1")
  "$H" reply --as codex "$REQ" --verdict APPROVE --commit "$SHA" --body "$S/ok.md" 2>&1 | tee "$S/p2"
  REP=$(id < "$S/p2")
  echo "(the operator is asked to click Approve)"
  until_marked "decide-$REQ-$REP" && echo "the pop-up closed at $(date -u +%H:%M:%SZ)" || echo "no click within 5 minutes"
  sleep 1
  echo
  echo '$ handoff note --as claude --item H2 --to operator --title "May I deploy H2?" ...'
  "$H" note --as claude --item H2 --to operator --title "May I deploy H2?" --body "$S/ask.md" 2>&1 | tee "$S/p3"
  NOTE=$(id < "$S/p3")
  echo "(the operator is asked to click Deny, type a reason, and click Deny again)"
  until_marked "decide-note-$NOTE" && echo "the pop-up closed at $(date -u +%H:%M:%SZ)" || echo "no click within 5 minutes"
  sleep 1
  echo
  echo '$ handoff show <each entry after the reply>   (what the clicks wrote)'
  for ((i = REP + 1; ; i++)); do "$H" show "$i" >"$S/e" 2>/dev/null || break; cat "$S/e"; echo; done
  echo '$ handoff   (nothing should be left)'
  env -u HANDOFF_ROLE "$H" </dev/null 2>&1
} > "$OUT" 2>&1
