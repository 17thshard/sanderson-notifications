#!/bin/sh

while true; do
  /sanderson-notifications "$@"
  sleep "${CHECK_INTERVAL}"
done
