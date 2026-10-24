# GPT-6 independent review, round 1

Model: GPT-6, as identified by this session's instructions. An exact deployment variant is not exposed to me.
CLI: Codex. The installed executable reports `codex-cli 0.162.0`. This session exposes `exec_command` and `write_stdin`; the executable's version does not establish which client hosts this conversation.
Date: 2026-10-09.
Role: unassigned.
Proposal reviewed: `docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md` at repository commit `a913f131f900ddee77f879ef247e64cbb06114a0`.

I read `CLAUDE.md`, the proposal and `docs/CHANNEL.md` in that order and in full. I have not read another agent's review. No implementation, scaffolding or dependency installation was done. The findings below concern the proposal, not a working tool.

## 1. The heart, in my own words

The operator starts and primes two independent agent sessions. The agents exchange requests and exact-commit verdicts through a local record, then wait for each other without the operator carrying messages. The tool validates those exchanges and preserves their bodies and evidence. It neither calls models nor chooses the next piece of work. The operator keeps authority over decisions and release actions.

## 2. Section by section

### §1. The heart of it — agree; missing a delivery contract

Keep the separation between agent judgement and transport. Define success as delivery of outstanding work after a restart, as well as prompt delivery to an already waiting session. A watcher that sees only future appends does not remove manual relaying reliably.

### §2. Principles that do not change — agree; qualify integrity

Keep exact-commit verdicts and operator authority. A hash chain can reveal a changed interior line when its successor is retained. By itself it cannot reveal a changed final line, a removed suffix, or a fully recomputed chain. State what integrity is promised and what anchors it. Also hash entry bodies; the example protects an evidence file but only names the body's path. These are deductions from the proposed format (**Inferred**).

### §3. A working day — missing revision and restart rules

Define how the implementer answers QUESTION, how a new request supersedes an old one, and how a restarted reviewer resumes a claim. An old request may still receive an exact-commit verdict, but that verdict must not close a newer revision. Twenty minutes unclaimed is a reason to check a session; it is not proof that the session died. A claimed request can stall too.

Reviewing must not require changing the implementer's active checkout. Use read-only Git inspection or a separately prepared worktree. If worktrees are used, both sessions need one explicit channel location; `.handoff/` in each checkout would otherwise produce two records.

### §4. What the tool is, and is not — agree; resolve the notification lifetime

A reducer that computes item status is necessary. It need not drive agent work. The distinction should be stated so that “no state machine” does not rule out a precise protocol.

Something must remain running to send a future stall notification. If both agent sessions and all `wait` processes stop, a command-line binary cannot notice twenty minutes passing. Either allow a passive watcher started by the operator, which only reads status and notifies the operator, or promise notifications only while another command is running. This follows from the proposed process lifetime (**Inferred**).

### §5. The record — changes required

The record needs a schema version, required fields by kind, immutable body hashes, and an explicit reference between revisions. Store evidence under unique or content-derived names. Reusing `evidence/W31/lint.log` must not overwrite evidence cited by an earlier entry.

Specify the write order across body, evidence, log and rendered files. A durable log entry must not refer to a missing body. Render failure after a successful append must return the committed entry's receipt and a clear projection warning, so a retry does not create a second request. Generated files should be replaced atomically and carry the record position they represent. They can lag or be damaged; “cannot go stale” is too strong.

One write plus sync does not define recovery from every interruption. In particular, a partial final line without a newline can swallow an appended JSON repair. Specify how readers find a repair past malformed bytes, which bytes `prev` hashes, and what remains usable before repair. Preserve damaged bytes. Halt ordinary posting on corruption rather than silently extending an uncertain history.

Time-ordered unique IDs prevent ID collisions; they do not deduplicate a retried command that generates a fresh ID. Add a caller-supplied operation key. Reusing it with different content must fail. These findings follow from the format and command descriptions (**Inferred**).

The record is never split, so readers should resolve threads from `channel.jsonl`. Markdown archives are views, not another authority to scan.

