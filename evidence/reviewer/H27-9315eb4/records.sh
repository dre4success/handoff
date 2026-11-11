#!/usr/bin/env bash
# Is there a tell anywhere? Read-only. Counts entries of kind tell and fyi in
# every handoff record under the operator's home folder, and looks for a tag
# or a remote branch that holds the H26 merge, the one build that had tell.
# The records' paths are left out: they name the operator's other projects.
# Run as  R=<repository> bash records.sh > records.log
set -u
R=${R:?repository}
printf '# command: find every .handoff/channel.jsonl under the home folder (12 levels, Library, node_modules and the Trash left out) and count its entries of kind tell and fyi; git tag and git branch -r --contains 17b7920; the installed handoff\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the operator'"'"'s Mac\n# commit: %s (under review; nothing here runs it)\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "${C:-}"
n=0; entries=0; tell=0; fyi=0
while IFS= read -r f; do
  n=$((n+1)); entries=$((entries + $(wc -l < "$f"))); tell=$((tell + $(grep -c '"kind":"tell"' "$f"))); fyi=$((fyi + $(grep -c '"kind":"fyi"' "$f")))
done < <(find "$HOME" -maxdepth 12 -name channel.jsonl -path '*/.handoff/*' -not -path '*/Library/*' -not -path '*/node_modules/*' -not -path '*/.Trash/*' 2>/dev/null)
echo "## handoff records found: $n, with $entries entries in all"
echo "## entries of kind tell: $tell; of kind fyi: $fyi"
echo "## tags that contain the H26 merge (17b7920): [$(git -C "$R" tag --contains 17b7920 2>/dev/null | tr '\n' ' ')]; tags in the repository: $(git -C "$R" tag | wc -l | tr -d ' ')"
echo "## remote branches that contain it: [$(git -C "$R" branch -r --contains 17b7920 2>/dev/null | tr '\n' ' ')]; remotes: $(git -C "$R" remote | wc -l | tr -d ' ')"
echo "## the handoff installed now: $("$(command -v handoff)" version) (the H26 merge's build: tell still works there until the next install)"
