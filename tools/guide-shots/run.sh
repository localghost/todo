#!/usr/bin/env bash
# Makes the screenshots of the user guide again (mise run guide:shots): builds
# the app, runs it on a temporary database, drives headless Chromium with
# shots.mjs, and stops the app. Needs Node 22 or newer and headless Chromium
# ($CHROME, or `npx playwright install chromium-headless-shell`).
set -euo pipefail
cd "$(dirname "$0")/../.."
addr=127.0.0.1:8897
tmp=$(mktemp -d)
pid=""
trap '[ -n "$pid" ] && kill "$pid" 2>/dev/null; rm -rf "$tmp"' EXIT
go build -o "$tmp/todo" ./cmd/todo
"$tmp/todo" -addr "$addr" -db "$tmp/guide.db" -config config.yaml >"$tmp/app.log" 2>&1 &
pid=$!
for _ in $(seq 50); do
  curl -fsS -o /dev/null "http://$addr/login" 2>/dev/null && break
  sleep 0.1
done
node tools/guide-shots/shots.mjs "http://$addr" docs/guide-images
