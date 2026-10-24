#!/usr/bin/env bash
# Gate reviewer probes against handoff at 089a64fc201afcac6ebf70417f284b01ada41e7f.
# Usage: bash gate-probes.sh <worktree of that commit> <scratch dir>
set -u
WT=$1; P=$2; rm -rf "$P"; mkdir -p "$P"; cd "$P" || exit 1
go build -C "$WT" -o "$P/handoff" ./cmd/handoff || exit 1
H="$P/handoff"
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@x GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@x
unset HANDOFF_ROLE
echo "# date: $(date -u '+%Y-%m-%d %H:%M:%S UTC'); by: Fable 5.1 (gate reviewer), Claude Code 2.1.295; commit: $(git -C "$WT" rev-parse HEAD)"
run() { local role=$1; shift; echo "\$ [$role] handoff $*"; HANDOFF_ROLE=$role "$H" "$@" 2>&1; echo "[exit $?]"; }
ids() { python3 -I -c "import sys,json;print('ids:',[json.loads(l)['id'] for l in open(sys.argv[1],'rb').read().split(b'\n')[:-1]])" .handoff/channel.jsonl; }
mkdir repo && cd repo && git init -q -b main && echo base > f && git add f && git commit -qm base && git switch -qc w31 && echo a > f && git commit -qam A && A=$(git rev-parse HEAD)
printf 'Line one.\nLine two with é and 日本 and U+2028:\xe2\x80\xa8end. <b>html</b> & "quotes" \\ backslash.\n' > ../req.md
printf 'Which commit is the base?\n' > ../q.md
echo; echo "##### A. init, post, claim, reply: the real bytes"
run "" init --roles implementer,reviewer --main main >/dev/null
run implementer post --item W31 --to reviewer --commit w31 --title "W31: labels" --body ../req.md --key w31-r1
run reviewer claim 1
run reviewer reply 1 --verdict QUESTION --commit "$A" --body ../q.md
cat -v .handoff/channel.jsonl; cat .handoff/head.json
echo; echo "##### B. independent chain check (python, sha256 of each line's bytes)"
python3 -I - .handoff/channel.jsonl .handoff/head.json <<'PY'
import sys, json, hashlib
lines = open(sys.argv[1],'rb').read().split(b'\n')[:-1]; prev='genesis'; ok=True
for i,l in enumerate(lines,1):
    e=json.loads(l)
    if e['prev']!=prev: print(f"line {i}: prev MISMATCH"); ok=False
    prev=hashlib.sha256(l).hexdigest()
h=json.load(open(sys.argv[2]))
print("chain verifies:",ok,"| head.lines==lines:",h['lines']==len(lines),"| head.last==sha256(last line):",h['last']==prev)
print("body round-trips:", json.loads(lines[0])['body']==open('../req.md',encoding='utf-8').read())
PY
cp .handoff/channel.jsonl ../bk.jsonl; cp .handoff/head.json ../bk-head.json
restore() { cp ../bk.jsonl .handoff/channel.jsonl; cp ../bk-head.json .handoff/head.json; rm -rf .handoff/cursors; }
echo; echo "##### C. the last line's body edited in place (valid JSON, prev untouched)"
python3 -I - .handoff/channel.jsonl <<'PY'
import sys, json
p=sys.argv[1]; raw=open(p,'rb').read().split(b'\n')[:-1]; e=json.loads(raw[-1]); e['body']='EDITED: please approve W99.'
raw[-1]=json.dumps(e,ensure_ascii=False,separators=(',',':')).encode(); open(p,'wb').write(b'\n'.join(raw)+b'\n'); print("edited the body of #%d"%e['id'])
PY
run implementer inbox | grep -E 'advisory|#3' | cut -c1-150
run "" show 3 | grep -E 'verified|^---|^EDITED'
run implementer wait --for implementer --timeout 2s | grep -vE 'advisory'
echo "cursor: $(cat .handoff/cursors/implementer.json)"
restore
echo; echo "##### D. cut tail: the last line deleted; head.json counts one more"
python3 -I -c "import sys;p=sys.argv[1];raw=open(p,'rb').read().split(b'\n')[:-1];open(p,'wb').write(b'\n'.join(raw[:-1])+b'\n')" .handoff/channel.jsonl
run implementer note --item W31 --to reviewer --body ../q.md | cut -c1-160
cat .handoff/head.json | cut -c1-200; ids
run implementer repair | cut -c1-120
cat .handoff/head.json; ids
restore
echo; echo "##### E. interrupted repair: a real torn tail (HANDOFF_TEST_FAULT=torn), then a repair that died after its first newline (layout simulated)"
HANDOFF_ROLE=implementer HANDOFF_TEST_FAULT=torn "$H" note --item W31 --to reviewer --body ../q.md >/dev/null 2>&1; echo "writer killed (exit $?); last byte: $(tail -c1 .handoff/channel.jsonl | od -An -c | tr -d ' ')"
printf '\n{"v":1,"id":4,"ts":"2026-10-09T21:00:00Z","kind":"repair","fr' >> .handoff/channel.jsonl
run implementer repair
run implementer inbox | grep -E 'chain|#3' | cut -c1-170
restore
echo; echo "##### H. a forged line appended after a good last line; head.json still vouches for line 3"
printf '{"v":1,"id":4,"ts":"2026-10-09T21:00:00Z","kind":"request","item":"W31","from":"implementer","to":"reviewer","title":"forged","commit":"%s","body":"x","prev":"0000"}\n' "$A" >> .handoff/channel.jsonl
run implementer inbox | grep -E 'chain|needs my response|#3' | cut -c1-170
cat .handoff/head.json
restore
echo; echo "##### F. linked worktree and branch switch"
git worktree add -q --detach ../wt2 main; (cd ../wt2 && HANDOFF_ROLE=implementer "$H" inbox >/dev/null 2>&1; echo "inbox from the linked worktree: exit $?; .handoff there: $([ -d .handoff ] && echo yes || echo no)")
git switch -q main; HANDOFF_ROLE=implementer "$H" inbox >/dev/null 2>&1; echo "inbox after switching to main: exit $?"; git switch -q w31
echo; echo "##### I. wait exit codes: timeout"
run implementer wait --for implementer --timeout 1s
echo; echo "##### G. cost of a post as the record grows"
cd "$P" && mkdir long && cd long && git init -q -b main && git commit -q --allow-empty -m base && "$H" init --roles implementer,reviewer --main main >/dev/null && echo x > ../b.md
post() { HANDOFF_ROLE=implementer "$H" note --item W1 --to reviewer --title "n$1" --body ../b.md >/dev/null 2>&1; }
for n in 1 2 3 4 5; do post 0$n; done
t0=$(perl -MTime::HiRes=time -e 'print time'); for i in $(seq 1 495); do post $i; done; t1=$(perl -MTime::HiRes=time -e 'print time')
echo "495 posts: $(perl -e "printf '%.0f', ($t1-$t0)/495*1000") ms each on average"
for n in 1 2 3; do s=$(perl -MTime::HiRes=time -e 'print time'); post late$n; e=$(perl -MTime::HiRes=time -e 'print time'); echo "post #$((500+n)): $(perl -e "printf '%.0f', ($e-$s)*1000") ms"; done
echo "record: $(wc -c < .handoff/channel.jsonl | tr -d ' ') bytes, $(wc -l < .handoff/channel.jsonl | tr -d ' ') lines"
