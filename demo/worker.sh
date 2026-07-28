#!/usr/bin/env bash
# Fake background worker. Healthy at first, then complains once redis dies (~7s in).
echo "worker started"
sleep 0.8
echo "connected to redis :6379"
job=4418
t=0
while true; do
  if [ "$t" -ge 8 ]; then
    echo "WARN redis unavailable, retrying in 2s" >&2
  else
    echo "job #$job processed 230ms"
    job=$((job + 1))
  fi
  t=$((t + 2))
  sleep 2
done
