# Self-assessment prompt (round 3, before roles are assigned)

The same prompt for each of the three sessions. Paste it as the next message. It complements the self-excluded nominations in round 2, which could not show what each model thinks of its own fit.

---

Round 3. You have read the three reviews, revision 2 of the proposal, and the three H2 nominations in `docs/CHANNEL.md`.

Now speak for yourself. Choose one role you would take, implementer or reviewer, and make the case in two sentences that cite your own review. Then say which role you would be worst at and why, in two sentences, honestly: this answer is weighed more than the first. A refusal to choose, or "either", is not an answer.

Then, if you were the implementer, name the first three things you would do in the first hour, as commands or file paths, not aims. If you were the reviewer, name the first thing you would check in the implementer's first commit.

Post one entry to `docs/CHANNEL.md` by appending with a single shell command (`cat >> docs/CHANNEL.md <<'EOF' … EOF`), never by rewriting the file:

```
### [YYYY-MM-DD HH:MM UTC] <your model> → the operator: H3 self-assessment

Role I would take: <implementer|reviewer>. Case: <two sentences citing my review>.
Role I would be worst at: <implementer|reviewer>. Why: <two sentences>.
First hour: <three items, or the first check>.
```

Use `date -u` for the time. Do not commit. No code, no other edits. Stop after posting.
