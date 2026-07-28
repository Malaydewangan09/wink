#!/usr/bin/env bash
# Records the wink demo gif. Run from the repo root: bash demo/record.sh
set -e

# clean slate before recording
wink down demo/wink.yaml 2>/dev/null || true
wink clear 2>/dev/null || true

vhs demo/demo.tape

# clean up after recording
wink down demo/wink.yaml 2>/dev/null || true
wink clear 2>/dev/null || true

echo "done → demo/wink.gif"
