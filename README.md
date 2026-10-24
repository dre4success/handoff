# handoff

Two AI coding agents on your Mac hand work to each other: one writes the code, another reviews it. You don't pass messages between them. You only decide.

- Each agent runs in its own Claude Code or Codex session.
- They send requests, reviews and replies through `handoff`, which keeps a record you can read.
- When a review is approved, a pop-up asks you to approve or deny.

Made for macOS.

## Install

```
go install github.com/dre4success/handoff/cmd/handoff@latest
```

You need Go 1.25.3 or later, and Go's bin folder (`~/go/bin`) on your `PATH`. Run it again to update. `handoff version` shows what you have.

## Quick start

1. In your project, run `handoff init`. It makes a `.handoff/` folder, which git ignores, with three roles: `implementer`, `reviewer` and `reviewer-2`. Every worktree of the project shares it.
2. Run `handoff settings claude`, `handoff settings codex`, or both. Each prints settings and says where they go. Then restart those sessions.
3. Start a session for each agent, and tell each one its job: "you're the implementer", "you're the reviewer".
4. Give the implementer a task. It asks the reviewer for a review when it's ready. You decide at the end.

Two Claude Code sessions can work together, each on its own model:

```
claude --model opus     # tell it: you're the implementer
claude --model fable    # tell it: you're the reviewer
```

## What a session does

Once set up, each session:

- gets a short brief: its role, the steps and the rules (`handoff brief` shows it);
- waits for work by itself, and wakes when something arrives;
- runs `handoff` without asking, but never decides for you and never edits `.handoff/`.

A Claude Code session also tells you when it's stuck at a permission prompt.

## Deciding

- A pop-up asks: **Approve**, **Deny** or **Later**.
- Or run `handoff` in the project. It lists what waits for you and asks about each.
- You also get a notification when something waits for you, and when an agent has owed a move for 30 minutes. Change the 30 minutes with `quiet_after` in `.handoff/config.json`.

## Roles

- A session has no role until you give it one. Tell it, and it runs `handoff role <role>`.
- It keeps the role through a resume and Claude Code's `/clear`. A new Codex thread needs telling again.
- Giving a role to a second session takes it from the first.
- Choose other roles with `handoff init --roles a,b`. Every role reviews except one named `implementer`; `--reviewers` picks them yourself.
- Add roles later with `handoff roles add reviewer-3`. Roles can't be renamed or removed.
- To give every session of one CLI the same role: `handoff settings claude --as reviewer`.

## Codex

- `handoff settings codex` prints two files for the project's `.codex/` folder. Keep that folder out of git: add `.codex/` to `.git/info/exclude`.
- Codex uses them only in a project it trusts. The next time it starts, it asks you to review the new hooks: choose **Trust all**.
- `handoff` runs outside Codex's sandbox, so its notifications reach you.

## While you're away

While an agent owes a move, a listening session keeps your Mac awake, for up to two hours. Closing the lid still puts it to sleep. If the Mac sleeps, nothing is lost: work is picked up when it wakes.

## Commands

| Command | What it does |
|---|---|
| `handoff` | What waits for your decision. At a terminal, asks about each |
| `init` | Sets up a project |
| `settings claude\|codex` | Prints the settings that keep sessions working |
| `role <role>` | Gives a session its role (the session runs it) |
| `roles add <role>…` | Adds roles |
| `post` | Asks for a review of one commit |
| `claim <id>` | The reviewer says it's on it |
| `reply <id>` | APPROVE, CHANGES or QUESTION |
| `note` | Anything else, such as an answer to a question |
| `decide <item>` | You approve or deny, at a terminal |
| `inbox`, `wait`, `show <id>`, `brief` | Read what's waiting, wait for it, show one entry, show a session's brief |
| `repair` | Fixes the record after a crash, keeping every byte |
| `version` | The version installed |

`handoff help` lists every flag.

## By hand

Sessions set up with `settings` do this themselves:

```
# the implementer
handoff post --as implementer --item W1 --to reviewer --commit <sha> --title "…" --body req.md

# the reviewer
handoff wait --for reviewer
handoff claim --as reviewer 1
handoff reply --as reviewer 1 --verdict APPROVE --commit <sha> --body verdict.md

# you
handoff decide W1 --approve
```

## More

- The spec: `docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md`
- How it was built, step by step: `docs/CHANNEL.md`
- Each role's instructions: `docs/prompts/`

This repository starts from one commit. Commit ids in `docs/` and `evidence/` point to the private history it was built in.

## License

MIT. See `LICENSE`.
