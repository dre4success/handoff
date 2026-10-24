# H9b evidence: the operator decides with a click

Run by Opus 5.5 (implementer) on the operator's Mac. `collect.sh` regenerates every file here except `live-popup.txt` (real pop-ups, the operator clicking, made by `live-popup.sh`) and `codex-sandbox-probe.txt`. Each file names its date and commit. `collect.sh` sends notifications and pop-ups to files, never to the operator's screen.

## What H9b does

- **A pop-up asks you.** When something waits for your decision (a reviewer approved, or an agent asks you), a pop-up shows **Approve**, **Deny** or **Later**.
- **Approve** records the approval. **Deny** asks why (optional), then records the denial. **Later**, Cancel, or no click within 8 hours records nothing; `handoff` in the project still lists it.
- **Only a click decides.** The pop-up runs in its own process (`handoff _popup`), which refuses to start without the alert's lock. No button is the default, so a stray Return answers nothing. If you decided at a terminal meanwhile, that decision stands and the click is refused, with a notification saying so.
- **Codex's sandbox sends nothing.** macOS can't show a notification or a dialog from inside it, so a process there leaves the alert to one outside, in practice Claude's `wait`.
- **A failed notification says why**, in osascript's own words.

Design: proposal §17 row 65.

## Results (Run)

| What | File | Result |
|---|---|---|
| Real pop-ups on the operator's screen, the operator clicking | `live-popup.txt` | Both came to the front, and both clicks were written as decisions. Notes below |
| The new tests fail on `main` and pass here; the same steps through each binary | `regressions.log` | They don't build on `9b2dfec` (no pop-up there); all 13 pass here. Through `main`'s binary each decision is a notification; through this one, a pop-up. Unanswered pop-ups decide nothing |
| Round 2: the helper keeps the lock through garbage collection | `round2-lock-gc.log` | With the fix (`2e8de3a`) taken out, the test fails 5 of 5 ("lock was free while its dialog was up"); here it passes 5 of 5 |
| The pop-up's helper started by hand, as an agent might | `helper-refusals.txt` | Refused with no lock, with another file in its place, and while another process holds the lock. Nothing shown, nothing decided |
| osascript inside Codex's sandbox | `codex-sandbox-probe.txt` | Plain AppleScript runs. Notifications and dialogs fail with a syntax error, exit 1 |
| Every test, under the race detector | `go-test-race-v.log` | All pass |
| The new tests, 5 runs each | `repeat.log` | 65 passes, no failures |
| Vet, format, dependencies | `deps-vet-fmt.log` | Vet clean. `gofmt` flags only three of GPT-6's H8 probe files, kept as supplied under `evidence/reviewer/…/testdata`; the code is formatted. No third-party module, no `net` |

## Round 2: GPT-6's CHANGES (#31)

The helper held the alert's lock through descriptor 3, wrapped in a file that nothing referred to once the lock was checked. Go's garbage collector closes such a file, which freed the lock while the question was still on screen, so another process could show the same pop-up again. GPT-6 reproduced it (`evidence/reviewer/H9b-6783e4a/`). The helper now keeps the file open until it returns (`2e8de3a`). The test dialog runs garbage collection while it waits, and `TestH9bPopupAcrossProcesses` checks for a second that no other process can take the lock (`bf4f2b5`). Row 66.

## The live run (2026-10-15, at `ef7eceb`)

The operator was asked to click Approve on the first pop-up and Deny on the second.

- **"H1: codex approved #1"** closed 10 seconds after it appeared, and H1 was decided: `handoff` at the end lists nothing. The run didn't print that decision (#3), because the first version of the script guessed entry numbers.
- **"May I deploy H2?"** was recorded as **approve** (#5, "Approved in the pop-up.", 8 seconds after the note). That body is written only when macOS reports the Approve button. So Deny has not been clicked live; the tests cover it through a stand-in dialog.
- The run's "no click within 5 minutes" is the same script bug: it waited for the wrong entry. `live-popup.sh` now reads the numbers from what was posted and prints every entry the clicks wrote. A dry run, with pop-ups sent to a file, checked it.

Since that run, two commits touch the dialogs. The reason box after Deny now also closes after 8 hours (`4c457c7`). And `TestH9bDialogScriptsCompile` compiles both dialogs' AppleScript with `osacompile`, which shows nothing; it fails if either script has a syntax error (tried by breaking one). It skips in Codex's sandbox, where `display dialog` doesn't parse.

## Tests

`cmd/handoff/popup_test.go`, with clicks scripted through a stand-in dialog:
- Approve writes `approve`; Deny asks why and writes `deny` with the reason, or "Denied in the pop-up." without one;
- Cancel or no answer in the reason box, Later, or no click: nothing written, the pop-up not shown again, still listed;
- a dialog that can't be shown leaves the alert for another try;
- decided at a terminal before the pop-up: not shown; after it: the click is refused and the operator is told;
- a note to the operator gets the same pop-up, with its text;
- the helper needs the alert's lock; across real processes, one alert gives one pop-up, and the helper keeps the lock through garbage collection while its dialog is up.

`internal/notify/notify_test.go`: a process in Codex's sandbox sends nothing and leaves the alert for others; a decide alert goes to the pop-up, not a notification.

No test reaches the operator's screen. Until `9182ec1`, the refused-click test called the notice in the test process itself, which had no `HANDOFF_TEST_NOTIFY`, so a plain `go test` run could put "Your click on W1 was not recorded" on the operator's screen. The test process now sends notices to `/dev/null` unless a test names its own file.
