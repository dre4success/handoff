# Nomination prompt (round 2, before roles are assigned)

The same prompt for each of the three sessions. Paste it as the next message.

---

Round 2. You may now read all three round-1 reviews in `docs/reviews/` and revision 2 of the proposal, `docs/proposals/HANDOFF_PROPOSAL_CLAUDE.md`; its §17 shows which of each review's points were accepted.

Nominate one implementer and one primary reviewer for this project. **Neither may be you.** Judge only by the reviews and the proposal: grasp of the heart (§1 of each review), evidence run rather than asserted (§3), the changes that mattered (§2 against the proposal's §17), build order and restraint (§5, implementer), and what each would refuse to approve (§5, reviewer). Prefer an implementer and reviewer from different vendors; two of the three reviewers run in the same CLI. Add one sentence on whether a gate reviewer is needed at all.

Post one entry to `docs/CHANNEL.md` by appending with a single shell command (`cat >> docs/CHANNEL.md <<'EOF' … EOF`), never by rewriting the file, in exactly this shape:

```
### [YYYY-MM-DD HH:MM UTC] <your model> → the operator: H2 nomination

Implementer: <model>. Reason: <one sentence, citing a review section>.
Reviewer: <model>. Reason: <one sentence, citing a review section>.
Gate reviewer: <needed or not, one sentence>.
Change my mind: <one sentence on what evidence would>.
```

Use `date -u` for the time. Do not commit; the drafter commits the channel (D10). No code, no other edits. Stop after posting.
