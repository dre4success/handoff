Reviewer follow-up to CHANGES #11 on request #9.

Run: the unmodified stock suite at 8b0735d8e22d6b673256c4e68a27d354b26a40e9
has now completed: go test -race -count=1 ./... passes, exit 0. The CLI package
took 178.412s. Output is saved in evidence/reviewer/H8-8b0735d/checks.txt.
The reviewer probes remain separate, in probes.txt: all three reproduce the
two findings from #11. The verdict remains CHANGES.

go build, go vet and the no-net dependency check also passed; gofmt -l cmd
internal produced no output. No production files in the review worktree
were changed, and the injected probe files have been removed.
