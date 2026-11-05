#!/bin/sh
# Whole-match tests for Itsuki and the ten previously implemented characters. Written before integration.
set -eu
MD_FIGHTER_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$MD_FIGHTER_ROOT"
MD_FIGHTER_TARGET=${1:-both}
if [ "$MD_FIGHTER_TARGET" = both ]; then
  MD_FIGHTER_COMPARE=0 sh "$0" native
  MD_FIGHTER_COMPARE=1 sh "$0" wasm
  exit 0
fi
case "$MD_FIGHTER_TARGET" in native|wasm) ;; *) echo 'usage: verify_itsuki_sessions.sh [both|native|wasm]' >&2; exit 2 ;; esac
MD_FIGHTER_OUTPUT="games/yuyuhakusho/verification/itsuki-e2e/$MD_FIGHTER_TARGET"
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
fighter_rules -players 2 -characters 7,7 > "$MD_FIGHTER_OUTPUT/rules/itsuki-itsuki.json"
fighter_rules -players 2 -characters 7,0 > "$MD_FIGHTER_OUTPUT/rules/itsuki-genkai.json"
fighter_rules -players 2 -characters 0,7 > "$MD_FIGHTER_OUTPUT/rules/genkai-itsuki.json"
fighter_rules -players 2 -characters 7,1 > "$MD_FIGHTER_OUTPUT/rules/itsuki-kurama.json"
fighter_rules -players 2 -characters 1,7 > "$MD_FIGHTER_OUTPUT/rules/kurama-itsuki.json"
fighter_rules -players 2 -characters 7,2 > "$MD_FIGHTER_OUTPUT/rules/itsuki-kuwabara.json"
fighter_rules -players 2 -characters 2,7 > "$MD_FIGHTER_OUTPUT/rules/kuwabara-itsuki.json"
fighter_rules -players 2 -characters 7,3 > "$MD_FIGHTER_OUTPUT/rules/itsuki-yusuke.json"
fighter_rules -players 2 -characters 3,7 > "$MD_FIGHTER_OUTPUT/rules/yusuke-itsuki.json"
fighter_rules -players 2 -characters 7,4 > "$MD_FIGHTER_OUTPUT/rules/itsuki-hiei.json"
fighter_rules -players 2 -characters 4,7 > "$MD_FIGHTER_OUTPUT/rules/hiei-itsuki.json"
fighter_rules -players 2 -characters 7,5 > "$MD_FIGHTER_OUTPUT/rules/itsuki-sensui.json"
fighter_rules -players 2 -characters 5,7 > "$MD_FIGHTER_OUTPUT/rules/sensui-itsuki.json"
fighter_rules -players 2 -characters 7,6 > "$MD_FIGHTER_OUTPUT/rules/itsuki-jin.json"
fighter_rules -players 2 -characters 6,7 > "$MD_FIGHTER_OUTPUT/rules/jin-itsuki.json"
fighter_rules -players 2 -characters 7,8 > "$MD_FIGHTER_OUTPUT/rules/itsuki-elder.json"
fighter_rules -players 2 -characters 8,7 > "$MD_FIGHTER_OUTPUT/rules/elder-itsuki.json"
fighter_rules -players 2 -characters 7,9 > "$MD_FIGHTER_OUTPUT/rules/itsuki-chu.json"
fighter_rules -players 2 -characters 9,7 > "$MD_FIGHTER_OUTPUT/rules/chu-itsuki.json"
fighter_rules -players 2 -characters 7,10 > "$MD_FIGHTER_OUTPUT/rules/itsuki-younger.json"
fighter_rules -players 2 -characters 10,7 > "$MD_FIGHTER_OUTPUT/rules/younger-itsuki.json"
fighter_rules -players 3 -characters 7,0,3 > "$MD_FIGHTER_OUTPUT/rules/three.json"
fighter_rules -players 4 -characters 7,0,3,10 > "$MD_FIGHTER_OUTPUT/rules/four.json"
fighter_rules -players 4 -characters 7,0,7,10 -team > "$MD_FIGHTER_OUTPUT/rules/team.json"
fighter_rules -players 2 -characters 7,9 -human-idle > "$MD_FIGHTER_OUTPUT/rules/human-idle.json"
fighter_render -players 2 -characters 7,7 -output "$MD_FIGHTER_OUTPUT/render/itsuki-itsuki" > "$MD_FIGHTER_OUTPUT/render/itsuki-itsuki.json"
fighter_render -players 2 -characters 7,0 -output "$MD_FIGHTER_OUTPUT/render/itsuki-genkai" > "$MD_FIGHTER_OUTPUT/render/itsuki-genkai.json"
fighter_render -players 2 -characters 7,1 -output "$MD_FIGHTER_OUTPUT/render/itsuki-kurama" > "$MD_FIGHTER_OUTPUT/render/itsuki-kurama.json"
fighter_render -players 2 -characters 7,2 -output "$MD_FIGHTER_OUTPUT/render/itsuki-kuwabara" > "$MD_FIGHTER_OUTPUT/render/itsuki-kuwabara.json"
fighter_render -players 2 -characters 7,3 -output "$MD_FIGHTER_OUTPUT/render/itsuki-yusuke" > "$MD_FIGHTER_OUTPUT/render/itsuki-yusuke.json"
fighter_render -players 2 -characters 7,4 -output "$MD_FIGHTER_OUTPUT/render/itsuki-hiei" > "$MD_FIGHTER_OUTPUT/render/itsuki-hiei.json"
fighter_render -players 2 -characters 7,5 -output "$MD_FIGHTER_OUTPUT/render/itsuki-sensui" > "$MD_FIGHTER_OUTPUT/render/itsuki-sensui.json"
fighter_render -players 2 -characters 7,6 -output "$MD_FIGHTER_OUTPUT/render/itsuki-jin" > "$MD_FIGHTER_OUTPUT/render/itsuki-jin.json"
fighter_render -players 2 -characters 7,8 -output "$MD_FIGHTER_OUTPUT/render/itsuki-elder" > "$MD_FIGHTER_OUTPUT/render/itsuki-elder.json"
fighter_render -players 2 -characters 7,9 -output "$MD_FIGHTER_OUTPUT/render/itsuki-chu" > "$MD_FIGHTER_OUTPUT/render/itsuki-chu.json"
fighter_render -players 2 -characters 7,10 -output "$MD_FIGHTER_OUTPUT/render/itsuki-younger" > "$MD_FIGHTER_OUTPUT/render/itsuki-younger.json"
fighter_render -players 3 -characters 7,0,3 -output "$MD_FIGHTER_OUTPUT/render/three" > "$MD_FIGHTER_OUTPUT/render/three.json"
fighter_render -players 4 -characters 7,0,3,10 -output "$MD_FIGHTER_OUTPUT/render/four" > "$MD_FIGHTER_OUTPUT/render/four.json"
fighter_render -players 4 -characters 7,0,7,10 -team -output "$MD_FIGHTER_OUTPUT/render/team" > "$MD_FIGHTER_OUTPUT/render/team.json"
PYTHONDONTWRITEBYTECODE=1 python3 - "$MD_FIGHTER_OUTPUT" <<'PY'
from pathlib import Path
import json,os,sys
out=Path(sys.argv[1]);root=out.parent
paths=sorted(out.glob('*/*.json'))
assert len(paths)==39,len(paths)
for path in paths:
    report=json.loads(path.read_text())
    assert report['pass'] and not report['reference_state_loaded'] and 7 in report['characters'],path
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
if os.environ.get('MD_FIGHTER_COMPARE')=='1':print('all thirty-nine eleven-character native/Wasm complete traces match')
PY
