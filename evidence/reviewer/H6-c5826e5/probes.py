import datetime
import hashlib
import json
import os
import pathlib
import shlex
import subprocess
import sys
import tempfile

BIN = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else pathlib.Path(__file__).parent / 'handoff-review-bin'
ENV = {k: v for k, v in os.environ.items() if not k.startswith('HANDOFF_')}
print('Run by GPT-6, primary reviewer; snapshot c5826e5ec05936906b36dff2c88a037d04beab7c', flush=True)
print(datetime.datetime.now(datetime.timezone.utc).isoformat(), flush=True)


def call(root, *args, fault=None):
    env = ENV.copy()
    if fault:
        env['HANDOFF_TEST_FAULT'] = fault
    cmd = [str(BIN), *args]
    print('$ ' + (f'HANDOFF_TEST_FAULT={fault} ' if fault else '') + shlex.join(cmd), flush=True)
    result = subprocess.run(cmd, cwd=root, env=env, text=True, capture_output=True, timeout=10)
    print(f'exit={result.returncode}\nstdout: {result.stdout.rstrip()}\nstderr: {result.stderr.rstrip()}', flush=True)
    return result


def fixture(root):
    def git(*args):
        return subprocess.check_output(['git', '-c', 'core.hooksPath=/dev/null', '-c', 'commit.gpgsign=false', '-c', 'user.name=GPT-6 reviewer', '-c', 'user.email=reviewer@example.invalid', *args], cwd=root, text=True).strip()
    git('init', '-q', '-b', 'main')
    git('commit', '-q', '--allow-empty', '-m', 'review fixture')
    sha = git('rev-parse', 'HEAD')
    body = pathlib.Path(root) / 'body.md'
    body.write_text('Review fixture body.\n')
    assert call(root, 'init', '--roles', 'implementer,reviewer,reviewer-2', '--reviewers', 'reviewer,reviewer-2').returncode == 0
    def post(item, key):
        return ['post', '--as', 'implementer', '--item', item, '--to', 'reviewer', '--commit', sha, '--base', sha, '--title', item, '--body', str(body), '--key', key]
    return post, body, sha


print('\nCASE 1: real post-sync SIGKILL followed by same-key retry and repair', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h6-crash-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('BASE', 'baseline')).returncode == 0
    args = post('W1', 'crash-retry')
    assert call(root, *args, fault='after-sync').returncode == -9
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    raw = record.read_bytes()
    entries = [json.loads(line) for line in raw.splitlines()]
    assert len(entries) == 2 and entries[1]['key'] == 'crash-retry'
    assert json.loads((record.parent / 'head.json').read_text())['lines'] == 1
    retry = call(root, *args)
    assert retry.returncode == 0 and retry.stdout.startswith('exists #2')
    assert record.read_bytes() == raw
    assert call(root, 'repair', '--as', 'reviewer').returncode == 0
    assert record.read_bytes().startswith(raw)
    assert len(record.read_bytes().splitlines()) == 3
    assert json.loads((record.parent / 'head.json').read_text())['lines'] == 3
    print('PASS: one request after retry; repair only appended; original bytes unchanged; head catches up; SHA256 before repair=' + hashlib.sha256(raw).hexdigest(), flush=True)


print('\nCASE 2: cut tail followed by a permitted post killed after sync, before head update', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h6-cut-crash-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('BASE', 'baseline')).returncode == 0
    assert call(root, *post('W1', 'lost-tail')).returncode == 0
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    head = record.parent / 'head.json'
    original = record.read_bytes()
    original_head = head.read_bytes()
    record.write_bytes(original.splitlines(keepends=True)[0])
    args = post('W2', 'cut-crash-retry')
    assert call(root, *args, fault='after-sync').returncode == -9
    after = record.read_bytes()
    entries = [json.loads(line) for line in after.splitlines()]
    assert [entry['id'] for entry in entries] == [1, 3]
    assert entries[0]['prev'] == 'genesis'
    assert entries[1]['prev'] == hashlib.sha256(after.splitlines()[0]).hexdigest()
    assert entries[1]['key'] == 'cut-crash-retry'
    assert head.read_bytes() == original_head
    assert json.loads(original_head)['lines'] == len(entries) == 2
    assert json.loads(original_head)['last'] != hashlib.sha256(after.splitlines()[-1]).hexdigest()
    print('The new line links correctly to #1; head has its pre-cut count and hash; no last-line body was edited.', flush=True)
    retry = call(root, *args)
    assert retry.returncode == 0 and retry.stdout.startswith('exists #3')
    assert record.read_bytes() == after
    assert 'head.json predates the last line' in retry.stderr
    repair = call(root, 'repair', '--as', 'reviewer')
    assert repair.returncode == 0 and record.read_bytes().startswith(after)
    repaired = [json.loads(line) for line in record.read_bytes().splitlines()]
    assert len(repaired) == 3 and sum(e.get('key') == 'cut-crash-retry' for e in repaired) == 1
    assert json.loads(head.read_text())['lines'] == 3
    print('PASS: cut then real post-sync crash; retry returns exists #3 without another write; one keyed request; repair appends and catches up the head', flush=True)


