# Codex probe for round 3 (Run, 2026-10-18 22:20 UTC)

Does a project's `.codex/config.toml` give the commands a Codex session runs a variable? Asked after GPT-6's H10b CHANGES (#45): with no role, a bare `handoff` that the rule allows can reach the operator's question.

A throwaway `codex exec` (codex-cli 0.162.1, gpt-6-luna, low effort) in a scratch git project `<probe>`, with only `config.toml` (here) in its `.codex/`. Started with a clean environment, so nothing came from the shell that started it:

`env -i HOME="$HOME" PATH="$PATH" USER="$USER" LANG=en_US.UTF-8 TERM=xterm codex exec -m gpt-6-luna -c model_reasoning_effort=low --sandbox workspace-write "Run exactly this shell command once: env | grep -E '^(HANDOFF|CODEX|CLAUDE)' | sort ; then print its output verbatim and nothing else."`

By Opus 5.5 (implementer), on the operator's Mac.

What it showed (`codex-stdout.txt`, `codex-stderr.txt`):
- The command saw `HANDOFF_ROLE=codex`, from `config.toml`. Codex logged no warning about an ignored project setting.
- Codex also gives its commands `CODEX_THREAD_ID`, `CODEX_SESSION_ID` and `CODEX_CI=1`, and in the sandbox `CODEX_SANDBOX=seatbelt`.

`codex exec` loaded this project's config without a trust prompt. In the TUI, Codex reads a project's `.codex/` only once the project is trusted.
