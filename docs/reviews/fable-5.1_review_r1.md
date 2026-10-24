# Review of the handoff proposal, round 1

**Model:** Claude Fable 5.1 (`claude-fable-5-1`).
**CLI:** Claude Code 2.1.295 on macOS 27.0, arm64 (the operator's MacBook Air).
**Date:** 2026-10-09.

What I read: `CLAUDE.md`, the proposal in full, and `docs/CHANNEL.md`. Also the two website notes of 7 October the proposal builds on; the website channel's protocol header, status board and every line that mentions a worktree; Codex's current listener script in `/tmp`; and tripartite's `README.md` and `PLAN.md`. I did not read `docs/reviews/gpt-6_review_r1.md`. My CLI showed me that agent's channel entry as a file change while I was gathering evidence; the worktree point in section 2 below was already written from the website channel's own text before that.

Evidence labels: **Run** (I ran it on this machine today; commands and output in the appendix), **Source** (a document, or a tool's own description), **Inferred**.

## 1. The heart, in my own words

Two agents in two sessions pass work to each other through one append-only record on disk, and a person reads that record and is called only to decide. The one thing missing today is a reliable way for a session to wait, so the operator carries every message. `handoff` is a command both sessions call to write entries that are checked before they land, to ask what is waiting, and to block until something is. It never runs a model, never drives the loop, and cannot produce an approval by itself. The judgement and the loop stay with the agents.

## 2. Section by section

**§1 The heart.** Agree. One gap: `docs/CHANNEL.md` names a third role, `reviewer 2`, and the kickoff starts three agents. The proposal has two roles. Say what a second reviewer is: a role in `config.toml`, addressable with `--to`, and whether APPROVE on an item needs one verdict or both. If there are only two roles, say so.

**§2 Principles.** Agree. One inconsistency: this section lists four evidence labels (Run, Source, Live, Inferred). `CLAUDE.md`, the kickoff and the website channel use three. "Live" comes from Codex's 7 October note (Source). Define it or drop it.

**§3 A working day.** Agree with the shape. Two things are missing.

- "The operator gets a notification when a request has sat unclaimed for 20 minutes." Nothing in the design runs when no `wait` is running; §4 rules out a daemon. So the stall check can only live inside a `wait` process, and the one that notices a request to the reviewer going unclaimed is the implementer's `wait`. If both sessions have died, nobody notifies anyone. Write that down: "the stall check runs in any live `wait`; if none is live, the operator finds out by looking".
- The Wait row says a session runs `handoff wait` and calls it again on timeout. In Claude Code that cannot be a foreground call (section 3). The row should say the duty prompt differs per CLI.

**§4 What it is and is not.** Agree.

**§5 The record.** Agree with the shape: one JSONL file, a hash chain, bodies as files, generated Markdown. Four changes.

- *Where `.handoff/` lives. This is the largest point in this review.* §10 recommends committing `.handoff/` in the project. In the website project both agents already work in worktrees. Claude built W2 to W5 in `.claude/worktrees/<branch>`, and Codex validated `4f76dff` in a detached worktree (Source: `channel-archive/IMPLEMENTATION_CHANNEL_archive_1.md` lines 91, 207, 290, 317, 370; `_archive_2.md` line 1028). A committed `.handoff/` is a different file in every worktree and on every branch. Two sessions in two worktrees would write two records, and a branch switch would swap the record under a running `wait`. The website channel avoids this only because `docs/proposals/` is gitignored and lives in the main checkout. The record needs exactly one location per repository, independent of branch and worktree. Two options keep it in the project. One is `$(git rev-parse --git-common-dir)/handoff/`, shared by all worktrees and never tracked. The other is `.handoff/` in the main worktree, gitignored, found from any worktree through the common dir. I prefer the second, because a person can find it and back it up, with `doctor` warning if the directory is tracked. Backup then needs its own line in §10.
- *Duplicates.* "Ids are time-ordered and unique, so a repeated post is a duplicate the tool refuses." A new id per post means two identical posts get different ids, so the id cannot detect the duplicate. Say what a duplicate is: the same kind, item, from, to, commit and title within a window, or an explicit `--key` the caller repeats on retry. The real case is a session retrying after its own timeout.
- *Write order.* The body `.md` and the JSONL line are two writes. Write the body first, then the line under the lock. A crash then leaves an orphan body, never a line pointing at nothing. Say it.
- *Chain details.* `prev` of the first line (null, or a fixed genesis string), and what `render` compares to detect a hand edit (the hash of the last render, kept in a small state file). Small, but the reviewer will ask.

**§6 Commands.** Agree with the set. Changes:

- *Roles.* `HANDOFF_ROLE` "set by the duty prompt" cannot work in Claude Code. Each Bash call is a fresh shell; an `export` in one call is gone in the next (Run, section 3). Variables set when `claude` is launched do reach every call (Run). So the role comes from `HANDOFF_ROLE=implementer claude` at launch, by the operator, or from `--as <role>` on every command, written into the duty prompt. Require one of them and refuse to run without a role. Never default.
- *`wait --emit` "never exits".* Give it `--timeout` as well, so the process leaves on its own, with exit 3, a minute before the Monitor's 30-minute kill. A kill is not evidence; an exit code is.
- *`inbox --quiet`* is named in §7 for the Stop hook but not defined. Define its exit code: 0 when nothing waits, 1 when something does. That is all a hook can use.
- *A second revision.* §13 check 5 says the earlier claim "shows as superseded". Nothing in `post` says how. Either `post --supersedes <id>`, or the rule "a new request on the same item from the same role supersedes the open one". Pick one and write it.
- *`--on <branch>`* appears only in the refusals column. Add it to the usage.
- *`verify-merge`.* Keep it a separate command. Do not fold the clean-merge check into `post --parents` as a refusal. A failed merge check is itself worth recording as a `note`; a refusal leaves a hole in the record where the interesting failure happened.
- *"Git is read only"* needs one precise sentence. `git merge-tree --write-tree` writes a tree object to the object database. Run: in a scratch repository the object count went from 8 to 9 after one call, with no ref, index or working-tree change; the man page says the same. Write "never changes a ref, the index or a working tree".

**§7 Waiting, per CLI.** This is section 3 of this review. In short: the Monitor facts are right, and so is the Stop-hook net. Two mechanisms are missing: a background shell call, and a session cron. One risk is missing: the monotonic clock stops during sleep, in Go and in Rust, on macOS.

**§8 Priming and presence.** Agree. Two additions.

- Claude Code's `SessionStart` hook has a `compact` matcher, and the hook's stdout is added to the context (Source: hooks documentation; matchers are `startup`, `resume`, `clear`, `compact`, `fork`). A project hook that runs `handoff brief --for "$HANDOFF_ROLE"` on `compact`, `startup` and `resume` re-primes a session with no paste from the operator. "The operator pastes the brief" becomes the fallback, not the rule.
- `osascript` notifications work from a tool call (Run: exit 0, one notification shown). Fine as the default.

**§9 Safety rules.** Agree, with the git wording fix from §6.

**§10 Per-project setup.** Disagree with point 2 (commit `.handoff/`), for the worktree reason in §5. Recommend: gitignored, one location per repository, found through the git common dir, and a named backup.

**§11 Lineage.** Agree. Tripartite's `PLAN.md` draws the cycle (`INIT → INTAKE → PLAN → PLAN_REVIEW → EXECUTE → OUTPUT_REVIEW → REVISE → DECISION_GATE`), which is exactly what §4 forbids. Nothing to port.

**§12 Language.** Section 4.

**§13 Verification.** Agree with the twelve. Changes:

- Check 9 (laptop sleep) cannot be run by an agent; it needs the operator to close the lid. Mark it "The operator present", like check 11. It will fail unless the deadline uses the wall clock (section 3).
- Add: a role on the command line that differs from the launch environment is recorded as an anomaly, not refused.
- Add: two worktrees of one repository resolve to the same record, and `git switch` does not change it.
- Add: a killed writer never leaves a line without a body.
- Add: a `wait` in a real Claude Code session survives one compaction and still wakes on the next post.
- Add: the Stop hook keeps a session on duty when its inbox is non-empty and lets it stop when empty, in a real session.

**§14 Order of work.** One change: move `wait` into step 1, and trial it on this repository, between two of the three sessions the operator has already started, before anything else is built. The website trial is gated on release 1; a trial here is not. The sessions building the tool should use it from the first day it can post and wait. That finds the CLI problems in section 3 on day one, not day four.

**§15 Decisions.** Add: where the record lives (§5); whether there is a second reviewer and what APPROVE then needs (§1); role by launch environment or by flag (§6).

**§16 Risks.** Add three rows.

| Risk | Mitigation |
|---|---|
| The monitor expires and the session does not re-arm it, so the session is deaf | Background `wait` instead of a monitor; the Stop hook; a session cron; the stall notification |
| The wait deadline is measured on a clock that stops during sleep | A wall-clock deadline, tested with an injected clock |
| Two worktrees, two records | One location per repository through the git common dir |

**§17.** Fine.

## 3. The CLI I run in

Claude Code 2.1.295 on macOS 27.0 (Run). Facts, each with its evidence.

**Foreground shell calls are capped at 10 minutes.** Source: the Bash tool's own description ("max 600000 for a foreground command"); a foreground `sleep` is refused outright. So `handoff wait --timeout 25m` as written cannot be a foreground call in Claude Code. It would be killed at 10 minutes with an exit code that is not the tool's. The Claude Code duty prompt must say "run it in the background".

**A background shell call re-invokes the session when it exits.** Source: the Bash tool's description ("keeps running across turns and re-invokes you when it exits"). Run: a 12-minute loop started in the background at 18:51:20 UTC ran to completion, past the 10-minute foreground cap, and its completion notice re-invoked this session in a new turn (appendix). This is the simplest primary mechanism for Claude Code: `handoff wait --for implementer --timeout 25m` in the background. Its exit (0 on an entry, 3 on timeout) re-invokes the session once; the session reads its inbox and calls `wait` again. One tool call per wake, and the exit code says why it woke.

**The Monitor tool streams lines.** Source: its description. Each stdout line is one notification, and lines within 200 ms are batched. A monitor lives at most 30 minutes (`timeout_ms` is capped at 1,800,000) and sends one notice at expiry. The session must re-arm it. A monitor that emits too many lines is stopped. Run: I armed a monitor on `tail -n0 -F` of a scratch `channel.jsonl`, then appended one line from a separate shell call. The watcher saw it 9 ms after the write (sent `1791571924.084`, received `1791571924.093`), and the notification reached this session in the same turn. That is the `wait --emit` design, and it works. The 30-minute cap in §7 is right.

**Which to prefer.** Background `wait --timeout` over Monitor `--emit`, for Claude Code: no 30-minute re-arm, a meaningful exit code, and no risk of the stream being stopped for volume. Keep `--emit` for Codex or any CLI that wants a stream.

**Nets for a session that forgets to re-arm.**

- *Stop hook.* Source (hooks documentation): it runs "when the main Claude Code agent has finished responding" and not on a user interrupt; exit code 2, or `{"decision":"block","reason":…}`, makes Claude continue with the reason as its instruction; after eight consecutive continuations the turn ends anyway, and the count resets whenever Claude calls a tool; the hook input carries `stop_hook_active` and a `background_tasks` array, each task with a type (`shell`, `monitor`, …) and status, so the hook can tell "waiting" from "done". So the net in §7 is real: `handoff inbox --for "$HANDOFF_ROLE" --quiet` exits 1 when work waits; the hook blocks with "work is waiting; run handoff inbox"; a session cannot go idle on an unanswered request. The hook must let the session stop when a `wait` is in `background_tasks`, or it blocks every turn.
- *Session cron.* Source: the CronCreate tool's description. A session can schedule a prompt on a cron expression; the job fires only while the session is idle, lives in memory only, and expires after 7 days. A job every 20 minutes saying "run handoff inbox; if nothing waits and no wait is running, start one" is a third net with no process to keep alive. Not in the proposal.
- Neither net survives the session being closed. Nothing can. Then the honest state is "absent", as §8 says.

**The operator can still type.** Background tasks and monitors run while the session is idle; their events arrive in the chat and are marked as not the user's reply (Source: Monitor description; Run: the test event arrived mid-turn, labelled as a background event).

**Long sessions.** My context is summarised when it grows long, and work continues from the summary (Source: my own instructions). The background task and the monitor belong to the harness, not the context, so they survive it (Inferred). What does not survive is detail: which `wait` is running and what the duty prompt said. The fix is the `SessionStart` hook on `compact` running `handoff brief` (Source: hooks documentation).

**Environment variables.** Run: `export HANDOFF_ROLE=reviewer` in one Bash call (pid 83306); the next call (pid 84186) saw it empty. Run: variables set on the `claude` process (`CLAUDE_CODE_SESSION_ID`, `CLAUDE_PID` and eight others) are visible in every call. So the role comes from the operator's launch command or from `--as` on every call, never from a paste. The hooks documentation also describes `CLAUDE_ENV_FILE`, which a `SessionStart` hook can append `export` lines to; it is per project, so by itself it cannot tell two sessions in one project apart.

**Machine sleep.**

- Run: `pmset -g assertions` shows idle sleep prevented by three `caffeinate -i -t 300` processes, each a child of a running `claude` process. Claude Code keeps this Mac from idle sleep in five-minute windows while it works. That does not cover the lid being closed, and I do not know whether it covers an idle session with a background task running (Inferred: probably not; the assertion is tied to activity).
- Source: macOS `clock_gettime(2)`: `CLOCK_MONOTONIC` "will continue to increment while the system is asleep"; `CLOCK_UPTIME_RAW` "does not increment while the system is asleep" and equals `mach_absolute_time`. Source: Go's runtime on darwin reads `mach_absolute_time` for its monotonic clock (`/usr/local/go/src/runtime/sys_darwin.go`, `nanotime1` at line 363, the import at line 682), and `time/time.go` line 54 warns "On some systems the monotonic clock will stop if the computer goes to sleep." Source: Rust's `Instant` on Apple uses `CLOCK_UPTIME_RAW` on purpose (`library/std/src/sys/time/unix.rs` lines 55 to 67, with the man page quoted in the comment). So in either language `time.Since`, `context.WithTimeout`, `time.After`, `Instant::elapsed` and `thread::sleep` do not count time the machine was asleep. A `wait --timeout 25m` built on them is a 25-minute awake timer, not a wall-clock one. §7 promises wall-clock time and §13 check 9 tests it. The implementation must take the deadline from the wall clock (`time.Now().Round(0)` in Go, `SystemTime` in Rust), compare on every poll tick, and record a gap when a tick arrives late. That is testable without sleeping the Mac, with an injected clock.
- Inferred: the Monitor's own 30-minute limit is a timer inside the Claude Code process. I do not know which clock it uses. If it stops during sleep, a monitor armed before a long sleep lives longer than 30 minutes of wall time. Harmless.
- Inferred: a background process is suspended during sleep and resumes on wake. No write happens while the machine is asleep, so no kqueue event is lost; the one-minute poll in §7 covers a write by a session that woke first.
- Not run: an actual sleep. Check 9 needs the operator.

**MCP.** The proposal says Claude Code backgrounds MCP calls longer than two minutes. I did not verify it, and the design does not need it: shell plus hooks covers everything above.

## 4. Language

Go. The tool is files, one lock, JSON, a subprocess for git and a directory watch; Go's standard library has all of it except the watch, which is one dependency (`fsnotify`). It compiles in a second or two, which matters because the agents will run hundreds of edit-build-test cycles, and Go diffs are plain enough for the reviewer to hold one in a single view. Rust buys nothing here: there is no concurrency beyond one lock, no performance pressure, and the sleep-clock problem in section 3 is identical in both standard libraries, so neither is safer by default. The one argument for Rust is the operator's learning, which is the operator's call; if Rust, budget the slower cycle and fix the dependency list up front (`notify`, `serde`, `clap`, `fd-lock`) so it is a decision and not a drift. Run: Go 1.25.3 and Rust 1.97.1 are both installed.

## 5. If I were the implementer; if I were the reviewer

**Implementer.** Build in this order, and use the tool on this repository from the first day it can post and wait:

1. The record: location through the git common dir, lock, hash chain, body-then-line write order, `post`, `show`, `thread`, `inbox`. Tests for checks 1 to 4.
2. `wait`, with a wall-clock deadline and an injected clock in tests, `--timeout` and `--emit`, kqueue watch plus the one-minute poll. Check 8.
3. The two duty prompts, one per CLI, with the exact commands each session runs (background shell call for Claude Code, foreground for Codex), the Stop hook and the `compact` hook. Then the two-session trial on this repository, with two of the three sessions already open. That is check 10 without waiting for the website release.
4. `render`, `board`, `verify-*`, `brief`, presence, notifications, `init`, `evidence`, `doctor`, `archive`.

Push back on: committing `.handoff/`; `HANDOFF_ROLE` from a paste; `--emit` as the only wait mode for Claude Code; folding the merge check into `post`; `archive` and `mcp` before the trial; any retry logic inside the tool for posting, because the caller is an agent and can retry with a key.

**Reviewer.** I would not approve without seeing:

- Run evidence under `evidence/` for checks 1 to 4, 7 and 8, with the command, the date and who ran it, as `CLAUDE.md` requires.
- A test in which `wait` ends 25 minutes after start by the wall clock while the injected monotonic clock has advanced five minutes.
- A test that two worktrees of one repository resolve to the same record path, and that `git switch` does not change it.
- A test that `reply --verdict APPROVE` from `--as implementer` is refused, and that a `--commit` different from the request's is refused; exit non-zero, nothing written.
- `go vet` and `staticcheck` clean, and `grep -r tripartite` over the module empty, per the lineage rule in `CLAUDE.md`.
- The duty prompt for my own CLI, run by me in a real session, waking once on a real post.

## 6. Open questions for the operator

1. Where does the record live: `.handoff/` gitignored in the main worktree and found through the git common dir (my recommendation), inside `.git/`, or committed as §10 says? If committed, how should two worktrees share it?
2. Is there a `reviewer 2`? If so, does APPROVE need both verdicts, or does the second reviewer act only at gates you name?
3. Role by launch (`HANDOFF_ROLE=implementer claude`) or by `--as` on every command? I recommend both: the launch sets it, the prompt repeats it, the tool refuses when neither is present.
4. Should the first trial run here, on this repository, between two of these three sessions, before the website release? I recommend yes.
5. For Claude Code, a background `wait --timeout` (one wake per exit) or Monitor `--emit` (a stream)? I recommend the background `wait`; section 3 says why.
6. Should `wait` keep the Mac awake (`caffeinate -i` around the process), or is sleep acceptable with a wall-clock deadline? I lean to the deadline only; this is a laptop.
7. Name: I would keep `handoff`. It names the act. `relay` names what you do today, which the tool removes.
8. Who commits `docs/CHANNEL.md` in this repository? The kickoff says each session commits only its review file, so three sessions leave the channel modified in one working tree. I assume you commit it.

## Appendix: Run evidence

All on 2026-10-09, the operator's Mac, from this Claude Code session. Output trimmed to the relevant lines.

```
$ claude --version
2.1.295 (Claude Code)
$ sw_vers
ProductName: macOS   ProductVersion: 27.0   BuildVersion: 26A428
$ go version; cargo --version
go version go1.25.3 darwin/arm64
cargo 1.97.1 (c980f4866 2026-06-30)
$ git --version
git version 2.55.0
```

Monitor latency (Monitor tool armed on the scratch file; one line appended from a separate Bash call):

```
monitor command: tail -n0 -F "$f" | while IFS= read -r line; do printf 'recv=%s %s\n' "$(perl -MTime::HiRes=time -e 'printf "%.3f", time')" "$line"; done
append:          printf '{"id":"TEST1","to":"implementer","sent":%s}\n' "$sent" >> "$f"
appended TEST1 at sent=1791571924.084 (18:52:04 UTC)
monitor event:   recv=1791571924.093 {"id":"TEST1","to":"implementer","sent":1791571924.084}
expiry notice:   [Monitor expired after 10m with 1 event delivered. Re-arm it if you still need the watch.]  (armed with timeout_ms 600000; arrived at 19:01 UTC as a background event)
```

Environment variables between Bash calls:

```
call 1: export HANDOFF_ROLE=reviewer; echo "HANDOFF_ROLE=[$HANDOFF_ROLE] pid=$$"
        HANDOFF_ROLE=[reviewer] pid=83306
call 2: echo "HANDOFF_ROLE=[$HANDOFF_ROLE] pid=$$"
        HANDOFF_ROLE=[] pid=84186
inherited from the claude process (names only): CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_MESSAGING_SOCKET
  CLAUDE_CODE_MESSAGING_TOKEN CLAUDE_CODE_EXECPATH CLAUDECODE CLAUDE_CODE_SESSION_ID
  CLAUDE_CODE_CHILD_SESSION CLAUDE_CODE_SESSION_ATTENDED CLAUDE_PID CLAUDE_EFFORT
```

Sleep assertions and their owners:

```
$ pmset -g | grep sleep
 sleep  1 (sleep prevented by sharingd, powerd, caffeinate, caffeinate, Claude, Electron, caffeinate)
$ ps -eo pid,ppid,etime,command | grep '[c]affeinate -i'
81622 18158  02:48 caffeinate -i -t 300
82949 74545  01:01 caffeinate -i -t 300
82895 78419  01:05 caffeinate -i -t 300
parents 18158, 74545, 78419: claude --dangerously-skip-permissions
```

Clocks:

```
$ man clock_gettime | grep -A2 'CLOCK_MONOTONIC\b\|CLOCK_UPTIME_RAW\b'
CLOCK_MONOTONIC   ... will continue to increment while the system is asleep.
CLOCK_UPTIME_RAW  ... does not increment while the system is asleep. The returned
                  value is identical to the result of mach_absolute_time() ...
$ grep -n 'mach_absolute_time\|func nanotime1' /usr/local/go/src/runtime/sys_darwin.go
363:func nanotime1() int64 {
682://go:cgo_import_dynamic libc_mach_absolute_time mach_absolute_time "/usr/lib/libSystem.B.dylib"
$ sed -n 54p /usr/local/go/src/time/time.go
// On some systems the monotonic clock will stop if the computer goes to sleep.
$ grep -n 'CLOCK_ID' .../library/std/src/sys/time/unix.rs
67:    pub(crate) const CLOCK_ID: libc::clockid_t = libc::CLOCK_UPTIME_RAW;   (#[cfg(target_vendor = "apple")])
70:    pub(crate) const CLOCK_ID: libc::clockid_t = libc::CLOCK_MONOTONIC;    (not apple)
```

`git merge-tree --write-tree` in a scratch repository with two one-commit branches:

```
$ git count-objects -v | grep count        -> count: 8
$ git merge-tree --write-tree master side  -> 3683f870be446c7cc05ffaef9fa06415276e1828
$ git count-objects -v | grep count        -> count: 9
$ git status --short | wc -l               -> 0
$ git for-each-ref | wc -l                 -> 2 (unchanged)
man git-merge-tree: "does not make any new commits and does not read from or write to either the working tree or index."
```

macOS notification from a tool call:

```
$ osascript -e 'display notification "osascript test from a Bash tool call" with title "handoff review"'; echo $?
0
```

Codex's current listener (Source, `/tmp/codex-channel-listener-20261009-121652.py` and its state file): polls the website channel every 300 s by SHA-256, closes after 3600 s idle on the wall clock, records `poll_gap` events. Its last state: `closed_idle` at 13:31 UTC after no change since 12:31.

Website channel today: 77 headings in the current file, 99 and 48 in the two archives (`grep -c '^### \['`).

Background shell call past the 10-minute foreground cap (Bash tool, `run_in_background`, a loop writing one timestamp a minute for 12 minutes):

```
background heartbeat finished: started 2026-10-09T18:51:20Z ended 2026-10-09T19:03:21Z
[exited with code 0]
```

The completion notice arrived as a background event and re-invoked this session in a new turn, after I had ended the previous one. The task ran for 12 minutes, past the 10-minute foreground cap, and exited 0.
