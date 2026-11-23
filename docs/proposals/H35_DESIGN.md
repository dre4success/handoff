# H35: a spare session takes over a stuck seat

Status: draft, round 9, for the reviewers, then the operator. Built on H34 (parts 1 to 8).

## What happened

On 2026-11-23, reviewer-3 went quiet at 02:19, an Antigravity session at its cap. From then on every H34 item waited for it. handoff told the operator (the quiet alert), who was asleep. Nothing else could happen until morning.

The operator wants any session to be able to take over a stuck role, "like leader election in an etcd cluster" (#232), and wants it simple: "So simple to use that I don't have to think too much on what to do" (#237).

## The idea in one paragraph

Before going away, the operator starts one more session and gives it the role `spare`. `away` then asks whether the spare should cover, and the operator's yes is recorded in the away entry as their word for it, as the go-ahead is. While the operator is away, if a seat gets stuck with work waiting for it, the spare is woken and told which seat. It runs `handoff take <role>`, and from then on it is that role: it gets the role's work, its brief and its listener. The stuck session is shut out, as when a role moves today (H34 part 6). At `back`, the operator reads who took what.

## Words

- **A seat** is a role: implementer, reviewer, reviewer-2. Work goes to a seat, never to a session.
- **A session** holds a seat. Whoever holds the seat inherits its open work.
- **The spare** is a session given the built-in role `spare`. It holds no work and is never sent any. Like `operator`, it isn't listed in the config. There is one spare. If two are wanted later, they are `spare` and `spare-2`, as with reviewers.

## When a seat is stuck

Three things must hold:
1. **The operator is away** (`handoff away`), with cover recorded in that entry (below). When the operator is here, they decide.
2. **Work has waited for the seat for `quiet_after`** (30 minutes by default): what part 2's `WaitsFor` counts, so a held APPROVE and an fyi for another session don't count.
3. **Its session can't take it.** How handoff knows, and so how soon it acts and for which seats, depends on how exact the sign is:

| Sign | Exact? | Acts at | Seats |
|---|---|---|---|
| Antigravity's cap (part 2's block) | Yes | `quiet_after` | any |
| The session's process has ended, where the CLI gives its pid (Claude Code) | Yes | `quiet_after` | any |
| A permission prompt (part 2's block) | No: it stays recorded through a long turn after the operator answers it (H34 design, part 2) | twice `quiet_after` | reviewer seats only |
| Nothing heard: no listener, no entry from the role, no command from its session | No: a long turn looks the same | twice `quiet_after` | reviewer seats only |

**Silence never takes the implementer seat.** Tonight the implementer went 69 and 65 minutes between its own entries while it built, with a CHANGES waiting the whole time (the reviewer, #655). By silence alone, a spare would have shut out a healthy session in the middle of its work, and a second agent would have started on the same branch. For the implementer seat with no exact sign, the operator is told, as now. A reviewer seat taken wrongly costs one review done again; the panel keeps its size.

Later, a real sign of life could lift this, such as a cheap hook on each tool call that stamps the binding, where a CLI has one. Not in H35.

## Who may take a seat

Only the spare, and only one seat.
- **Review stays independent.** A reviewer never takes a second reviewer seat. The implementer never takes a reviewer seat. A spare has done neither tonight, so it can take any seat, and the panel keeps its size.
- **`handoff role spare` is refused in a session that has ever held another role in this project**, and a session's lineage through `/clear` and `/background` counts as that session. Its file in `.handoff/sessions/` is never deleted (row 107), so the check is whether one exists. Yesterday's implementer can't be today's spare and review what it wrote (the reviewer, #655). A session whose role is fixed by `--as` or `HANDOFF_ROLE` has no file, so it would pass as having no history: the same gap as row 107 (e), and the row says so (the reviewer's note for the builder, #660).
- **`role spare` in another session moves the spare to it**, as every role moves, and the old spare is told it no longer holds it (the reviewer, #660; reviewer-3, #708). So when the spare's session dies, the operator starts another and says "you're the spare", with nothing more to think about. There is still only ever one spare.
- **`role spare` in the spare's own session changes nothing**: "You are the spare already." It writes no new binding, so its generation, and the cover the away entry names, stay as they are (the reviewer, #744, #753). Without this, `role spare` run again would quietly end the cover.
- **One takeover per spare, per away entry** (reviewer-2, #755). Once a takeover entry names the spare's generation under the away entry, finished, cut or superseded, `take` of any other seat is refused. While a takeover is cut, the refusal says how to finish it: "Your takeover of reviewer-3 (#700) was cut short; finish it with `handoff take reviewer-3`." To have the spare cover again, the operator runs `handoff away` again.
- **A spare given after `away` can't take a seat**, since the away entry names the spare that was there when the operator said yes. To have the new one cover, run `handoff away` again: it asks about cover again, naming the new spare.
- **The spare's own lineage counts as the spare, by its generation.** A spare that runs `/clear` or `/background` while the operator is away goes on as the same spare. The away entry names the spare's binding's generation as well as its session, and a role's move by `/clear` or `/background` keeps the binding's generation (`carry`). So `take` accepts the binding that holds `spare` at that generation, whatever its session id now is. It doesn't use `Was`, which only `/background` adds to, so an fyi's delivery stays as it is (row 93; reviewer-3's note, #708; reviewer-2 and reviewer-3, #727, #728). A spare given anew by `role spare` has a new generation, so it isn't the one the entry names.
- **With no spare, nothing changes.** The operator is told, as now, and part 5's alert says why the work waits.

I considered and left out:
- **Reviewers covering the implementer.** It shrinks the panel for every item in flight, and their open reviews would need asking again.
- **A vote among sessions.** They share one disk, so a lock decides, and a vote would cost wakes.

## The takeover

Two ideas carry every rule below (the reviewer, #753):
- **The takeover is its entry in the record.** The bindings follow from the entry. A takeover can be finished or superseded, never undone.
- **A conversation is its binding's generation.** `/clear` and `/background` change a session's id but keep its generation. So the spare, the old holder and a shutout are each found by generation, whatever id the session has now.

0. **The operator's word is recorded.** With a spare bound, `away` always asks about cover, whether or not the go-ahead approves anything now (below). On a yes, the `away` entry records that cover is on, and the spare's session and generation. Only the spare named there, or the session it went on in at that generation, may take a seat: any session can run `handoff role spare`, and one that names itself spare after the operator left has no word behind it.
1. **The spare listens** like any session: its hook, or a background wait (part 8). Its listener checks, once a minute, for a stuck seat. When it finds one, it wakes the spare once for it: "reviewer-3's seat is stuck: its session can't be woken (Antigravity's 10 wakes used, since 02:19). #553, #586 and #593 wait for it. Take it with `handoff take reviewer-3`." Nothing is written to the record for this; it is a wake, like an alert.
2. **The spare runs `handoff take reviewer-3`.** Under the sessions' lock and the record's write lock, the command checks everything again: still away, cover on in that away entry, still stuck by the table above, still the spare named there.
3. **It writes one entry first**, kind `takeover`: the seat; the session it takes the seat from, and that binding's generation; the spare's session, and its binding's generation as the spare; the new generation, as the seat's term; why it was stuck; the work that waited; and the away entry it acts under. The record's append is synced to disk before it returns (`record.Store`), so from here the takeover is durable. **The entry, not the bindings, is the takeover:** from the moment it is in the record, the seat's old generation is fenced. Any binding holding the seat at the generation the entry names is refused, and its listener delivers nothing more as the seat, whatever the binding still says. That covers the old session, and any session it went on in by `/clear` or `/background`, since such a move keeps the generation (reviewer-2, #727). A move by `/clear` or `/background` after the entry carries a shutout, not the seat: `carry`, under the sessions' lock, reads the record's takeovers, and a binding at a fenced generation goes on as a lost mark naming the takeover. The write fence (`fenced`, under the record's lock) and the delivery fence (part 6) read the record's takeovers as well as the binding. Under the record's lock they only refuse: they never take the sessions' lock, which is always taken before the record's, never after (reviewer-3, #718).
4. **Then it moves the bindings**, as `handoff role` does, in this order: the old session is marked (part 6), its mark naming the takeover entry; then the seat's last assignment (below) is set to the entry's term; then the spare's binding takes the seat at that term. Each write is atomic (a rename), so a binding is always the old one or the new one. The spare's binding is written last on purpose: until the takeover is finished, the spare is still `spare`, so its listener runs and can wake it, and the one session that can finish the job stays reachable (the reviewer, #719).
5. **The new holder carries on.** It reads its brief and inbox as that role. A review the old holder had claimed stays claimed: a claim is the seat's, not the session's, so `claim` from the new holder returns the claim already there, and the new holder reviews the exact commit asked. The takeover entry names each request that waited for the seat and counts as their latest entry, so their quiet clock starts again at the takeover, not at a claim an hour back (reviewer-3's note, #708). Taking the implementer seat, its brief adds: start from the commit the record names; leave any uncommitted changes you didn't make; work in a worktree of your own. The old session's work in progress isn't in handoff.

The spare can't act as the seat until its binding says so, at step 4, and that comes after the entry is durable, at step 3. So no session holds a seat without a takeover entry behind it.

### A takeover cut short

`take` can stop anywhere: a crash, a kill, a full disk. What `take` does next is decided only from the record and the bindings as they are, read under both locks, never from what an earlier run returned. `Store.Append` can sync the entry and then fail to update `head.json`, and it returns an error even though the entry is in the record (reviewer-2, #717). So after any error, and on any run, `take` first looks in the verified record for its own takeover entry. It finds it by the seat, the spare's generation and the away entry the entry cites, never by the spare's session id. So the spare can finish its own cut takeover after it has run `/clear` or `/background`, since those keep its generation, and it never writes a second entry for one already in the record (reviewer-2, #745).

| Cut | What is so | What happens next |
|---|---|---|
| Before the entry: the record refused the write, or `take` stopped first | Nothing changed: no takeover entry for this seat names the spare's generation | `take` says nothing was taken. Run again, it starts over, and all its checks run again. |
| After the entry, whatever the append returned: before either binding, or between them | The record says the spare took the seat. The old session is fenced by the entry, marked or not. The spare is still `spare` | The work waits. `take` run again finishes the bindings, if nothing has moved since (below). It writes no second entry. |
| After both | Done | `take` run again says the spare holds the seat already, under that entry. |

A record left torn by a write cut in the middle is broken, as today, and refuses every write, `take` included, until it is repaired (`handoff repair`).

**Finishing checks that nothing moved.** Run again, `take` doesn't check again that the operator is away or that the seat is stuck: either may no longer hold, and the old session is fenced already. Instead it checks, under both locks, the exact generations the entry names:
- the binding holding `spare` at the generation the entry names is the one running `take`, under whatever session id, and it hasn't been moved, marked or given another role;
- the seat's last assignment is the old generation the entry names, or the entry's own term, if a run already got that far;
- the seat is held at the generation the entry names, by the old session or one it went on in, or by no one;
- no later `takeover` of the seat is in the record.

**The seat's last assignment** is kept on its own, because a binding can't keep it: `handoff role` in a session rewrites that session's binding, so a session given the seat and then another role no longer shows that it held the seat (reviewer-2, #766).
- It is one small file per seat, `.handoff/seats/<role>.json`, holding the generation the seat was last given at.
- Every move of a seat to a new generation writes it under the sessions' lock, before the binding: `handoff role` and a takeover's finish. `/clear` and `/background` keep the generation, so they don't touch it.
- A seat with no file yet, last given before H35, counts as given at the generation of the binding that holds it. `take` writes the file before its entry, so a cut takeover always has one to check.
- It never goes down. Written before the binding, a move cut between the two leaves the seat looking given on, which is the safe side: a cut takeover then doesn't finish, and the operator is told the seat is empty.

So **superseded is final**. Once the operator gives the seat to another session, its last assignment is later than the entry's old generation for good. That holds whatever happens to that session afterwards: it ends, or it is given another role in the same session. The cut takeover never finishes (reviewer-2, #755, #766).

If all hold, it finishes. If any doesn't, the takeover is **superseded**: `take` writes nothing, changes no binding, and says why, for example "reviewer-3 was given to another session after your takeover (#700) was cut short; it stays theirs." The spare's listener stops waking it for that takeover.

Who finishes it:
- **`take`, run again by the spare.** After a cut, the spare's listener finds a takeover naming its generation that its binding doesn't show yet, and that nothing has superseded. It wakes the spare once: "Your takeover of reviewer-3 (#700) was cut short; run `handoff take reviewer-3` again to finish it."
- **A command from the old session** finds the takeover against it as it starts, under the sessions' lock and before it takes the record's. It marks its own binding if it isn't marked, then is refused with the lost line. If it got as far as a write, the fence refuses it under the record's lock and marks nothing. It never finishes the spare's side.

Nothing else finishes a takeover, and nothing undoes one: a takeover in the record stands, finished or superseded, and the old session stays fenced by it either way. A retry finishes it even if the operator is back by then, as long as nothing has moved.

**The operator's word comes first.** If the spare never finishes it, say its session ended, no session holds the seat. The work waits, and the operator is told as for any quiet seat, with the reason: "the spare's takeover of reviewer-3 (#700) was cut short". `handoff role reviewer-3` in a session gives the seat on, as today. That supersedes the cut takeover, so a late retry by the spare can't take the seat back from the operator's choice (reviewer-2, #717; reviewer-3, #718). The same goes if the operator gives `spare` to another session meanwhile.

`back`'s list of who took which seat, and its question about each session shut out, come from the `takeover` entries, never from the bindings. So a cut takeover is still listed, with "cut short; the spare hasn't finished it" until it is finished, or with "superseded: you gave reviewer-3 to another session".

handoff still never decides, assigns or approves (row 105). It delivers the wake; the spare takes the seat by its own command, under the operator's recorded word in the away entry, as an approval under the go-ahead cites it.

## What the operator sees

- **At `away`, with a spare:** one question, always asked, whether or not the go-ahead approves anything now. "Cover: the spare can take any seat that gets stuck while you're away. Turn cover on? [y/N]"
  - Yes: the away entry records cover, and the spare's session.
  - No: `away` is written as today, without cover, and says "Away without cover: if a seat gets stuck, you'll be told, as now."
  - `away` still needs the operator's terminal, as today: with none, it is refused, cover or not, so an agent's shell can't run it (reviewer-2, #717; reviewer-3, #718).
  - When the go-ahead also approves work now, that question comes first, as today, and a no to it writes nothing. The cover question comes second. Each yes means one thing.
  - The away entry is written only if the spare named in the question is still the spare under the lock. If the spare changed while the operator was asked: "the spare changed while you were asked; nothing written; run it again", as `away` does today when what it approves changes.
- **At `away`, with no spare:** one line. "No spare: if a seat gets stuck, you'll be told, as now. To have one, start a session and give it the role spare."
- **At the takeover:** one notification. "Under your go-ahead #428, the spare took reviewer-3: its session can't be woken (Antigravity's 10 wakes used, since 02:19). #553, #586 and #593 are with it now. No spare is left." So the operator knows a second stuck seat waits for them (the reviewer, #660).
- **In an approval the spare took part in:** "W1: approved under your go-ahead #428, by reviewer, reviewer-2 and reviewer-3 (the spare, since 02:52)". The operator gave the go-ahead to a panel they knew.
- **In `handoff` on its own:** "took over: reviewer-3, by the spare (#700), at 02:52."
- **At `back`:** who took which seat, and what still waits. The takeover stays: the spare keeps the seat. For each takeover whose shutout stands, one question: "reviewer-3's old session was shut out when the spare took its seat at 02:52 (#700). Let it back in? [y/N]", with one line under it: yes means it, or the conversation it went on in, can take a role again when you give it one; no means it stays out.

## A session shut out by a takeover

It can't take its seat back by its own command, before or after `back` (reviewer-2, #273). Only the operator's recorded word lets it back in, which is the question at `back`. On a no, it stays out, and `handoff role` in it is refused with a line saying to run `handoff back`.

What the record holds (reviewer-3, #708):
- **The shutout is the takeover's, not one session id's** (reviewer-2, #745). It covers every session that traces to the takeover: the one it marked, a session that carried its lost mark on by `/clear` or `/background`, and one those moves left with a "went on" mark. The question at `back`, the `letin`, and `handoff role`'s check all name the takeover, so they agree whichever of those sessions the operator answers for, or types in later.
- **A yes is an entry of its own**, kind `letin`, from the operator: a reply to the takeover, naming it, and the sessions its shutout covered when it was written, for the record. Once it is in the record, `handoff role` is no longer refused for that takeover in any session it covers, including one that carries the mark on afterwards. A role given then is a fresh binding at a new generation. So the old generation stays fenced: the session's old writes and its old listener can't act as the seat, letin or not (reviewer-2, #727).
- **A no writes nothing.** The session stays out, and the next `back` asks again.
- **`back` while away** writes its `back` entry as today, then a `letin` for each yes.
- **`back` when not away** asks only the question, for each takeover whose shutout stands, and writes a `letin` for each yes and no `back` entry, since the operator is back already. With no session shut out, it refuses as today: "you aren't away; nothing written".

So there is still no new command.

## The test of simplicity (#241)

Someone who knows `away`, `back` and the inbox:
- **Sets up cover:** starts one more session, tells it "you're the spare", and goes away. `away` asks whether the spare should cover, and they say yes.
- **Understands a takeover:** reads one notification.
- **Comes back:** reads `back`, and answers one yes/no per session shut out.

What's new to them is one role name, `spare`. `take` is the agent's command, not theirs.

## For the operator to decide

1. **The spare only** (my recommendation), or reviewers may cover the implementer seat too.
2. **Only while away** (my recommendation), or at any time.
3. **What may take a seat, and which** (the one with a real choice in it). My recommendation, and the reviewer's (#655): an exact sign (Antigravity's cap, an ended process) takes any seat at `quiet_after`; a permission prompt or silence alone takes only a reviewer seat, at twice `quiet_after`; silence never takes the implementer seat, and you're told instead. The other way is to let silence take any seat, which tonight would have taken a working implementer's.
4. **The times:** `quiet_after` and twice it, as above.
5. **The names:** the role `spare`, and the agent's command `take`.

## Tests to have

- A blocked reviewer seat with work waiting: the spare is woken once, takes it, gets the work; the old session is shut out.
- The same for the implementer seat, with approved and changed work.
- Not stuck: the operator here; work younger than `quiet_after`; a claimed review quiet but not blocked, before twice `quiet_after`; a held APPROVE alone; the implementer seat silent, or at a permission prompt, however long.
- A reviewer seat silent, or at a prompt, for twice `quiet_after`: taken. An ended Claude Code process with work waiting: taken at `quiet_after`.
- Not eligible: no spare; cover not in the away entry, because the operator said no; a spare bound after it; a session that ever held another role refused `role spare`; a second take by the spare.
- `take` racing the old session coming back: one holds the seat, and the other is shut out or refused.
- A takeover cut short, each run killed at a pause point (H36):
  - after the entry, before either binding;
  - between the two bindings;
  - after the entry, with the old session then moved by `/clear`, and again by `/background`, before `take` runs again. The session it went on in is fenced at the old generation and carries a lost mark, not the seat, and `take` run again finishes.
  For each cut, the test checks that:
  - the old session's write as the seat is refused;
  - the spare can't act as the seat;
  - `back` lists the takeover as cut short;
  - the spare's listener wakes it once;
  - `take` run again finishes the bindings and writes no second entry.
- `take` whose append the record refuses: no entry, no binding changed, and `take` run again starts over.
- `take` whose entry is synced but whose `head.json` update fails (the store's fault point after the sync): the error is reported, the old session is fenced, and `take` run again finds the entry and finishes, with no second entry.
- A cut takeover superseded, then the spare's late retry: the operator gives the seat to another session, or `spare` to another session, and `take` run again writes nothing, moves nothing and says why; the old session stays fenced; `back` lists it as superseded.
- The old session's command after a takeover marks its binding before it takes the record's lock; one at the write fence is refused there and marks nothing; no lock is taken out of order (a test holding each lock in turn).
- `away` with a spare and nothing approved now: the cover question is asked. Yes records cover; no writes away without it; a spare changed while asked writes nothing; and with no terminal, `away` is refused, as today.
- `away`, the notification with "No spare is left.", the approval line, `handoff` and `back` lines; the question at `back`, asked again while a session remains shut out, and `role` refused in it on a no. A yes writes one `letin` and lets `role` through; `back` when not away writes only the `letin`, and with no one shut out refuses as today.
- `role spare` in a second session moves the spare and tells the first; a spare given after `away` can't take; `away` run again names the new one. A spare after `/clear`, and one after `/background`, each tested on its own, can still take, by the generation the away entry names; an fyi's delivery after each is unchanged.
- After a `letin`, a role given to the session works, while a write or a listener of its old generation is still refused.
- `role spare` in the spare's own session, and after its `/clear`: "You are the spare already.", no new generation, and it can still take.
- With a takeover in the record for the spare, cut, finished or superseded, `take` of another seat is refused, and a cut one's refusal says how to finish it.
- A cut takeover superseded by the operator's `role` in session B, and then the seat empty again, two ways: B's process ends; or B is given another role in the same session, which rewrites its binding. Each time, `take` run again still writes nothing and moves nothing.
- A move cut between the seat's last assignment and the binding: the seat's file is ahead, and a cut takeover of the seat is superseded, not finished.
- The old session moved by `/clear`, and again by `/background`, before `back`: the question names the takeover once; a yes lets `role` through in the session it went on in, and in the old id resumed; the old generation stays fenced in both.
- A cut after the entry, then the spare moved by `/clear`, and again by `/background`, before `take` runs again: the entry is found by the spare's generation, the takeover finishes with no second entry, and the same after `back`.
- After a takeover, a claimed review stays claimed, `claim` from the new holder returns it, and the quiet alert for it waits `quiet_after` from the takeover.

## How it ships

In three parts, each its own request: the spare role and its brief; the stuck check and the spare's wake; `take` and the operator's lines.
