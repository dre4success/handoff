# H7 trial: two sessions, no relay (checks 10 and 11)

Ran 2026-10-12 to 14 on the operator's Mac, in this repository, with `bin/handoff` built from `main` at `7f92b9a`.
- **Implementer:** Opus 5.5, in Claude Code, background `wait`.
- **Reviewer:** GPT-6, in Codex, foreground `wait`.
- **Operator:** the person who decides.

The plan is `docs/TRIAL_PLAN.md`.

## Result

- **Check 10 passes (Run).** Each session's `wait` picked up what was posted to it within about a second, in both sessions.
- **Check 11 passes (Run), with one interruption.** All eight steps went through `handoff`: request, claim, QUESTION, note, superseding request, claim, APPROVE, the operator's decision. Codex then stalled for about 46 hours at a permission prompt. The operator restarted it with "wake up". No review message was relayed.
- The record's hash chain verified on a copy of the original record, using Fable's independent check (`checks.txt`). The copy published here is redacted and no longer verifies; see the last section.
- H7 merged into `main` at `d15b100`, the approved commit, as a fast-forward. Not pushed.

## What happened

| # | When (UTC) | What | Woken |
|---|---|---|---|
| 1 | 10-12 20:56:57 | Implementer: request, README at `4e4ffad` | Codex's first `wait` returned it at once (posted before it started) |
| 2 | 21:21:47 | Reviewer: claim | |
| 3 | 21:37:53 | Reviewer: QUESTION, three points | Implementer's background `wait`: 21:37:54 |
| 4 | 21:38:41 | Implementer: note answering #3 | Reviewer's `wait`: 21:38:41.8 |
| 5 | 21:38:49 | Implementer: revision `d15b100`, supersedes #1 | Reviewer's `wait`: 21:38:49.8 |
| 6 | 21:38:56 | Reviewer: claim | |
| | 21:38 to 10-14 | Codex stopped at its sandbox prompt for `git worktree add`. The operator sent "wake up" on 10-14 | |
| 7 | 10-14 20:14:47 | Reviewer: APPROVE on `d15b100` | Implementer's background `wait`: 20:14:48 |
| 8 | 20:41:17 | The operator: `decide`, approve, "Approved. Merge H7." | |

**Relaying.** Before Codex had started, the operator pasted one setup message from it ("I haven't started the trial") into the implementer's session, and pasted the reviewer prompt into Codex. No message about the work itself was carried. The H7 entry in `docs/CHANNEL.md` records this.

## Findings, for step 3

1. **A stall nobody saw.** Codex sat at a permission prompt for two days after claiming #5. Nothing told the operator. Needed: a notice when a claim goes quiet, and the permissions a review needs pre-approved for each CLI.
2. **Sleep pauses everything.** The Mac slept often. `wait` reported each gap and lost nothing. One deadline fell during sleep and returned on wake (wait 3). That's partial evidence for check 9, which stays open. The gap warning also fires on 18 to 19 second wake-ups during sleep, which is noise.
3. **Restarting the wait by hand.** The implementer restarted its 25-minute `wait` 44 times. Each restart was a turn, and the turns buried the operator's messages. Needed: a hook that restarts it.
4. **A plan bug.** "`--as` on every command" fails for `show`. Corrected in `docs/TRIAL_PLAN.md`.
5. **`decide` is too heavy for the operator.** It needs `cd`, a body file, `--as operator` and an entry number. Needed: one action with nothing to remember.
6. **Leftover.** Codex's review worktree is still at `/private/tmp/handoff-review-H7-4e4ffad`. Removing it needs its approval or the operator's.

## Files

| File | What |
|---|---|
| `collect.sh` | Copies the record and writes `checks.txt` |
| `checks.txt` | The chain check, every entry, and the record read through the tool, as run on the original record on 2026-10-14 |
| `record/` | Copies of `channel.jsonl`, `head.json` and `config.json` after #8, redacted |
| `implementer-wait.log` | Every implementer `wait`: start, end, exit code, sleep gaps |
| `h7-r1.md`, `h7-note-q3.md`, `h7-r2.md` | The implementer's request and note bodies |
| `reviewer/` | Codex's own evidence, as it supplied it |

## The record here is redacted

Before this repository was published, the operator's name was removed from every file (H14, §17 rows 77 and 78). In `record/channel.jsonl` the name was in two entries, lines 1 and 7, so their bytes changed. Each entry holds the hash of the line before it, so lines 2 and 8 still hold the hashes of the original lines 1 and 7. The copy is a transcript now, not a record that verifies:

- Fable's check, run on this copy, finds a mismatch at lines 2 and 8. The other six links, and `head.json`, still match.
- The tool reads the copy as broken at line 2 and withholds every entry.
- `checks.txt` is the check as run on the original record on 2026-10-14, when the chain verified. The original isn't published, so that result can't be reproduced from this repository.

Nothing else in the record changed. `evidence/h14/trial-record.log` shows all of this.
