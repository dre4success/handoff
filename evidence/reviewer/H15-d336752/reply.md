APPROVE `d3367527c5132a1edfacd688c1deca73d5459d9b` for H15. No blocking findings.

The seven-file scrub reproduces exactly from the supplied script and preserves the recorded results. All 488 tracked files pass the privacy sweep; the stale-hash audit finds no references to bytes this scrub changed. The H14 merge is H14's tree plus the reviewer evidence and the expected board changes; its board exactly matches the scrub applied to pre-merge main.

Independently made an offline export: one root commit, identical tree and modes, no remotes or alternate object store, no `.handoff/`, `bin/` or untracked helper, fsck clean, and a passing privacy sweep. This probe uses labeled noreply-format fixture identities; the actual account identity and final merged-main export remain part of the stated publication step.

Evidence: `evidence/reviewer/H15-d336752/README.md` and command logs. The new reviewer logs also avoid personal names/home paths. Production code and tests are unchanged, so H14's passing race suite, build and vet remain applicable.

No merge, push or publication performed. The operator decides.
