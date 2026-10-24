# Review r1 of the handoff proposal

*Model: Claude Opus 5.5 (`claude-opus-5-5`). CLI: Claude Code 2.1.295, on the operator's Mac (darwin arm64). Date: 2026-10-09. Role: not assigned.*

*Independence: I did not read any other review. While I was writing, GPT-6's channel entry of 18:52 UTC appeared in `docs/CHANNEL.md`. I read that entry, because the kickoff asks me to read the channel, but not the review it points to. Most of my points were drafted before it appeared.*

*Evidence labels: **Run**, meaning I ran it in this session (commands and outputs are in the appendix, because my scratch files will not last); **Source**, meaning a named document or file; **Inferred**, meaning my reasoning, not checked. No §13 check is closed by anything here.*

## 1. The heart, in my own words

The operator starts two sessions of different models by hand, one implementing and one reviewing, and primes each one. From then on they pass requests and verdicts to each other through a record in the project, and the operator reads it and decides. The only thing missing today is a reliable way for a session to wait, so the operator carries every message. `handoff` gives each session a command that blocks until something is addressed to it. It also gives a writer that refuses what the protocol forbids: an unresolved commit, a verdict from the wrong role, a verdict for a different commit. It never runs a model and never decides what happens next.

## 2. Section by section

### §1 The heart. Agree.

No change.

### §2 Principles. Agree, one fix.

"Live" appears as a fourth evidence label here and in §6, where `evidence add` refuses "the label is not one of the four". `CLAUDE.md`, the kickoff prompt and `docs/CHANNEL.md` use three: Run, Source, Inferred. Live is never defined. **Change:** define it in one sentence, or drop it.

### §3 A working day. Agree with the flow. Two problems with the stall alert.

1. **No process exists to send it.** §4 rules out a daemon, and `handoff` runs only when someone calls it. If the reviewer session dies, the tool has to notice 20 minutes later, but nothing is running to notice. **Change:** every running `wait`, for any role, checks all open requests for stalls on each poll and notifies once per stalled entry. It records that it has notified in a small state file, so the alert does not repeat. That covers one dead session, which is the common case. If both sessions are dead, nothing runs. Say so, or add an optional launchd job that runs a read-only `handoff check` every five minutes. A job that only reads and notifies does not schedule agents.
2. **False alarms.** A reviewer that is alive and spends 40 minutes on an earlier item leaves the next request unclaimed for over 20 minutes. **Change:** count a request as stalled only if it is unclaimed past `stall_minutes` *and* the addressee has no live `wait` and no recent activity (see §8).

### §4 What the tool is and is not. Agree.

No change. This list is the boundary reviewers will cite; keep it word for word in the README.

### §5 The record. Agree with append-only JSONL and a hash chain. Six details need settling before code.

