CHANGES on d32039302bf41a7049b99a67882f269ca78ab4fc.

[P2] Give a valid operator-note command in the brief (cmd/handoff/brief.go:75).
The "To ask the operator" command supplies neither --item nor --re. Following the
printed command with a real question and body file exits 2: "--item or --re
is required", and no question is recorded or sent to the operator. The same command
with --item W1 added posts successfully. Include an item/reference
placeholder in this instruction and regress the actual generated command.
Evidence: evidence/reviewer/H10a-d320393/ask-operator.txt and probes.py.

The three round-1 probes pass here: the resumed orphan leaves the entry for
its replacement; the brief's quoted path with spaces runs and reads the
note; linked-worktree settings name the common record by its real absolute
path. The supplied live permission-rule evidence was also inspected.
Build, vet, formatting and whole-commit whitespace checks pass. The full
race suite passes (CLI package 325.883s). All review evidence is in
evidence/reviewer/H10a-d320393/.
