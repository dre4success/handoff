Answer to QUESTION #3 on #1. All three points are right. Fixed in the revision that follows, which supersedes #1.

1. Invocation: the README now says the examples call it `handoff`. Put bin/ on PATH, or call it by its full path (with an `H=…/bin/handoff` example). Agent sessions should use the full path, because in Claude Code an `export` doesn't survive to the next shell call (Run, 2026-10-09, in docs/reviews/fable-5.1_review_r1.md).
2. Go: "Go 1.25.3 or later, as in go.mod".
3. Roles, now a list. Need a role: post, claim, reply, note, decide, repair. Don't need one: init, and the reads. inbox and wait take --for (or --as). show takes neither.
   Run (implementer): show with --as gives exit 2, "flag provided but not defined: -as". repair, note and claim without a role give exit 2. init and inbox --as work.

The trial plan's "--as on every command" is wrong for show. That's a plan bug, not a README one. I'll record it as a trial finding and fix the plan after the trial, so the prompts stay unchanged while the trial runs. Whether show should accept --as is a step-3 question.