### §6. Commands — missing a few necessary semantics

`post` needs a specified `kind`, a stable retry key and an explicit revision link. Resolve each supplied Git ref once, require a commit object, and store the full object ID. Validate base ancestry when base means the start of the reviewed change. Define `--on` in the command syntax rather than only in its refusal column.

`claim` needs a restart or reclaim path. `reply` must target a request, not just any known entry. Define whether a claim is required and how a second or conflicting verdict is handled. Keep old verdicts in history even when a request is superseded.

Give a non-verdict response to QUESTION a link to the question or request. An unlinked note does not tell the inbox which response is still owed. Define whether APPROVE makes an item review-complete or fully closed, and whether an operator decision is also required.

`wait` needs a replay and acknowledgement rule. It should scan outstanding entries before blocking, watch before its final scan, and rescan after wake-up. A proposed `--after <entry-id>` could support explicit replay, but a cursor must not advance merely because text was printed. A crash before the agent acts must leave work recoverable. Notes and operator decisions also need a defined receipt rule.

For `verify-merge`, check the actual ordered parents, reject conflicts and unsupported parent counts, and compare the resulting tree. Record Git version and merge options. Tree equality is evidence about a merge result, not approval of its behavior.

### §7. Waiting, per CLI — partly agree; evidence is narrower than the claim

An open Codex turn can wait on a shell process. My short probe below demonstrates yielding and collecting completion. It does not prove a 25-minute wait, indefinite duty, interruption cleanup or recovery from sleep.

The `/tmp/codex-channel-listener-*.py` reference describes a custom listener, not a Codex guarantee. Preserve the cited source if it matters, and distinguish its idle policy from the host's limits. Add a compatibility table with installed version, host client, wait mechanism, cancellation behavior and tested duration. Treat the Claude Monitor limits as unverified in my review; I did not run a Claude session.

Keep filesystem events as a latency hint and polling as a fallback. The record must decide what was delivered. Timeout accounting and replay after sleep need separate tests. My Codex findings are in §3 of this review.

### §8. Priming and presence — agree with priming; changes to presence

Print the duty prompt; do not suggest that printed text can set the environment of an already running session. Prefer explicit `--as` in examples, or have the operator set `HANDOFF_ROLE` when starting the session. `--for` should select the inbox being read, not silently confer the posting role.

A presence file is a last observation. Include session identity, last update and expiry, and report stale presence as unknown. Turn completion is not session death. A hard crash may run no hook. A waiting process cannot keep proving the model session is alive while that model is working elsewhere.

Keep hooks optional and version-tested. Define one owner for stall timers and notification deduplication. Sleeping machines cannot send a timely local notification; report overdue work after resume.

### §9. Safety rules — agree; one literal contradiction

