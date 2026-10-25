#!/usr/bin/env bash
# The reviewer's H18 probes: what git says in each kind of failure, and what
# handoff then says. Run from a checkout of the commit under review:
#   OUT=<this directory> bash probes.sh
# Everything happens in a scratch folder (and, for one probe, a 4 MB disk
# image mounted under /Volumes and detached afterwards). Alerts and sleep
# holds go to scratch files. Git speaks English (LC_ALL=C) unless a probe
# says otherwise, and reads no config of the operator's.
set -u
OUT=${OUT:?}
COMMIT=$(git rev-parse HEAD)
WT=$(pwd -P)
BY="Fable 5.1 (reviewer), Claude Code, on the operator's Mac (darwin arm64, $(go version | awk '{print $3}'), $(git --version | awk '{print $3}'))"
SCRATCH=$(cd "$(mktemp -d)" && pwd -P)
MP=
cleanup() { [ -n "$MP" ] && hdiutil detach -quiet "$MP" 2>/dev/null; rm -rf "$SCRATCH"; }
trap cleanup EXIT
export HANDOFF_TEST_NOTIFY="$SCRATCH/alerts.txt" HANDOFF_TEST_AWAKE="$SCRATCH/awake.txt"
unset HANDOFF_ROLE CLAUDE_CODE_SESSION_ID CLAUDE_PID CODEX_THREAD_ID GIT_DIR GIT_CEILING_DIRECTORIES GIT_DISCOVERY_ACROSS_FILESYSTEM
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 LC_ALL=C LANG=C
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
go build -o "$SCRATCH/handoff" ./cmd/handoff || exit 1
H="$SCRATCH/handoff"

# say <label> <dir> [VAR=value...]: the git call handoff makes, in that
# folder and environment, then `handoff init` there.
say() {
  local label=$1 dir=$2; shift 2
  echo "## $label"
  local gt; gt=$(cd "$dir" && env "$@" git rev-parse --path-format=absolute --git-common-dir 2>&1; echo "[exit $?]")
  echo "$gt" | sed 's/^/  git: /'
  case "$gt" in
    *"not a git repository (or any"*) echo "  git's words contain \"not a git repository (or any\": yes";;
    *"not a git repository"*) echo "  git's words contain \"not a git repository\": yes, but not \"(or any\"";;
    *) echo "  git's words contain \"not a git repository\": no";;
  esac
  (cd "$dir" && env "$@" "$H" init </dev/null 2>&1; echo "[exit $?]") | sed 's/^/  handoff: /'
}

