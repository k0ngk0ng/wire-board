#!/bin/sh
# Whole-match tests for Hiei and the eight previously implemented characters. Written before integration.
set -eu
MD_FIGHTER_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$MD_FIGHTER_ROOT"
MD_FIGHTER_TARGET=${1:-both}
if [ "$MD_FIGHTER_TARGET" = both ]; then
  MD_FIGHTER_COMPARE=0 sh "$0" native
  MD_FIGHTER_COMPARE=1 sh "$0" wasm
  exit 0
fi
case "$MD_FIGHTER_TARGET" in native|wasm) ;; *) echo 'usage: verify_hiei_sessions.sh [both|native|wasm]' >&2; exit 2 ;; esac
MD_FIGHTER_OUTPUT="games/yuyuhakusho/verification/hiei-e2e/$MD_FIGHTER_TARGET"
mkdir -p .local/md-yuyuhakusho/tmp .local/md-yuyuhakusho/build "$MD_FIGHTER_OUTPUT/rules" "$MD_FIGHTER_OUTPUT/render"
export GOCACHE="$MD_FIGHTER_ROOT/.local/go-cache"
export GOMODCACHE="$MD_FIGHTER_ROOT/.local/go-mod"
export GOTMPDIR="$MD_FIGHTER_ROOT/.local/md-yuyuhakusho/tmp"
if [ "$MD_FIGHTER_TARGET" = wasm ]; then
  GOOS=js GOARCH=wasm go build -o .local/md-yuyuhakusho/build/fighter-session.wasm ./games/yuyuhakusho/cmd/verify-chu-session
  GOOS=js GOARCH=wasm go build -o .local/md-yuyuhakusho/build/fighter-render.wasm ./games/yuyuhakusho/cmd/verify-chu-runtime
  MD_FIGHTER_EXEC="$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
else
  go build -o .local/md-yuyuhakusho/build/fighter-session ./games/yuyuhakusho/cmd/verify-chu-session
  go build -o .local/md-yuyuhakusho/build/fighter-render ./games/yuyuhakusho/cmd/verify-chu-runtime