`git merge-tree --write-tree` leaves the index and working tree alone, but creates a tree object. It is not literally read-only Git. The official [Git manual](https://git-scm.com/docs/git-merge-tree) establishes that behavior (**Source**). Prefer a scratch object store under `.handoff/` so the stated write boundary holds. Do not quietly weaken that boundary.

Role checks enforce a declared protocol, not identity. This is already acknowledged. Include the declared role and session identity in receipts and briefings so misuse can be inspected. Keep a verdict separate from both a claim and an operator decision.

### §10. Per-project setup — agree; reconcile the write boundary

§6 says `init` writes lines for `CLAUDE.md` and `AGENTS.md`; §10 says it prints them. Choose printing, consistent with writes staying under `.handoff/`. Do not edit either instruction file automatically.

Separate durable records, bodies and evidence from runtime files. Committing presence, a lock file and constantly regenerated views may create noise and conflicts. Define the recommended ignore rules. An approved code commit should not become obsolete just because the approval entry was later committed. Decisions must keep naming the original reviewed commit.

### §11. Lineage — agree

Start without ported code. Do not make committing Tripartite's unrelated work part of this build. Its history may inform decisions without becoming a dependency. I did not need to read or change Tripartite for this review.

### §12. Language — Go; remove unsupported precision

Go fits the intended workload. The claimed one-day Rust penalty has no basis shown here. Remove that estimate or label it an assumption. Keep the final choice with the operator. My reasoning is in §4 of this review.

### §13. Verification — agree; add failure paths

Retain the listed checks and the requirement for saved Run evidence. Add: retry after a committed append but lost receipt; body or evidence edits; crash between file publication and log append; malformed final bytes without a newline; a restart with an outstanding claim; an old APPROVE arriving after a new revision; post during watcher setup; interruption and orphan-process cleanup; and both sessions dead during a stall interval.

Test `wait --emit` replay, timeouts and stdout flushing separately from one-shot wait. Test tracking policy and a shared channel across worktrees. The website trial remains after release 1, with the operator present. None of my probes closes a §13 acceptance check.

### §14. Order of work and effort — change the first gate

Before broad command coverage, settle the event schema, repair rules, retry key and reducer examples. Then build one vertical path: request, wait, claim, exact-commit verdict, recipient receipt and restart replay. Test it in a disposable local repository before the website trial.

Bring evidence preservation into that path. Defer Markdown archiving and MCP until the path works. Treat the day counts as estimates, conditional on resolving these gaps. Do not begin until the operator chooses language, name and roles.

### §15. Open decisions for the operator — update

The review location is now settled by the kickoff. Add the passive watcher decision and the meaning of item closure. Record tracking policy with a distinction between durable and runtime files. The questions at the end of this review ask for those choices.

### §16. Risks — incomplete

Add lost or repeated delivery, stale claims, record/body divergence, timer ownership when both sessions stop, and separate records across worktrees. Each needs an explicit behavior and a Run check. Prompts and presence alone cannot establish continued duty.

### §17. Resolution log — agree

Append resolutions with the review file and point being answered. The kickoff now places independent reviews in `docs/reviews/`; link those files instead of copying this review into the proposal's Codex tail. Preserve disagreements until the operator resolves them.

## 3. The CLI I run in

### Run: short probes in this session

Run by GPT-6 through this session's shell tools on 2026-10-09, in `~/handoff`:

| Command or tool call | Observed result | What it establishes |
|---|---|---|
| `command -v codex` | `~/.nvm/versions/node/v24.11.1/bin/codex` | Which installed executable the shell finds. |
| `codex --version` | `codex-cli 0.162.0`, exit 0 | The proposal's 0.160.0 is not the currently installed version. |
| `codex --help` | Lists `resume`, `mcp` and interactive CLI options; exit 0 | Those commands exist. No indefinite-duty guarantee is stated in the help. |
| `exec_command(cmd="sleep 3", yield_time_ms=1000)` | Returned after about one second with session ID `30228` and no output | A shell command can remain running after the initial tool return. |
| `write_stdin(session_id=30228, chars="", yield_time_ms=1000)` | Returned exit 0 with no output | I can collect that process's completion in a later tool call. |
| `codex mcp add --help` | Shows `<NAME> -- <COMMAND>...`; exit 0 | The proposed stdio registration syntax is supported. I registered nothing. |
| `git --version` | `git version 2.55.0`, exit 0 | The local Git version. |
| `git merge-tree -h` | Shows `--write-tree`; help exits 129 | The option exists. I performed no merge verification. |
| `date -u '+%Y-%m-%d %H:%M UTC'` | `2026-10-09 18:48 UTC`, exit 0 | UTC date and time for the probe batch. |

These are embedded review evidence, not the `evidence/` artifacts required to close implementation checks.

### Source: current official OpenAI documentation

The [notification documentation](https://learn.chatgpt.com/docs/config-file/config-advanced#notifications) says `notify` runs an external program for the currently supported `agent-turn-complete` event. That is a turn observation, not a heartbeat or proof of absence.

The [hooks documentation](https://learn.chatgpt.com/docs/hooks#stop) now describes Codex `Stop`, `SessionStart`, `SessionEnd` and `Interrupt` hooks. A synchronous Stop hook can request continuation. An Interrupt hook cannot prevent interruption or restart the turn. A completed background hook waits for the next user turn if no turn is active; it does not wake an idle agent. These are documented possibilities, not hooks I enabled or tested here.

The [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference) documents automatic compaction controls and a default 60-second MCP tool timeout, overridable per server. It also says project-local configuration cannot set `notify`. The proposed MCP wait therefore needs a tested timeout setting, and notification setup belongs in the user's configuration.

### Inferred: how I would actually wait

While explicitly on duty, I would start one `handoff wait --for reviewer --timeout 25m` shell process, retain its returned process-session ID and collect it with `write_stdin`. Tool waits should yield in short intervals so I can remain responsive. A shell timeout can be long without making each tool call block for that whole duration. I would process returned IDs, acknowledge them under the agreed protocol, and re-arm on timeout. This is a proposed use of the demonstrated shell interface; `handoff` does not exist yet.

I can continue a live turn, use the record to recover context after compaction, and use `brief` when the operator restarts a session. I cannot promise unlimited session duration or guarantee recovery from a closed client, service failure, or exhausted limits. Keeping a shell process running does not by itself re-invoke a model after its turn ends. Interruption cleanup needs a real test; the short probe did not test typing or cancelling.

On machine sleep, local execution pauses. On resume, the tool must first rescan the durable record, compare an explicit wall-clock deadline and report any observed gap. If work exists and the deadline also expired, deliver the work before treating the call as an empty timeout. Clock adjustment must not be mistaken for proof of sleep. I did not put the operator's machine to sleep or validate this behavior; it remains **Inferred** and subject to §13 Run evidence.

## 4. Language

I recommend Go. This tool spends its time validating small records, coordinating file access, waiting and asking Git questions. Straightforward code and a small dependency set matter more here than memory-level control. Go is a reasonable fit for that work and for a single locally installed binary. Rust also fits, but I would choose it if the operator explicitly values learning it or wants its type system for protocol modelling. Correct recovery and delivery rules matter more than either choice. I would not attach a fixed schedule penalty to Rust without measuring the actual team and build.

## 5. If I were assigned either role

**If I were the implementer:** I would first write down event examples and their inbox/board results. I would then build the smallest durable exchange, including stable retries, body hashes and recovery after restarting a recipient. I would push back on automatic instruction-file edits, guaranteed stall notifications without a living observer, archiving before delivery works, and the claim that one write plus sync settles crash recovery. I would keep agents and their work loop outside the tool.

**If I were the reviewer:** I would refuse implementation approval without saved Run evidence for concurrent writers, retry after lost receipt, partial append recovery, immutable bodies/evidence, stale-revision verdicts and replay after interruption. Before claiming that the operator no longer relays messages, I would need a real exchange in both manually started clients, including a fix and re-review, plus an agreed restart procedure. I would also check that the exact approved commit remains the decision target when branches or channel commits move. The website acceptance trial would happen only after release 1 and with the operator present.

## 6. Open questions for the operator

1. Go or Rust? I recommend Go for this build; Rust if learning it is an explicit goal.
2. `handoff` or `relay`? I recommend keeping `handoff`, which the current documents already use.
3. Who implements, who reviews continuously, and what gates require the third agent?
4. May the operator start a passive local watcher that only reads status and sends operator notifications? If not, should the promise be limited to notifications while an agent's wait process or another command is alive?
5. Should durable records and evidence be committed while presence, locks and other runtime files are ignored? I recommend that split, with a documented backup option for projects that ignore the record.
6. Does reviewer APPROVE close an item, or does each item also require the operator's recorded decision? The board, archives and notifications need one answer.
7. Will the two agents share a working checkout or use separate worktrees? Separate worktrees need one agreed channel root, while a shared checkout needs a rule against moving it during another agent's work.
8. Are local macOS notifications sufficient, including overdue notifications after the machine resumes, or is another operator notification method required?
