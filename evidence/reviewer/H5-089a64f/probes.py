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
print('Run by GPT-6, primary reviewer; snapshot 089a64fc201afcac6ebf70417f284b01ada41e7f', flush=True)
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
with tempfile.TemporaryDirectory(prefix='handoff-h5-crash-') as root:
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


print('\nCASE 2: damage arriving during an emitting wait', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-corruption-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'first')).returncode == 0
    args = [str(BIN), 'wait', '--for', 'reviewer', '--emit', '--timeout', '5s']
    print('$ ' + shlex.join(args), flush=True)
    waiter = subprocess.Popen(args, cwd=root, env=ENV, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        first = waiter.stdout.readline()
        assert '#1 request' in first
        record = pathlib.Path(root) / '.handoff/channel.jsonl'
        bad = {'v': 1, 'id': 99, 'ts': '2026-10-09T20:00:00Z', 'kind': 'request', 'item': 'BAD', 'from': 'implementer', 'to': 'reviewer', 'commit': sha, 'body': 'must not be delivered', 'prev': 'broken'}
        with record.open('a') as f:
            f.write(json.dumps(bad) + '\n')
        rest, err = waiter.communicate(timeout=8)
        cursor = json.loads((record.parent / 'cursors/reviewer.json').read_text())['id']
        print(f'exit={waiter.returncode}\nstdout: {first + rest}stderr: {err}\ncursor: {cursor}', flush=True)
        assert waiter.returncode == 2 and '#99' not in rest and cursor == 1
        assert 'fails at line 2' in err and 'line 1 on are withheld' in err
        shown = call(root, 'show', '99')
        assert shown.returncode == 0 and 'verified: NO' in shown.stdout and 'state:' not in shown.stdout
        assert call(root, 'inbox', '--for', 'reviewer', '--quiet').returncode == 0
        print('PASS: new damage reported; wait exits 2; no forged delivery or cursor advance; forensic show marked; withheld work absent from inbox', flush=True)
    finally:
        if waiter.poll() is None:
            waiter.kill()
            waiter.communicate()


print('\nCASE 3: a wait starting on a broken record delivers the verified prefix only', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-prefix-') as root:
    post, body, sha = fixture(root)
    for n in range(1, 4):
        assert call(root, *post(f'W{n}', f'key{n}')).returncode == 0
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    bad = {'v': 1, 'id': 99, 'ts': '2026-10-09T20:00:00Z', 'kind': 'request', 'item': 'BAD', 'from': 'implementer', 'to': 'reviewer', 'commit': sha, 'body': 'must not be delivered', 'prev': 'broken'}
    with record.open('a') as f:
        f.write(json.dumps(bad) + '\n')
    waited = call(root, 'wait', '--for', 'reviewer', '--timeout', '5s')
    cursor = json.loads((record.parent / 'cursors/reviewer.json').read_text())['id']
    assert waited.returncode == 2 and len(waited.stdout.splitlines()) == 2 and cursor == 2
    assert '#1 request' in waited.stdout and '#2 request' in waited.stdout
    assert '#3 request' not in waited.stdout and '#99' not in waited.stdout
    shown = call(root, 'show', '3')
    assert 'verified: NO' in shown.stdout and 'state:' not in shown.stdout
    inbox = call(root, 'inbox', '--for', 'reviewer')
    assert '#3 ' not in inbox.stdout and '#99 ' not in inbox.stdout
    assert call(root, 'repair', '--as', 'reviewer').returncode == 2
    assert call(root, *post('W4', 'key4')).returncode == 2
    print('PASS: only #1 and #2 delivered; #3 and #99 withheld; cursor 2; posting and repair refused', flush=True)


print('\nCASE 4: QUESTION and CHANGES escalation routing', flush=True)
for verdict in ('QUESTION', 'CHANGES'):
    with tempfile.TemporaryDirectory(prefix='handoff-h5-note-') as root:
        post, body, sha = fixture(root)
        assert call(root, *post('W1', 'request')).returncode == 0
        assert call(root, 'reply', '1', '--as', 'reviewer', '--verdict', verdict, '--commit', sha, '--body', str(body)).returncode == 0
        for recipient in ('operator', 'reviewer-2'):
            assert call(root, 'note', '--as', 'implementer', '--to', recipient, '--re', '2', '--body', str(body)).returncode == 0
            shown = call(root, 'show', '1')
            assert 'state: answered' in shown.stdout
            assert call(root, 'inbox', '--for', 'implementer', '--quiet').returncode == 1
            assert call(root, 'inbox', '--for', 'reviewer', '--quiet').returncode == 0
        assert call(root, 'note', '--as', 'implementer', '--to', 'reviewer', '--re', '2', '--body', str(body)).returncode == 0
        assert 'state: claimed' in call(root, 'show', '1').stdout
        assert call(root, 'inbox', '--for', 'implementer', '--quiet').returncode == 0
        assert call(root, 'inbox', '--for', 'reviewer', '--quiet').returncode == 1
        print(f'PASS: {verdict} remains owed after both escalation routes; direct reviewer answer transfers the work', flush=True)


print('\nCASE 5: editing a middle QUESTION into APPROVE cannot manufacture approval', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-verdict-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'request')).returncode == 0
    assert call(root, 'claim', '1', '--as', 'reviewer').returncode == 0
    assert call(root, 'reply', '1', '--as', 'reviewer', '--verdict', 'QUESTION', '--commit', sha, '--body', str(body)).returncode == 0
    assert call(root, 'note', '--as', 'implementer', '--item', 'W1', '--to', 'operator', '--body', str(body)).returncode == 0
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    lines = record.read_bytes().splitlines(keepends=True)
    forged = json.loads(lines[2])
    forged['verdict'] = 'APPROVE'
    lines[2] = (json.dumps(forged) + '\n').encode()
    record.write_bytes(b''.join(lines))
    shown = call(root, 'show', '1')
    assert 'state: claimed' in shown.stdout and 'awaiting operator' not in shown.stdout
    forged_show = call(root, 'show', '3')
    assert 'verified: NO' in forged_show.stdout and 'verdict: APPROVE' in forged_show.stdout
    assert call(root, 'inbox', '--for', 'operator', '--quiet').returncode == 0
    print('PASS: edited verdict is forensic only; request stays claimed; no approval reaches the operator', flush=True)


print('\nAll independent process probes passed.', flush=True)


