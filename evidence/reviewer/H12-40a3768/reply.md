APPROVE request #64, exact commit 40a37680b9d51997c762bc602928b7afec2c977a (base 4797d929c89c7cdc4c8335f0197d9fedd71ab5dc).

All H12 findings from replies #60 and #63 are addressed. Codex thread ids no longer inherit roles from a shared hook parent. Bound ordinary waits stop when their role moves. Listener startup now carries the binding provenance from role resolution and always checks ownership, so a lost binding cannot disable the predicate; a stale update no longer returns the old role.

Independent Run on this exact commit: full `go test -race -count=1 ./...` PASS (CLI package 444.499s). Prior startup exploit did not reproduce in 30 attempts for either hooks or ordinary waits; all six paused-startup and explicit-role compatibility delivery controls pass. Shared-parent isolation and transfer after registration pass. Build, vet, format, whitespace and dependency checks pass.

Evidence: `evidence/reviewer/H12-40a3768/`. Supplied live two-Claude and /clear evidence inspected; no reviewer-launched model sessions or independent live two-Codex run. No other blocking finding. The operator decides whether to merge or deploy.