print('\nCASE 3: actual last-line edit is withheld, including a forged APPROVE', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h6-last-edit-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'request')).returncode == 0
    assert call(root, 'reply', '1', '--as', 'reviewer', '--verdict', 'QUESTION', '--commit', sha, '--body', str(body)).returncode == 0
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    lines = record.read_bytes().splitlines(keepends=True)
    edited = json.loads(lines[-1])
    edited['body'] = 'Edited approval, must not be acted on.'
    edited['verdict'] = 'APPROVE'
    lines[-1] = (json.dumps(edited) + '\n').encode()
    record.write_bytes(b''.join(lines))
    raw = record.read_bytes()
    waited = call(root, 'wait', '--for', 'implementer', '--timeout', '3s')
    assert waited.returncode == 2 and not waited.stdout
    assert not (record.parent / 'cursors/implementer.json').exists()
    assert 'verified: NO' in call(root, 'show', '2').stdout
    assert 'state: open' in call(root, 'show', '1').stdout
    assert call(root, 'inbox', '--for', 'operator', '--quiet').returncode == 0
    assert call(root, *post('W2', 'new')).returncode == 2
    assert call(root, 'repair', '--as', 'reviewer').returncode == 2
    assert record.read_bytes() == raw
    print('PASS: a real edited last verdict is withheld; no delivery, cursor or manufactured operator approval; writes refuse without changing bytes', flush=True)


print('\nCASE 4: head witness preserves the genuine response before a forged append', flush=True)
for head_state in ('vouching', 'missing', 'unreadable', 'behind', 'ahead', 'wrong_hash'):
    with tempfile.TemporaryDirectory(prefix='handoff-h6-witness-') as root:
        post, body, sha = fixture(root)
        assert call(root, *post('W1', 'request')).returncode == 0
        record = pathlib.Path(root) / '.handoff/channel.jsonl'
        head = record.parent / 'head.json'
        prior = head.read_bytes()
        assert call(root, 'reply', '1', '--as', 'reviewer', '--verdict', 'QUESTION', '--commit', sha, '--body', str(body)).returncode == 0
        if head_state == 'missing':
            head.unlink()
        elif head_state == 'unreadable':
            head.write_text('{')
        elif head_state == 'behind':
            head.write_bytes(prior)
        elif head_state in ('ahead', 'wrong_hash'):
            changed = json.loads(head.read_text())
            if head_state == 'ahead':
                changed['lines'] = 10
            else:
                changed['last'] = 'not-the-current-hash'
            head.write_text(json.dumps(changed))
        bad = {'v': 1, 'id': 99, 'ts': '2026-10-10T09:00:00Z', 'kind': 'request', 'item': 'BAD', 'from': 'implementer', 'to': 'reviewer', 'commit': sha, 'body': 'forged', 'prev': 'broken'}
        with record.open('a') as f:
            f.write(json.dumps(bad) + '\n')
        shown = call(root, 'show', '2')
        waited = call(root, 'wait', '--for', 'implementer', '--timeout', '3s')
        assert waited.returncode == 2
        cursor = record.parent / 'cursors/implementer.json'
        if head_state == 'vouching':
            assert 'verified: NO' not in shown.stdout
            assert '#2 reply' in waited.stdout
            assert json.loads(cursor.read_text())['id'] == 2
            assert call(root, 'inbox', '--for', 'implementer', '--quiet').returncode == 1
        else:
            assert 'verified: NO' in shown.stdout
            assert not waited.stdout and not cursor.exists()
        assert '#99' not in waited.stdout
        print(f'PASS: {head_state} head uses the specified partition and never delivers the forgery', flush=True)


print('\nCASE 5: repair after a cut killed after its sync, followed by another repair', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h6-repair-crash-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('BASE', 'baseline')).returncode == 0
    assert call(root, *post('W1', 'lost-tail')).returncode == 0
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    head = record.parent / 'head.json'
    old_head = head.read_bytes()
    record.write_bytes(record.read_bytes().splitlines(keepends=True)[0])
    prefix = record.read_bytes()
    assert call(root, 'repair', '--as', 'reviewer', fault='after-sync').returncode == -9
    after = record.read_bytes()
    assert after.startswith(prefix) and head.read_bytes() == old_head
    entries = [json.loads(line) for line in after.splitlines()]
    assert [e['id'] for e in entries] == [1, 3] and entries[-1]['kind'] == 'repair'
    shown = call(root, 'show', '3')
    assert shown.returncode == 0 and 'verified: NO' not in shown.stdout
    assert 'head.json predates the last line' in shown.stderr
    assert call(root, 'repair', '--as', 'reviewer').returncode == 0
    assert record.read_bytes().startswith(after)
    assert json.loads(head.read_text())['lines'] == 3
    clean = call(root, 'show', '3')
    assert clean.returncode == 0 and not clean.stderr
    print('PASS: the real kill inside cut-tail repair leaves a recoverable entry; second repair appends, catches up head and clears the advisory', flush=True)


print('\nCASE 6: torn-tail repair killed after its sync preserves damage through recovery', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h6-torn-repair-crash-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('BASE', 'baseline')).returncode == 0
    assert call(root, *post('W1', 'torn'), fault='torn').returncode == -9
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    head = record.parent / 'head.json'
    before = record.read_bytes()
    old_head = head.read_bytes()
    assert call(root, 'repair', '--as', 'reviewer', fault='after-sync').returncode == -9
    after = record.read_bytes()
    assert after.startswith(before) and head.read_bytes() == old_head
    lines = after.splitlines()
    repair = json.loads(lines[-1])
    assert repair['kind'] == 'repair'
    assert repair['damage']['sha256'] == hashlib.sha256(lines[-2]).hexdigest()
    assert repair['damage']['after'] == hashlib.sha256(lines[0]).hexdigest()
    assert 'verified: NO' not in call(root, 'show', str(repair['id'])).stdout
    assert call(root, 'repair', '--as', 'reviewer').returncode == 0
    assert record.read_bytes().startswith(after)
    assert json.loads(head.read_text())['lines'] == 4
    clean = call(root, 'show', str(repair['id']))
    assert clean.returncode == 0 and not clean.stderr
    print('PASS: the real kill in the torn-tail repair path retains the original damaged bytes and proof; recovery only appends and updates head', flush=True)


print('\nAll independent process probes passed.', flush=True)