fi
fighter_rules() {
  if [ "$MD_FIGHTER_TARGET" = wasm ]; then "$MD_FIGHTER_EXEC" .local/md-yuyuhakusho/build/fighter-session.wasm "$@"
  else .local/md-yuyuhakusho/build/fighter-session "$@"; fi
}
fighter_render() {
  if [ "$MD_FIGHTER_TARGET" = wasm ]; then "$MD_FIGHTER_EXEC" .local/md-yuyuhakusho/build/fighter-render.wasm "$@"
  else .local/md-yuyuhakusho/build/fighter-render "$@"; fi
}
fighter_rules -players 2 -characters 4,4 > "$MD_FIGHTER_OUTPUT/rules/hiei-hiei.json"
fighter_rules -players 2 -characters 4,0 > "$MD_FIGHTER_OUTPUT/rules/hiei-genkai.json"
fighter_rules -players 2 -characters 0,4 > "$MD_FIGHTER_OUTPUT/rules/genkai-hiei.json"
fighter_rules -players 2 -characters 4,1 > "$MD_FIGHTER_OUTPUT/rules/hiei-kurama.json"
fighter_rules -players 2 -characters 1,4 > "$MD_FIGHTER_OUTPUT/rules/kurama-hiei.json"
fighter_rules -players 2 -characters 4,2 > "$MD_FIGHTER_OUTPUT/rules/hiei-kuwabara.json"
fighter_rules -players 2 -characters 2,4 > "$MD_FIGHTER_OUTPUT/rules/kuwabara-hiei.json"
fighter_rules -players 2 -characters 4,3 > "$MD_FIGHTER_OUTPUT/rules/hiei-yusuke.json"
fighter_rules -players 2 -characters 3,4 > "$MD_FIGHTER_OUTPUT/rules/yusuke-hiei.json"
fighter_rules -players 2 -characters 4,6 > "$MD_FIGHTER_OUTPUT/rules/hiei-jin.json"
fighter_rules -players 2 -characters 6,4 > "$MD_FIGHTER_OUTPUT/rules/jin-hiei.json"
fighter_rules -players 2 -characters 4,8 > "$MD_FIGHTER_OUTPUT/rules/hiei-elder.json"
fighter_rules -players 2 -characters 8,4 > "$MD_FIGHTER_OUTPUT/rules/elder-hiei.json"
fighter_rules -players 2 -characters 4,9 > "$MD_FIGHTER_OUTPUT/rules/hiei-chu.json"
fighter_rules -players 2 -characters 9,4 > "$MD_FIGHTER_OUTPUT/rules/chu-hiei.json"
fighter_rules -players 2 -characters 4,10 > "$MD_FIGHTER_OUTPUT/rules/hiei-younger.json"
fighter_rules -players 2 -characters 10,4 > "$MD_FIGHTER_OUTPUT/rules/younger-hiei.json"
fighter_rules -players 3 -characters 4,0,3 > "$MD_FIGHTER_OUTPUT/rules/three.json"
fighter_rules -players 4 -characters 4,0,3,10 > "$MD_FIGHTER_OUTPUT/rules/four.json"
fighter_rules -players 4 -characters 4,0,4,10 -team > "$MD_FIGHTER_OUTPUT/rules/team.json"
fighter_rules -players 2 -characters 4,9 -human-idle > "$MD_FIGHTER_OUTPUT/rules/human-idle.json"
fighter_render -players 2 -characters 4,4 -output "$MD_FIGHTER_OUTPUT/render/hiei-hiei" > "$MD_FIGHTER_OUTPUT/render/hiei-hiei.json"
fighter_render -players 2 -characters 4,0 -output "$MD_FIGHTER_OUTPUT/render/hiei-genkai" > "$MD_FIGHTER_OUTPUT/render/hiei-genkai.json"
fighter_render -players 2 -characters 4,1 -output "$MD_FIGHTER_OUTPUT/render/hiei-kurama" > "$MD_FIGHTER_OUTPUT/render/hiei-kurama.json"
fighter_render -players 2 -characters 4,2 -output "$MD_FIGHTER_OUTPUT/render/hiei-kuwabara" > "$MD_FIGHTER_OUTPUT/render/hiei-kuwabara.json"
fighter_render -players 2 -characters 4,3 -output "$MD_FIGHTER_OUTPUT/render/hiei-yusuke" > "$MD_FIGHTER_OUTPUT/render/hiei-yusuke.json"
fighter_render -players 2 -characters 4,6 -output "$MD_FIGHTER_OUTPUT/render/hiei-jin" > "$MD_FIGHTER_OUTPUT/render/hiei-jin.json"
fighter_render -players 2 -characters 4,8 -output "$MD_FIGHTER_OUTPUT/render/hiei-elder" > "$MD_FIGHTER_OUTPUT/render/hiei-elder.json"
fighter_render -players 2 -characters 4,9 -output "$MD_FIGHTER_OUTPUT/render/hiei-chu" > "$MD_FIGHTER_OUTPUT/render/hiei-chu.json"
fighter_render -players 2 -characters 4,10 -output "$MD_FIGHTER_OUTPUT/render/hiei-younger" > "$MD_FIGHTER_OUTPUT/render/hiei-younger.json"
fighter_render -players 3 -characters 4,0,3 -output "$MD_FIGHTER_OUTPUT/render/three" > "$MD_FIGHTER_OUTPUT/render/three.json"
fighter_render -players 4 -characters 4,0,3,10 -output "$MD_FIGHTER_OUTPUT/render/four" > "$MD_FIGHTER_OUTPUT/render/four.json"
fighter_render -players 4 -characters 4,0,4,10 -team -output "$MD_FIGHTER_OUTPUT/render/team" > "$MD_FIGHTER_OUTPUT/render/team.json"
PYTHONDONTWRITEBYTECODE=1 python3 - "$MD_FIGHTER_OUTPUT" <<'PY'
from pathlib import Path
import json,os,sys
out=Path(sys.argv[1]);root=out.parent
paths=sorted(out.glob('*/*.json'))
assert len(paths)==33,len(paths)
for path in paths:
    report=json.loads(path.read_text())
    assert report['pass'] and not report['reference_state_loaded'] and 4 in report['characters'],path
    if path.parent.name=='rules':
        assert report['completed'] and report['human_auto_takeovers']==0,path
        assert 0<report['first_damage_tick']<=report['first_ko_tick']<=report['retirement_tick']<=report['result_tick']<=report['clear_tick'],path
    else:
        assert report['complete'] and report['hud_window_complete'] and report['restored_pixel_frames']==30,path
    if os.environ.get('MD_FIGHTER_COMPARE')=='1':
        native=json.loads((root/'native'/path.relative_to(out)).read_text())
        native.pop('environment');report.pop('environment')
        assert native==report,(str(path),'full native/Wasm state or pixel trace differs')
    print(path.relative_to(out),report['ticks'],'pass')
if os.environ.get('MD_FIGHTER_COMPARE')=='1':print('all thirty-three nine-character native/Wasm complete traces match')
PY
