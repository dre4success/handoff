# Codex probe (Run, 2026-10-18 12:14 UTC)

Before H10b was written: a throwaway `codex exec` (codex-cli 0.162.1, gpt-6-luna, low effort) in a scratch git project `<probe>`, with these files in its `.codex/`:
- `hooks.json`: a `SessionStart` hook (`start.sh`) and a `Stop` hook (`stop.sh`);
- `probe.rules`: `prefix_rule(..., decision="allow")` for `probe-cmd.sh`.

Command: `codex exec -m gpt-6-luna -c model_reasoning_effort=low --dangerously-bypass-hook-trust -c 'projects."<probe>".trust_level="trusted"' --sandbox workspace-write "Run <probe>/probe-cmd.sh and show its output verbatim. Then tell me your codeword if you were given one. Then say DONE."` By Opus 5.5 (implementer), on the operator's Mac.

What it showed (`hook.log`, `codex-stderr.txt`, `codex-stdout.txt`):
- `SessionStart` fired at the first turn (`source=startup`); its stdout reached the model, which gave the codeword `KIWI-startup`.
- The first `Stop` waited for a trigger file, then exited 2 with a reason on stderr: Codex logged "Stop Blocked" and the model continued in the same turn, answering `PONG`. The next `Stop` arrived with `stop_hook_active: true`; the probe let it end.
- The hook's environment had no `CODEX_SANDBOX` (only `CODEX_MANAGED_*`): hooks run outside the sandbox.
- `probe-cmd.sh`, allowed by the rule, ran with `CODEX_SANDBOX` unset and could write outside the workspace: an allowed command runs outside the sandbox.
