#!/bin/sh
# Whole-match tests for Jin and the five previously implemented characters. Written before integration.
set -eu
MD_FIGHTER_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$MD_FIGHTER_ROOT"
MD_FIGHTER_TARGET=${1:-both}
if [ "$MD_FIGHTER_TARGET" = both ]; then
  MD_FIGHTER_COMPARE=0 sh "$0" native
  MD_FIGHTER_COMPARE=1 sh "$0" wasm
  exit 0
fi
case "$MD_FIGHTER_TARGET" in native|wasm) ;; *) echo 'usage: verify_jin_sessions.sh [both|native|wasm]' >&2; exit 2 ;; esac
MD_FIGHTER_OUTPUT="games/yuyuhakusho/verification/jin-e2e/$MD_FIGHTER_TARGET"
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
fighter_rules -players 2 -characters 6,6 > "$MD_FIGHTER_OUTPUT/rules/jin-jin.json"
fighter_rules -players 2 -characters 6,0 > "$MD_FIGHTER_OUTPUT/rules/jin-genkai.json"
fighter_rules -players 2 -characters 6,1 > "$MD_FIGHTER_OUTPUT/rules/jin-kurama.json"
fighter_rules -players 2 -characters 1,6 > "$MD_FIGHTER_OUTPUT/rules/kurama-jin.json"
fighter_rules -players 2 -characters 6,2 > "$MD_FIGHTER_OUTPUT/rules/jin-kuwabara.json"
fighter_rules -players 2 -characters 2,6 > "$MD_FIGHTER_OUTPUT/rules/kuwabara-jin.json"
fighter_rules -players 2 -characters 6,3 > "$MD_FIGHTER_OUTPUT/rules/jin-yusuke.json"
fighter_rules -players 2 -characters 3,6 > "$MD_FIGHTER_OUTPUT/rules/yusuke-jin.json"
fighter_rules -players 2 -characters 0,6 > "$MD_FIGHTER_OUTPUT/rules/genkai-jin.json"
fighter_rules -players 2 -characters 6,8 > "$MD_FIGHTER_OUTPUT/rules/jin-elder.json"
fighter_rules -players 2 -characters 8,6 > "$MD_FIGHTER_OUTPUT/rules/elder-jin.json"
fighter_rules -players 2 -characters 6,9 > "$MD_FIGHTER_OUTPUT/rules/jin-chu.json"
fighter_rules -players 2 -characters 9,6 > "$MD_FIGHTER_OUTPUT/rules/chu-jin.json"
fighter_rules -players 2 -characters 6,10 > "$MD_FIGHTER_OUTPUT/rules/jin-younger.json"
fighter_rules -players 2 -characters 10,6 > "$MD_FIGHTER_OUTPUT/rules/younger-jin.json"
fighter_rules -players 3 -characters 6,0,3 > "$MD_FIGHTER_OUTPUT/rules/three.json"
fighter_rules -players 4 -characters 6,0,3,10 > "$MD_FIGHTER_OUTPUT/rules/four.json"
fighter_rules -players 4 -characters 6,0,6,10 -team > "$MD_FIGHTER_OUTPUT/rules/team.json"
fighter_rules -players 2 -characters 6,9 -human-idle > "$MD_FIGHTER_OUTPUT/rules/human-idle.json"
for MD_FIGHTER_CASE in jin-jin jin-genkai jin-kurama jin-kuwabara jin-yusuke jin-elder jin-chu jin-younger three four team; do
  case "$MD_FIGHTER_CASE" in
    jin-jin) set -- -players 2 -characters 6,6 ;;
    jin-genkai) set -- -players 2 -characters 6,0 ;;
    jin-kurama) set -- -players 2 -characters 6,1 ;;
    jin-kuwabara) set -- -players 2 -characters 6,2 ;;
    jin-yusuke) set -- -players 2 -characters 6,3 ;;
    jin-elder) set -- -players 2 -characters 6,8 ;;
    jin-chu) set -- -players 2 -characters 6,9 ;;
    jin-younger) set -- -players 2 -characters 6,10 ;;
    three) set -- -players 3 -characters 6,0,3 ;;
    four) set -- -players 4 -characters 6,0,3,10 ;;
    team) set -- -players 4 -characters 6,0,6,10 -team ;;
  esac
  fighter_render "$@" -output "$MD_FIGHTER_OUTPUT/render/$MD_FIGHTER_CASE" > "$MD_FIGHTER_OUTPUT/render/$MD_FIGHTER_CASE.json"
done
PYTHONDONTWRITEBYTECODE=1 python3 - "$MD_FIGHTER_OUTPUT" <<'PY'
from pathlib import Path
import json,os,sys
out=Path(sys.argv[1]);root=out.parent
paths=sorted(out.glob('*/*.json'))
assert len(paths)==30,len(paths)
for path in paths:
    report=json.loads(path.read_text())
    assert report['pass'] and not report['reference_state_loaded'] and 6 in report['characters'],path
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
if os.environ.get('MD_FIGHTER_COMPARE')=='1':print('all thirty eight-character native/Wasm complete traces match')
PY
