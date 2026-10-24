#!/bin/sh
# Collects the H7 trial evidence. Run from the repository root after the trial.
# Copies the live record (never edits it), then checks the copy's hash chain
# with Fable's independent check from evidence/gate/H5-089a64f/probes.sh (block B).
set -e
out=evidence/trial
cp .handoff/channel.jsonl .handoff/head.json .handoff/config.json "$out/record/"
{
echo "# collected $(date -u +%FT%TZ) by Opus 5.5 (implementer), main at $(git rev-parse HEAD)"
echo
echo "## chain check on the copy (Fable's block B, unchanged)"
python3 -I - "$out/record/channel.jsonl" "$out/record/head.json" <<'PY'
import sys, json, hashlib
lines = open(sys.argv[1],'rb').read().split(b'\n')[:-1]; prev='genesis'; ok=True
for i,l in enumerate(lines,1):
    e=json.loads(l)
    if e['prev']!=prev: print(f"line {i}: prev MISMATCH"); ok=False
    prev=hashlib.sha256(l).hexdigest()
h=json.load(open(sys.argv[2]))
print("chain verifies:",ok,"| head.lines==lines:",h['lines']==len(lines),"| head.last==sha256(last line):",h['last']==prev)
PY
echo
echo "## entries"
python3 -I - "$out/record/channel.jsonl" <<'PY'
import sys, json
for l in open(sys.argv[1],'rb').read().split(b'\n')[:-1]:
    e=json.loads(l)
    extra=" ".join(f"{k}={e[k]}" for k in ("re","supersedes","verdict","decision") if e.get(k))
    print(f"#{e['id']} {e['ts']} {e['kind']:9} {e['from']:>11} -> {e.get('to') or '-':<11} {(e.get('commit') or '')[:12]:12} {extra}")
PY
echo
echo "## the record through the tool"
./bin/handoff inbox --for operator
./bin/handoff show 5 | sed -n '1,9p'
} > "$out/checks.txt" 2>&1
cat "$out/checks.txt"
