# handoff

Two AI coding agents on your Mac hand work to each other: one writes the code, the other reviews it. You only decide.

Each agent runs in its own Claude Code, Codex or Antigravity session. They talk through `handoff`, which keeps a record you can read. When a review is approved, a pop-up asks you to approve or deny. Made for macOS.

## Install

```
go install github.com/dre4success/handoff/cmd/handoff@latest
```

You need Go 1.25.3 or later, with `~/go/bin` on your `PATH`. Run it again to update. Your project must be a git repository.

After an update, a listener from the old build keeps running until something is delivered to its role. So before agents ask several reviewers in one post, make sure nothing from the old build still listens:

- send each role with a session a note, or let it finish a turn;
- stop any `handoff wait` you started yourself;
- remove any older `handoff` from your `PATH`.

Until then, `post` refuses to ask several reviewers at once, names the old listener and says to send its role a note.

## Quick start

1. Once: run `handoff settings claude --global`, `handoff settings codex --global` or `handoff settings antigravity --global`, for each CLI you use, and add what it prints where it says. A session can do this for you.
2. In the project, run `handoff init`. It adds a `.handoff/` folder, which git ignores, with three roles: `implementer`, `reviewer` and `reviewer-2`.
3. Start a session for each agent in the project's folder and tell it its role: "you're the implementer", "you're the reviewer". A session started outside the project, even in the folder above it, can miss work; it's warned when it takes a role.
4. Give the implementer a task. It asks for a review when it's ready. You decide at the end.

```
claude --model opus     # you're the implementer
claude --model fable    # you're the reviewer
```

## Deciding

Choose **Approve**, **Deny** or **Later** in the pop-up, or run `handoff` in the project. You also get a notification when something waits for you, when an agent has owed a move or left a note unread for 30 minutes (`quiet_after` in `.handoff/config.json`), and when a session is stuck at a permission prompt or can't be woken.

A quiet notification says why, when handoff knows: the session can't be woken, nothing is listening for it, or no session has the role. For a review that has been claimed, "nothing is listening" is also how a long review looks: the session is working, not waiting. Notifications come from handoff itself, from a command or a session's listener. With no session listening in the project, none comes, so run `handoff` to see the same lines.

## While you're away

Leave a go-ahead, in your own terminal in the project:

```
handoff away "Merge when the reviewers approve."
```

Every agent with a session is told. Until you run `handoff back`, an item is approved for you once every reviewer it went to approves the same commit. There's no pop-up, just a notification saying who approved it. That is the one rule: handoff doesn't read your words, and one approval is enough for an item sent to one reviewer. If they approve different commits, the pop-up asks you. `handoff back` lists what was approved.

To tell every agent something without a go-ahead, use `handoff fyi "Carry on."`. They act on it, and none replies through handoff.

So that a stuck session doesn't hold everything up while you're away, start one more session before you go and tell it "you're the spare" (`handoff role spare`; a session that has had another role here can't be the spare). `handoff away` then asks whether the spare covers: say yes. If a session gets stuck with work waiting, the spare is woken, takes its seat, and you get one notification. An Antigravity session at its wake limit, or one whose process ended, counts after 30 minutes; a reviewer at a permission prompt or silent counts after an hour; a silent implementer never counts, and you're told instead. The stuck session can do nothing as that seat afterwards; give it a role again if you want it back.

## Good to know

- A session waits for work by itself. It never decides for you or edits `.handoff/`. `handoff brief` shows what it's told.
- A session keeps its role through a resume, `/clear` and `/background`. The role goes with the conversation: if you resume the id it left, that one has no role. A copy made with `/fork`, or a new Codex thread, needs telling again. Giving a role to another session takes it from the first, which is told so and can then do nothing as any role until you give it one again.
- The implementer works on a branch of its own and never merges or pushes without your decision. Approved work stays in its inbox, and in `handoff`, until its commit is in main; if it waits too long, you're told. To stop an approved item from being merged after all, run `handoff decide <item> --deny`.
- `reviewer-2` is for a second reviewer; with one, leave it unused. For other roles, use `handoff init --roles a,b` or, later, `handoff roles add <role>`.
- With several reviewers, an agent asks them all in one post (`--to reviewer,reviewer-2`). That's one round: under your go-ahead, the item is approved only once each of them approves the same commit.
- An agent withdraws a request of its own that no longer needs an answer, such as one still open after its item is merged, or on work you dropped (`handoff withdraw <id>`). It then leaves inboxes and stops alerting you. If it withdraws one reviewer's request on an item that goes on, your go-ahead won't approve that item unless the reviewer is asked again: you decide it. So with a reviewer out of rotation, you decide each item it was asked on.
- Antigravity: to type to a session that's waiting for work, press Esc first. It listens with a wait it runs in the background, so it can wait for work all night. If it stops running one, handoff can still wake it 10 times in a row; after that, you're told, and it waits for a message from you. After reopening a session, type something in it, or it won't hear of new work.
- Codex asks you to review the new hooks: choose **Trust all**. `handoff` runs outside Codex's sandbox, so its notifications reach you. Without hooks, a Codex session doesn't wait for work, so you tell it when there is some.
- If a session can't take its work, such as Antigravity after its 10 wakes or one stopped at a permission prompt, handoff tells whoever sends it work, and tells you when work waits for it. `handoff` on its own shows each session: listening, blocked, or when it last listened.
- While an agent owes a move, your Mac stays awake for up to two hours. If it sleeps, nothing is lost.

## Commands

| Command | What it does |
|---|---|
| `handoff` | What waits for your decision |
| `init` | Sets up a project |
| `settings claude\|codex\|antigravity` | Prints the settings that keep sessions working |
| `role <role>` | Gives a session its role (the session runs it) |
| `roles add <role>…` | Adds roles |
| `post`, `claim`, `reply`, `note` | Ask for a review, take one on, give a verdict, say anything else |
| `withdraw <id>` | Withdraw a request that no longer needs an answer |
| `decide <item>` | Approve or deny, at a terminal; `--deny` also holds approved work not yet merged |
| `fyi "<message>"` | Tell every agent at once, at a terminal |
| `away "<go-ahead>"`, `back` | Approve in advance while you're away, and stop |
| `inbox`, `wait`, `show <id>`, `brief` | Read what's waiting |
| `repair` | Fixes the record after a crash |
| `version` | The version installed |

`handoff help` lists every flag.

## More

The spec is in `docs/proposals/`, how it was built in `docs/CHANNEL.md`, and the prompts it was built with in `docs/prompts/`. Commit ids in `docs/` and `evidence/` point to the private history it was built in.

MIT license; see `LICENSE`.
