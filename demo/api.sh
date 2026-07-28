#!/usr/bin/env bash
# Fake API service for the demo recording. Deterministic-ish, log-realistic.
echo "listening on :3000"
sleep 1
routes=("GET /users 200 12ms" "GET /orders 200 8ms" "POST /login 200 41ms" "GET /health 200 1ms" "GET /users/42 200 9ms" "POST /orders 201 33ms")
i=0
while true; do
  echo "${routes[$((i % 6))]}"
  # a couple of errors so the /error search has something to find
  if [ "$i" -eq 5 ] || [ "$i" -eq 11 ]; then
    echo "ERROR TypeError at auth.js:42" >&2
  fi
  i=$((i + 1))
  sleep 1.4
done
