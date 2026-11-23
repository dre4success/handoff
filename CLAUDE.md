# CLAUDE.md

`handoff` is a small local CLI. Two coding agents on one machine, one implementing and one reviewing, hand work to each other through an append-only record. The operator decides.

## Source of truth

`docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md` is the spec. Read it before any work. Respond to review points by appending to its resolution log (§17), never by editing earlier text.

## Boundaries that never move

- The tool never calls a model, never starts or stops an agent, and never drives the work loop. It delivers and wakes; it never decides, assigns or approves (spec §17 row 105).
- No network. Git is read-only. Writes go only under `.handoff/` of the target project.
- A verdict is accepted only from a reviewer, for the exact commit the request named. A claim is never an approval. Nothing the tool does can manufacture an approval.
- Every commit in an entry is resolved and checked before it is written.
- `CHANNEL.md` and `board.md` are generated from `channel.jsonl`. Never hand-edit them.

## Conventions

- Go, with no third-party dependencies.
- A check passes only with Run evidence under `evidence/`: the command, the date and who ran it.
- Conventional Commit subjects (`feat:`, `fix:`, `docs:`, `test:`), one concern per commit.
- No merge or push without the operator.
