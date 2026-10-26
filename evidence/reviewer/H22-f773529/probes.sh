#!/usr/bin/env bash
# The reviewer's probes for H22 (f773529): what `init` says about hooks in
# every setup, before (b6d164e, H21) and after. Run as
#   S=<scratch> OUT=<this dir> bash probes.sh
# with $S/h22/before/handoff built from b6d164e and $S/h22/after/handoff from
# f773529. Every CLI setting it reads is a scratch file; the operator's own
# settings are never read, since CLAUDE_CONFIG_DIR, CODEX_HOME or HOME point
# at scratch in every run.
set -u
S=${S:?scratch}; OUT=${OUT:?out}
BEFORE=$S/h22/before/handoff; AFTER=$S/h22/after/handoff
COMMIT=f77352941b7d26e221d17c3ba20c0ad726f565e4
WORK=$(mktemp -d "$S/h22/probes.XXXXXX"); WORK=$(cd "$WORK" && pwd -P)
export HANDOFF_TEST_NOTIFY="$WORK/alerts.txt" HANDOFF_TEST_AWAKE="$WORK/awake.txt" LC_ALL=C
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID CLAUDE_CONFIG_DIR CODEX_HOME
header() { printf '# command: %s\n# date: %s\n# by: Fable 5.1 (reviewer), Claude Code, on the operator'"'"'s Mac (darwin arm64, %s, %s)\n# commit: %s\n\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$(go version | awk '{print $3}')" "$(git --version | awk '{print $3}')" "$COMMIT"; }
clean() { sed "s|$BEFORE|handoff|g; s|$AFTER|handoff|g; s|$WORK|<scratch>|g"; }

NONE="$WORK/none"; SET="$WORK/set"
mkdir -p "$NONE/claude" "$NONE/codex" "$SET/claude" "$SET/codex" "$SET/bad/claude" "$SET/other/claude" "$SET/args/claude" "$SET/nostop/codex" "$SET/home/.claude" "$SET/home/.codex"
# User settings, as settings --global prints them.
CLAUDE_CONFIG_DIR="$SET/claude" "$AFTER" settings claude --global | sed -n '/^{/,$p' > "$SET/claude/settings.json"
out=$(CODEX_HOME="$SET/codex" "$AFTER" settings codex --global)
printf '%s\n' "$out" | awk '/\/hooks\.json:$/{f=1;next} f&&/^$/{exit} f' > "$SET/codex/hooks.json"
# Per-project settings, as settings (no --global) prints them in a project.
p=$(mktemp -d "$WORK/proj.XXXXXX"); git -C "$p" init -q -b main; git -C "$p" commit -q --allow-empty -m base
(cd "$p" && CLAUDE_CONFIG_DIR="$NONE/claude" CODEX_HOME="$NONE/codex" "$AFTER" init >/dev/null)
(cd "$p" && CLAUDE_CONFIG_DIR="$NONE/claude" "$AFTER" settings claude) | sed -n '/^{/,$p' > "$SET/proj-claude.json"
(cd "$p" && CODEX_HOME="$NONE/codex" "$AFTER" settings codex) | awk '/\/hooks\.json:$/{f=1;next} f&&/^$/{exit} f' > "$SET/proj-codex-hooks.json"
# Other shapes.
echo '{' > "$SET/bad/claude/settings.json"
echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"cc-status stop"}]}]}}' > "$SET/other/claude/settings.json"
echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"handoff","args":["hook","listen"]}]}]}}' > "$SET/args/claude/settings.json"
python3 -I -c '
import json,sys
h=json.load(open(sys.argv[1])); h["hooks"].pop("Stop",None); json.dump(h,open(sys.argv[2],"w"),indent=2)' "$SET/codex/hooks.json" "$SET/nostop/codex/hooks.json"
cp "$SET/claude/settings.json" "$SET/home/.claude/settings.json"; cp "$SET/codex/hooks.json" "$SET/home/.codex/hooks.json"

proj_claude() { mkdir -p "$1/.claude"; cp "$SET/proj-claude.json" "$1/.claude/settings.json"; }
proj_claude_local() { mkdir -p "$1/.claude"; cp "$SET/proj-claude.json" "$1/.claude/settings.local.json"; }
proj_codex() { mkdir -p "$1/.codex"; cp "$SET/proj-codex-hooks.json" "$1/.codex/hooks.json"; }
proj_both() { proj_claude "$1"; proj_codex "$1"; }

