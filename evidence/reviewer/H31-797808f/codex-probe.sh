#!/usr/bin/env bash
# codex-probe.sh <scratch dir>: a throwaway Codex session (codex exec) started in <scratch dir>/P, which is
# no git repository; given reviewer-2 from <scratch dir>/P/proj; then resumed from P/proj (codex exec resume
# <id>). <scratch dir>/shim is first on PATH, so Codex's hooks, which run `handoff` by name, run the shim,
# which runs this build. This session's own CLAUDE_ variables are taken out, so Codex's hooks don't inherit
# them. With a role, the resumed run's Stop hook listens, so the run is stopped once that hook has started.
set -u
D=$1; P=$D/P; BIN=$D/../bin31r2/handoff
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CLAUDE_PROJECT_DIR CLAUDECODE
export PATH=$D/shim:$PATH HANDOFF_TEST_NOTIFY=$D/alerts.txt HANDOFF_TEST_AWAKE=$D/awake.txt
scrub() { sed -e "s|$P/proj|<P/proj>|g" -e "s|$P|<P>|g" -e "s|$D|<scratch>|g" -e "s|$HOME|~|g"; }
turns() { python3 -I - "$1" "$P" <<'PY'
import glob, json, os, sys
sid, P = sys.argv[1], sys.argv[2]
def sc(s): return str(s).replace(P + "/proj", "<P/proj>").replace(P, "<P>")
for p in sorted(glob.glob(os.path.expanduser("~/.codex/sessions/*/*/*/rollout-*-" + sid + ".jsonl"))):
    n = 0; t = []
    for l in open(p, errors="replace"):
        n += 1
        try: e = json.loads(l)
        except ValueError: continue
        if e.get("type") == "turn_context": t.append("line %d: %s" % (n, sc((e.get("payload") or {}).get("cwd"))))
    print("   the rollout: %d lines; its turn_context lines: %s" % (n, "; ".join(t)))
PY
}
warns() { (cd "$P/proj" && CODEX_THREAD_ID="$sid" "$BIN" role "$@" 2>&1 | grep '^handoff: warning' | scrub | cut -c1-400); }
n0=$(wc -l < "$D/log/runs.jsonl")
echo "## 1. codex exec, started in <P>"
out=$(cd "$P" && codex exec --skip-git-repo-check -c model_reasoning_effort='"low"' "Reply with the word ALPHA only." 2>&1 < /dev/null)
sid=$(printf '%s\n' "$out" | sed -n 's/^session id: //p' | head -1)
printf '%s\n' "$out" | grep -E '^(workdir|session id):' | scrub; turns "$sid"
echo "## 2. this build's handoff role reviewer-2, as that session, from <P/proj>:"; w=$(warns reviewer-2); echo "   ${w:-no warning}"
echo "## 3. codex exec resume <id>, from <P/proj>, until its Stop hook has started"
(cd "$P/proj" && exec codex exec resume "$sid" -c model_reasoning_effort='"low"' "Reply with the word BRAVO only." > "$D/codex-resume.out" 2>&1 < /dev/null) &
job=$!
for i in $(seq 1 90); do sleep 1; python3 -I -c '
import json, sys
n0 = int(sys.argv[2]); sid = sys.argv[3]
for l in open(sys.argv[1]).read().splitlines()[n0:]:
    try: r = json.loads(l)
    except ValueError: continue
    i = r.get("in") or {}
    if i.get("session_id") == sid and i.get("hook_event_name") == "Stop" and i.get("cwd", "").endswith("/proj"): sys.exit(0)
sys.exit(1)' "$D/log/runs.jsonl" "$n0" "$sid" && break; done
sleep 2; pkill -f "codex exec resume $sid"; wait $job 2>/dev/null
grep -E '^(workdir):' "$D/codex-resume.out" | scrub; turns "$sid"
echo "## 4. the runs of handoff by Codex's hooks: the hook, the folder it ran in, and what a brief said"
python3 -I - "$D" "$n0" "$sid" <<'PY' | scrub
import json, sys
D, n0, sid = sys.argv[1], int(sys.argv[2]), sys.argv[3]
for l in open(D + "/log/runs.jsonl").read().splitlines()[n0:]:
    try: r = json.loads(l)
    except ValueError: continue
    i = r.get("in") or {}
    if i.get("session_id") != sid: continue
    out = ""
    if r["args"].startswith("hook brief"):
        try:
            t = open(r["f"] + ".out").read(); w = [x for x in t.splitlines() if x.startswith("handoff: warning")]
            out = "\n      the brief: %s" % ("WARNED: " + w[0][:330] if w else "no warning; first line: " + (t.strip().splitlines() or ["(nothing)"])[0][:80])
        except OSError: pass
    print("%s handoff %-12s ran in %s  %s(%s) stdin cwd=%s%s" % (r["t"], r["args"], r["pwd"], i.get("hook_event_name"), i.get("source"), i.get("cwd"), out))
PY
echo "## 5. this build's handoff role, as that session, from <P/proj>, after the resumed turn:"; w=$(warns); echo "   ${w:-no warning}"
echo "$sid" >> "$D/codex-session.txt"