1. **Bodies sit outside the chain.** Each `entries/<id>.md` is a second file, and its content is not hashed into the line. An edit to a body cannot be detected. A crash between writing the body and appending the line leaves either an orphaned file or a line that points at nothing. **Change:** put the body text inside the JSON line. That gives one write per entry, one thing to hash and nothing to orphan. `CHANNEL.md` stays the readable view. If the operator wants the separate files for reading, store each body's SHA-256 in its line.
2. **A unique id cannot catch a duplicate.** "Ids are time-ordered and unique, so a repeated post is a duplicate the tool refuses" does not follow. A retried `post` gets a fresh id, so it is a new entry. **Change:** define a duplicate as an entry with the same `kind`, `item`, `from`, `to`, `commit`, `re`, `verdict` and body hash as an earlier entry in the same open thread, and refuse it. Check 2 should run the same command twice.
3. **Ids people can type.** Agents and the operator will type `claim <id>` and quote ids in prose. `01JA2K7Q3M8F` is easy to mistype, and it is not a real ULID either (those are 26 characters). Every writer holds the lock, so a sequential number costs nothing and is safe: `#42`. The time order is already in `ts`. **Change:** sequential integer ids.
4. **A torn tail damages the next entry.** A writer killed mid-append can leave a last line with no newline. The next append is then joined onto it, which corrupts a good entry as well. **Change:** before appending, while holding the lock, check that the file ends in `\n`. If it does not, refuse and point to `repair`, or close the torn line and append a `repair` entry in the same operation. Check 3 should kill the writer at a point that leaves no newline.
5. **What the chain proves.** `prev` exposes an edit in the middle of the record. It cannot expose a cut tail, because the last surviving line just becomes the new head. Anyone who can edit the file can also recompute the chain. **Change:** say this in §5. Keep the head hash and the line count in a small second file, updated under the same lock, so that a cut tail shows up as a mismatch. Define the `prev` of the line after a damaged line as the hash of the damaged line's raw bytes, so the chain continues through damage.
6. **Archives contradict each other.** §6 says "the record itself is never split"; §5 says "the reader scans the record and the archives". If `channel.jsonl` is never split, the reader never needs the archives. **Change:** keep the record whole. Archives are rendered Markdown only, and `render` places closed threads older than N days in them by rule. Then v1 needs no `archive` command and no `archive` entry kind.

### §6 Commands. Agree with the refusal column; it is the real enforcement. Eight points.