{
printf '# command: bash probes.sh (the git call handoff makes, then handoff init, in each situation; built from HEAD)\n# date: %s\n# by: %s\n# commit: %s\n\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" "$BY" "$COMMIT"

mkdir "$SCRATCH/p1"
say "P1 a folder in no repository (no ceiling; under $(dirname "$SCRATCH" | sed "s|$HOME|~|"))" "$SCRATCH/p1"
echo "  then: git init -q -b main"
(cd "$SCRATCH/p1" && git init -q -b main && "$H" init </dev/null >"$SCRATCH/o" 2>&1; echo "  [exit $?]"; head -2 "$SCRATCH/o" | sed 's/^/  handoff init: /'; "$H" inbox </dev/null 2>&1 | sed 's/^/  handoff inbox with no commit yet: /'; echo "  [exit ${PIPESTATUS[0]}]")

mkdir "$SCRATCH/p2"
say "P2 a folder in no repository, GIT_CEILING_DIRECTORIES set as the test sets it" "$SCRATCH/p2" GIT_CEILING_DIRECTORIES="$SCRATCH"

if hdiutil create -quiet -size 4m -fs HFS+ -volname h18probe "$SCRATCH/vol.dmg" 2>/dev/null && MP=$(hdiutil attach -nobrowse -readwrite "$SCRATCH/vol.dmg" 2>/dev/null | awk '/\/Volumes\//{print $NF}') && [ -n "$MP" ]; then
  mkdir "$MP/proj"
  say "P3 a folder in no repository on a volume of its own ($MP, a scratch disk image; device $(stat -f %d "$MP") vs $(stat -f %d /Volumes) for /Volumes)" "$MP/proj"
  hdiutil detach -quiet "$MP" && MP=
else
  echo "## P3 skipped: no scratch volume could be mounted"
fi

mkdir "$SCRATCH/p4"
say "P4 a folder in no repository, git speaking German" "$SCRATCH/p4" LC_ALL=de_DE.UTF-8 LANG=de_DE.UTF-8 LANGUAGE=de

mkdir "$SCRATCH/A" && (cd "$SCRATCH/A" && git init -q -b main && echo x > f && git add f && git commit -qm base && git worktree add -q "$SCRATCH/p5") && rm -rf "$SCRATCH/A"
echo "## P5 a linked worktree whose repository was removed (.git is a file: $(cat "$SCRATCH/p5/.git" | sed "s|$SCRATCH|<scratch>|"))"
say "P5 (continued)" "$SCRATCH/p5" | tail -n +2
echo "  then, as the hint says: git init -q -b main"
(cd "$SCRATCH/p5" && git init -q -b main 2>&1 | sed 's/^/  git init: /'; echo "  [exit ${PIPESTATUS[0]}]"; "$H" init </dev/null 2>&1 | sed 's/^/  handoff init again: /')

mkdir "$SCRATCH/p6"
say "P6 GIT_DIR naming a path that does not exist" "$SCRATCH/p6" GIT_DIR="$SCRATCH/nowhere"
echo "  then, as the hint says: git init -q -b main (with GIT_DIR still set)"
(cd "$SCRATCH/p6" && GIT_DIR="$SCRATCH/nowhere" git init -q -b main 2>&1 | sed 's/^/  git init: /'; echo "  [exit ${PIPESTATUS[0]}]"; GIT_DIR="$SCRATCH/nowhere" "$H" init </dev/null 2>&1 | sed 's/^/  handoff init again: /')

mkdir "$SCRATCH/p7" && (cd "$SCRATCH/p7" && git init -q -b main)
say "P7 a repository owned by someone else (GIT_TEST_ASSUME_DIFFERENT_OWNER=1)" "$SCRATCH/p7" GIT_TEST_ASSUME_DIFFERENT_OWNER=1

mkdir "$SCRATCH/p8" && (cd "$SCRATCH/p8" && git init -q -b main && rm .git/HEAD)
say "P8 a repository whose .git/HEAD was removed" "$SCRATCH/p8"
echo "  then, as the hint says: git init -q -b main"
(cd "$SCRATCH/p8" && git init -q -b main 2>&1 | sed 's/^/  git init: /'; echo "  [exit ${PIPESTATUS[0]}]"; "$H" init </dev/null >"$SCRATCH/o" 2>&1; echo "  [exit $?]"; head -1 "$SCRATCH/o" | sed 's/^/  handoff init again: /')

mkdir "$SCRATCH/p9" && (cd "$SCRATCH/p9" && git init -q --bare -b main)
say "P9 a bare repository" "$SCRATCH/p9"

mkdir "$SCRATCH/p10"
echo "## P10 outside any repository: version, --version, help, settings claude"
(cd "$SCRATCH/p10" && for c in version --version; do printf '  handoff %s: %s [exit %s]\n' "$c" "$("$H" $c 2>&1)" "$?"; done; "$H" help </dev/null >/dev/null 2>&1; echo "  handoff help: exit $?, $("$H" help 2>&1 | wc -l | tr -d ' ') lines"; "$H" settings claude </dev/null 2>&1 | sed 's/^/  handoff settings claude: /'; echo "  [exit ${PIPESTATUS[0]}]")

echo "## P11 init's lines in a fresh repository, as printed; each, after its name, through /bin/sh"
mkdir "$SCRATCH/p11" && (cd "$SCRATCH/p11" && git init -q -b main && "$H" init </dev/null 2>&1 | sed -n '/To keep/,$p' | sed 's/^/  | /')
(cd "$SCRATCH/p11" && "$H" init </dev/null 2>/dev/null; rm -rf .handoff; "$H" init 2>/dev/null | sed -n -E 's/^  (Claude Code|Codex): *//p' | while read -r line; do
  /bin/sh -c "$line" >"$SCRATCH/out.txt" 2>&1; echo "  sh -c '$line': exit $?, $(wc -l <"$SCRATCH/out.txt" | tr -d ' ') lines; mentions \"hook listen\": $(grep -c 'hook listen' "$SCRATCH/out.txt")"
done)
mkdir "$SCRATCH/bin" && cp "$H" "$SCRATCH/bin/handoff" && mkdir "$SCRATCH/p11b" && (cd "$SCRATCH/p11b" && git init -q -b main && PATH="$SCRATCH/bin:$PATH" handoff init </dev/null 2>&1 | sed -n '/To keep/,$p' | sed 's/^/  on PATH | /')

echo
echo "# alerts sent to the scratch file during these probes: $(cat "$SCRATCH/alerts.txt" 2>/dev/null | wc -l | tr -d ' ')"
} 2>&1 | sed "s|$SCRATCH|<scratch>|g; s|$WT|<worktree>|g; s|$HOME|~|g" > "$OUT/probes.log"
