#!/bin/sh
# Full application browser acceptance. Requires agent-browser on PATH.
set -eu
cd "$(dirname "$0")/../.."
. scripts/env.sh
mkdir -p .local/game-loading
npm --prefix web run build > .local/game-loading/build.txt 2>&1
node scripts/game-loading/check-build.mjs > .local/game-loading/build-check.json
go build -o .local/game-loading/acceptance-server .
loading_tmp=$(mktemp -d "$PWD/.local/game-loading/server-XXXXXX")
loading_pid=''
cleanup() {
  if [ -n "$loading_pid" ]; then kill "$loading_pid" 2>/dev/null || true; wait "$loading_pid" 2>/dev/null || true; fi
  rm -rf "$loading_tmp"
  rm -f .local/game-loading/acceptance-server
}
trap cleanup EXIT INT TERM
ADDR=127.0.0.1:18189 DATA_DIR="$loading_tmp" INVITE_CODE=test-invite ADMIN_USERNAME=LoadingAdmin ADMIN_PASSWORD=loading-admin-test-123 .local/game-loading/acceptance-server > .local/game-loading/server.log 2>&1 &
loading_pid=$!
python3 -B - <<'PY'
import time,urllib.request
for _ in range(100):
 try:
  with urllib.request.urlopen('http://127.0.0.1:18189/healthz',timeout=1) as response: assert response.status==200
  break
 except Exception: time.sleep(.1)
else: raise SystemExit('Local server failed to start')
PY
python3 -B scripts/game-loading/check-browser.py
python3 -B scripts/game-loading/check-peers.py
python3 -B scripts/game-loading/check-home.py
