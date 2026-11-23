# The reviewer's evidence for H33 (`48f1a93`, request #188)

Run by Fable 5.2 (reviewer) on the operator's Mac, 2026-11-19, from a detached worktree at `48f1a93574bbd96263f12e253845f493a8d0c2dc`. The reply is CHANGES. The commit is a design and a facts file; no code changes.

## Real sessions

- `real-probe.log`: throwaway Claude Code sessions (2.1.326, Haiku) in a scratch project, with the installed build of main.
  - C1: a print-mode run. Its start hooks run, and it takes the reviewer role.
  - C2: that session resumed in a terminal with nothing typed. A listener runs below it, and a request sent 15 s later wakes it. A request that was already waiting wakes it at the start.
  - C3: a session started in a terminal and closed with nothing typed. Its start hooks ran; it left no transcript, and resuming it fails.
- The log keeps my three failed tries, each marked: a flag mistake, a wrong answer to the folder's trust question, and a request refused as a second one on its item.
- `real-probe.py`: the driver. `shim-handoff.txt`: the shim in handoff's place on the sessions' PATH, which notes each run and sends alerts to a scratch file.
- The scratch project's Claude Code data is purged. No alert was written and no probe process is left.

## Offline

- `checks.log`: this reviewer session's own handoff record, which kept its role when its process id changed; the two lines of `session.go` behind that; the merge with main, the build, and the name sweep. The patterns are kept outside the repository.
- Inferred there: that the new process id on 13 Nov came from a resume. The record and the process's start time are 16 s apart; nothing else says what started it.

## Not tried

Codex and Antigravity sessions, `/resume` typed inside a running Claude Code session, and iTerm2's scripting.
