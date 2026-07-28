#!/usr/bin/env bash
# Fake redis. Runs ~7 seconds, then crashes to trigger wink's dead state + notification.
echo "redis 7.2 ready to accept connections"
sleep 2
echo "1 client connected"
sleep 2
echo "saving snapshot to disk"
sleep 3
echo "FATAL: out of memory" >&2
exit 1
