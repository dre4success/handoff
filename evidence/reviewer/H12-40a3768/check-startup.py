#!/usr/bin/env python3
"""Run the prior exploit with its original exit convention: 1 = not found."""
from pathlib import Path
import subprocess
import sys

probe = Path(__file__).resolve().parent / "startup-race.py"
result = subprocess.run([sys.executable, str(probe), sys.argv[1]], text=True, capture_output=True, timeout=150)
print(result.stdout + result.stderr, end="", flush=True)
assert result.returncode == 1 and "found: False" in result.stdout, "prior exploit reproduced or probe did not complete"
print("PASS: prior startup exploit not reproduced in 30 attempts.", flush=True)
