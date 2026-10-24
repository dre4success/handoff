#!/usr/bin/env python3
"""Keep timestamped reviewer command output outside the live record."""
import datetime
import os
from pathlib import Path
import shlex
import subprocess
import sys

log = Path(__file__).with_name("session.log")

def record(message):
    print(message, flush=True)
    with log.open("a") as stream:
        stream.write(message + "\n")

def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

command = sys.argv[1:]
record(f"START {now()} cwd={os.getcwd()} command={shlex.join(command)}")
process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
try:
    for line in process.stdout:
        record(line.rstrip("\n"))
    code = process.wait()
except KeyboardInterrupt:
    process.terminate()
    code = process.wait()
    record("Collector interrupted; child stopped.")
record(f"END {now()} exit={code}")
sys.exit(code)
