#!/bin/sh
set -eu
MD_VERIFY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$MD_VERIFY_ROOT"
MD_VERIFY_TARGET=${1:-both}
if [ "$MD_VERIFY_TARGET" = both ]; then
  sh "$0" native
  MD_SESSION_COMPARE=1 sh "$0" wasm
  exit 0
fi
case "$MD_VERIFY_TARGET" in
  native|wasm) ;;
  *) echo 'usage: verify_chu_sessions.sh [both|native|wasm]' >&2; exit 2 ;;
esac
MD_VERIFY_OUTPUT="games/yuyuhakusho/verification/session-e2e/$MD_VERIFY_TARGET"
mkdir -p .local/md-yuyuhakusho/tmp .local/md-yuyuhakusho/build "$MD_VERIFY_OUTPUT"
export GOCACHE="$MD_VERIFY_ROOT/.local/go-cache"
export GOMODCACHE="$MD_VERIFY_ROOT/.local/go-mod"
export GOTMPDIR="$MD_VERIFY_ROOT/.local/md-yuyuhakusho/tmp"
if [ "$MD_VERIFY_TARGET" = wasm ]; then
  GOOS=js GOARCH=wasm go build -o .local/md-yuyuhakusho/build/verify-chu-session.wasm ./games/yuyuhakusho/cmd/verify-chu-session
  set -- "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" .local/md-yuyuhakusho/build/verify-chu-session.wasm
else
  go build -o .local/md-yuyuhakusho/build/verify-chu-session ./games/yuyuhakusho/cmd/verify-chu-session
  set -- .local/md-yuyuhakusho/build/verify-chu-session
fi
"$@" -players 2 > "$MD_VERIFY_OUTPUT/two-ai.json"
"$@" -players 3 > "$MD_VERIFY_OUTPUT/three-ai.json"
"$@" -players 4 > "$MD_VERIFY_OUTPUT/four-ai.json"
"$@" -players 4 -team > "$MD_VERIFY_OUTPUT/team-ai.json"
"$@" -players 2 -human-idle > "$MD_VERIFY_OUTPUT/mixed-idle.json"
PYTHONDONTWRITEBYTECODE=1 python3 - "$MD_VERIFY_OUTPUT" <<'PY'
import json, os
from pathlib import Path
import sys
root=Path('games/yuyuhakusho/verification/session-e2e')
for path in sorted(Path(sys.argv[1]).glob('*.json')):
    report=json.loads(path.read_text())
    assert report['pass'] and report['completed'] and report['reference_state_loaded'] is False, path
    assert report['first_damage_tick'] > 0 and report['first_ko_tick'] >= report['first_damage_tick'], path
    assert report['retirement_tick'] >= report['first_ko_tick'] and report['result_tick'] >= report['retirement_tick'], path
    assert report['clear_tick'] >= report['result_tick'] and report['callbacks'] > report['ticks'], path
    assert len(report['final_state_sha256']) == 64 and len(report['trace_sha256']) == 64, path
    assert report['human_auto_takeovers'] == 0, path
    print(path.name, report['ticks'], report['winner_code'], report['trace_sha256'])
native, wasm = root/'native', root/'wasm'
if os.environ.get('MD_SESSION_COMPARE')=='1':
    for path in sorted(native.glob('*.json')):
        a=json.loads(path.read_text())
        b=json.loads((wasm/path.name).read_text())
        assert a.pop('environment') != b.pop('environment'), path.name
        assert a == b, f'native/Wasm complete trace or final state differs: {path.name}'
    print('all five native/Wasm full-state traces match')
PY
