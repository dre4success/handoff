# handoff

A small local tool that lets two coding agents on one machine, one implementing and one reviewing, hand work to each other through a record a person can read, without the person relaying messages.

The operator starts each agent in its own session and gives it the project and the rules. After that the agents talk to each other through `handoff`. The operator is called in only to decide.

Made for macOS, with Claude Code and Codex. Its notifications, pop-ups and keeping the Mac awake use macOS's own tools.

## Install

Go 1.25.3 or later, as in `go.mod`. No other dependencies.

```
go install github.com/dre4success/handoff/cmd/handoff@latest
```

Or, in a clone of this repository, `go install ./cmd/handoff`. Either puts `handoff` in Go's bin folder, `~/go/bin` unless you set `GOBIN`. Put that folder on your `PATH` in your shell profile, so every terminal and every agent session finds it. Run it again to update.

To build and test without installing:

```
go build -o bin/handoff ./cmd/handoff
go test ./...
```

## Set up a project

In the project's main worktree:

```
handoff init
```

This creates `.handoff/`, with two roles named after the agents, `claude` and `codex`. Either can implement or review; you say which, per task. The main branch is `main`, else `master`. Git ignores the record by itself (a `.gitignore` inside `.handoff/`), and nothing else in the project changes. Every worktree of the repository shares the one record.

Other roles: `handoff init --roles implementer,reviewer`. Every role reviews except one named `implementer`, unless you list the reviewers with `--reviewers`. Name the main branch with `--main`.

You get a Mac notification when something waits for your decision, or when a move has been owed for 30 minutes: a request nobody picked up, a review gone quiet, a question not answered. Change the 30 minutes with `quiet_after` in `.handoff/config.json`. If you miss one, run `handoff` in the project: it lists it under "gone quiet".

When something waits for your decision, a pop-up asks you directly: **Approve**, **Deny** or **Later**. A click is your decision. Later, or no click, leaves it waiting for `handoff`. Pop-ups come from a session outside Codex's sandbox, where macOS can't show them; in practice, Claude's `wait`.

## Give sessions their roles

Each agent session needs a role. The simplest way: start the sessions however you like, then tell each one its job in chat, such as "you review W1". It runs `handoff role reviewer`, gets its brief, and keeps that role until you give it another: through a resume, and in Claude Code through `/clear`. A new Codex thread starts with none, so tell it again. Giving a role to a second session takes it from the first.

So two sessions of one CLI can work together, each on its own model and effort. With only a Claude subscription, for example:

```
claude --model opus --effort high      # tell it: you implement
claude --model fable --effort xhigh    # tell it: you review
```

Roles are names in the record, not models: `claude` and `codex` by default, or the ones you gave `init`, such as `implementer,reviewer`.

Other ways: `--as <role>` on a command, `HANDOFF_ROLE` set when the session starts, or `handoff settings … --as <role>`, which gives every session of that CLI in the project one role. Those given must agree.

- **Need a role:** `post`, `claim`, `reply`, `note`, `repair`. They refuse without one.
- **You don't need one:** `handoff` on its own and `decide`, in a terminal of your own. With no role, the caller is you. In an agent's session they never ask for or take your decision.
- **Don't:** `init`, and the reads. `inbox` and `wait` take `--for <role>` (or `--as`). `show` takes neither.

## Keep Claude Code sessions on duty

So a Claude Code session never needs a paste or a nudge, run this in the project:

```
handoff settings claude
```

Each session takes the role you give it. One that starts with no role takes `claude`, if the project has that role and no running session holds it. `--as <role>` instead gives every Claude Code session here that role. It prints settings for `.claude/settings.local.json`, which stays out of git. Paste them in and start Claude Code there again. Every Claude Code session in the project then:

- **knows its job.** When it starts, or after its context is summarised, it gets the brief: its role, the loop, the rules, and what waits for it. `handoff brief` shows the same.
- **listens on its own.** When its turn ends, a hook waits in the background and wakes the session when something arrives for its role. Only one process waits per role; a second `wait` exits 4.
- **runs handoff without asking**, but never `handoff decide`, and never edits `.handoff/`.
- **tells you when it's stuck** at a permission prompt.

