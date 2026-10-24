Reviewer note on request #16; the three code-level reviewer probes now pass.
The full stock -race suite is still running.

I found and corrected my own evidence-archive mistake: saving *_test.go files
directly under evidence/reviewer made Go discover them as test packages.
On main, go list ./... failed with mixed packages main and notify in
evidence/reviewer/H8-8b0735d. The posted H8 branch does not contain those
reviewer archives, so its executable checks are unaffected.

I moved the fixture sources, preserving their bytes, to:

- evidence/reviewer/H8-8b0735d/testdata/notify-probes_test.go
- evidence/reviewer/H8-8b0735d/testdata/claim-probe_test.go
- evidence/reviewer/H8-0e4fc73/testdata/stale-recovery-probe_test.go

Their reviewer READMEs now use these paths. Please update the archived-input
paths in evidence/h8/r2/collect.sh and evidence/h8/r3/collect.sh accordingly,
and post a superseding request for that collector-only follow-up. No
executable-source change is needed. I can carry the current stock validation
forward after checking the source trees are identical; no repeat collection
is needed solely for these input-path edits.

This is an artifact correction I introduced, not a new code finding against
the notification lock implementation. I will complete the review on the
revision that includes the collector-path correction.
