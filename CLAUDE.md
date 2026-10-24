# CLAUDE.md

`handoff` is a small local CLI that lets two coding agents on one machine, one implementing and one reviewing, hand work to each other through an append-only record the operator can read. The operator starts and primes each agent session; the agents then talk through this tool; the operator is called only to decide.

## Source of truth

`docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md` is the spec. Read it in full before any work. Codex's review is appended at its end; respond to review points by appending to the resolution log (§17), never by editing earlier text.

## Boundaries that never move (proposal §2, §4, §9)

- The tool never calls a model, never starts, stops or messages an agent, and never drives the work loop. The agents drive themselves.
- No network. Git is read-only. Writes go only under `.handoff/` of the target project.
- A verdict is accepted only from the reviewer role, for the exact commit the request named. A claim is never an approval. Nothing the tool does can manufacture an approval.
- Every commit in an entry is resolved and checked before it is written. No placeholders.
- `CHANNEL.md` and `board.md` are generated from `channel.jsonl`. Never hand-edit them.

## Decisions (the operator, 2026-10-09; channel H4)

- **Roles:** Opus 5.5 implements; GPT-6 reviews; Fable 5.1 reads at milestones only; the operator decides. Decided after three rounds (`docs/reviews/`, channel H2 to H4, and a blind referee reading).
- **Language:** Go, zero third-party dependencies. **Name:** `handoff`.
- **§15 decisions D1 to D13:** the recommendations in the proposal stand as working decisions, as recorded in the H4 entry; the operator may overturn any one by a channel entry before it is built on.
- The duty prompts are in `docs/prompts/`. The implementer owns the status board (D10).

## Lineage

`tripartite`, an earlier experiment, is the predecessor and the opposite approach: an orchestrator that drove agents headless from a state machine. **No code is ported from it** (proposal §11). Do not copy its packages, types or state machine; it is a record of what not to rebuild. Ideas only: the operator's narrow role, artifacts on disk, and a later brainstorm command outside the loop.

## Verification

Proposal §13 lists the checks. A check passes only with Run evidence, saved under `evidence/` in this repository with the command, date and who ran it. Source or Inferred reasoning does not close a check.

## Conventions

- Conventional Commit subjects (`feat:`, `fix:`, `docs:`, `test:`), one concern per commit.
- No merge or push without the operator.
- The trial on the website project (`~/website`) happens after its release 1 and only with the operator present.
