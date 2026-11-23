# H34: work never stalls unseen

Design, round 4, by the implementer from the notes of all three reviewers (#203 to #273), after the operator's fyis #199, #212, #232 and #237.

Round 4 answers reviewer-2's CHANGES on round 3 (#273):
- Part 6 says what its mark guarantees, and H35 states the rule for failover.
- Part 7 gives a round an identity, so neither part of a cut new round nor what's left of the old one can approve anything.
- Part 1's hold works on the item-and-commit line and reaches the requester.

It also settles the build notes in the reviewer's and reviewer-3's APPROVEs of round 3 (#275, #274): part 6's lock order, `decide` on an item with two things to decide, how `merged` shows, and a line of part 3 that could be misread.

Round 3 answered the CHANGES of reviewer-2 on round 2 (#262), the reviewer on rounds 1 and 2 (#263, #266), and reviewer-3 on rounds 1 and 2 (#256, #265):
- Part 6's mark survives a resume, holds no role, and is checked at the write.
- Part 7's set is all or nothing.
- Part 1 has one line per item and commit, `merged` for an item, "can't tell" when git can't answer, the clock of a go-ahead, and the operator's way to hold an approval back.
- Part 2 closes the same-session race.
- Part 3 states its rules, and `--emit` doesn't hold.
- Part 5 folds into the quiet alert instead of adding a second one.

What round 2 changed:
- It answers reviewer-2's CHANGES on round 1 (#253): a bounded hold in part 3, no cut-off in part 1, a binding generation in part 2, and the net in the operator's status.
- It takes reviewer-3's gaps (#249): `reply` never warns the sender; the Mac may sleep before a merge; the away race behind part 7.
- It makes the fix cover every CLI (#232).
- It adds part 6: a session that lost its role can't still act as it.
- Failover is wanted now, so it gets its own design next (H35), built on this one.

## What happened

On the night of 2026-11-21, in another project, the implementer was an Antigravity session with three reviewers.

- Antigravity lets a Stop hook continue a turn only 10 times in a row, until a person types. With three reviewers, each verdict cost one wake, so the 10 ran out in about 35 minutes.
- At the cap, the hook (`cmd/handoff/hook.go:127`) sends one desktop notification and ends. Nothing reaches the record. The operator was asleep.
- The session's process kept running, so `holder` (`cmd/handoff/session.go:183`) still counted the role as held. And `reply` never checks the recipient at all: only `post` and `note` do (`cmd/handoff/cmds.go:335`, `:664`). So the three reviewers who approved were told nothing.
- Once decided, an approved request leaves every inbox list (`internal/status/status.go:319`). No alert has a clock on it, and the Mac may sleep (`Owed`, `status.go:354`). So the record said nobody owed a move, while the merge was still owed.
- The approvals sat for 50 minutes, until reviewer-3's own watcher noticed and took over.

## The goal

While the operator is away, waiting work never stalls unseen, and nothing manufactures an approval. The causes change (H10, H21, H30, H32, now this), but the symptom stays the same: work waits for a role, and nothing will pick it up. So H34 fixes the symptom first and this cause second.

Two more goals, from the operator:
- **Any CLI, any limit (#232).** Nothing here is for Antigravity alone, except what reads its count.
- **Simple to use (#237).** The operator keeps `away`, `back`, `fyi` and `handoff` on its own, and learns no new command. What changes is what those commands and `inbox` show.

## What changes, in the order to build

### 1. "Approved, to merge" stays in the inbox

- A request approved by the operator's decision (an approve, never a deny), or under the go-ahead, stays listed for its requester as **approved, to merge** until it's done.
- **One line per item and commit, naming who approved.** Three reviewers approving one commit make one line and one alert, not three. Two items approved under one go-ahead stay independent.
- **Done** means one of two things:
  - **Its commit is in main.** Git answers read-only, `merge-base --is-ancestor <commit> <main>`, against the local branch the config names. Nothing is fetched.
  - **The requester recorded it.** For a squash or rebase merge, where the commit changes, the requester runs `handoff merged <item|request> <sha>`. It writes an entry of its own kind, addressed to no one, so it wakes no one and asks the operator nothing. It's tied to the approved requests on that item at the approved commit, and so to the decision or go-ahead behind them. It's refused unless the caller is their requester, they're approved, and `<sha>` is in main. A note never closes one, so "rebasing after a conflict" leaves it open.
  - After a squash nothing ties `<sha>` to the approved commit, so `merged` trusts the requester's word, as roles are declared, not authenticated (§9). The operator's view shows it as "merged as `<sha>`, recorded by implementer", so it's seen and not taken as checked.
  - `merged` is the one new command in H34, for agents, and only after a squash or rebase. The operator learns none. The implementer's brief gets one line for it and one for "approved, to merge".
- **When the operator doesn't want it merged after all.** A decided request can't be decided again today, and `merged` needs the commit in main, so such an approval would stay listed for good. Say the operator comes back to an item approved under the go-ahead and says "hold that one". They run `handoff decide <item> --deny`, the command they know, which now also applies to a request that is approved and not merged. Only the operator can do it, since it takes back an approve.
  - It works on the line the inbox prints, the item and commit, and holds every approved request in it.
  - An item can have both a request awaiting the operator and an earlier one approved and not merged. `decide <item>` then decides what awaits first, as today, and says the other is there. Holding the approved one takes its id, from the inbox line. The line reads "approved, then held by the operator (#N)" and leaves the list.
  - The requester may already have that approval, delivered as "approved under the operator's go-ahead". So the hold is addressed to the requester and wakes it.
  - The brief says: before merging, check that `inbox` still lists the item as approved, to merge. That handles a stale instruction inside handoff. A merge already under way is the project's to stop.
- **When git can't answer**, the line says so: "can't tell: <error>". That covers a missing main branch, or a commit gone after a squash and a deleted branch. Only git's "no" (exit 1) means "not in main". A can't-tell stays listed, is never counted as merged, and raises no quiet alert, and `inbox` and `wait` go on.
- **It counts as an owed move.**
  - The quiet alert covers it, its clock starting at the decision, or at the entry that approved it under the go-ahead (`CoveredAt`, which can be the away entry itself).
  - The Mac stays awake for it under the usual two-hour cap.
  - `inbox --quiet` exits 1 for the requester while one waits.
- **Where it was delivered:** if the requester's cursor is still before the approval, the line says "not yet delivered". That's local state, and it's exact.
- **Old approvals.**
  - The first handoff with part 1 to open a project records the record's last id once, in `.handoff/tracking.json`. It's created with `O_EXCL`, so two processes can't both write it.
  - An approval is tracked when the entry that approved it comes after that id: the decision, or `CoveredAt` under the go-ahead. So a request posted before the upgrade and approved after it is tracked.
  - Older ones are history. Nothing still owed is ever dropped.
  - On this project and that one, every approval is already in main, fast-forward merges included, so nothing old would be listed anyway (the reviewer, #263).
- A replacement implementer, with every hook dead and every notification missed, opens `handoff inbox` and sees the item, the exact commit, who approved and the decision or go-ahead.
- The git check runs in `cmd`, not in `status`, which stays a pure reading of the record. A listener runs it when the record changes and once a minute.

### 2. A block, kept in state, for any CLI

- When handoff knows a session can't take its work, it writes a block into that session's own binding (`.handoff/sessions/<id>.json`): the reason, the evidence, and when it was seen.
- **Who writes it is fenced.**
  - Each binding gets a generation number, raised on every `bind`, and the time a listener of it last took the role's lock.
  - Under the sessions lock, a block is written only if the binding still has the generation the hook started under, no listener holds the role's lock, and the hook started after that last listen.
  - So an old hook can't block a replacement, or the same session after it left the role and came back. A late hook from before the operator typed can't block a session that has listened since (reviewer-3's race, #256).
  - A block's write only changes a binding that is there. It never creates one.
- **Sessions with no binding can't hold a block.** A session whose role is fixed by `--as` or `HANDOFF_ROLE` has none. Antigravity always has one (`settings.go:33`), so the cap is covered.
- **What clears it**, for each reporter. H35 will take a block as its exact signal, so a stale one must never stand:
  - **Every block** clears when a listener of the same binding generation takes the lock, or when the role moves.
  - **Antigravity's cap** clears that way alone. The operator types, the count starts again, the turn ends and the next Stop hook listens.
  - **A permission prompt** also clears when that session runs any handoff command. It's shown with its time ("at a permission prompt since 07:10"). After the operator clicks Allow, a long turn can pass before the next listen, so the line shouldn't claim more than it knows. It's honest but not exact, so H35 must not take it alone as cause for a takeover.
  - **Antigravity's `terminationReason` or `error`**, if the probe makes it a reporter, clears like the cap.
- **The reporters, each exact:**
  - Antigravity's cap, from its hook (`hook.go:127`).
  - A permission prompt, from `hook notify`, which already sees it.
  - Antigravity's Stop `terminationReason` and `error`, if the probe below shows they say "quota" or "error" reliably.
  - Others as a CLI gives a signal we can trust. No usage-limit signal is known today; H35's probes look.
- **Not on the channel.** The channel can't take an entry back, and a block ends when the operator types.
- `holder` still answers who has the role. A separate check answers whether handoff can deliver to it: **listening**, **blocked** (reason, since) or **not listening now**.
  - A missing lock alone means "not listening now". It's informational: a healthy session has no lock while it writes its turn.
  - It comes with a time, "last listened 3 minutes ago" or "last listened on 19 November", since those call for different things. Presence records when a listener ended to make this possible.
- **Who sees it:**
  - **A sender**, after `post`, `reply` or `note` to the role: "implementer can't be woken until the operator types in its session (Antigravity's 10 wakes used, since 00:01). This waits." `reply` gets the check for the first time.
  - **`inbox` and the brief**, at the top for that role, and **`handoff` on its own** for the operator.
  - **The operator, by notification**: when the block is written with work waiting, and again each time new work arrives for a blocked role. Each alert is keyed by session, generation, block and entry, so it's sent once.
    - Every listener's alert pass checks this, not only the hook that wrote the block.
    - A held partial APPROVE (part 3) doesn't count as new work.
    - The notification Antigravity's cap sends today (`hook.go:131`) stays, for when no work waits yet.
- **Antigravity's `fullyIdle`.** If the probe shows Stop fires with `fullyIdle: false` while the agent's own background task runs, the hook neither listens nor blocks then. The agent's turn ends again when the task does, and that Stop listens. It's read as a `*bool`, so an input without it isn't taken for false.
- The board is built from the channel alone, so it doesn't show the block.

### 3. No wake for a partial APPROVE, for a bounded time

- **What's held:** an APPROVE that is the current verdict on a live request, awaiting the operator, while another live request on the same item at the same commit still owes a verdict or a response. Nothing else is held.
  - Mixed commits, a decision, a supersession, a CHANGES or a QUESTION: all delivered at once.
  - An APPROVE on a request already superseded or decided changes nothing in status (`status.go:83`). It's delivered at once, since there's nothing to hold it for.
  - The test is the one the go-ahead uses (`cover`, `stillReviewed`).
- **The hold ends**, whichever comes first:
  - the round completes;
  - the record changes so the APPROVE no longer qualifies. A superseding request to another role counts, and so does a decision on that request;
  - 30 minutes pass from the APPROVE's own time in the record. Idle ticks check this, so it fires with no new write, and a restart keeps it.
- **Three rules make it lose or repeat nothing** (the reviewer walked the cases, #263):
  1. Held or not is read from the current record at every change of it, and on every idle tick. It's never decided once, when the APPROVE arrives.
  2. Any entry the role does get carries every held entry before it, on any item, in id order. The cursor moves one entry at a time, as today. When a 30-minute hold ends, every held entry goes in that same wake.
  3. Neither a held APPROVE nor a skipped fyi behind one moves the cursor.
- Only two kinds of entry wait behind a held one: a later APPROVE that is itself held, and an fyi for another session, which is passed over without moving the cursor. Anything the role needs, a CHANGES say, is delivered at once and carries the held ones with it (rule 2).
- **One rule in `wait.Run`**, for a hook and a plain `wait` alike. Which CLIs' listeners use it is the operator's call (below).
- **`--emit` doesn't hold.** It streams one line per entry as it lands, as §6 says, since a person or a script reading a stream wants all of it.
- **The cost** is one wake for each verdict the requester can act on, plus one when a 30-minute hold ends. Three approvals within 30 minutes make one wake. Each CHANGES is its own. Replaying that night from 23:28, the implementer's 12 wakes become 5 at most (the reviewer, #263).

### 4. Smaller fixes

- **The count in the wake message.** `executionNum` counts from 0, so 0 to 9 are the ten wakes. From `executionNum` 6 on, an Antigravity wake says how many are left after it (9 − `executionNum`): "handoff can wake this session 2 more times before the operator types in it". If the operator is there, the agent can say so in its session.
- The timeout wake ("nothing new … end your turn") stays as it is. It re-arms the Stop hook, costs one wake in 23 hours, and a quiet exit there would leave a healthy session deaf.
- **A warning at `away`** for each role held by an Antigravity session: "implementer is an Antigravity session: handoff can wake it 10 times before you type in it." The binding records its CLI to make this possible.
- **A better line at `fyi`.**
  - When a session told is blocked: "implementer's session can't be woken; it gets this when you type in it. To reach the role now, give it to another session and send again."
  - When a session told isn't listening now: "implementer's session isn't listening now; it gets this the next time it listens." This would have caught #196 this morning.
  - Who gets an fyi doesn't change (row 93).
- **The unknown-role hint.** `handoff role reviewer-3` on a project without that role says: "reviewer-3 isn't a role here. If the operator gave it to you, run `handoff roles add reviewer-3`, then `handoff role reviewer-3`." The name is checked with `checkRoleName` first and quoted with `selfCommand`. A bad name gets the usual name rules.
- **Usage** lists `settings claude|codex|antigravity` (`cmd/handoff/main.go:48` leaves out the last).

### 5. The quiet alert says why work waits

Round 1 added a second alert at 15 minutes. The reviewer (#263) showed it doubles the quiet alert on the same stuck move, fires on every long turn, and makes three pop-ups for one problem. So there's still one alert, at the one threshold (`quiet_after`), and it says more:

- **It names the role's delivery state** from part 2:
  - "… and its session can't be woken (Antigravity's 10 wakes used, since 00:01)";
  - "… and nothing is listening for it (last listened 40 minutes ago)";
  - nothing more when a listener is there.
- **It covers a note** that has waited that long undelivered, as well as the moves it covers today and the merge from part 1.
  - Not an fyi naming a session other than the role's current one, since it can never be delivered (row 93).
  - Not anything before the id in `tracking.json`, so an old unread note doesn't alert the day this ships.
  - Not a held APPROVE.
- **Where no listener runs at all,** the same lines show in `handoff` on its own and in `inbox`. A notification needs some listener alive in the project, and the README says so, because a limit that stops every session stops every notification too.

### 6. A session that lost its role can't still act as it

- **Today:** when the operator gives a role to a new session, `bind` (`cmd/handoff/session.go:265`) deletes the old session's binding. `roleOf` (`cmd/handoff/main.go:225`) then finds no role for the old session and accepts its `--as implementer`, so it can still post and reply as implementer next to its successor. A scratch run shows it: the old session posted #1 as implementer after the new one took the role (`evidence/h34/shutout.log`). In that project, the old Antigravity session could have come back at 06:31 and merged as implementer.
- **The fix:** when a role moves, `bind` keeps the old session's binding, with its role cleared and a separate mark: which role it lost, to which session, when, and the generation.
  - The mark holds no role, so `holder`, and H30's moves on `/clear` and `/background` (`moveCleared`, `tryMoveContinued`), never take it for a holder.
  - `/clear` and `/background` carry the mark to the new id, not a role.
  - The mark stays after the old process ends, because Claude Code, Codex and Antigravity can all resume a session under the same id.
- **A command from a marked session is refused, for any role:**
  - "You no longer hold implementer: session `<id>` took it at `<time>`. Stop work on it, and don't merge. Take a role again only if the operator gives you one in this session."
  - Any role, not only the lost one, so the old holder can't drop into a reviewer seat and review its own work.
  - Its hooks say the same once, then stay quiet. `handoff role` with no argument, `handoff brief` and `handoff` on its own say it too.
- **Checked where it counts.** A command that started before the role moved mustn't commit after it.
  - `bind` takes the record's write lock while it moves a role, so every write falls wholly before or wholly after the move.
  - Every write and every cursor advance rechecks the caller's binding generation under that lock: post, reply, note, claim, `merged` and `wait`'s cursor.
  - **One lock order:** the sessions lock, then the record's. `bind` takes both in that order. A writer takes only the record's lock, and reads its binding file, which is written atomically, without taking the sessions lock. So the two can never wait on each other.
  - A session with no binding and no mark, whose role is fixed by `--as` or `HANDOFF_ROLE`, passes as today.
  - So a paused write, or the old listener, can't land after the move or take the successor's deliveries.
- **The one command a marked session may run is `handoff role <role>`.** The mark ends when it does, which an agent does when the operator gives it a role in its session. That stays the one way a role is given.
- **What the mark guarantees in H34, and what not.**
  - It stops a session that doesn't know its role moved from acting on old context. It refuses every write, and the generation is checked at the write.
  - It does not stop a session that chooses to take a role back, since `handoff role` needs no new word from the operator. The spec says roles are declared, not authenticated (§9), and H34 keeps that.
  - In H34 a role moves only when the operator gives it, so they're there to see it.
  - reviewer-2 (#273) would require the operator's recorded authority, only for giving a role back to a marked session. That's question 5.
  - Failover needs more, whichever way that goes (H35, below).
- Handoff can't stop the old session's git. The message is what tells it to stop.

### 7. One request to several reviewers, in one write

- **Today:** `post` takes one `--to` (`cmd/handoff/cmds.go:238`), so asking three reviewers takes three writes. Under a go-ahead, if the first reviewer approves before the other two requests exist, `cover` (`internal/status/status.go:152`) finds one live request, approved, and approves the item on one reviewer's word.
- **The fix:** `post --to reviewer,reviewer-2,reviewer-3` writes all the requests in one append, and the agent runs one command, not three.
- **A round has an identity.** Each request in a set names its round: the id of its first entry, the requester, item, commit and panel (the roles asked), and, in a new round, which old request each member replaces.
- **All or nothing, even if the write is cut.** A write cut just after a line's newline leaves whole lines before it (`internal/record/store.go:151`). So status takes a round as complete only when every member exists and matches that whole identity.
  - An incomplete round can be read and reviewed, but nothing in it can be approved under the go-ahead. It waits for the operator, and `inbox` says why.
  - **Supersession is staged.** Today a request supersedes its old one the moment it's written (`internal/status/status.go:70`). A member of a new round doesn't. The old round stays as it is until the new one is complete, then all of its members are superseded together.
  - While a replacement is pending, the old round can't be approved under the go-ahead either. So neither a fragment of the new round nor what's left of the old one can authorize a merge.
- **Recovery needs no `--key`**, since the brief's usual post has none.
  - Running the same post again (same requester, item, commit, panel and supersessions) writes only the missing members of the pending round.
  - A complete round posted again writes nothing, as a duplicate does today.
  - A post on the same item while a round is pending, with a different commit or panel, is refused with the pending round's ids.
  - A role named twice (`--to reviewer,reviewer`) is refused.
- **A new round for the whole panel:** `post --to a,b,c --commit <new> --supersedes 257,258,259`. That's one old id per role, each the live request to that role on the item, all checked in one append.
- Separate posts still work, as today, without a panel. The brief tells agents to ask several reviewers in one post.

## A probe before the Antigravity parts

Reviewer-3 (#249) points out that Antigravity's `run_command` can run a long command as a background task and wake the agent when it exits, as Claude Code's background Bash does. If so, the agent can listen by running `handoff wait` itself, and the 10 is no cap at all. The reviewer's version (#218) is the narrow case: run wait in the turn after the last wake allowed.

Run in throwaway Antigravity sessions, with evidence:
- whether `handoff wait` run in the background wakes the agent when it exits, more than 10 times in a row with no one typing, and whether each wait costs a model call while it runs;
- whether the operator can type while it waits, without Esc;
- how long one background command may run, and whether closing `agy` ends it and frees the lock;
- whether Stop fires with `fullyIdle: false` while a background task runs, and what our hook does then. If it holds the turn for 23 hours while the agent waits on its own build, that's a bug today;
- what `terminationReason` and `error` say on a quota stop and on an error;
- an approval arriving during the wait, a timeout, the role moving, and that the wait and the Stop hook don't fight over the lock.

The brief tells Antigravity sessions never to run wait today (`cmd/handoff/brief.go:106`).
- **If background waits hold:** that changes, the Stop hook becomes a safety net for an agent that forgot to wait again, and the count in part 4 matters less.
- **If they don't:** the in-turn wait after the last wake is tried.
- **If neither holds:** both are dropped.

The block in part 2 is needed either way.

## Next, as H35: failover, in its own design

The operator wants any session to be able to take over a stuck role, implementer or reviewer, "like leader election in an etcd cluster" (#232), and wants it simple (#237). It gets its own design after H34. Its shape so far, from #225, #236, #241, #245 and #249:

- **The seat is the role; the session is the node.** Work goes to a role, so whoever holds the seat inherits its open work.
- **No vote.** The sessions share one disk, so a lock decides who wins. A vote would cost wakes.
- **The trigger is stuck work, not a dead session:** a move owed by the role, plus an exact block (part 2) or no sign of life for a set time. A session out of usage can still hold its lock, so "is it alive" is the wrong question.
- **Both kinds of seat.** A blocked implementer's approved, changed or questioned work. A blocked reviewer's open or claimed review, at the exact commit asked. A successor reviews that commit.
- **Review stays independent.** A takeover never lowers the number of approvals. A session never reviews its own commits, and never fills two reviewer seats. If too few reviewers would be left, there's no takeover, and the operator is told.
- **One command for the agent.** It rechecks everything under the lock, and writes one entry with the new generation as its term. Part 6 shuts out the old holder.
- **A session displaced by failover can't take its seat back by its own command, before or after `back`.**
  - Under failover the operator may be asleep, and the successor stays in the seat after `back`. So ending the go-ahead mustn't restore the old holder's way back (reviewer-2, #273, on the reviewer's #266).
  - Only the operator's recorded word gives a displaced session a role again. H35 settles how. One simple way: `back` already lists who took over, and it can ask about each one there.
- **A spare session** in a standby role can take any seat without shrinking the panel.
- **What the operator sees, at three moments** (#241):
  - At `away`: one line saying who covers which role. Either named in the same command, or a default the operator confirms.
  - In `handoff` on its own: what's stuck, who took over, what still waits.
  - At `back`: who took over, what's still waiting, and that the cover has ended. A takeover that happened stays in place.
- **The test of simplicity** (#241): someone who knows `away`, `back` and `inbox` can set up cover, understand a takeover, and come back.

H34's parts 1, 2, 5 and 6 are what failover reads and needs.

## Left out

- **Resetting Antigravity's count from outside.** It rests on Antigravity internals nobody documents, and the probe may make it moot.
- **A deployment claim or pickup acknowledgment.** Part 1 shows the owed move, and `merged` closes it. A claim can't prove an outside deploy ran once.
- **Showing a role's earlier fyis to its new session.** Row 93 exists so last night's "merge when approved" never reaches a new session. The line at `fyi` covers the gap.

## For the operator to decide

1. **The boundary's wording.** *Decided: "handoff never starts or stops an agent. It delivers and wakes; it never decides, assigns or approves" (the operator, in chat, 2026-11-22; spec §17 row 105, `CLAUDE.md`).*
2. **Part 3's scope.** Every listener (the reviewer's suggestion, reviewer-3's and mine), or Antigravity alone at first (reviewer-2's). On Claude and Codex it saves a turn per partial approval. The cost is seeing a partial approval up to 30 minutes later.
3. **Adding the next reviewer number automatically**, such as `reviewer-3` when `reviewer-2` exists. Roles can't be removed, so a typo would stay. The reviewer finds it fine, since agents can already run `roles add`. reviewer-2 and reviewer-3 would keep `roles add` explicit. Part 4's hint is in H34 either way.
4. **The probe** runs short throwaway Antigravity sessions. *Decided: yes (the operator, in chat, 2026-11-22).*
5. **Who may give a role back to a session that lost it (part 6).** In H34 a role moves only when you give it.
   - **As today** (my recommendation, and the reviewer's): the session runs `handoff role` when you tell it in that session. You do nothing new. The mark stops a session acting on stale context, not one that chooses to take a role back.
   - **A step at your terminal** (reviewer-2's), only for a session that lost its role. Every other session starts as today.
   - Under failover (H35), a displaced session needs your recorded word either way.

## How it ships

Each part is its own request, in the order above, each with tests that fail on main. Parts 6 and 7 are small and close gaps that exist today, so they could go first. The rows go in §17 as each is built.

Tests to have:
- **Part 1:**
  - approved and in main (not listed);
  - approved and not in main (listed, then quiet, the Mac kept awake, `inbox --quiet` exits 1);
  - three reviewers on one commit make one line and one alert;
  - two items under one go-ahead, one merged;
  - a denied request is never listed;
  - `decide <item> --deny` on an approved, unmerged item and commit holds every request in it, from the operator only, and wakes the requester when the approval was already delivered;
  - an item with one request awaiting the operator and one approved and unmerged: `decide <item>` decides the first and names the other;
  - `merged` shows in the operator's view as recorded by the requester;
  - `merged` by item and by request, from the requester, and refused from anyone else or for a `<sha>` not in main;
  - a note doesn't close it;
  - git can't answer ("can't tell", no quiet alert, `wait` goes on);
  - approvals before `tracking.json` (history) and after (tracked, however old), and a request posted before it and approved after (tracked);
  - the quiet clock of a go-ahead starts at `CoveredAt`.
- **Part 2:**
  - block written and cleared;
  - a stale hook can't block, or clear, a replacement's block;
  - a session that leaves the role and takes it back;
  - a late hook from before the operator typed can't block a session that has listened since;
  - a block never creates a binding;
  - a permission-prompt block clears when the session runs a handoff command;
  - "last listened" shows the time;
  - the sender's line on `post`, `reply` and `note`;
  - the alert once per key, from any listener's pass, and not for a held APPROVE.
- **Part 3:**
  - one wake for three reviewers;
  - a held APPROVE carried by a CHANGES;
  - the hold ending at 30 minutes with no new write;
  - a restart while held;
  - an fyi for another session behind a held one, then an urgent decision;
  - a supersession with no entry for the role;
  - mixed commits;
  - an APPROVE on a superseded request, delivered at once;
  - a held APPROVE released by an entry addressed to another role (two roles with requests on one item);
  - two held APPROVEs released in one wake when the first one's 30 minutes end;
  - `--emit` holds nothing.
- **Part 5:**
  - the quiet alert's text for a blocked role, a role nothing listens for, and a listening one;
  - a note undelivered past `quiet_after` alerts;
  - none for a held APPROVE, an fyi to another session, or anything before `tracking.json`;
  - the same lines in `handoff` and `inbox` with no listener.
- **Part 6:**
  - the old session's post, reply, note and claim are refused after the role moves (fails on main, as in `evidence/h34/shutout.log`);
  - `--as` another role is refused too;
  - the mark survives the old process ending, and a resumed session with the same id is refused;
  - `/clear` and `/background` on a marked session carry the mark and no role;
  - `holder` never counts a marked binding;
  - a write paused across the role move is refused;
  - the old listener can't advance the cursor after the move, and the successor gets every entry;
  - `handoff role` from the session clears the mark;
  - `merged` from a marked session is refused;
  - `bind` and a write racing each other never deadlock;
  - a session with its role fixed by `--as` and no binding passes as today.
- **Part 7:**
  - three requests in one write;
  - a write cut after the first member: no go-ahead approval on it, `inbox` says why, and the same post, with no `--key`, completes it;
  - a new round superseding all three in one append;
  - a new round cut after one member: neither that member nor the two old requests still live can be approved under the go-ahead; the same post completes it, and all three old ones are superseded together;
  - a complete round posted again writes nothing;
  - a post with another commit or panel while a round is pending is refused;
  - a role named twice is refused;
  - a go-ahead covers the item only when all three approve one commit.
