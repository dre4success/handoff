import hashlib
import json
from pathlib import Path
import re
import subprocess

PREVIOUS = '48eb3b5661c2d0a930ec5344d0e15b329518bdb7'
ORIGINAL = '178b7ca4e29637bd95e7d9010dbf78d2d26bc723'

def git(*args):
    return subprocess.check_output(['git', *args])

subprocess.run(['git', 'diff', '--exit-code', PREVIOUS, 'HEAD', '--',
                'cmd', 'internal', 'go.mod', 'go.sum', 'LICENSE', 'evidence/trial/record'], check=True)
print('PASS: production source, tests, module, license and archived record bytes identical to round 1.')
subprocess.run(['git', 'diff', '--check', PREVIOUS, 'HEAD', '--', '.', ':(exclude)evidence'], check=True)
print('PASS: changed source/docs whitespace.')
paths = git('ls-files', '-z').decode().rstrip('\0').split('\0')
contents = {p: Path(p).read_text(errors='surrogateescape') for p in paths}
pattern_files = list(Path('/private/tmp/claude-501').glob('*/**/scratchpad/h14-patterns.txt'))
assert len(pattern_files) == 1
print('Privacy scan:', len(paths), 'tracked files and their paths')
for line in pattern_files[0].read_text().splitlines():
    label, pattern = line.split('\t', 1)
    rx = re.compile(pattern)
    hits = [(p, n) for p, text in contents.items() for n, line in enumerate(text.splitlines(), 1) if rx.search(line)]
    names = [p for p in paths if rx.search(p)]
    print(label + ':', len(hits), 'lines,', len(names), 'paths')
    assert not hits and not names

original = git('show', ORIGINAL + ':evidence/trial/record/channel.jsonl').splitlines()
current = Path('evidence/trial/record/channel.jsonl').read_bytes().splitlines()
changed = [i for i, (a, b) in enumerate(zip(original, current), 1) if a != b]
assert len(original) == len(current) == 8 and changed == [1, 7]
print('Archive differs from original only at lines:', changed)
prev, mismatches = 'genesis', []
for i, line in enumerate(current, 1):
    if json.loads(line)['prev'] != prev:
        mismatches.append(i)
    prev = hashlib.sha256(line).hexdigest()
head = json.loads(Path('evidence/trial/record/head.json').read_bytes())
assert mismatches == [2, 8] and head['last'] == prev and head['lines'] == len(current)
print('Redacted archive link mismatches:', mismatches, '; head matches the last line and line count.')
print('PASS: observed redaction and verification limits match the corrected documentation; no record was re-chained.')
