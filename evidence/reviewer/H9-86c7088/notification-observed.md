Follow-up to note #25: an actual notification failure is now observed.

Run by Codex, reviewer, on 2026-10-15, in the normal command sandbox.
The sandbox environment previously printed CODEX_SANDBOX=seatbelt and
CODEX_SANDBOX_NETWORK_DISABLED=1.

The authorized H9 reply command completed successfully, exit 0:

```text
handoff reply --as reviewer 23 --verdict APPROVE --commit 86c70888be0557fff943187c3a714f4a78668240 --body evidence/reviewer/H9-86c7088/approve.md --key h9-reviewer-r1-approve
posted #26 reply H9 reviewer → implementer: APPROVE on #23 @ 86c70888be05
handoff: notify: could not tell the operator, will retry: exit status 1
```

The next foreground listener printed the same delivery failure:

```text
handoff wait --as reviewer --for reviewer --timeout 25m
handoff: notify: could not tell the operator, will retry: exit status 1
```

Both commands used ~/handoff/bin/handoff.
No test-notification sink was configured by these calls. This is evidence
that the tool's normal notification path reported a failed send; it is not
a direct standalone osascript probe or proof of the particular OS denial.
The notification helper does not include osascript's stderr in that error,
so the cause cannot be determined from exit status 1 alone.

The approval receipt remains valid. Do not infer a successful desktop
notification from it. This is relevant to which process should display
H9b's UI, and does not change the H9 terminal verdict.
