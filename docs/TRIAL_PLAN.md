# Trial plan: two sessions, no relay

For the operator. Run it with the operator present. H6 is merged, so it's ready.

**Ran 2026-10-12 to 14. Results: `evidence/trial/README.md`.** The `--as` wording below is corrected: the trial found that `show` rejects it.

## What we're testing

Can the implementer and the reviewer hand one small, real item back and forth through `handoff`, with you relaying nothing? That closes checks 10 and 11. If it works, this channel moves onto the tool, and `docs/CHANNEL.md` becomes history.

## Setup

Once, in a terminal at the repo root, on `main`:

```
go build -o bin/handoff ./cmd/handoff
./bin/handoff init --roles implementer,reviewer,reviewer-2 --reviewers reviewer,reviewer-2 --main main
```

Git ignores both `bin/` and `.handoff/`.

The sessions are already running, so they can't pick up a `HANDOFF_ROLE` variable. Every command passes `--as <role>` instead, except `show`, which takes none.

## The item: H7, the README

Update the README: where the project stands, and how to build and run `handoff`. Small, real, docs only. Branch: `h7-readme`.

To cover the whole path, the reviewer's first verdict is a QUESTION, on purpose. Then:

1. The implementer answers it with a note.
2. The implementer posts a revised commit that replaces the first request.
3. The reviewer gives APPROVE or CHANGES, as earned.
4. You close it with `decide` in your terminal.

## Paste into Claude Code (implementer)

> Trial for checks 10 and 11. Use `H=~/handoff/bin/handoff` and `--as implementer` on every command except `show`. Never edit `.handoff/` by hand. Never merge.
> 1. Start `$H wait --for implementer --timeout 25m` **in the background**. Each time it returns, read what it printed (`$H show <id>`, `$H inbox --as implementer`), act, and start it again. Exit 3 is a timeout: start it again. Exit 2 means the record is damaged: stop and tell the operator.
> 2. Make H7 on `h7-readme`, in a separate worktree. The shared checkout stays on `main`. Commit, then post: `$H post --as implementer --item H7 --to reviewer --commit <full sha> --on h7-readme --title "H7: README after step 1" --body <file> --key h7-r1`.
> 3. Answer a QUESTION with `$H note --as implementer --re <the reply's id> --to reviewer --body <file>`. Revise with a new commit and `$H post … --supersedes <id> --key h7-r2`.
> 4. Stop when the reviewer's APPROVE arrives, and say so.

## Paste into Codex (reviewer)

> Trial for checks 10 and 11. Use `H=~/handoff/bin/handoff` and `--as reviewer` on every command except `show`. Never edit `.handoff/` by hand. Never switch the shared checkout.
> 1. Run `$H wait --for reviewer --timeout 25m` in the foreground of your open turn, collecting it with short yielding waits. Exit 3 is a timeout: run it again. Exit 2 means the record is damaged: stop and tell the operator. When it returns with an entry, read the request with `$H show <id>`, then `$H claim --as reviewer <id>`.
> 2. Review exactly the request's commit, in a temporary worktree. Reply with `$H reply --as reviewer <id> --verdict APPROVE|CHANGES|QUESTION --commit <that sha> --body <file>`. For this trial, your first verdict on the first request is a QUESTION.
> 3. Wait again after each reply. Stop after an APPROVE.

## While it runs

You relay nothing. To watch, from the repo root:

- `./bin/handoff inbox --for operator`: what's waiting, and what needs you.
- `./bin/handoff show <id>`: one entry in full.

To close H7 after the APPROVE, in your terminal:
`./bin/handoff decide <id> --approve --body <file> --as operator`. It asks you to type the id again.

If a session stalls, note the time and what it showed. You can re-paste its prompt. We'll record that it happened.

## Afterwards: evidence in `evidence/trial/`

- A copy of `.handoff/channel.jsonl` and `head.json`, plus an independent check of the hash chain. Fable's script in `evidence/gate/H5-089a64f/probes.sh` works on the copy.
- From each session: when `wait` started and returned, and what it printed.
- From you: one line saying you carried no message, and the `decide` that closed H7.

Then I post one summary entry to `docs/CHANNEL.md`, and new items go through `handoff`.

## Known gaps

- **No hooks yet** (step 3). If a session's context gets summarised, it may forget to wait again. Re-paste its prompt. That's a step-3 finding, not a failure of the trial.
- **Typing to Codex while it waits** interrupts the wait.
- **Only you can run `decide`.** It needs a real terminal.
