#!/usr/bin/env bash
# probes.sh <the build of 82d04ea> <the base's build, 2967c9a> <an empty scratch folder>: the reviewer's
# offline cases for H32. Scratch HOME; alerts go to a scratch file.
set -u
BIN=$1; BASE=$2; T=$(cd "$3" && pwd -P); HERE=$(cd "$(dirname "$0")" && pwd -P)
export HOME=$T/home HANDOFF_TEST_NOTIFY=$T/alerts.txt HANDOFF_TEST_AWAKE=$T/awake.txt GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
unset CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID HANDOFF_ROLE CLAUDE_PROJECT_DIR CLAUDECODE CLAUDE_CONFIG_DIR CODEX_HOME
for v in $(env | sed -n 's/^\(ANTIGRAVITY_[A-Z_]*\)=.*/\1/p'); do unset "$v"; done
mkdir -p "$HOME"
g() { git -c user.name=probe -c user.email=probe@example.invalid "$@"; }
scrub() { sed -e "s|$T|<scratch>|g" -e "s|$BIN|handoff|g" -e "s|$BASE|handoff|g" -e "s|/private/tmp/claude[^ ]*|handoff|g"; }
printf '# command: bash probes.sh <the build of 82d04ea> <the build of the base, 2967c9a> <an empty scratch folder>\n# date: %s\n# by: Fable 5.2 (reviewer), Claude Code, on the Mac of the operator (darwin arm64, %s, git %s)\n# commit: 82d04ea0aaff11f3b156ec621457dda4baa1b32b\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')"
newproj() { # newproj <dir>: a project with one request its reviewer approved, awaiting the operator
  mkdir -p "$1" && g -C "$1" init -q -b main && g -C "$1" commit -q --allow-empty -m start
  (cd "$1" && "$BIN" init > /dev/null && echo "A request." > "$T/b.md" && echo "Fine." > "$T/r.md" \
   && HANDOFF_ROLE=implementer "$BIN" post --item W1 --to reviewer --commit HEAD --title "one" --body "$T/b.md" > /dev/null 2>&1 \
   && HANDOFF_ROLE=reviewer "$BIN" claim 1 > /dev/null && HANDOFF_ROLE=reviewer "$BIN" reply 1 --verdict APPROVE --commit "$(g -C "$1" rev-parse HEAD)" --body "$T/r.md" > /dev/null 2>&1)
}
count() { python3 -I -c 'import json,sys; print(sum(1 for l in open(sys.argv[1]) if json.loads(l).get("kind") in ("decision","fyi","away","back")))' "$1/.handoff/channel.jsonl"; }

echo "## Q1. the operator's commands, run with a terminal on stdin and one CLI's variables set"
echo "## (each line: what the command said; 'the operator's' means refused as an agent's session)"
newproj "$T/q1"
row() { # row <label> <build> VAR=value...
  local label=$1 build=$2; shift 2
  echo "-- $label"
  for c in "fyi hello" "away gone" "back" "decide W1 --approve" ""; do
    printf '   %-22s %s\n' "handoff $c" "$(cd "$T/q1" && env "$@" python3 -I "$HERE/withtty.py" "$build" $c 2>&1 | scrub | cut -c1-170)"
  done
}
row "the base's build, Antigravity's variable" "$BASE" ANTIGRAVITY_CONVERSATION_ID=c-1
echo "   entries written as the operator so far: $(count "$T/q1")"
row "this build, Antigravity's variable" "$BIN" ANTIGRAVITY_CONVERSATION_ID=c-1
row "this build, Claude Code's variables" "$BIN" CLAUDE_CODE_SESSION_ID=s-1 CLAUDE_PID=$$
row "this build, Codex's variable" "$BIN" CODEX_THREAD_ID=t-1
row "this build, Antigravity's other nine variables, without the conversation's id" "$BIN" ANTIGRAVITY_AGENT=1 ANTIGRAVITY_AGENTAPI_EXE=x ANTIGRAVITY_APP_DATA_DIR=x ANTIGRAVITY_CSRF_TOKEN=x ANTIGRAVITY_LS_ADDRESS=x ANTIGRAVITY_LS_VERSION=x ANTIGRAVITY_PROJECT_ID=x ANTIGRAVITY_SOURCE_METADATA=x ANTIGRAVITY_TRAJECTORY_ID=x
row "this build, a CLI handoff doesn't know (GEMINI_CLI=1, as Gemini CLI documents)" "$BIN" GEMINI_CLI=1
row "this build, no variable (the operator's own terminal)" "$BIN" NOTHING=1
echo "   entries written as the operator in all (the lines above answered n to each question): $(count "$T/q1")"
echo "-- this build, no terminal, Antigravity's variable: $(cd "$T/q1" && ANTIGRAVITY_CONVERSATION_ID=c-1 "$BIN" decide W1 --approve < /dev/null 2>&1 | scrub | cut -c1-150)"
echo "-- this build, no terminal, no variable:            $(cd "$T/q1" && "$BIN" decide W1 --approve < /dev/null 2>&1 | scrub | cut -c1-150)"

