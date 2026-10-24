import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

BASE = 'e1a1e5021e885208669aba807fbbe1ad1b05ac3a'
SCRUB = 'c8b0163'
H14 = '82d98a7c634d57e12725a8f2a0165894e8937fb8'
MERGE = 'a9ada46407cab06f53bd1f7445b9045c628a610e'
env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
env.update(GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1')
scripts = list(Path('/private/tmp/claude-501').glob('*/**/scratchpad/h14-scrub.py'))
assert len(scripts) == 1
script = scripts[0]
patterns = [line.split('\t', 1) for line in script.with_name('h14-patterns.txt').read_text().splitlines()]

def git(*args, cwd=None):
    return subprocess.check_output(['git', *args], cwd=cwd, env=env)

def tree(revision):
    result = {}
    for entry in git('ls-tree', '-rz', '--full-tree', revision).split(b'\0'):
        if entry:
            meta, name = entry.split(b'\t', 1)
            mode, kind, oid = meta.decode().split()
            assert kind == 'blob' and mode in ('100644', '100755')
            result[name.decode()] = (mode, oid)
    return result

def extract(revision, prefix):
    root = Path(tempfile.mkdtemp(prefix=prefix, dir='/private/tmp'))
    with tarfile.open(fileobj=io.BytesIO(git('archive', revision))) as archive:
        archive.extractall(root, filter='data')
    return root

def init(root):
    git('init', '-q', '-b', 'main', cwd=root)
    git('add', '-A', cwd=root)

def scrub(root):
    p = subprocess.run(['python3', '-I', str(script), str(root)], cwd=root,
                       env=env, capture_output=True, text=True, check=True)
    print(p.stdout.strip())

def sweep(root, names, label):
    contents = {f: (root / f).read_text(errors='surrogateescape') for f in names}
    print(label + ':', len(names), 'files')
    for title, pattern in patterns:
        rx = re.compile(pattern)
        lines = [(f, n) for f, text in contents.items() for n, line in enumerate(text.splitlines(), 1) if rx.search(line)]
        files = [f for f in names if rx.search(f)]
        print(' ', title + ':', len(lines), 'lines;', len(files), 'paths')
        assert not lines and not files
    return contents

revision = git('rev-parse', 'HEAD').decode().strip()
expected = tree(revision)
subprocess.run(['git', 'diff', '--exit-code', H14, revision, '--',
                'cmd', 'internal', 'go.mod', 'go.sum', 'LICENSE'], env=env, check=True)
print('PASS: product source, tests, module and license identical to approved H14.')
subprocess.run(['git', 'diff', '--check', BASE, revision, '--', '.', ':(exclude)evidence'], env=env, check=True)
print('PASS: source/docs whitespace.')

current = sweep(Path.cwd(), sorted(expected), 'Requested tracked tree')
emails = sorted(set(re.findall(r'[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}', '\n'.join(current.values()), re.I)))
assert emails == ['review@test.invalid', 'reviewer@example.invalid']
print('PASS: email-shaped values are only the two documented fixture addresses.')
for title, pattern in [('private key', r'-----BEGIN (?:OPENSSH |RSA |EC |DSA )?PRIVATE KEY-----'),
                       ('credential prefix', r'\b(?:ghp_|github_pat_|sk-proj-|AKIA)[A-Za-z0-9_-]{15,}')]:
    assert not any(re.search(pattern, text) for text in current.values()), title
print('PASS: no common private-key or credential prefixes; all tracked entries are regular files.')

reproduction = extract(BASE, 'handoff-h15-scrub-')
init(reproduction)
scrub(reproduction)
reproduced_names = set(git('ls-files', '-z', cwd=reproduction).decode().rstrip('\0').split('\0'))
scrub_tree = tree(SCRUB)
assert reproduced_names == set(scrub_tree)
for name, (mode, oid) in scrub_tree.items():
    path = reproduction / name
    data = path.read_bytes()
    actual_oid = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()
    actual_mode = '100755' if path.stat().st_mode & 0o100 else '100644'
    assert (actual_mode, actual_oid) == (mode, oid), name
print('PASS: supplied scrub exactly reproduces c8b0163, including all paths, modes and bytes.')
print('Scrub reproduction retained at:', reproduction)

h14_tree, merged_tree = tree(H14), tree(MERGE)
different = {f for f in set(h14_tree) | set(merged_tree) if h14_tree.get(f) != merged_tree.get(f)}
folders = ('evidence/reviewer/H13-178b7ca/', 'evidence/reviewer/H14-48eb3b5/', 'evidence/reviewer/H14-82d98a7/')
assert all(f == 'docs/CHANNEL.md' or (f.startswith(folders) and f not in h14_tree) for f in different)
print('Merge differences from H14:', len(different), 'paths; only the board and added reviewer evidence.')
parent_tree = tree(MERGE + '^1')
for f in different - {'docs/CHANNEL.md'}:
    assert merged_tree[f] == parent_tree[f], f
print('PASS: added reviewer evidence is identical to the pre-merge main copy.')
old_board = git('show', H14 + ':docs/CHANNEL.md').splitlines()
merged_board = git('show', MERGE + ':docs/CHANNEL.md').splitlines()
without_rows = lambda lines: [l for l in lines if not l.startswith((b'| H13 |', b'| H14 |'))]
assert without_rows(old_board) == without_rows(merged_board)
board_reproduction = extract(MERGE + '^1', 'handoff-h15-board-')
init(board_reproduction)
scrub(board_reproduction)
assert (board_reproduction / 'docs/CHANNEL.md').read_bytes() == git('show', MERGE + ':docs/CHANNEL.md')
print('PASS: merged board exactly equals the supplied scrub applied to the pre-merge main board; only H13/H14 rows differ from H14.')
print('Board reproduction retained at:', board_reproduction)

export = extract(revision, 'handoff-h15-export-')
init(export)
assert not any(f.startswith(('.handoff/', 'bin/', '.git/')) for f in expected)
assert not (export / '.handoff').exists() and not (export / 'bin').exists()
assert not (export / 'reviewer_h15.py').exists()
commit_env = dict(env, GIT_AUTHOR_NAME='handoff review fixture', GIT_COMMITTER_NAME='handoff review fixture',
                  GIT_AUTHOR_EMAIL='0+handoff-review@users.noreply.github.com',
                  GIT_COMMITTER_EMAIL='0+handoff-review@users.noreply.github.com')
subprocess.run(['git', 'commit', '-q', '-m', 'offline review fixture: public export'], cwd=export, env=commit_env, check=True)
assert git('rev-list', '--count', '--all', cwd=export).strip() == b'1'
assert len(git('rev-list', '--parents', '-n', '1', 'HEAD', cwd=export).split()) == 1
assert git('rev-parse', 'HEAD^{tree}', cwd=export) == git('rev-parse', revision + '^{tree}')
assert set(git('ls-files', '-z', cwd=export).decode().rstrip('\0').split('\0')) == set(expected)
assert git('remote', cwd=export).strip() == b''
assert not (export / '.git/objects/info/alternates').exists()
identity = git('log', '-1', '--format=%ae %ce', cwd=export).decode().strip()
assert re.fullmatch(r'0\+handoff-review@users\.noreply\.github\.com 0\+handoff-review@users\.noreply\.github\.com', identity)
subprocess.run(['git', 'fsck', '--full', '--no-reflogs'], cwd=export, env=env, check=True)
print('PASS: offline export has one root commit, identical tracked tree and modes, no remote/alternates, no record/bin/untracked helper, and explicitly supplied noreply-format fixture identities.')
print('Export identity is a test fixture, not a verification of the actual account ID intended for publication.')
print('Export retained at:', export)
sweep(export, sorted(expected), 'Exported tree')
assert git('rev-parse', 'HEAD').decode().strip() == revision
print('PASS: source worktree revision unchanged; no push or source-history mutation.')
