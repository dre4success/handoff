import collections
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

BASE = '178b7ca4e29637bd95e7d9010dbf78d2d26bc723'
SCRUB = '9dbdf5f'
scratchpads = list(Path('/private/tmp/claude-501').glob('*/**/scratchpad/h14-scrub.py'))
assert len(scratchpads) == 1, 'expected the supplied scrub script'
script = scratchpads[0]
patterns = script.with_name('h14-patterns.txt')

def git(*args):
    return subprocess.check_output(['git', *args])

files = git('ls-files', '-z').decode().rstrip('\0').split('\0')
contents = {f: Path(f).read_bytes().decode('utf-8', 'surrogateescape') for f in files}
print('Tracked files scanned:', len(files))
for line in patterns.read_text().splitlines():
    label, pattern = line.split('\t', 1)
    rx = re.compile(pattern)
    hits = [(f, i) for f, s in contents.items() for i, line in enumerate(s.splitlines(), 1) if rx.search(line)]
    names = [f for f in files if rx.search(f)]
    print(label + ':', len(hits), 'content lines;', len(names), 'paths')
    assert not hits and not names

rx = re.compile(r'[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}', re.I)
emails = collections.defaultdict(list)
hosts = collections.defaultdict(list)
for f, s in contents.items():
    for n, line in enumerate(s.splitlines(), 1):
        for match in rx.finditer(line):
            emails[match.group()].append((f, n))
        for match in re.finditer(r'https?://([^\s/<>\)\]"\x27]+)', line):
            hosts[match.group(1)].append((f, n))
print('Email-shaped values (review fixtures versus actual identities):')
for value, hits in sorted(emails.items()):
    print(' ', value, len(hits), 'lines; first:', hits[0])
print('URL hosts:', ', '.join(sorted(hosts)))
for label, pattern in [
    ('private key', r'-----BEGIN (?:OPENSSH |RSA |EC |DSA )?PRIVATE KEY-----'),
    ('common credential prefix', r'\b(?:ghp_|github_pat_|sk-proj-|AKIA)[A-Za-z0-9_-]{15,}'),
    ('user home path', r'/(?:Users|home)/[^\s/]+'),
    ('host identifying a local machine', r'\b[^\s/]*\.(?:local|lan)\b'),
]:
    rx = re.compile(pattern)
    hits = [(f, i) for f, s in contents.items() for i, line in enumerate(s.splitlines(), 1) if rx.search(line)]
    print(label + ':', len(hits), 'lines;', hits[:12])
print('NUL-containing tracked files:', [f for f, s in contents.items() if '\0' in s])

print('\nReproduce supplied scrub on a separate archive of H13:')
root = Path(tempfile.mkdtemp(prefix='handoff-h14-scrub-', dir='/private/tmp'))
with tarfile.open(fileobj=io.BytesIO(git('archive', BASE))) as archive:
    archive.extractall(root, filter='data')
env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
env.update(GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1')
for args in (['git', 'init', '-q', '-b', 'main'], ['git', 'add', '-A'], ['python3', '-I', str(script), str(root)]):
    p = subprocess.run(args, cwd=root, env=env, capture_output=True, text=True, check=True)
    print(p.stdout.strip())
expected = {}
for entry in git('ls-tree', '-rz', '--full-tree', SCRUB).split(b'\0'):
    if not entry:
        continue
    meta, name = entry.split(b'\t', 1)
    mode, kind, oid = meta.decode().split()
    assert kind == 'blob'
    expected[name.decode()] = (mode, oid)
actual_names = set(subprocess.check_output(['git', 'ls-files', '-z'], cwd=root).decode().rstrip('\0').split('\0'))
assert actual_names == set(expected), 'renamed path sets differ'
mismatches = []
for f, (mode, oid) in expected.items():
    path = root / f
    data = path.read_bytes()
    actual_oid = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()
    actual_mode = '100755' if path.stat().st_mode & 0o100 else '100644'
    if (actual_mode, actual_oid) != (mode, oid):
        mismatches.append(f)
print('Reproduction directory:', root)
print('Content/mode mismatches:', mismatches)
assert not mismatches
print('PASS: supplied script exactly reproduces scrub tree, paths, bytes and file modes.')

print('\nRead-only archived trial chain check, same byte hashing as trial/collect.sh:')
for revision in (BASE, 'HEAD'):
    lines = git('show', revision + ':evidence/trial/record/channel.jsonl').split(b'\n')[:-1]
    head = json.loads(git('show', revision + ':evidence/trial/record/head.json'))
    prev, broken = 'genesis', []
    for n, line in enumerate(lines, 1):
        e = json.loads(line)
        if e['prev'] != prev:
            broken.append(n)
        prev = hashlib.sha256(line).hexdigest()
    print(revision, 'mismatched prev at lines:', broken,
          'head line count matches:', head['lines'] == len(lines),
          'head last matches:', head['last'] == prev)
    if revision == BASE:
        assert not broken and head['last'] == prev
    else:
        assert broken, 'expected to reproduce the suspected archive regression'