n=0; both=0; once=0
newrepo() { n=$((n+1)); REPO="$WORK/p$n"; mkdir -p "$REPO"; git -C "$REPO" init -q -b main; git -C "$REPO" commit -q --allow-empty -m base; }
INITARGS=()
# case <label> <project setup fn or -> <CLAUDE_CONFIG_DIR or -> <CODEX_HOME or -> <HOME or -> <subdir or ->
case_run() {
  local label=$1 setup=$2 claude=$3 codex=$4 home=$5 sub=$6 bin b d
  echo; echo "## $label"
  for bin in before after; do
    b="$S/h22/$bin/handoff"; newrepo; d=$REPO
    [ "$setup" != - ] && "$setup" "$d"
    [ "$sub" != - ] && mkdir -p "$d/$sub"
    (cd "$d/${sub#-}" && env ${claude/#-/} ${codex/#-/} ${home/#-/} "$b" init ${INITARGS[@]+"${INITARGS[@]}"}) > "$WORK/out-$bin" 2>&1; echo "# exit $?" >> "$WORK/out-$bin"
    sed "s|$d|<repo>|g; s|$b|handoff|g" "$WORK/out-$bin" > "$WORK/norm-$bin"
  done
  echo "### after:"; cat "$WORK/out-after"
  if cmp -s "$WORK/norm-before" "$WORK/norm-after"; then echo "### before (b6d164e): identical, but for the repo's path"; else echo "### before (b6d164e), where it differs:"; diff "$WORK/norm-before" "$WORK/norm-after" | grep '^[<>]'; fi
  [ "$sub" != - ] && { [ -d "$d/.handoff" ] && echo "### from $sub: the record is at the top level" || echo "### from $sub: NO record at the top level"; }
  grep -q "Both CLIs have them:" "$WORK/out-after" && both=$((both+1))
  grep -q "Set them once, for every project:" "$WORK/out-after" && once=$((once+1))
  return 0
}
C="CLAUDE_CONFIG_DIR=" X="CODEX_HOME="

{
  header "init in 15 setups, built from b6d164e (before) and f773529 (after); user settings from settings --global, project settings from settings, and other shapes"
  echo "settings files made for the probes:"; wc -l "$SET/claude/settings.json" "$SET/codex/hooks.json" "$SET/proj-claude.json" "$SET/proj-codex-hooks.json" | clean
  echo "user Claude Code Stop hook: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["hooks"]["Stop"][0]["hooks"][0]["command"])' "$SET/claude/settings.json" | clean)"
  echo "user Codex Stop hook: $(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["hooks"]["Stop"][0]["hooks"][0]["command"])' "$SET/codex/hooks.json" | clean)"
  echo "the no-Stop Codex file's events: $(python3 -I -c 'import json,sys; print(sorted(json.load(open(sys.argv[1]))["hooks"]))' "$SET/nostop/codex/hooks.json")"

  echo; echo "# Part 1: the three setups the implementer's evidence covers"
  case_run "a. nothing set up" - $C$NONE/claude $X$NONE/codex - -
  case_run "b. Codex alone set up (user)" - $C$NONE/claude $X$SET/codex - -
  case_run "c. both set up (user)" - $C$SET/claude $X$SET/codex - -

  echo; echo "# Part 2: setups the evidence does not cover"
  case_run "d. Claude Code alone set up (user)" - $C$SET/claude $X$NONE/codex - -
  case_run "e. both set up in the project's own files (.claude/settings.json, .codex/hooks.json); user settings empty" proj_both $C$NONE/claude $X$NONE/codex - -
  case_run "f. Claude Code in the user's settings, Codex in the project's .codex/hooks.json" proj_codex $C$SET/claude $X$NONE/codex - -
  case_run "g. Claude Code in the project's .claude/settings.local.json, Codex in the user's" proj_claude_local $C$NONE/claude $X$SET/codex - -
  case_run "h. both in the user's home (~/.claude, ~/.codex), CLAUDE_CONFIG_DIR and CODEX_HOME unset, HOME at scratch" - - - HOME=$SET/home -
  case_run "i. both set up (user), init run from a subfolder of the project" - $C$SET/claude $X$SET/codex - sub/dir
  INITARGS=(--roles a,b --reviewers b)
  case_run "j. both set up (user), init --roles a,b --reviewers b" - $C$SET/claude $X$SET/codex - -
  INITARGS=()
  case_run "k. Claude Code's user settings.json is not JSON ('{'); Codex set up" - $C$SET/bad/claude $X$SET/codex - -
  case_run "l. Claude Code's user settings.json has a Stop hook that is not handoff's; Codex set up" - $C$SET/other/claude $X$SET/codex - -
  case_run "m. Claude Code's user Stop hook is command \"handoff\" with args [\"hook\",\"listen\"]; Codex set up" - $C$SET/args/claude $X$SET/codex - -
  case_run "n. Codex's user hooks.json has handoff's hooks but no Stop hook; Claude Code set up" - $C$SET/claude $X$SET/nostop/codex - -
  case_run "o. Claude Code set up in both the user's and the project's settings (which file init names)" proj_claude $C$SET/claude $X$SET/codex - -

  echo; echo "# Part 3: init a second time, both set up (after only)"
  newrepo; d=$REPO; (cd "$d" && env $C$SET/claude $X$SET/codex "$AFTER" init >/dev/null); (cd "$d" && env $C$SET/claude $X$SET/codex "$AFTER" init); echo "# exit $?"

  echo; echo "# tally: after says \"Both CLIs have them:\" in $both setups and \"Set them once, for every project:\" in $once, of $((n/2))"
  echo "# alerts sent: $(cat "$WORK/alerts.txt" 2>/dev/null | wc -l | tr -d ' ') (to the test file only)"
} 2>&1 | clean > "$OUT/p1-init-wording.log"
echo "probes done"
rm -rf "$WORK"
