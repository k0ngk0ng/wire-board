#!/bin/sh
set -eu
MD_RUNTIME_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$MD_RUNTIME_ROOT"
MD_RUNTIME_TARGET=${1:-both}
if [ "$MD_RUNTIME_TARGET" = both ]; then
  sh "$0" native
  MD_RUNTIME_COMPARE=1 sh "$0" wasm
  exit 0
fi
case "$MD_RUNTIME_TARGET" in native|wasm) ;; *) echo 'usage: verify_chu_runtime.sh [both|native|wasm]' >&2; exit 2 ;; esac
MD_RUNTIME_OUTPUT="games/yuyuhakusho/verification/runtime-e2e/$MD_RUNTIME_TARGET"
mkdir -p .local/md-yuyuhakusho/tmp .local/md-yuyuhakusho/build "$MD_RUNTIME_OUTPUT"
export GOCACHE="$MD_RUNTIME_ROOT/.local/go-cache"
export GOMODCACHE="$MD_RUNTIME_ROOT/.local/go-mod"
export GOTMPDIR="$MD_RUNTIME_ROOT/.local/md-yuyuhakusho/tmp"
if [ "$MD_RUNTIME_TARGET" = wasm ]; then
  GOOS=js GOARCH=wasm go build -o .local/md-yuyuhakusho/build/verify-chu-runtime.wasm ./games/yuyuhakusho/cmd/verify-chu-runtime
  set -- "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" .local/md-yuyuhakusho/build/verify-chu-runtime.wasm
else
  go build -o .local/md-yuyuhakusho/build/verify-chu-runtime ./games/yuyuhakusho/cmd/verify-chu-runtime
  set -- .local/md-yuyuhakusho/build/verify-chu-runtime
fi
"$@" -players 2 -output "$MD_RUNTIME_OUTPUT/two-ai" > "$MD_RUNTIME_OUTPUT/two-ai.json"
"$@" -players 4 -team -output "$MD_RUNTIME_OUTPUT/team-ai" > "$MD_RUNTIME_OUTPUT/team-ai.json"
PYTHONDONTWRITEBYTECODE=1 python3 - "$MD_RUNTIME_OUTPUT" <<'PY'
import json, os, sys
from pathlib import Path
out=Path(sys.argv[1])
for name in ('two-ai','team-ai'):
    report=json.loads((out/(name+'.json')).read_text())
    assert report['pass'] and report['complete'] and report['hud_window_complete'] and report['restored_pixel_frames']==30, report
    assert report['unready_tick_blocked'] and report['invalid_input_atomic'] and report['asset_version_checked'], report
    assert report['sprite_frames'] > 100 and report['max_draw_requests'] > 3, report
    assert report['reference_state_loaded'] is False, report
    print(name,report['ticks'],report['pixel_trace_sha256'])
root=out.parent
if os.environ.get('MD_RUNTIME_COMPARE')=='1':
    for name in ('two-ai','team-ai'):
        a=json.loads((root/'native'/(name+'.json')).read_text())
        b=json.loads((root/'wasm'/(name+'.json')).read_text())
        a.pop('environment'); b.pop('environment')
        assert a==b, (name,'native/Wasm state or rendered pixel trace differs')
    print('native/Wasm state and pixel traces match')
PY
