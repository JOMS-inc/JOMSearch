#!/usr/bin/env bash
set -e

LOCKFILE=/tmp/jomsearch-deploy.lock
if [ -e "$LOCKFILE" ]; then
  echo "Deploy already running, skipping."
  exit 0
fi
trap "rm -f $LOCKFILE" EXIT
touch "$LOCKFILE"

cd ~/JOMSearch
git fetch origin
git reset --hard origin/main

cd go-services
go build -o /home/adminuser/JOMSearch/bin/search-api ./cmd/search-api

sudo systemctl restart jomsearch
