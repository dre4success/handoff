# Step 3 plan: make it usable day to day

For the operator. Drafted 2026-10-14, after the H7 trial.

## The goal

You assign a task in chat. The agents hand it back and forth on their own. You hear from the tool only when you're needed, and answering takes one click or one word.

## What gets built, in order

**H8. You hear about stalls.**
- `wait` records that its session is listening.
- If an agent claims something and then goes quiet for a set time (say 20 minutes), you get a Mac notification: "codex claimed H8 25 min ago, nothing since."
- You also get one when something is waiting for your decision.

This would have caught the trial's two-day stall within minutes.

**H9. Deciding is one step.**
- `handoff` on its own, in a project, lists what's waiting for you and asks approve, deny or skip.
- It takes item names (`H8`), not entry numbers. The note is optional. No `--as`.
- Your choice: also a Mac pop-up with Approve and Deny buttons, so you don't need a terminal at all. See below.

**H10. Sessions keep themselves going.**
- Claude Code: hooks that restart `wait`, re-prime a session after its context is summarised, and pre-allow `handoff` so it never stops at a prompt.
- Codex: the same as far as its settings allow. I'll check what it supports first.
- `init` prints the exact lines to paste into each CLI's settings.

**H11. Any project picks it up.**
- `handoff brief` tells an agent its role, the loop and what's waiting.
- `handoff init` with no flags uses defaults and adds `.handoff/` to `.gitignore`.
- Roles named after the agents (`claude`, `codex`), so either can implement or review per task.
- Then the line you agreed goes into your global instructions.

**H12. Small fixes.**
- `wait` keeps the Mac from idle sleep while it runs.
- The sleep warning stops firing on short wake-ups.
- The README shows `go install`.

**Later:** `CHANNEL.md` rendering and the status board (check 7). You read through `handoff` now, so these matter less.

## Your decisions

1. **Pop-up approval without a terminal?** The spec says `decide` needs a terminal, so that an agent can't approve for you. A pop-up click guards just as well, and neither is real authentication. **I recommend yes.**
2. **Roles named after agents as the default?** **I recommend yes.** It fits your way of working, where who implements changes per task.

## How it runs

- Each item goes on its own branch. Codex reviews it through `handoff`, and you approve it.
- From now on, this project's own channel runs through `handoff`.
- Codex needs one paste to start listening again.
