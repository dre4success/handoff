# H12 evidence

Run by Opus 5.5 (implementer) on the operator's Mac. Each file starts with its command, date and commit. `bash evidence/h12/collect.sh` made the first six, at 6714cbf (round 3; after it come only row 75 and evidence).

- `round3.log` (new in round 3): GPT-6's `startup-race.py`, read from main and given the binary to run, on round 2's commit (589c1cd) and here. There, for both the hook and a plain wait, a listener whose role moved while it started took the new holder's note. Here, in 30 tries each, it never did.
- `round2.log` (new in round 2): this round's tests on round 1's commit (50a8c2d) fail there. A second Codex thread under the same parent, and a startup in the same Claude process, take the first session's role, and a plain wait keeps waiting after its role moves. Here they pass.
- `regressions.log`: the H12 tests copied onto main (4797d92) fail there: `handoff role` is an unknown command. In the agent-session test, main asks the operator's question in a Codex session and waits for an answer until the test kills it. Here they pass, with the settings tests as this branch changes them.
- `go-test-race.log`: every test under `-race`, one line per test; all pass.
- `demo.txt`: `settings claude` and `settings codex` with no `--as`. Two Claude Code sessions, stood in for by their variables, take the roles the operator gives them and work a round with no `--as`. One refuses the other's role, and a session with no role is told how to get one.
- `deps-vet-fmt.log`: vet, gofmt, no dependency outside the standard library, no `net` package, `git diff --check` from main clean.
- `live-two-claude.txt` (`live-two-claude.sh`, run again at e374bdf): two real throwaway Claude Code sessions (Haiku) in one scratch project, set up by `settings claude` with no `--as`. Each was told its role in its first prompt, ran `handoff role`, and listened by its hook. A request to reviewer woke only the reviewer's session, 7 s after it was posted; it reviewed and approved. The reply woke only the implementer's session, 5 s later, which read it and waited for the operator. No hook was left running. The sessions' transcripts were removed after.
- `probe-ids/`: the probes made before the code: which session ids each CLI gives commands and hooks, and that a resume keeps Claude Code's. `clear.txt` (round 2): in an interactive session, `/clear` gives a new id with the same `CLAUDE_PID` and `source: "clear"`.

No repeat runs: the lock and the wait are unchanged. Not tried live: a new Codex thread, a shared Codex server and two Codex sessions; the tests cover them with stand-in sessions.