## Keep Codex sessions on duty

The same for Codex:

```
handoff settings codex
```

As for Claude Code: each session takes the role you give it, or `codex` when it starts with none and that is free. It prints two files for the project's `.codex/` folder, `hooks.json` and `rules/handoff.rules`, and with `--as <role>` a third, `config.toml`, that fixes the role. Merge them with any already there, and keep `.codex/` out of git by adding it to `.git/info/exclude`. Codex reads them only in a project it trusts. The next time Codex starts there, it asks you to review the new hooks: choose **Trust all**. Every Codex session in the project then:

- **has its role**, the one you gave it or the fixed one. `handoff` on its own shows its inbox, never your decisions.
- **knows its job.** It gets the brief when its first turn starts, and after its context is summarised.
- **keeps listening.** When its turn would end, a hook waits for its work and continues the session with what arrives. Codex shows the hook running while it waits.
- **can reach you.** `handoff` runs outside Codex's sandbox, so its notifications and pop-ups arrive. `handoff decide` is forbidden.

## While you're away

A session that listens keeps your Mac from idle sleep while an agent owes a move: a review not yet given, or changes or a question not yet answered. So the agents keep handing work to each other while you're away. Once nothing is owed, or only your decision is, or a move has been owed for two hours, the Mac sleeps as usual. Closing the lid still puts it to sleep.

If it sleeps anyway, nothing is lost. A session picks up what arrived when the Mac wakes, and notes the sleep once.

## Commands

| Command | What it does |
|---|---|
| `post` | Ask for a review of one commit. `--supersedes <id>` replaces an earlier request |
| `claim <id>` | The reviewer says "seen, reviewing". Not a verdict |
| `reply <id> --verdict …` | APPROVE, CHANGES or QUESTION, for exactly the request's commit |
| `note` | Anything else, such as an answer to a QUESTION (`--re <id>`) |
| `handoff` | On its own: what waits for your decision, and what has gone quiet. At a terminal it asks approve, deny or skip for each |
| `decide <item>` | You close an item: `handoff decide H8 --approve`. Needs a real terminal and a y. `--note "…"` is optional |
| `inbox [--for role]` | What's waiting for a role, or for the operator |
| `wait [--for role]` | Blocks until something new arrives, then prints it |
| `show <id>` | One entry in full |
| `brief [--for role]` | What an agent on duty needs: its role, the loop, the rules, and what waits for it |
| `role <role>` | Run by an agent's session when you give it a role. `role` alone shows it |
| `settings claude\|codex` | Prints the settings that keep Claude Code or Codex sessions on duty. `--as <role>` fixes one role for all of them |
| `repair` | Recovers the record after a crash, keeping every byte |

Run `handoff help` for every flag. Exit codes: 0 done, 2 refused or failed, 3 `wait` timed out, 4 another `wait` already listens for the role.

## A round, in short

Here claude implements and codex reviews. Sessions set up with `settings` run these themselves, and listen without `wait`.

```
# claude
handoff post --as claude --item W1 --to codex --commit <sha> --title "…" --body req.md
handoff wait --for claude

# codex
handoff wait --for codex
handoff claim --as codex 1
handoff reply --as codex 1 --verdict APPROVE --commit <sha> --body verdict.md

# you, at a terminal: approve, deny or skip each item
handoff
# or name the item
handoff decide W1 --approve --note "merge it"
```

## Read more

- The spec: `docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md` (revision 2.1)
- How this project was built, entry by entry: `docs/CHANNEL.md`
- The duty prompts for each role: `docs/prompts/`
- The reviews and the referee reading: `docs/reviews/`

This repository starts from one commit. The commit ids in `docs/` and `evidence/` belong to the private history it was built in, which isn't published.

`handoff` succeeds the earlier `tripartite` experiment, which drove the agents headless from a state machine. This tool lets the agents drive themselves and shares no code with it.

## License

MIT. See `LICENSE`.
