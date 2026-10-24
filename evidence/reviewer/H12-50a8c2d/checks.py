#!/usr/bin/env python3
import subprocess

def run(args):
    print("$", " ".join(args), flush=True)
    result = subprocess.run(args, text=True, capture_output=True)
    print(result.stdout + result.stderr, end="", flush=True)
    print("exit:", result.returncode, flush=True)
    assert result.returncode == 0
    return result.stdout

assert not run(["gofmt", "-l", "cmd", "internal"]).strip()
run(["git", "diff", "--check", "4797d929c89c7cdc4c8335f0197d9fedd71ab5dc", "HEAD"])
deps = run(["go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", "./..."])
assert all(not line or line.startswith("github.com/dre4success/handoff") for line in deps.splitlines())
deps = subprocess.check_output(["go", "list", "-deps", "./..."], text=True).splitlines()
assert not any(p == "net" or p.startswith("net/") for p in deps)
print("PASS: formatted, diff whitespace clean, standard-library-only dependencies plus this module, no net package.", flush=True)
