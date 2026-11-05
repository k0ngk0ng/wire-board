"""Freeze or audit already executed native/Wasm sound-chip evidence.

This does not replace the Go commands recorded in validation.json. It rejects
missing/failed logs, changed frozen native fixture producers, or changed source
and evidence identities. All writes remain in the selected worktree directory.
"""
from pathlib import Path
import argparse,gzip,hashlib,json

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/fmpsg-ym'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def record(path):
 raw=path.read_bytes();return dict(path=str(path.relative_to(ROOT)),bytes=len(raw),sha256=sha(raw))
def report_log(path):
 rows=[json.loads(s)for s in path.read_text().splitlines()if s.startswith('{')]
 assert not any(x.get('Action')in ['fail','build-fail']for x in rows),str(path)
 passed=[x for x in rows if x.get('Action')=='pass']
 top=[x for x in passed if x.get('Test')and'/'not in x['Test']]
 sub=[x for x in passed if '/'in x.get('Test','')]
 packages=[x['Package']for x in passed if 'Test'not in x]
 assert len(top)==14 and len(sub)==57 and len(packages)==3,(path,len(top),len(sub),packages)
 return dict(top_level=len(top),sub_scenarios=len(sub),packages=packages,log=record(path))
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--audit',action='store_true');a=p.parse_args()
 target=OUT/'validation.json'
 if a.audit:
  frozen=json.loads(target.read_text())
  for item in frozen['sources']+frozen['artifacts']:
   path=ROOT/item['path'];assert record(path)==item,('frozen identity changed',item['path'])
  print(json.dumps(dict(pass_=True,source_files=len(frozen['sources']),artifact_files=len(frozen['artifacts']),validation_sha256=sha(target.read_bytes()))));return
 native=json.loads((OUT/'native-capture.json').read_text());mixed_dir=OUT.parent/'fmpsg-mixed';mixed=json.loads((mixed_dir/'native-capture.json').read_text())
 assert native['pass']and mixed['pass']
 for metadata,path,producer in [(native,OUT/'ym-native.json.gz',ROOT/'scripts/yuyuhakusho/fmpsg_capture_ym.py'),(mixed,mixed_dir/'native-frames.json.gz',ROOT/'scripts/yuyuhakusho/fmpsg_capture_mixed.py')]:
  assert sha(path.read_bytes())==metadata['gzip_sha256']and sha(gzip.decompress(path.read_bytes()))==metadata['json_sha256']
  assert sha(producer.read_bytes())==metadata['producer_sha256']
 assert (native['cases'],native['steps'],native['stereo_samples'])==(29,416,616921)
 assert (mixed['cases'],mixed['frames'],mixed['stereo_frames'])==(5,192,141304)
 checks=dict(native_race=report_log(OUT/'race-final.jsonl'),wasm=report_log(OUT/'wasm-final.jsonl'))
 assert (OUT/'vet-final.txt').read_text()==''
 paths=set()
 for folder in ['games/yuyuhakusho/audio/chips','games/yuyuhakusho/audio/dsp']:
  paths.update(p for p in (ROOT/folder).rglob('*')if p.is_file()and p.suffix in ['.go','.md'])
 paths.update((ROOT/'scripts/yuyuhakusho').glob('fmpsg*.py'))
 paths.update((ROOT/'docs/research/md-yuyuhakusho').glob('FMPSG*.md'))
 paths.update(ROOT/p for p in ['go.mod','go.sum','scripts/yuyuhakusho/inspect_dac_dsp_state.py','scripts/yuyuhakusho/extract_audio_assets.py','scripts/yuyuhakusho/research_audio_loops.py','scripts/yuyuhakusho/reference.py'])
 sources=[record(p)for p in sorted(paths)]
 evidence=set(p for folder in [OUT,mixed_dir,OUT.parent/'fmpsg-psg']for p in folder.rglob('*')if p.is_file()and p!=target)
 evidence.update(p for p in (OUT.parent).glob('fmpsg-psg-*')if p.is_file())
 evidence.update(p for p in (ROOT/'games/yuyuhakusho/audio/chips/psg/testdata').rglob('*')if p.is_file())
 artifacts=[record(p)for p in sorted(evidence)]
 env=dict(PATH='/opt/homebrew/bin:/Users/jason/.nvm/versions/node/v24.15.0/bin:/usr/bin:/bin',LANG='C.UTF-8',LC_ALL='C.UTF-8',LC_CTYPE='C.UTF-8',TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 package=['./games/yuyuhakusho/audio/chips/...','./games/yuyuhakusho/audio/dsp']
 result=dict(pass_=True,scope='Full fixed-profile native/Wasm YM+PSG+DAC synthesis given declared ordered chip inputs and times; no world/firmware/independent timing or speaker equivalence claim',native_raw_ym=native,native_mixed=mixed,checks=checks,vet_pass=True,standalone_go=True,world_runtime_wired=False,score_runtime_wired=False,independent_event_schedule_solved=False,source_manifest_sha256=sha(json.dumps(sources,separators=(',',':')).encode()),sources=sources,artifacts=artifacts,commands=dict(native_race=dict(argv=['/opt/homebrew/bin/go','test','-race','-json','-count=1',*package],environment=env),wasm=dict(argv=['/opt/homebrew/bin/go','test','-exec','/opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec','-json','-count=1',*package],environment=dict(env,GOOS='js',GOARCH='wasm')),vet=dict(argv=['/opt/homebrew/bin/go','vet',*package],environment=env)))
 result['pass']=result.pop('pass_');target.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(dict(pass_=True,sources=len(sources),artifacts=len(artifacts),validation_sha256=sha(target.read_bytes()))))
if __name__=='__main__':main()
