"""Reproduce native World DMA segment/progression and mask-restore evidence."""
import argparse,gzip,hashlib,json,os,shutil,subprocess,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/world-dma'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def frozen():
 first=json.loads((BASE/'test-first.json').read_text());blank=json.loads((BASE/'first-blank-adapter.json').read_text());correction=json.loads((BASE/'viewport-and-count-correction.json').read_text());assert first['test_first']and not first['production_exists']
 for path,digest in first['files'].items():
  if path==blank['frozen_path']:raw=gzip.decompress((ROOT/blank['frozen_archive']).read_bytes())
  elif path in correction['frozen_archives']:
   x=correction['frozen_archives'][path];raw=(ROOT/x['archive']).read_bytes();raw=gzip.decompress(raw)if x['compressed']else raw
  else:raw=(ROOT/path).read_bytes()
  assert hashlib.sha256(raw).hexdigest()==digest,path
 assert hashlib.sha256(gzip.decompress((ROOT/correction['first_blank_test_revision_archive']).read_bytes())).hexdigest()==blank['current_sha256']
 for path,digest in correction['current_sha256'].items():assert sha(ROOT/path)==digest,path
 assert 'undefined: videoclock.DMATarget'in(BASE/'missing-api-red.jsonl').read_text()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','native-race','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');a=p.parse_args();frozen()
 if a.audit:
  m=json.loads((BASE/'validation.json').read_text());assert m['pass']
  for group in ['sources','artifacts']:
   for path,digest in m[group].items():assert sha(ROOT/path)==digest,path
  for path,info in m.get('compressed_logs',{}).items():
   raw=gzip.decompress((ROOT/path).read_bytes());assert len(raw)==info['raw_bytes']and hashlib.sha256(raw).hexdigest()==info['raw_sha256']
  print(json.dumps({'pass':True,'sources':len(m['sources']),'artifacts':len(m['artifacts'])}));return
 assert a.mode and a.out;out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
 env=dict(os.environ,TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2');go=shutil.which('go');assert go
 command=[go,'test','-json','-count=1','-timeout=5m']
 if a.mode=='native-race':command+=['-race']
 elif a.mode=='wasm':
  goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1',GODEBUG='gccheckmark=1,clobberfree=1')
 command+=['./games/yuyuhakusho/audio/videoclock']
 if a.mode=='vet':command=[go,'vet','./games/yuyuhakusho/audio/videoclock','./games/yuyuhakusho/engine','./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/session']
 start=time.time()
 with(out/'output.jsonl').open('w')as log:proc=subprocess.run(command,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
 record={'command':command,'cwd':str(ROOT),'environment':{k:env[k]for k in ['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH','GODEBUG']if k in env},'exit_code':proc.returncode,'elapsed_seconds':time.time()-start,'output_sha256':sha(out/'output.jsonl')}
 if a.mode!='vet':
  rows=[json.loads(l)for l in(out/'output.jsonl').read_text().splitlines()];record['tests_passed']=sum(r['Action']=='pass'and'Test'in r for r in rows);record['top_level_passed']=sum(r['Action']=='pass'and'Test'in r and '/'not in r['Test']for r in rows);record['failures']=[r for r in rows if r['Action']in['fail','build-fail','skip']]
 (out/'command.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));raise SystemExit(proc.returncode)
if __name__=='__main__':main()
