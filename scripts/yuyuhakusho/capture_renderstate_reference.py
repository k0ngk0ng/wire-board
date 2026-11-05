#!/usr/bin/env python3
"""Freeze existing bridge trajectories before pixel-free render-state code."""
import concurrent.futures,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/renderstate/testdata'
LOCAL=ROOT/'.local/md-yuyuhakusho/renderstate-reference'
OUT.mkdir(parents=True,exist_ok=True);LOCAL.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TEST_TELEMETRY_DIR=str(LOCAL/'telemetry'),GOTELEMETRY='off')
binary=LOCAL/'capture';subprocess.run(['go','build','-o',str(binary),'./games/yuyuhakusho/cmd/capture-renderstate-reference'],cwd=ROOT,env=env,check=True)
configs=[]
legacy=json.loads((ROOT/'games/yuyuhakusho/verification/additional-arena-legacy-native.json').read_text())
for check in legacy['checks']:
 if check['kind']!='render':continue
 path=ROOT/check['expected_path'];d=json.loads(path.read_text());configs.append({'name':'legacy-'+path.stem,'arena':0,'characters':d['characters'],'team':d['team'],'reference_path':check['expected_path']})
for arena in [2,4,6,8,10,12,14]:
 for name in ['duel','three','four','team']:
  path=ROOT/f'games/yuyuhakusho/verification/additional-arena-e2e/native/render/{arena:x}/{name}.json';d=json.loads(path.read_text())
  configs.append({'name':f'arena-{arena:x}-{name}','arena':arena,'characters':d['characters'],'team':d['team'],'reference_path':str(path.relative_to(ROOT))})
 path=ROOT/f'games/yuyuhakusho/verification/additional-arena-pan/native/arena-{arena:x}.json'
 configs.append({'name':f'arena-{arena:x}-pan','arena':arena,'characters':[7,9],'team':False,'held_fixture':f'games/yuyuhakusho/verification/additional-arena-pan/native/arena-{arena:x}/held-inputs.json','reference_path':str(path.relative_to(ROOT))})
assert len(configs)==49
keys=[(c['arena'],tuple(c['characters']),c['team'],c.get('held_fixture'))for c in configs];assert len(set(keys))==49
def run(c):
 p=LOCAL/(c['name']+'.json');p.write_text(json.dumps(c)+'\n')
 r=subprocess.run([str(binary),'-config',str(p)],cwd=ROOT,env=env,text=True,capture_output=True)
 if r.returncode:raise RuntimeError(c['name']+': '+r.stderr[-2000:])
 v=json.loads(r.stdout);assert v['pass'] and not v['reference_state_loaded']
 original=json.loads((ROOT/c['reference_path']).read_text())
 assert v['ticks']==original['ticks'] and v['pixel_trace_sha256']==original['pixel_trace_sha256'],(c['name'],'existing full pixel trajectory differs')
 print(c['name'],v['ticks'],'pass',flush=True);return v
with concurrent.futures.ThreadPoolExecutor(max_workers=3)as pool:rows=list(pool.map(run,configs))
payload={'schema':1,'scope':'Current unmodified runtimebridge; complete new configurations/fixed Held, no original CPU or reference state imported','rows':rows}
raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();(OUT/'bridge-reference.json').write_bytes(raw)
sources=[ROOT/'games/yuyuhakusho/runtimebridge/session.go',ROOT/'games/yuyuhakusho/presentation/compose.go',ROOT/'games/yuyuhakusho/presentation/hud.go',ROOT/'games/yuyuhakusho/cmd/capture-renderstate-reference/main.go',Path(__file__).resolve()]
manifest={'pass':True,'cases':49,'legacy_cases':14,'arena_cases':28,'pan_cases':7,'all_existing_full_pixel_traces_equal':True,'fixture_sha256':hashlib.sha256(raw).hexdigest(),'source_sha256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in sources}}
(OUT/'bridge-reference-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
