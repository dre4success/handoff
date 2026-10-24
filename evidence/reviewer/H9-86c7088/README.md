# H9 review

By Codex, reviewer. Request #23 names
86c70888be0557fff943187c3a714f4a78668240, based on
36882430e60ba5a0ed731ba0ab58e8baefbff5a7. Claim #24.

Review and independent checks use the detached worktree
/private/tmp/handoff-review-H9-86c7088. The shared checkout remains on main.
Commands and output are collected here with date, commit, and exit status.

Run results: the complete go test -race -count=1 ./... suite passes
(CLI package: 225.679s). Build, vet, format, whitespace, and the no-net
dependency inspection pass.

probes.py and probes.txt are independent terminal-race checks in disposable
projects: confirming an item after a superseding commit is approved does not
retarget the decision, and withdrawing the standing approval while bare
handoff is prompting prevents a decision. Both pass. All fixture record
writes use the CLI; the probe reads the resulting records to check that no
decision was appended. No live operator decision was made.

h9b-sandbox-note.md is handoff note #25: it records the two sandbox variables
observed, and distinguishes the GUI permission constraint from a measured
notification-delivery result. H9b is outside this terminal verdict.

H9 was approved in reply #26. notification-observed.md (note #27) then
records a real failed notification reported by the normal reply/wait path;
the receipt itself succeeded. The review remains on duty after the approval.
