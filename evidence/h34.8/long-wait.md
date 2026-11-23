# H34.8: a long background `handoff wait` in Antigravity

Run by the implementer (Claude Code, Opus 5.5) on 2026-11-23, from 05:34 to 06:36 UTC, for the reviewer's #650. The operator allowed throwaway Antigravity sessions for the H34 probe in chat ("feel free to run anti throwaway sessions").

- **Setup:** agy 1.5.14, model `gemini-3.8-flash-low`, driven through a pty by `evidence/h34/agy-probe/drive.py`.
  - Two scratch projects, each with `handoff init --roles implementer,reviewer`.
  - H34.8 round 1's code, built at `770ca05` (round 1 rebased onto H34.5 round 2; it differs from round 1's `564f89d` only in evidence logs), first on the agent's `PATH`, so the global Antigravity hooks (`~/.gemini/config/hooks.json`) ran it. Its brief said `--timeout 25m`; each session was told the timeout to use.
  - Each session took the role reviewer by its own `handoff role reviewer`.
  - Each was told to listen as its brief says, with `--timeout 25m` (session A) or `--timeout 60m` (session B). Each time a wait finished, it was to log the time and exit code to `woke.txt`, then start the same wait again.
  - Nothing was sent to either, and no one typed after the first message.
- **Left out of the repository:** the agy logs and the raw pty output, which hold the operator's account details. Below are the times taken from them.

Each finding is marked Run (seen) or Inferred.

1. **A 25-minute background wait runs to its end and wakes the agent. Run.**
   - Session A's first wait held the role's lock from 05:34:38. It exited 3 at 05:59:40, and the agent logged `05:59:40 exit=3`. The next wait held the lock from 05:59:44.
   - It happened again: `06:24:50 exit=3`, and the next wait from 06:24:54.
2. **So does a 60-minute one. Run.** Session B's wait held the lock from 05:34:40 and exited 3 at 06:34:44. The agent logged `06:34:44 exit=3`, and the next wait held the lock from 06:34:44.
3. **No model call while a wait runs; 2 or 3 at each wake. Run.** Times of `streamGenerateContent` in each session's log:
   - A: 05:34:33, :35, :37, :40 (the first message); then none until 05:59:40, :44, :46; then none until 06:24:50, :53, :56.
   - B: 05:34:35, :37, :40, :43; then none until 06:34:43, :47.
   - So idle costs about 7 calls an hour with `--timeout 25m`, and 2 with `--timeout 60m`.
4. **No limit on a background command was met at 60 minutes. Run.** Longer wasn't tried. Inferred: a limit, if there is one, is above an hour.
5. **The Stop hook stood aside the whole time. Run.** The role's presence said `"how":"wait"` throughout. No hook held the lock, and no hook continued a turn.
6. **Closing agy ends the waits. Run.** At 06:35:44 both sessions were closed (Esc, Ctrl-C twice). Their wait processes (pids 98048 and 99404) were gone at the next check.

**Cleanup:** for the two probe conversations only (`c3ce925b-…` and `73f18f60-…`), removed:
- in `~/.gemini/antigravity-cli/`: `brain/<id>/`, `conversations/<id>.db` and `annotations/<id>.pbtxt`;
- the id's entry in `cache/conversation_metadata.json`, and the two scratch projects' entries in `cache/last_conversations.json`;
- the rows in `conversation_summaries.db` (`conversation_summaries`, `conversations_fts`).

`history.jsonl` and `settings.json` were unchanged, and no new `log/` file was written. Afterwards, `brain/` and `conversations/` list as before.

**What changes for H34.8:** the brief gives an Antigravity session `--timeout 60m` rather than 25, measured to run and costing about 2 model calls an hour idle; the hook's reminder says the same. A Claude Code session's wait stays at 25 minutes.
