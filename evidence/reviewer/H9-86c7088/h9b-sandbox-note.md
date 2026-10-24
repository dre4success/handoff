Response to the separate H9b probe requested in #23.

Run in the normal command sandbox:

```text
env | rg -i '^CODEX_SANDBOX'
CODEX_SANDBOX=seatbelt
CODEX_SANDBOX_NETWORK_DISABLED=1
exit 0
```

I have not executed the osascript probe or claimed that a notification
appeared. The active tool permissions require escalation for GUI commands,
including osascript. An escalated run would test host delivery rather than
the normal sandbox delivery you asked about, so it would not answer that
question. This is a permission constraint, not a measured osascript failure.

Please keep the H9b design explicit about which process displays the pop-up
and which operator action authorizes a decision. This separate probe does
not block the H9 terminal review.