echo; echo "## Q2. Antigravity's hooks"
newproj "$T/q2"; mkdir -p "$T/elsewhere" "$T/notaproject"
agy() { # agy <hook> <folder to run in> <stdin JSON> [flags]: what the hook writes to stdout, and its exit
  local hook=$1 dir=$2 in=$3; shift 3
  out=$(cd "$dir" && printf '%s' "$in" | ANTIGRAVITY_CONVERSATION_ID=c-2 "$BIN" hook "$hook" "$@" 2> "$T/err.txt"; echo "[exit $?]")
  printf '%s' "$out" | tr '\n' ' ' | scrub | cut -c1-230; echo; [ -s "$T/err.txt" ] && echo "      stderr: $(head -1 "$T/err.txt" | scrub | cut -c1-150)"
}
hin() { printf '{"conversationId":"c-2","hookEventName":"%s","cwd":"%s","executionNum":%s}' "$1" "$2" "${3:-0}"; }
echo "a. SessionStart, no role, run where a user's hooks.json would be (no repository), stdin's cwd the project:"; agy brief "$T/elsewhere" "$(hin SessionStart "$T/q2")"
echo "b. the same, stdin's cwd a folder that is no project:"; agy brief "$T/elsewhere" "$(hin SessionStart "$T/notaproject")"
echo "c. the same, stdin's cwd not absolute:"; agy brief "$T/elsewhere" "$(hin SessionStart "q2")"
(cd "$T/q2" && ANTIGRAVITY_CONVERSATION_ID=c-2 "$BIN" role reviewer-2 > /dev/null)
echo "d. SessionStart with the role reviewer-2:"; agy brief "$T/elsewhere" "$(hin SessionStart "$T/q2")"
echo "   is it JSON with one system message: $(cd "$T/elsewhere" && hin SessionStart "$T/q2" | ANTIGRAVITY_CONVERSATION_ID=c-2 "$BIN" hook brief | python3 -I -c 'import json,sys; d=json.load(sys.stdin); s=d["injectSteps"]; print(len(s) == 1 and s[0]["systemMessage"]["systemMessage"].startswith("handoff brief: you are reviewer-2"))')"
echo "e. Stop, nothing waiting, --timeout 2s (stands for the 23 hours):"; agy listen "$T/elsewhere" "$(hin Stop "$T/q2" 0)" --timeout 2s
(cd "$T/q2" && echo "A note." > "$T/n.md" && HANDOFF_ROLE=implementer "$BIN" note --to reviewer-2 --item W1 --title "for you" --body "$T/n.md" > /dev/null 2>&1)
echo "f. Stop with a note waiting, executionNum 9:"; agy listen "$T/elsewhere" "$(hin Stop "$T/q2" 9)" --timeout 5s
(cd "$T/q2" && echo "Another note." > "$T/n.md" && HANDOFF_ROLE=implementer "$BIN" note --to reviewer-2 --item W1 --title "for you too" --body "$T/n.md" > /dev/null 2>&1)
: > "$T/alerts.txt"
echo "g. Stop with a note waiting, executionNum 10:"; agy listen "$T/elsewhere" "$(hin Stop "$T/q2" 10)" --timeout 5s
echo "   the operator is told: $(head -1 "$T/alerts.txt" | scrub | cut -c1-260)"
echo "h. the same again (executionNum 11): "; agy listen "$T/elsewhere" "$(hin Stop "$T/q2" 11)" --timeout 5s
echo "   told in all: $(grep -c . "$T/alerts.txt")"
echo "i. Stop with the note still waiting, executionNum 0 (the person typed):"; agy listen "$T/elsewhere" "$(hin Stop "$T/q2" 0)" --timeout 5s
echo "j. Stop for a conversation with no role:"; out=$(cd "$T/elsewhere" && printf '{"conversationId":"c-9","hookEventName":"Stop","cwd":"%s","executionNum":0}' "$T/q2" | ANTIGRAVITY_CONVERSATION_ID=c-9 "$BIN" hook listen --timeout 2s; echo "[exit $?]"); echo "$out" | tr '\n' ' '; echo
echo "k. a Claude Code Stop hook's input with a cwd and executionNum 10 in it (not Antigravity's): $(cd "$T/q2" && printf '{"session_id":"s-none","hook_event_name":"Stop","cwd":"%s","executionNum":10}' "$T/notaproject" | CLAUDE_CODE_SESSION_ID=s-none CLAUDE_PID=$$ "$BIN" hook listen --timeout 1s; echo "[exit $?]")"

echo; echo "## Q3. settings and the brief"
echo "a. handoff settings antigravity --global, outside any project: $(cd "$T/elsewhere" && "$BIN" settings antigravity --global | python3 -I -c 'import sys,json; t=sys.stdin.read(); j=json.loads(t[t.index("\n{"):]); print("valid JSON, hooks:", sorted(j["handoff"]), "| names", "~/.gemini/config/hooks.json" in t)')"
echo "b. handoff settings antigravity, outside any project: $(cd "$T/elsewhere" && "$BIN" settings antigravity 2>&1 | head -1 | scrub | cut -c1-130)"
echo "c. handoff settings antigravity --as reviewer: $(cd "$T/q2" && "$BIN" settings antigravity --as reviewer 2>&1 | head -1 | cut -c1-150)"
b() { (cd "$T/q2" && ANTIGRAVITY_CONVERSATION_ID=c-2 "$BIN" brief 2>&1) | grep -c 'settings antigravity --global'; }
echo "d. the brief by hand in an Antigravity session offers its settings: with no hooks.json: $(b)"
mkdir -p "$T/q2/.agents" && (cd "$T/q2" && "$BIN" settings antigravity | python3 -I -c 'import sys; t=sys.stdin.read(); print(t[t.index("\n{"):])' > .agents/hooks.json)
echo "   with the printed hooks in .agents/hooks.json: $(b)"
python3 -I -c 'import json,sys; p=sys.argv[1]; j=json.load(open(p)); j["handoff"]["enabled"]=False; json.dump(j, open(p,"w"))' "$T/q2/.agents/hooks.json"
echo "   with that hook turned off (enabled false): $(b)"
echo "e. the brief by hand says not to run wait: $(cd "$T/q2" && ANTIGRAVITY_CONVERSATION_ID=c-2 "$BIN" brief 2>&1 | grep -ci "don't run")"
echo; echo "alerts written to the scratch file in all: $(cat "$T/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
