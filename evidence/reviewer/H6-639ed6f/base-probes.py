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
print('Run by GPT-6, primary reviewer; snapshot 1b75b877390178520922a52e976e6c30052d13b3', flush=True)
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
    repair = call(root, 'repair', '--as', 'reviewer')
    print('record bytes unchanged by both refusals=' + str(record.read_bytes() == after), flush=True)
    if retry.returncode == 2 and repair.returncode == 2:
        assert 'head.json counts the same 2 lines' in retry.stderr
        assert record.read_bytes() == after
        print('BUG: a supported cut-tail/post-sync-crash sequence is classified as an edit; same-key retry and repair both refuse a genuine synced entry', flush=True)
    else:
        assert retry.returncode == 0 and retry.stdout.startswith('exists #3')
        assert repair.returncode == 0 and record.read_bytes().startswith(after)
        print(f'No expected bug: retry exit={retry.returncode}, repair exit={repair.returncode}', flush=True)


