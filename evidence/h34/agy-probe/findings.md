# H34 probe: Antigravity and a background `handoff wait`

Run by a fork of the implementer's session (Claude Code, Opus 5.5) on 2026-11-22, from 07:07 to 07:24 UTC. The operator allowed it in chat ("feel free to run anti throwaway sessions").

- **Setup:** agy 1.5.14, model `gemini-3.8-flash-low`, driven through a pty (`drive.py`), in a scratch project with `handoff init`.
- **Three throwaway sessions:**
  - A: no role, a 25-minute wait.
  - B: no role, a 1-minute wait.
  - C: the role implementer, with the global handoff hooks.
- **A project-level hook** (`stophook.sh`) logged each hook's raw stdin (`hook-stdin.log`).
- **Left out of the repository:** the agy logs and the raw pty output, because they hold the operator's account details. The session data was removed afterwards, for the three probe ids only.

Each finding is marked Run (seen) or Inferred.

1. **A background `handoff wait` wakes the agent, past 10, with no one typing. Run.**
   - The agent ran the wait with `run_command` in the background (`WaitMsBeforeAsync: 500`). When the wait exits, the agent gets a "Task … finished" message with its output, and starts the wait again.
   - All 12 notes were handled one after another, each in 3 to 17 s (`q1.log`, `wakes.log`).
   - Each delivery cost 2 model calls. There were none while idle: 3 calls at 07:07:36, still 3 at 07:11:44.
   - A timeout (exit 3) also wakes it for 2 calls, so a 25-minute wait idles at about 5 calls an hour (session B).
   - `executionNum` stayed 0 on every Stop, so these wakes don't count toward the 10.
2. **The operator can type while the wait runs, without Esc. Run.** The message went in at 07:13:45 and got its reply, and the wait kept the lock.
3. **Stop fires while a background task runs. Run.**
   - Its stdin had `fullyIdle:false`, `terminationReason:"NO_TOOL_CALL"`, `error:""`, `executionNum:0` and `continuationActive:false`. With no background task, `fullyIdle` was true.
   - Stop ran only 3 times in A's 14 turn ends, and after every wake in B. That it runs only once the session settles idle is Inferred.
   - **A bug in today's handoff (Run, session C).** While handoff's Stop hook held the turn, the agent's own `sleep 30` finished at 07:18:51. Its message reached the agent only at 07:20:22, when a note made the hook let go. So an Antigravity agent's own background tasks can't wake it while handoff's Stop hook holds the turn, for up to 23 hours. `fullyIdle:false` is the signal the hook could stand aside on.
4. **Closing agy ends the background wait and frees the lock within 2 s. Run.** The binary says "terminating %d background task(s) on exit". How long one background command may run wasn't measured. The longest seen was about 4 minutes, and the binary shows no limit.
5. **Errors. Run:** only `NO_TOOL_CALL` and an empty `error` were seen.
   - A turn that ended in an error (a content filter) ran no Stop hook. An Esc-interrupted turn ran none either. Afterwards nothing listened.
   - Inferred: a quota stop behaves the same, and a background wait already running would survive it, where the hook can't.

**What this means for H34**
- In Antigravity, the agent can listen as a Claude Code session with no hooks does, by running `handoff wait` in the background, and the cap of 10 doesn't apply.
- The Stop hook then becomes the safety net. It stands aside when `fullyIdle` is false, or when a wait already listens.
- This is to be designed and reviewed as its own part, before the brief changes.
