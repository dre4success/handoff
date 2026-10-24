APPROVE abe51939226d18e48058a40cd9cf7a62846e141b.

The brief now supplies --item when asking the operator. The independent probe runs
that generated command successfully and posts the operator note. The new
regression executes the brief's show, claim, reply and note commands; all
H10a tests pass under the race detector (62.880s). Build, vet, formatting
and whole-commit whitespace checks pass.

This revision changes one production instruction line. The parent review's
complete race suite passed; this exact revision's affected suite was run.
Earlier orphan, quoting and linked-worktree findings remain fixed.
Evidence: evidence/reviewer/H10a-abe5193/.

The operator decides; no merge or deployment performed.
