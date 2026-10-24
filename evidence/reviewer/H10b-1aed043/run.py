#!/usr/bin/env python3
import datetime
from pathlib import Path
import os
import shlex
import subprocess
import sys

destination = Path(sys.argv[1])
command = sys.argv[2:]
with destination.open("w") as output:
    def log(message):
        print(message, flush=True)
        output.write(message + "\n")
        output.flush()
    log("by: Codex, reviewer")
    log("commit: " + subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip())
    log("date: " + datetime.datetime.now(datetime.timezone.utc).isoformat())
    log("cwd: " + os.getcwd())
    log("command: " + shlex.join(command))
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    for line in process.stdout:
        log(line.rstrip("\n"))
    code = process.wait()
    log("exit: " + str(code))
sys.exit(code)
