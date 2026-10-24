# H16: the reviewer's evidence for `500e00d`

Collected by Fable 5.1 (reviewer), Claude Code, on the operator's Mac, 2026-10-24, in a detached worktree at `500e00df48b1f56426682fd715682733bc8a94a7`, with `OUT=<this directory> bash collect.sh`. Alerts and sleep holds went to scratch files: one alert, the APPROVE in probe P3, went there and none to the screen. Verdict: `handoff` reply to request #83.

| File | What it shows |
|---|---|
| `build-vet-deps.log` | `go build`, `go vet` and `gofmt` clean; no `net` package; no dependencies; six commits on the base; the evidence commit adds only `evidence/h16/` on top of `dd07744`; nothing under `internal/record` changes |
| `h3-record.log` | The record's torn-tail and `head.json` tests under `-race`, the reviewer's standing H3 check |
| `regressions.log` | This commit's seven new or changed tests, run on main (`ba519c1`): six fail, one passes; here all seven pass |
| `merge-dry-run.log` | `git merge-tree --write-tree` of main (`6908c2e`) and `500e00d`: clean, with objects written to a temporary directory and the repository's object count unchanged. The two "garbage found" warnings are `git count-objects` seeing the reviewer's own worktree directory, nothing in the object store |
| `probes.sh`, `probes.log` | P1: hooks run as settings from before H16 run them, with `--prefer`, at every kind of start, at `Stop` and at a notification, for Claude Code and Codex: none fails, none binds. P2: ten `roles add` at once all land; five of one name land once. P3: `roles add` during a reviewer's `wait`; the new role is addressable at once and can hold, claim and reply. P4: `role` racing `roles add`, twenty times: config never corrupt, no binding without a role. P5: `quiet_after`, `main` and every byte of the record kept. P6: seven refusals leave `config.json` unchanged |
| `go-test-race.log` | Every test under the race detector: 152 pass, none fail |
