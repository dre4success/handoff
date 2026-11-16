# H32: Antigravity sessions

Run by Opus 5.5 (implementer) on the operator's Mac, 2026-11-15, Antigravity CLI 1.5.9 (it updated itself from 1.5.5 at 21:50:45, before the first probe), model `gemini-3.8-flash-low`, on branch `h32-antigravity`. §17 row 102 says what changed. Each probe drove a throwaway interactive session through a pseudo-terminal, in a throwaway git repository after `handoff init`, written `<scratch>`. Every probe set `HANDOFF_TEST_NOTIFY`.

## What Antigravity does

- `agy-probe.py`, `dump.sh`, `probe1-hooks.log`: hooks at SessionStart, PreInvocation, PostToolUse and Stop, each logging what it was given. SessionStart answered with an injected word, and the first Stop held 70 s and then continued the turn.
  - A hook runs in the folder of its hooks.json (`<scratch>/proj/.agents`); the stdin's `cwd` is the folder the agent is in. Its parent is Antigravity's process, the same for every hook of a session, and a new one after a resume.
  - Hooks and the agent's commands both have `ANTIGRAVITY_CONVERSATION_ID`, the stdin's `conversationId`; a resume keeps it.
  - SessionStart fired at the first message, and not on the resume. The model said the injected word, and after the held Stop's continue it said the word the reason asked for.
- `agy-probe2.py`, `dump2.sh`, `probe2-hooks.log`: a Stop hook that continues every time. The turn was continued 10 times (`executionNum` 0 to 9); the 11th continue was ignored and the turn ended. `executionNum` started again at 0 with the next message typed. Then a Stop held 60 s while a message was typed: the message waited until the hook ended.
- `agy-probe3.py`, `probe3-hooks.log`: while a Stop holds, Esc ends the hook (its 60 s were never logged), and a message typed then is answered at once. A message sent with `alt+m`, Antigravity's send-at-once key, still waited.
- `agy-probe4.py`, `probe4-hooks.log`: a Stop hook with `"timeout": 86400` held 660 s, then continued the turn.
- `agy-tty.py`, `tty.log`: the agent's commands run with a terminal on stdin and stdout.
- `withtty.py`, `operator-check.log`: so, before this branch, a command an Antigravity agent runs passed both of `fyi`'s checks on the installed build. `decide`, `away` and `back` share them. This branch refuses it as an agent's session. `decide` itself wasn't run: this session's settings forbid it.

## This branch, for real

- `agy-real.py`, `real.log`: the hooks this branch's `handoff settings antigravity` prints, in a throwaway project, with this branch's build.
  - The session took the reviewer role; its binding noted Antigravity's process.
  - A request posted from outside continued the session's turn, and it ran `show 1`.
  - Esc, then a message, was answered.
  - Closed and resumed, its binding noted the new process at the end of its first turn, and a second request continued it again; it ran `show 2`.
  - The log's end lists the steps in the conversation's own transcript: the brief at the first message ("has no role in it"), each request as a system message, and each answer.

## Checks

- `mutate.py`, `mutations.log`: each of the item's twenty conditions taken out in turn. Each one fails a test. A first run found one that held nothing: the tests set `ANTIGRAVITY_CONVERSATION_ID` for hooks, which hid the stdin's `conversationId`. Antigravity documents the stdin, so a test now gives the stdin alone.
- `go-test-race.log`: build, vet, gofmt and every test with `-race`. A first full run failed one older test, `TestH10bSettingsCodex`, which expects the words of `settings`' refusal of an unknown CLI; they now name antigravity, and the test was changed to match.

After these files were made, the probes' conversations and their data were deleted, and the throwaway folders taken out of Antigravity's trusted workspaces.
