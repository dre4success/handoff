from pathlib import Path
import re
import subprocess

common = Path(subprocess.check_output(['git', 'rev-parse', '--git-common-dir'], text=True).strip())
folder = common.parent / 'evidence/reviewer/H15-d336752'
patterns = list(Path('/private/tmp/claude-501').glob('*/**/scratchpad/h14-patterns.txt'))
assert len(patterns) == 1
files = [p for p in folder.rglob('*') if p.is_file()]
print('New reviewer evidence files scanned:', len(files))
for line in patterns[0].read_text().splitlines():
    title, pattern = line.split('\t', 1)
    rx = re.compile(pattern)
    hits = [(p.name, n) for p in files for n, line in enumerate(p.read_text(errors='surrogateescape').splitlines(), 1) if rx.search(line)]
    names = [p.name for p in files if rx.search(p.name)]
    print(title + ':', len(hits), 'lines;', len(names), 'paths')
    assert not hits and not names
print('PASS: new reviewer evidence needs no additional personal-reference scrub.')
