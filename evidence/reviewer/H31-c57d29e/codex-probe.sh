#!/usr/bin/env bash
# codex-probe.sh <scratch dir>: a throwaway Codex session (codex exec) started in <scratch dir>/P,
# which is no git repository, then resumed from <scratch dir>/P/proj (codex exec resume <id>).
# <scratch dir>/shim is first on PATH, so Codex's hooks, which run `handoff` by name, run the shim.
# Then this build's `handoff role`, which only reads, is run as that session from P/proj.
set -u
D=$1; P=$D/P; BIN=$D/../bin31/handoff
export PATH=$D/shim:$PATH HANDOFF_TEST_NOTIFY=$D/alerts.txt HANDOFF_TEST_AWAKE=$D/awake.txt
scrub() { sed -e "s|$P/proj|<P/proj>|g" -e "s|$P|<P>|g" -e "s|$D|<scratch>|g" -e "s|$HOME|~|g"; }
rollouts() { # the rollouts of the session: how many, and what each says about folders (no text)
  python3 -I - "$1" "$P" <<'PY'
import glob, json, os, sys
sid, P = sys.argv[1], sys.argv[2]
def sc(s): return str(s).replace(P + "/proj", "<P/proj>").replace(P, "<P>")
fs = sorted(glob.glob(os.path.expanduser("~/.codex/sessions/*/*/*/rollout-*-" + sid + ".jsonl")))
print("rollouts named for the session: %d" % len(fs))
for p in fs:
    n = 0; metas = []; turns = []
    for l in open(p, errors="replace"):
        n += 1
        try: e = json.loads(l)
        except ValueError: continue
        pl = e.get("payload") or {}
        if e.get("type") == "session_meta": metas.append((n, pl.get("cwd")))
        if e.get("type") == "turn_context" and (not turns or turns[-1] != pl.get("cwd")): turns.append(pl.get("cwd"))
    print("  %d lines; session_meta on lines %s; its turns' folders, in order: %s" % (n, ", ".join("%d (cwd %s)" % (a, sc(b)) for a, b in metas), " > ".join(sc(t) for t in turns)))
PY
}
n0=$(wc -l < "$D/log/runs.jsonl")
echo "## 1. codex exec, started in <P>"
out=$(cd "$P" && codex exec --skip-git-repo-check -c model_reasoning_effort='"low"' "Reply with the word ALPHA only." 2>&1 < /dev/null)
sid=$(printf '%s\n' "$out" | sed -n 's/^session id: //p' | head -1)
printf '%s\n' "$out" | grep -E '^(workdir|session id|sandbox):|^ALPHA' | scrub
rollouts "$sid"
echo "## 2. codex exec resume <id>, from <P/proj>"
(cd "$P/proj" && codex exec resume "$sid" -c model_reasoning_effort='"low"' "Reply with the word BRAVO only." 2>&1 < /dev/null) | grep -E '^(workdir|session id|sandbox):|^BRAVO|rror' | scrub
rollouts "$sid"
echo "## 3. this build's handoff role, as that session, from <P/proj> (it has no role, so this only reads), then handoff role reviewer-2"
(cd "$P/proj" && env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID CODEX_THREAD_ID="$sid" "$BIN" role 2>&1; env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_PID CODEX_THREAD_ID="$sid" "$BIN" role reviewer-2 2>&1 | head -4) | scrub
echo "## 4. the runs of handoff by Codex's hooks: the hook, the folder it ran in, the stdin's cwd"
python3 -I - "$D" "$n0" <<'PY' | scrub
import json, sys, os
D, n0 = sys.argv[1], int(sys.argv[2])
for l in open(D + "/log/runs.jsonl").read().splitlines()[n0:]:
    r = json.loads(l); i = r.get("in") or {}
    out = ""
    if r["args"].startswith("hook brief"):
        try: out = " | printed: " + (open(r["f"] + ".out").read().strip().splitlines() or ["(nothing)"])[0][:90]
        except OSError: pass
    print("%s handoff %-12s ran in %s  %s(%s) stdin cwd=%s transcript_path given: %s%s" % (r["t"], r["args"], r["pwd"], i.get("hook_event_name"), i.get("source"), i.get("cwd"), bool(i.get("transcript_path")), out))
PY
echo "$sid" >> "$D/codex-session.txt"
