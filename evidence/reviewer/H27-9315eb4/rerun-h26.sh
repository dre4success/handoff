#!/usr/bin/env bash
# H27 says only the name changes. So the reviewer's H26 probes, both rounds
# (evidence/reviewer/H26-8162dbf/probes2.sh and round1-probes.sh, as committed),
# are run again at 9315eb4 with the command and the kind renamed and nothing
# else touched, and each log is compared with the one saved at 8162dbf. Run as
#   S=<scratchpad> B=<handoff built at the commit> BASE=<handoff built at 1af34c7> OLD=<handoff v0.1.2> WT=<worktree at the commit> OUT=<this dir> bash rerun-h26.sh
set -u
S=${S:?}; B=${B:?}; BASE=${BASE:?}; OLD=${OLD:?}; WT=${WT:?}; OUT=${OUT:?}
T=$(mktemp -d "$S/h27-rerun.XXXXXX")
# The rename, applied to the scripts: the command the probes run, and the
# kind their checks look for. Their headings and remarks keep the word tell.
rename() { sed -E 's/"\$(B|0)" tell /"$\1" fyi /g; s/"kind":"tell"/"kind":"fyi"/g; s/(\['"'"'kind'"'"'\] == )'"'"'tell'"'"'/\1'"'"'fyi'"'"'/'; }
# The fold, applied to both logs before comparing: the name, its article, the
# column the usage pads it to, and what differs from run to run (times, the
# header, the scratch folder's name, the hashes and scratch commits that
# follow from the times). A record's size in bytes is left in: fyi is one
# letter shorter than tell.
fold() { perl -pe 's/\b[Aa]n fyi\b/a tell/g; s/\bfyi\b/tell/g; s/\d{4}-\d\d-\d\dT[\d:]{8}Z/<ts>/g; s/work\.[A-Za-z0-9]{6}/work/g; s/\b[0-9a-f]{64}\b/<hash>/g; s/@ [0-9a-f]{12}\b/@ <sha>/g' | grep -v -E '^# (date|commit|command): ' | tr -s ' '; }
for p in probes2 round1-probes; do
  git -C "$WT" show "HEAD:evidence/reviewer/H26-8162dbf/$p.sh" > "$T/$p.orig.sh"
  rename < "$T/$p.orig.sh" > "$T/$p.sh"
  S="$S" B="$B" BASE="$BASE" OLD="$OLD" WT="$WT" bash "$T/$p.sh" > "$OUT/h26-$p-as-fyi.log" 2>&1
  {
    printf '# command: the H26 %s.sh as committed, with the command and kind renamed (rerun-h26.sh), run at this commit; its log against %s.log saved at 8162dbf, both folded\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac\n# commit: %s\n\n' "$p" "$p" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(git -C "$WT" rev-parse HEAD)"
    echo "## lines of the script the rename changed: $(diff "$T/$p.orig.sh" "$T/$p.sh" | grep -c '^>') of $(wc -l < "$T/$p.sh" | tr -d ' ')"
    git -C "$WT" show "HEAD:evidence/reviewer/H26-8162dbf/$p.log" | fold > "$T/$p.then"
    fold < "$OUT/h26-$p-as-fyi.log" > "$T/$p.now"
    echo "## lines then: $(wc -l < "$T/$p.then" | tr -d ' '); now: $(wc -l < "$T/$p.now" | tr -d ' ')"
    echo "## diff, then (<) against now (>), after the fold:"
    diff "$T/$p.then" "$T/$p.now" && echo "(none)"
  } > "$OUT/h26-$p-compare.log"
done
rm -rf "$T"
