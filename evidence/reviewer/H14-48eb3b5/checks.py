import subprocess

def run(args):
    print('$ ' + ' '.join(args), flush=True)
    p = subprocess.run(args, capture_output=True, text=True)
    print(p.stdout + p.stderr, end='', flush=True)
    assert p.returncode == 0, p.returncode
    return p.stdout

assert not run(['gofmt', '-l', 'cmd', 'internal']).strip()
run(['git', 'diff', '--check', 'ae65b6ce30b5c5eb6a03d81f99d8cffdc25581b4', 'HEAD', '--', '.', ':(exclude)evidence'])
deps = run(['go', 'list', '-deps', '-f', '{{if not .Standard}}{{.ImportPath}}{{end}}', './...'])
assert all(not p or p.startswith('github.com/dre4success/handoff') for p in deps.splitlines())
deps = run(['go', 'list', '-deps', './...']).splitlines()
assert not any(p == 'net' or p.startswith('net/') for p in deps)
print('PASS: Go formatting, source/docs whitespace, module/standard-library dependencies only, no network package. Raw evidence whitespace is outside this check.')