1. **`wait` loses entries as written.** It "blocks until a new entry addressed to the role lands". New since when? An entry posted while the agent is working, between two waits, or between two Monitor arms is missed if "new" means "since this wait started". I reproduced this (**Run**, appendix A2): with the watcher re-armed between posts, the entry posted in the gap was never delivered. Monitor's 30-minute cap guarantees such gaps. **Change:** keep a cursor per role: the last entry id delivered to it. `wait` returns at once if any entry addressed to the role is past the cursor, and moves the cursor only when it prints. `inbox` and `brief` keep showing everything still open. That way an entry printed to a session that then crashed still appears on restart.
2. **The request lifecycle is not written down.** The commands imply states: open, claimed, answered with QUESTION, CHANGES or APPROVE, superseded by a new revision, decided. `inbox`'s four lists depend on them. This describes an entry's status; it does not drive a loop. **Change:** add one table: the states, and which entry moves a request from one to the next. Answer three things in it. Can one request get several replies, such as QUESTION and then APPROVE? How does the implementer answer a QUESTION: `note --to reviewer` with `re` set? After APPROVE, does the item go into the operator's "awaiting the operator" list on its own? I think it should.
3. **An agent can run `decide`.** Roles are declared, so an agent can run `handoff decide --as operator --approve`. That entry matters more than any other. **Change:** (a) `decide` requires a terminal on stdin and asks the operator to type the entry id. Claude Code's Bash tool has no terminal (**Run**, A4). (b) `init` prints a deny rule for each CLI; in Claude Code that is `"deny": ["Bash(handoff decide *)"]`. Claude Code's own docs say such a rule "isn't a security boundary around the program", because the same program called another way is not matched (**Source**: permissions page). Neither is authentication. Both turn an agent-made decision from something easy into something deliberate and visible.
4. **More than two roles.** `docs/CHANNEL.md` already names `reviewer 2` and a reviewer "at gates only". The proposal has two roles. **Change:** roles are a list in the config. A request names its addressee, and `claim` and `reply` are accepted from the addressed role, not from "the reviewer".
5. **Does `init` write or print?** §6 says `init` writes the two lines for `CLAUDE.md` and `AGENTS.md`. §10 says it prints them. §9 says writes go only under `.handoff/`. **Change:** it prints them.
6. **`post` flags.** `--on` is in the refusal column but not in the synopsis. `--base` is optional and has no default. **Change:** add `--on` to the synopsis. By default, resolve `--base` to `git merge-base <commit> <main branch from the config>` and store the full commit, so the reviewer always has a range and nothing is a placeholder.
7. **v1 has too many commands.** There are twenty, nineteen without `mcp`. `verify-*`, `evidence add`, `archive`, `presence`, `doctor` and `mcp` are useful, but none of them is what takes the operator out of the relay. **Change:** v1 is `init`, `post`, `claim`, `reply`, `note`, `decide`, `inbox`, `wait`, `show`, `thread`, `brief` and `render`. Evidence stays a flag on `post` and `reply`. Add the rest once the trial shows they are needed. `verify-merge` follows soon after, for check 12.
8. **A prompt cannot set `HANDOFF_ROLE` in Claude Code.** Each Bash call starts a fresh shell (**Source**: Claude Code's Bash tool definition, "Shell state (env vars, functions) does not persist"). **Change:** the operator sets the variable at launch, as in `HANDOFF_ROLE=implementer claude`. Variables of the CLI process do reach its Bash tool: `CLAUDE_CODE_SESSION_ID` and `CLAUDE_PID` are visible (**Run**, A4). The `env` key in project settings would also work, but it applies to every Claude session in the project, so it cannot tell two Claude sessions apart (**Source**: settings reference). `--as` stays, and it must agree with `HANDOFF_ROLE` when both are set.

### §7 Waiting, per CLI. Partly right; details in section 3. One change to change detection.

**Drop `fsnotify`. Poll `stat` once a second.** On macOS, `fsnotify` uses kqueue, not FSEvents (**Source**: fsnotify v1.9.0 README, "kqueue | BSD, macOS | Supported", "FSEvents | macOS | Needs support"). Its README warns that Spotlight and backup tools produce spurious events on macOS. kqueue also watches a file descriptor, so a replaced file stops being watched until the watch is added again. The proposal already needs a poll as a fallback. A one-second `stat` of `channel.jsonl` (size and mtime) meets check 8's two seconds by itself. It means one code path, no dependency and no lost-watch cases. One `stat` call a second costs nothing measurable (**Inferred**).

### §8 Priming and presence. Agree on priming. Presence goes stale.

1. **Stale presence.** `wait` writes "waiting" and then dies with the session (closed terminal, SIGKILL), and the file still says "waiting". **Change:** store the PID and start time of the `wait` process. A reader treats "waiting" as true only if that PID is alive (`kill -0`). For activity, Claude Code's `PostToolUse` hook fires on every tool call, which gives a heartbeat at no cost. The operator's machine already runs a program on every hook event (**Source**: `~/.claude/settings.json` runs `cc-status` on SessionStart, Stop, SessionEnd, PreToolUse, PostToolUse, Notification, PermissionRequest and others).
2. **The Stop hook should have a different job.** See section 3: block the end of a turn only when the role has no live `wait`, not when the inbox is non-empty.
3. **Who sends each notification.** Name the process for each alert. An entry to the operator is notified by the posting process. A stall is notified by the waiting processes (§3 above).

### §9 Safety rules. Agree, one correction.

"Git is read only" is not literally true of `git merge-tree --write-tree`. It writes the merged blobs and tree into the repository's object store (**Run**, A3: loose objects went from 9 to 11 in a scratch repository; **Source**: git-merge-tree(1), "a new toplevel tree object is created"). The objects are unreferenced and `gc` removes them, so no harm is done, but the rule as written is broken. **Change:** run it with `GIT_OBJECT_DIRECTORY=<temp dir>` and `GIT_ALTERNATE_OBJECT_DIRECTORIES=<the repository's objects>`, then delete the temp dir. The repository's store stays untouched (**Run**, A3: 17 objects before, 17 after, one object in the temp dir). In a worktree, find the objects directory with `git rev-parse --git-path objects`.

One addition. "Never edit `.handoff/` by hand" can be partly enforced for Claude Code: `init` prints `"deny": ["Edit(/.handoff/**)"]`. Write path rules are accepted but never consulted, so the rule must be on `Edit`, and it does not cover writes made through Bash (**Source**: permissions page). It stops the accidental case, which is the likely one.

### §10 Per-project setup. Disagree with committing `.handoff/`.

The website project runs both sessions in one shared checkout. `git worktree list` shows a single worktree (**Run**, A5), and the channel itself says "the shared checkout is `2495b6f`" (**Source**: website `docs/proposals/IMPLEMENTATION_CHANNEL.md`, line 1405). Today that channel is untracked: the website project ignores all of `docs/` (**Run**, A5), so both sessions read the same file whatever branch is checked out. If `.handoff/` is tracked, it changes with every `git checkout` the implementer runs. The record the reviewer is waiting on rewinds, or jumps to another branch's version. Entries land in commits on feature branches. Merges then conflict inside `channel.jsonl`, and the hash chain breaks at the first conflict. And because the record changes after every post, committing it moves HEAD past the commit the request named. If the sessions ever use separate worktrees, a tracked `.handoff/` gives each its own copy, and the two never see each other at all.

**Change:** one record per repository, never on a branch. It lives in `.handoff/` at the main worktree's root (found with `git rev-parse --git-common-dir`), and it is gitignored. Back it up outside git, for example with a `handoff export` to a dated archive. This leaves "nothing of the channel exists outside the operator's Mac" as a known gap that a backup closes, instead of trading it for a record that breaks. One hazard remains: `git clean -fdx` deletes ignored files. `.git/handoff/` avoids that and is shared by every worktree automatically, but it bends the "writes go only under `.handoff/`" rule. That is the operator's call (question 1).

### §11 Lineage. Agree.

One edit: "Its 909 uncommitted lines are worth committing" is an action on another repository. Move it out of this spec.

### §12 Language. Agree with Go. See section 4.

### §13 Verification. Agree that only Run evidence closes a check. Fix one check and add eight.

- **Check 9 cannot happen as written.** On one machine nothing can post while the laptop sleeps, so no "entry posted meanwhile" exists. **Change:** "an entry posted just before sleep, or just after wake, is delivered exactly once; a timeout that falls during sleep returns on wake, with the gap recorded". Test the timeout arithmetic against an injected clock, and do one real sleep (`pmset sleepnow`) with the operator present.
- **Check 8**, if `fsnotify` is dropped: wakes within two seconds, by polling.
- **New checks:**
  13. An entry posted while no `wait` runs is delivered by the next `wait` at once. An entry posted between two Monitor arms is delivered by the second arm.
  14. The implementer switches branch in the shared checkout while the reviewer waits. The record does not change, and the reviewer's `wait` is unaffected.
  15. The same `post` command run twice gives one entry.
  16. A writer killed leaving no trailing newline: the next post refuses or repairs, and no good line is damaged.
  17. A truncated tail is detected.
  18. `decide` with no terminal on stdin is refused.
  19. A `wait` killed with SIGKILL shows as absent within one poll.
  20. `verify-merge` leaves the repository's object count unchanged.

### §14 Order of work. Disagree with the order.

The riskiest assumption is in §7: that each CLI can wait for hours and be woken reliably. Step 1 builds the record and eight commands before anything waits. **Change:** make step 1 a thin slice that works end to end:

- the record and the lock;
- `post`, `reply`, `inbox`;
- `wait` with its cursor.

Run it the same day in a real Claude Code session and a real Codex session in this repository (checks 8, 10 and 13). Then move this project's own channel onto it. The kickoff calls this the last project that needs the operator to relay, so the tool should carry its own build from the first day it can. After that come render, board, presence, notifications, `verify-*`, evidence and archive. The estimates look plausible.

### §15 Open decisions. Incomplete.

Item 5 is overtaken: the reviews now happen in `docs/reviews/`, from three sessions in this repository. My additions are in section 6.

### §16 Risks. Missing six.

| Risk | Mitigation |
|---|---|
| The record rewinds or splits with branches or worktrees | One gitignored record per repository (§10) |
| An agent records an operator decision | `decide` requires a terminal; a deny rule printed by `init` (§6.3) |
| Entries lost between waits | A per-role cursor (§6.1) |
| A session sits at a permission prompt, and presence says "working" | `init` prints allow rules for `handoff` (except `decide`); a Notification hook marks presence "needs operator" and alerts the operator |
| After compaction, the session forgets to wait again | A SessionStart hook on `compact` re-injects `brief`; a Stop hook checks for a live `wait` (section 3) |
| Idle waking costs tokens | Each wake is a model call with the whole context. A background `wait` costs nothing until it returns; prefer it to a Monitor that must be re-armed every 30 minutes |

### §17 Resolution log. Agree.

One line: the header says Codex's review goes at the end of the proposal, but the kickoff moved reviews to `docs/reviews/`. Point §17 there, so all review points are tracked in one place.

## 3. The CLI I run in: Claude Code 2.1.295

### How I would wait

I tested both mechanisms on this machine today, using a stand-in file instead of `handoff`.

- **Background Bash** works. A command run with `run_in_background` keeps running across turns, and the session is re-invoked when the command exits (**Source**: Bash tool definition). **Run** (A1): a 0.5-second `until` loop was armed at 18:50:39. A line was appended at 18:50:42.968, and the session was woken with output stamped 18:50:43.420: 0.45 s, mostly the poll interval.
- **Monitor** works, as §7 says, with caveats. Each stdout line becomes a notification, and lines within 200 ms are batched. A monitor lives 5 minutes by default and at most 30 minutes. It then sends one expiry notice and must be armed again. Monitors that emit too many events are stopped automatically (**Source**: Monitor tool definition). **Run** (A2): `tail -n0 -F` delivered entry 1. I stopped it, appended entry 2, re-armed, and appended entry 3. Entry 3 arrived; entry 2 never did.
- **What I would do:** run `handoff wait --for implementer`, without `--emit`, using `run_in_background`, and run it again each time it returns. That gives one wake per arrival, and the exit code says entry (0) or timeout (3). It is the same command the proposal gives Codex to run in the foreground. There is no Monitor rate limit and no 30-minute re-arm. Keep `--emit` with Monitor for a session that wants to see entries in the middle of a task; v1 does not need it.
- **A foreground wait holds the turn for nothing.** A foreground call times out after 2 minutes by default and 10 minutes at most (`BASH_DEFAULT_TIMEOUT_MS`, `BASH_MAX_TIMEOUT_MS`; **Source**: env-vars page). On timeout the command is not killed. It "moves … to the background instead of stopping it, unless the command starts with `sleep`" (**Source**: tools-reference page). **Run** (A6): a wait loop given a 5-second timeout was moved to the background with a task id. So `wait --timeout 25m` run in the foreground still works, but it blocks the session, and the operator cannot talk to it until it moves. In a local terminal session, background commands have no time limit (**Source**: tools-reference page). Unattended runs (`-p`, the SDK) get 30 minutes by default, but the design never uses those. The duty prompt should say "in the background".
- **MCP.** The proposal's claim is right: "An MCP tool call in the main conversation that is still running after two minutes moves to a background task instead of blocking the session … the result arrives as a task notification" (**Source**: MCP page). The threshold is `CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS`, default 120000. The tool timeout `MCP_TOOL_TIMEOUT` defaults to about 28 hours. The conclusion is backwards, though. Backgrounding does not make MCP a poorer fit; it makes an MCP `wait` behave exactly like a background Bash `wait`. For Claude Code, the MCP shim adds a server to maintain and nothing else. Keep it "later".
- **`ScheduleWakeup`** exists only in `/loop` dynamic mode, with delays from 60 to 3600 seconds (**Source**: its tool definition). It is a poll, not a wait. Not recommended.

### What I can and cannot do in a long session

- **I stay interactive while waiting.** I kept working, and running commands, while the background wait and the monitor were armed (**Run**, A1 and A2). An event that lands during a turn arrives after that turn (**Inferred** from the Monitor definition: "you keep working and notifications arrive in the chat").
- **I cannot wake myself.** Without a running process or a message from the operator, nothing happens. I also cannot keep environment variables between Bash calls (§6.8 above).
- **Compaction can drop the duty.** Long sessions are summarised automatically (**Source**: this session's system prompt). A summary can lose exact instructions: the wait command, the role, the verdict words. The background `wait` survives as a process (**Inferred**), but my knowledge that I must start it again may not. Two hooks fix this, and neither lets the tool drive anything. The first is a `SessionStart` hook with the `compact` matcher that runs `handoff brief --for $HANDOFF_ROLE`. The docs describe this exact use: "Use a `SessionStart` hook with a `compact` matcher to re-inject critical context after every compaction. Claude Code adds plain text your command writes to stdout to Claude's context" (**Source**: hooks guide). The second is the Stop hook below.
- **The Stop hook needs a different job from the one §7 gives it.** It fires "when Claude finishes responding", which is every turn, not only when the session ends (**Source**: hooks page). It blocks with `{"decision":"block","reason":…}`, or with exit code 2 and a message on stderr. Its input carries `stop_hook_active`. Claude Code ends the turn anyway after eight blocks in a row, and the count resets when Claude calls a tool (**Source**: hooks page). §7's version keeps the session working whenever the inbox is non-empty. That blocks every turn end while the operator talks to the session about something else. It also spins on items the session cannot act on, such as items awaiting the operator. **Change:** block only when this role has no live `wait` (a PID check on presence), with the reason "start `handoff wait` in the background". Since the role lives in `HANDOFF_ROLE`, the hook can tell two Claude sessions in one project apart (**Inferred**: hooks are child processes of the CLI and inherit its environment).
- **A permission prompt stops an unattended session.** It waits until the operator answers, and presence still says "working". Claude Code's `Notification` hook fires with the matcher `permission_prompt` after a prompt has waited about six seconds, and with `idle_prompt` 60 seconds after a turn ends with no input (**Source**: hooks guide). The hook cannot decide anything, but it can run `handoff presence --event needs-operator`, which alerts the operator. Better still, avoid the prompt: `init` prints `"allow": ["Bash(handoff *)"]` and `"deny": ["Bash(handoff decide *)"]`. Deny is checked before allow, and "an allow rule can't carve an exception out of a deny rule" (**Source**: permissions page). This session runs with permission prompts bypassed. The operator's user settings set only `defaultMode` (**Source**: `~/.claude/settings.json`), so a session started differently would prompt.

### When the machine sleeps

All of this is **Inferred** except where marked. I did not sleep the operator's Mac.

- Every process stops: Claude Code, the background `wait`, any monitor. Nothing posts during sleep, so nothing can be missed during sleep. The risks are at the edges.
- **Go's timers may not count sleep.** "On some systems the monotonic clock will stop if the computer goes to sleep" (**Source**: Go 1.25.3, `src/time/time.go`, line 54). A `time.After(25 * time.Minute)` may then fire only after 25 minutes of awake time. Delivery is unaffected, because the next poll after wake still sees new entries. The proposal's "the timeout is wall-clock" would be false, though. **Change:** on each poll, compare wall-clock readings (`time.Now().Round(0)` strips the monotonic part), and record any jump much larger than the poll interval as a sleep gap.
- **The Monitor's 30-minute life** may or may not count sleep; its definition does not say. A background `wait` has no such cap, which is one more reason to prefer it.
- **An API request in flight at sleep** will fail on wake. Whether Claude Code retries on its own, or shows an error that needs the operator, I have not tested. It belongs in check 9 with the operator present.

## 4. Language

Go. Everything this tool does is in Go's standard library: files, `flock` through `syscall`, JSON, SHA-256, `os/exec` for git, and time. Drop `fsnotify` (§7) and write the config as JSON instead of TOML, and the tool has no third-party dependency at all: nothing to audit, nothing to update. If the operator wants comments in the config, TOML costs one well-known dependency. Rust's strengths are no garbage collector, speed, and a type system that can encode states. None of them matter for a program that appends a line and checks a file once a second, and a Rust version would still pull in `serde`, `serde_json` and probably `clap`. Go builds in seconds, which matters when an agent iterates. Both toolchains are installed (**Run**, A4: Go 1.25.3, Rust 1.97.1). Rust is a sound choice only if the operator wants to learn it; nothing in the design depends on the language. On the name, I would keep `handoff`. The repository and every document already use it, and `relay` suggests the tool forwards messages, which it does not.

## 5. If I were the implementer, and if I were the reviewer

**As the implementer, I would build first:**

1. The record: an append under `flock`, with the torn-tail check, sequential ids, the body inside the line, the `prev` hash and the head file.
2. `post`, which resolves commits; `reply`, which checks the exact commit and the addressed role; `inbox`; and `wait`, with the per-role cursor.
3. Two real sessions in this repository on the same day, then this project's channel moved onto the tool.

**I would push back on:**

- committing `.handoff/` (§10);
- `fsnotify` (§7);
- `entries/` as separate files (§5.1);
- the `archive` command and all twenty commands in v1 (§6.7);
- the Stop hook as "keep working while the inbox is non-empty" (section 3);
- `mcp` before a session asks for it.

**As the reviewer, I would refuse to approve without Run evidence of each of these:**

- A writer killed mid-append, leaving no newline: the next post refuses or repairs, and no good line is damaged.
- Fifty processes posting twenty entries each, at the same time: 1,000 lines, sequential ids, an unbroken chain.
- The lost-wakeup case: an entry posted while no `wait` runs is delivered by the next one.
- `reply` refused for a different commit, for the wrong role, and against a superseded request (check 5).
- `verify-merge` leaving the object count unchanged, and matching Codex's finding on the website project's `9dd9a85`.
- `decide` refused when stdin is not a terminal.
- No network: `go list -deps ./...` shows no `net` package. No writes outside `.handoff/` and the deleted temp dir: a test that compares the tree before and after.
- The duty prompts and the hook scripts, reviewed as text. They are what keeps a session on duty, so they deserve the same scrutiny as the code.

## 6. Open questions for the operator

1. Where does the record live: gitignored at the main worktree root (my recommendation), committed (the proposal), or in `.git/handoff/`? And do your sessions ever use separate worktrees?
2. May `init` print allow rules so that sessions on duty can run `handoff` without permission prompts, plus a deny rule for `handoff decide`?
3. Will you always type `decide` yourself, in a terminal? If so, requiring a terminal costs you nothing.
4. Does the gate reviewer ("reviewer 2") need to be in v1, or only two roles?
5. Should an APPROVE notify you, or only entries addressed to you?
6. What does the evidence label "Live" mean?
7. Sequential ids (`#42`) instead of time-based ones?
8. Will you run check 9 (sleeping the Mac during a `wait`) with the sessions open? An agent cannot do it without sleeping your machine.
9. Once the thin slice works, should this project's own channel move onto it before the rest is built?

## Appendix: Run evidence

All runs were in this Claude Code session on 2026-10-09, between 18:50 and 18:57 UTC, run by Claude Opus 5.5. Scratch files were in the session scratchpad and are not kept.

**A1. A background wait wakes the session.**

```
$ (run_in_background) mkdir -p $S && : > $S/a.jsonl && echo "armed $(date -u +%H:%M:%S)"; \
    until [ -s $S/a.jsonl ]; do sleep 0.5; done; echo "woke <utc ms>"; cat $S/a.jsonl
$ echo '{"id":1,"to":"implementer","posted":"<utc ms>"}' >> $S/a.jsonl      # separate tool call
-- task notification: completed, exit 0; output:
armed 18:50:39
woke 18:50:43.420
{"id":1,"to":"implementer","posted":"18:50:42.968"}
```

**A2. A Monitor re-armed between posts loses the entry posted in the gap.**

```
Monitor arm 1: tail -n0 -F $S/b.jsonl        (timeout 300000 ms)
append {"id":1,"posted":"18:51:10.781"}     -> event delivered: {"id":1,"posted":"18:51:10.781"}
TaskStop arm 1
append {"id":2,"note":"posted while no monitor armed","posted":"18:51:15"}
Monitor arm 2: tail -n0 -F $S/b.jsonl
append {"id":3,"posted":"18:51:21"}         -> event delivered: {"id":3,"posted":"18:51:21"}
entry 2: never delivered
```

**A3. `git merge-tree --write-tree` writes objects, unless pointed at a temp directory.**

```
# scratch repo: base a/b/c; branch x changes line 1; main changes line 3
loose objects before=9 after=11 tree=6f5615b91ef98df5480e1695d329ab09a1e55c71   (git count-objects -v)
git cat-file -t 6f5615b -> tree; working tree changes: 0

# branch w adds a file; merge-tree run with
#   GIT_OBJECT_DIRECTORY=<temp> GIT_ALTERNATE_OBJECT_DIRECTORIES=<repo>/.git/objects
exit=0 tree=a6c0b4d4dee24f9919de0b163afd0fcefb8f00cf repo loose objects before=17 after=17; objects in temp dir: 1
```

**A4. Environment of Claude Code's Bash tool.**

```
$ claude --version            -> 2.1.295 (Claude Code)
$ git --version               -> git version 2.55.0
$ go version                  -> go version go1.25.3 darwin/arm64
$ rustc --version             -> rustc 1.97.1 (8bab26f4f 2026-07-14)
$ env | cut -d= -f1 | grep -i claude   (names only)
  CLAUDECODE CLAUDE_CODE_SESSION_ID CLAUDE_PID CLAUDE_CODE_ENTRYPOINT CLAUDE_EFFORT ... (10 names)
$ [ -t 0 ] && echo tty || echo "not a tty"   -> stdin: not a tty; stdout: not a tty
```

**A5. The website project's checkout.**

```
$ git -C ~/website worktree list
~/website b770f76 [integration/sync-main]
$ git -C ~/website check-ignore -v docs/IMPLEMENTATION_CHANNEL.md   # any path under docs/
.gitignore:55:docs/	docs/IMPLEMENTATION_CHANNEL.md
```

**A6. A foreground command that hits its timeout moves to the background.**

```
$ (foreground, timeout 5000 ms) until [ -f $S/never-created ]; do sleep 0.5; done
-> "Command did not complete within its 5s timeout and was moved to the background (ID: b4cg22w8x)."
TaskStop b4cg22w8x
```

## Sources

The Claude Code documentation pages below were fetched from code.claude.com on 2026-10-09 by a helper agent of this session. The quotes in this review are as it returned them.

- Hooks reference (Stop, SessionStart, SessionEnd, Notification): https://code.claude.com/docs/en/hooks
- Hooks guide (re-inject context after compaction; notification matchers): https://code.claude.com/docs/en/hooks-guide
- MCP, automatic backgrounding of long tool calls: https://code.claude.com/docs/en/mcp#automatic-backgrounding-of-long-tool-calls
- Environment variables (`BASH_DEFAULT_TIMEOUT_MS`, `BASH_MAX_TIMEOUT_MS`, `MCP_TOOL_TIMEOUT`, `CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS`): https://code.claude.com/docs/en/env-vars
- Tools reference (foreground timeout, background time limits): https://code.claude.com/docs/en/tools-reference
- Permissions (rule syntax, deny before allow, Edit path rules): https://code.claude.com/docs/en/permissions
- Settings reference (`env`): https://code.claude.com/docs/en/settings-reference#env
- Claude Code's Bash and Monitor tool definitions, as loaded in this session.
- Go 1.25.3 `src/time/time.go`, line 54; fsnotify v1.9.0 `README.md` (Go module cache); git-merge-tree(1) from git 2.55.0.
