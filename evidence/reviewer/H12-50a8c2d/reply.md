CHANGES for request #58, commit 50a8c2d9aa372f47308c40b7d2cb78699102a6fe (base 4797d929c89c7cdc4c8335f0197d9fedd71ab5dc).

1. [P1] Do not treat a Codex hook's parent PID as one session's identity (cmd/handoff/session.go:62–69, 139–141, 235–255).

   `hookSession` stores `os.Getppid()`, and a later SessionStart with another id moves a matching PID's role to that id. The installed Codex 0.162.1 supports a shared background server. OpenAI's maintainer explains that the app server hosts all active threads and runs hooks, while multiple TUI clients can connect concurrently. Thus separate Codex sessions can have the same hook parent; it does not establish that one session changed threads. [OpenAI maintainer's explanation, September 28](https://github.com/openai/codex/issues/48500).

   Independent Run: in `shared-codex-parent.txt`, thread one takes implementer, its hook records the parent, then a second thread's SessionStart under that same live parent silently moves implementer to thread two. Thread one's inbox then fails with "no role". This is a stand-in shared parent, not a live two-Codex run; applicability to the shared server is Source/Inferred from the installed CLI and the maintainer's description. The supplied single-Codex identity probe does not establish distinct per-session parents.

   Keep thread bindings separate when the parent can host several sessions. Preserve a role across a changed thread id only with evidence of that session's continuity, or require explicit reassignment where Codex supplies none. Starting a second independent session must not take the first session's role. Include the shared-server case in regression coverage.

2. [P2] Stop ordinary `wait` when its session loses the role (cmd/handoff/session.go:295; cmd/handoff/cmds.go:904–914).

   Role transfer stops `hook listen`, but `cmdWait` supplies no ownership check to `wait.Run`. The brief printed by `role` explicitly supports sessions without hooks using ordinary wait. Independent Run: session A takes reviewer and runs `wait --timeout 30s`; B takes reviewer. Two seconds later A still holds the listener lock, B's wait exits 4, and A consumes a newly posted note and advances reviewer's cursor. B then times out without that note. This can stall or misroute the handoff after the operator transfers a role.

   `moved-wait.txt` reproduces it on the exact commit. The otherwise equivalent `moved-hook.txt` control passes: A's hook exits and B receives the note. Apply the binding-liveness check to a normal wait owned by a bound session too, without changing the supported explicit-role waits outside sessions.

Validation and evidence: `evidence/reviewer/H12-50a8c2d/`. Full `go test -race -count=1 ./...` PASS on the exact commit (CLI package 420.342s); build, vet, format, branch diff whitespace and dependency checks PASS. All scratch record mutations used the CLI. No model sessions were launched by the reviewer, and no production code, shared checkout branch, or installed settings were changed.
