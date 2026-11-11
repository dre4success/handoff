# H29 round 2: the reviewer's evidence for `b8e1c7e`

Collected by Fable 5.2 (reviewer), Claude Code, on the operator's Mac, 2026-11-11, in a detached worktree at `b8e1c7e6f4fc2811bc1900ef62e8fbacf0e89aa5`, with the scripts here, which are round 1's (`evidence/reviewer/H29-0ae44e4/`) with the commit changed and `still-public.sh` widened. HOME and both CLIs' settings folders were scratch folders; alerts and sleep holds went to scratch files; none were sent. Verdict: APPROVE, `handoff` reply to request #145.

| File | What it shows |
|---|---|
| `still-public.log` | From `still-public.sh`. Of the 16 lines H29 removes from CLAUDE.md or rewrites, none is in any file under `evidence/h29/`, in any tracked file at this commit, or in the reviewer's round 1 folder, as a whole line or part of one. Round 1 found all 16 in `evidence/h29/docs.log`. Then, for the operator, the same counts by folder as round 1 of where the tree still says what CLAUDE.md drops. Counts only; the patterns are kept outside the repository |
| `probes.log` | From `probes.sh`, unchanged: the README at this commit, line by line, each claim quoted and then run (G1 to G7, as in round 1). Against round 1's log, times and hashes aside, one line differs: line 70 of the README, which now calls `docs/prompts/` the prompts handoff was built with |
| `go-test-race.log` | Every test under the race detector, no `-timeout`: 171 pass, 0 fail, 2 skipped, no data race. The same 173 test names at the base and here. The notify tests five times each: all pass |
| `build-vet-deps.log` | `go build`, `go vet`, `gofmt` clean; no dependencies; no `net`; the commits; the commit under review adds only `evidence/h29/` to `902e36e`. Nothing under `cmd` or `internal` differs from round 1, and `handoff` built at the base and here is the same file, byte for byte |
| `merge-and-sweep.log` | From `sweep.sh`. Read-only `git merge-tree --write-tree` of main (`9acf443`, three commits past the base: two board lines and the reviewer's round 1 evidence) and this commit: clean. The name sweep: the other projects' names go from 3 lines on main to 0, nothing else moves, and no added line matches |
