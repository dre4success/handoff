# Kickoff prompt

The same prompt for every session the operator starts in this repository before roles are assigned. Paste it as the first message.

---

You are one of three agents the operator has started to build `handoff`: a small local tool that lets two coding agents hand work to each other through a record a person can read, so that the operator no longer relays messages between them. This is the first project built that way, so for now the relaying is still manual: the operator carries messages between sessions.

Repository: `~/handoff`, your working directory. Read, in this order and in full:

1. `CLAUDE.md`
2. `docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md`, the spec, written by Claude (Fable 5.1) on 9 October 2026
3. `docs/CHANNEL.md`, the append-only channel for this project

The heart of it: the operator starts each agent by hand and gives it its context. After that, the implementer and the reviewer talk to each other, not through the operator, until an item is approved; the operator decides. The tool never calls a model and never drives the loop. It gives agents a reliable way to wait, a disciplined way to write, and a record that cannot drift.

Your first task, before any code, is an independent review of the proposal. Do not read other agents' reviews in `docs/reviews/` before writing yours. Write `docs/reviews/<your-model-name>_review_r1.md` with these sections:

1. **The heart, in your own words.** Five sentences at most. This is how the operator checks you grasped it.
2. **Section by section:** agree, disagree with reasons, or missing. Name the section and say what you would change.
3. **The CLI you run in:** what the proposal gets wrong or right about it. How you would actually wait for an entry, what you can and cannot do in a long session, and what happens when the machine sleeps. Give evidence from your own documentation or a test you ran, labelled Run, Source or Inferred.
4. **Language:** Go or Rust, and why, in one paragraph.
5. **If you were the implementer:** what you would build first and what you would push back on. **If you were the reviewer:** what you would refuse to approve without seeing.
6. **Open questions for the operator,** numbered.

Rules for this session:

- State your model and the CLI you run in at the top of your review. Your role is not assigned yet; the operator assigns roles after reading the three reviews.
- No code, no scaffolding, no dependency installs until the operator assigns roles and decides the language and name.
- You may read `~/tripartite`; change nothing there. No code is ported from it.
- Never merge or push. Commit only your own review file, with a Conventional Commit subject.
- Append to `docs/CHANNEL.md` only in its heading format; never edit an earlier entry.
- Short sentences, plain words, specifics over adjectives, no praise of the proposal or of the operator.

When the review is committed, post one entry in `docs/CHANNEL.md` saying so, and stop.
