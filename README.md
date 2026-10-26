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

Your project must be a git repository, since every review is of one commit. If it isn't one yet, run `git init` first.

## Quick start

Once, on your Mac: run `handoff settings claude --global`, `handoff settings codex --global`, or both. Each prints settings for your home folder and says where they go, or a session can add them for you. They set up every project at once, and do nothing where `handoff` isn't used.

Then, in each project:

1. Run `handoff init`. It makes a `.handoff/` folder, which git ignores, with three roles: `implementer`, `reviewer` and `reviewer-2`. Every worktree of the project shares it. It also says whether each CLI is set up.
2. Start a session for each agent, and tell each one its job: "you're the implementer", "you're the reviewer".
3. Give the implementer a task. It asks the reviewer for a review when it's ready. You decide at the end.

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

A session that isn't set up offers to set itself up. With your OK, it adds what `handoff settings --global` prints; then start it again.

## Deciding

- A pop-up asks: **Approve**, **Deny** or **Later**.
- Or run `handoff` in the project. It lists what waits for you and asks about each.
- You also get a notification when something waits for you, and when an agent has owed a move for 30 minutes. Change the 30 minutes with `quiet_after` in `.handoff/config.json`.

## Roles

- A session has no role until you give it one. Tell it, and it runs `handoff role <role>`.
- It keeps the role through a resume and Claude Code's `/clear`. A new Codex thread needs telling again.
- Giving a role to a second session takes it from the first.
- `reviewer-2` is for a second reviewer. With one reviewer, leave it unused: the implementer asks only reviewers that have a session, and tells you if what it sent waits for a role with none.
- The implementer commits its work on a branch of its own and asks for a review without asking you first. It never merges or pushes without your decision.
- If you ask a reviewer to look at work nobody has posted, it sends what it finds to the implementer as a note. Only a commit can get a verdict.
- Choose other roles with `handoff init --roles a,b`. Every role reviews except one named `implementer`; `--reviewers` picks them yourself.
- Add roles later with `handoff roles add reviewer-3`. Roles can't be renamed or removed.
- To give every session of one CLI the same role in one project: `handoff settings claude --as reviewer`, which sets up that project alone.

## Codex

- `handoff settings codex --global` prints two files for `~/.codex/`. The next time Codex starts, it asks you to review the new hooks: choose **Trust all**.
- Without hooks, a Codex session doesn't wait for work, since waiting there costs a whole model call every minute or so. It ends its turn, and you tell it when there's work.
- `handoff` runs outside Codex's sandbox, so its notifications reach you.
- To set up one project alone, leave out `--global`: the files go in its `.codex/` folder, which you keep out of git (add `.codex/` to `.git/info/exclude`), and Codex uses them only in a project it trusts.

## While you're away

While an agent owes a move, a listening session keeps your Mac awake, for up to two hours. Closing the lid still puts it to sleep. If the Mac sleeps, nothing is lost: work is picked up when it wakes.

## Commands

| Command | What it does |
|---|---|
| `handoff` | What waits for your decision. At a terminal, asks about each |
| `init` | Sets up a project |
| `settings claude\|codex` | Prints the settings that keep sessions working: `--global` for every project |
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
