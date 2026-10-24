CHANGES on `48eb3b5661c2d0a930ec5344d0e15b329518bdb7` for H14.

**[P2] Identify the redacted trial archive as a transcript whose original hash verification no longer applies** — `evidence/trial/record/channel.jsonl:1` (also entry 7).

The scrub changes the bodies of entries 1 and 7 while entries 2 and 8 retain the hashes of their original bytes. Running the trial's own byte-hashing check finds two broken links. Through this revision's production `record.Load`, H13's archive has eight verified entries; H14's has zero verified and eight unverified, with the chain broken at line 2 and entries withheld from line 1.

`evidence/trial/README.md:14` still says the copy's chain verifies, `checks.txt` still reports `chain verifies: True`, and `evidence/h14/README.md:15` says results are unchanged. A public reader cannot reproduce that verification from the supplied archive.

Make the public copy explicitly a redacted transcript and label the saved chain check as a historical result from the original private bytes, with fresh evidence describing the redacted copy's verification limits. Alternatively, supply a clearly labeled derived, valid sanitized fixture with fresh verification. Preserve the historical trial outcome and keep the personal references removed.

Reproduction and evidence: `evidence/reviewer/H14-48eb3b5/audit.txt` and `archive-loader.txt`; the helper source is saved alongside them. The supplied scrub script exactly reproduces the scrub commit's paths, bytes and modes. The full tracked-tree privacy sweep found none of the supplied identifiers; broader email, host and credential checks found only fixture addresses and public documentation hosts. The full race suite, build, vet and source checks pass. See README.md for full validation and limits.

No merge, push or publication performed. New reviewer evidence added after this requested tree must also be included in the final pre-publication scrub.
