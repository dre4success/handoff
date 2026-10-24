# Which session ids each CLI gives commands and hooks

Run by Opus 5.5 (implementer) on the operator's Mac, 2026-10-21, before the H12 code: throwaway one-turn sessions in scratch git repositories (Claude Code 2.1.302 with Haiku; Codex 0.162.1 with gpt-6-luna at low effort).

- Claude Code (`claude-runs.txt`, `claude-hooks.log`): a project whose `.claude/settings.local.json` ran `hook-dump.sh` on SessionStart and Stop. The model ran `printenv CLAUDE_CODE_SESSION_ID CLAUDE_PID`. The id it printed is the hook's stdin `session_id`. The hooks also have both variables, and their parent is `CLAUDE_PID`. A second run with `--resume` kept the session id in a new process (a new `CLAUDE_PID`).
- Codex (`codex-run.txt`): the model ran `printenv CODEX_THREAD_ID CODEX_SESSION_ID`. `CODEX_THREAD_ID` is the session id Codex printed and its transcript's name; `CODEX_SESSION_ID` is unset. `evidence/h10b/probe/hook.log` shows the same id as a Codex hook's stdin `session_id`, and that a hook's environment has no `CODEX_THREAD_ID`.
- Not tried: `/clear` in Claude Code and a new thread in Codex, which need an interactive session. The design assumes a new id in the same process, and `TestH12ARoleOutlivesClearAndResume` and `TestH12ACodexRoleOutlivesANewThread` test that case.
