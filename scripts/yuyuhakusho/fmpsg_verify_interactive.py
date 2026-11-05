"""Bind current interactive chip source to new and unchanged native oracles."""
from pathlib import Path
import argparse,base64,gzip,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/fmpsg-interactive'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def entry(path):
 raw=path.read_bytes();return dict(path=str(path.relative_to(ROOT)),bytes=len(raw),sha256=sha(raw))
def results(path):
 rows=[json.loads(line)for line in path.read_text().splitlines()if line.startswith('{')]
 assert not any(x.get('Action')in ['fail','build-fail']for x in rows),path
 passed=[x for x in rows if x.get('Action')=='pass']
 top=[x for x in passed if x.get('Test')and'/'not in x['Test']];sub=[x for x in passed if'/'in x.get('Test','')]
 packages=[x['Package']for x in passed if 'Test'not in x]
 assert (len(top),len(sub),len(packages))==(17,62,3),(path,len(top),len(sub),packages)
 return dict(top_level=len(top),sub_scenarios=len(sub),packages=packages,log=entry(path))
def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--audit',action='store_true');args=parser.parse_args();target=OUT/'validation.json'
 if args.audit:
  report=json.loads(target.read_text())
  for item in report['sources']+report['artifacts']:assert entry(ROOT/item['path'])==item,('frozen file changed',item['path'])
  print(json.dumps(dict(pass_=True,sources=len(report['sources']),artifacts=len(report['artifacts']),sha256=sha(target.read_bytes()))));return
 metadata=json.loads((OUT/'native-capture.json').read_text());raw=gzip.decompress((OUT/'native.json.gz').read_bytes());fixture=json.loads(raw)
 assert metadata['pass']and metadata['json_sha256']==sha(raw)and metadata['gzip_sha256']==sha((OUT/'native.json.gz').read_bytes())
 assert metadata['producer_sha256']==sha((ROOT/'scripts/yuyuhakusho/fmpsg_capture_interactive.py').read_bytes())
 events=sum(not x['Boundary']for c in fixture['Cases']for x in c['Operations']);frames=sum(x['Boundary']for c in fixture['Cases']for x in c['Operations']);samples=sum(len(base64.b64decode(x['PCM']))//4 for c in fixture['Cases']for x in c['Operations']if x['Boundary'])
 assert (len(fixture['Cases']),events,frames,samples,metadata['projection_nochange_checks_per_core'])==(5,1109,13,9563,1127)
 first=json.loads((OUT/'test-first.json').read_text());oldfile=OUT.parent/'fmpsg-ym/validation.json';old=json.loads(oldfile.read_text());before=OUT/'stream-before-interactive.go.txt'
 expected=next(x for x in old['sources']if x['path']=='games/yuyuhakusho/audio/chips/stream.go')['sha256']
 assert expected==sha(before.read_bytes())==first['old_stream_sha256']
 unchanged=[]
 for item in old['sources']:
  path=item['path']
  if path.startswith('games/yuyuhakusho/audio/')and path.endswith('.go')and path!='games/yuyuhakusho/audio/chips/stream.go':
   assert entry(ROOT/path)==item,('unrelated frozen DSP source changed',path);unchanged.append(item)
 assert (OUT/'vet-final.txt').read_text()==''
 checks=dict(native_race=results(OUT/'race-final.jsonl'),wasm=results(OUT/'wasm-final.jsonl'))
 sources=set()
 for folder in ['games/yuyuhakusho/audio/chips','games/yuyuhakusho/audio/dsp']:
  sources.update(p for p in (ROOT/folder).rglob('*')if p.is_file()and p.suffix in ['.go','.md'])
 sources.update((ROOT/'scripts/yuyuhakusho').glob('fmpsg*.py'))
 sources.update((ROOT/'docs/research/md-yuyuhakusho').glob('FMPSG*.md'))
 sources.update(ROOT/p for p in ['go.mod','go.sum','scripts/yuyuhakusho/reference.py','scripts/yuyuhakusho/extract_audio_assets.py','scripts/yuyuhakusho/inspect_dac_dsp_state.py'])
 recorded=[entry(p)for p in sorted(sources)]
 artifacts=set(p for p in OUT.rglob('*')if p.is_file()and p!=target)
 artifacts.add(oldfile)
 # Bind the exact prior rawYM/mixed/PSG fixtures which the current logs replay.
 for item in old['artifacts']:
  if item['path'].endswith(('.json.gz','.json')):
   path=ROOT/item['path'];assert entry(path)==item;artifacts.add(path)
 env=dict(PATH='/opt/homebrew/bin:/Users/jason/.nvm/versions/node/v24.15.0/bin:/usr/bin:/bin',LANG='C.UTF-8',LC_ALL='C.UTF-8',LC_CTYPE='C.UTF-8',TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 packages=['./games/yuyuhakusho/audio/chips/...','./games/yuyuhakusho/audio/dsp']
 report=dict(pass_=True,scope='Immediate interactive chip feedback, in-flight native-projected FIR/state and unchanged queued PCM/state oracles; no original driver CPU costs or independently assigned event times claim',native=metadata,checks=checks,vet_pass=True,prior_frozen_validation=entry(oldfile),preserved_prior_stream=entry(before),unchanged_frozen_audio_go_sources=unchanged,sources=recorded,source_manifest_sha256=sha(json.dumps(recorded,separators=(',',':')).encode()),artifacts=[entry(p)for p in sorted(artifacts)],commands=dict(native_race=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-race','-json','-count=1',*packages],environment=env),wasm=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-exec','/opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec','-json','-count=1',*packages],environment=dict(env,GOOS='js',GOARCH='wasm')),vet=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','vet',*packages],environment=env)))
 report['pass']=report.pop('pass_');target.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(dict(pass_=True,sources=len(recorded),artifacts=len(artifacts),sha256=sha(target.read_bytes()))))
if __name__=='__main__':main()
