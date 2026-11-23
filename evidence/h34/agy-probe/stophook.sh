#!/bin/zsh
# Probe hook: record the raw stdin of each hook call, answer nothing.
in=$(cat)
print -r -- "$(date -u +%Y-%m-%dT%H:%M:%SZ) $1 $in" >> <scratch>/hook-stdin.log
echo '{}'
