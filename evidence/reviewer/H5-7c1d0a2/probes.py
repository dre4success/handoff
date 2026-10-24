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
print('Run by GPT-6, primary reviewer; snapshot 7c1d0a2fd6e660079cdfc8beacf2fe705a7c38ad', flush=True)
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


print('\nCASE 2: corruption introduced after an emit wait has delivered its first entry', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-corruption-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'first')).returncode == 0
    args = [str(BIN), 'wait', '--for', 'reviewer', '--emit', '--timeout', '3s']
    print('$ ' + shlex.join(args), flush=True)
    waiter = subprocess.Popen(args, cwd=root, env=ENV, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    first = waiter.stdout.readline()
    assert '#1 request' in first
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    bad = {'v': 1, 'id': 99, 'ts': '2026-10-09T20:00:00Z', 'kind': 'request', 'item': 'BAD', 'from': 'implementer', 'to': 'reviewer', 'commit': sha, 'body': 'must not be delivered', 'prev': 'broken'}
    with record.open('a') as f:
        f.write(json.dumps(bad) + '\n')
    rest, err = waiter.communicate(timeout=8)
    cursor = (record.parent / 'cursors/reviewer.json').read_text()
    print(f'exit={waiter.returncode}\nstdout: {first + rest}stderr: {err}\ncursor: {cursor}', flush=True)
    if '#99 request' in rest and not err and json.loads(cursor)['id'] == 99:
        print('BUG: unverified entry delivered and cursor advanced, without reporting new corruption', flush=True)
    else:
        print('No expected bug observed', flush=True)


print('\nCASE 3: a complete damaged line followed by a partial repair line (simulated interrupted repair write)', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-repair-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'first')).returncode == 0
    assert call(root, *post('W2', 'torn'), fault='torn').returncode == -9
    record = pathlib.Path(root) / '.handoff/channel.jsonl'
    raw = record.read_bytes()
    with record.open('ab') as f:
        f.write(b'\n{"v":1,"id":3,"kind":"repair"')
    repair = call(root, 'repair', '--as', 'reviewer')
    if repair.returncode == 2 and 'hash chain' in repair.stderr:
        print('OBSERVED: repair refuses after a partial repair write; original good prefix unchanged=' + str(record.read_bytes().startswith(raw)), flush=True)


print('\nCASE 4: a note to the operator linked to a reviewer question', flush=True)
with tempfile.TemporaryDirectory(prefix='handoff-h5-note-') as root:
    post, body, sha = fixture(root)
    assert call(root, *post('W1', 'request')).returncode == 0
    assert call(root, 'reply', '1', '--as', 'reviewer', '--verdict', 'QUESTION', '--commit', sha, '--body', str(body)).returncode == 0
    assert call(root, 'note', '--as', 'implementer', '--to', 'operator', '--re', '2', '--body', str(body)).returncode == 0
    shown = call(root, 'show', '1')
    call(root, 'inbox', '--for', 'implementer')
    call(root, 'inbox', '--for', 'reviewer')
    if 'state: claimed' in shown.stdout:
        print('OBSERVED: operator-directed note clears the implementer response obligation without addressing the reviewer', flush=True)
