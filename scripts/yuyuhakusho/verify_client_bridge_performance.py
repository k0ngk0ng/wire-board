"""Performance revision verification; original schema2 freeze remains untouched."""
from pathlib import Path
from datetime import datetime,timezone
import argparse,concurrent.futures,hashlib,json,os,subprocess
ROOT=Path(__file__).resolve().parents[2]
DATA=ROOT/'games/yuyuhakusho/runtimebridge/testdata/performance-v1'
OLD=ROOT/'games/yuyuhakusho/runtimebridge/testdata/schema2-final-freeze.json'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def read(p):return json.loads(p.read_text())
def require(v,m):
 if not v:raise RuntimeError(m)
def sources():
 paths={ROOT/name for name in read(OLD)['source_sha256']}
 paths.update((ROOT/'games/yuyuhakusho/runtimebridge').glob('*.go'))
 paths.add(Path(__file__).resolve())
 return {str(p.relative_to(ROOT)):sha(p)for p in sorted(paths)}
def frozen():
 m=read(DATA/'test-first.json');lineage=read(DATA/'scene-fixture-lineage.json')
 for name,digest in m['sha256'].items():
  if name.endswith('/performance_test.go'):
   require(sha(ROOT/lineage['old_test'])==digest==lineage['old_sha256'],'test archive changed');digest=lineage['new_sha256']
  require(sha(ROOT/name)==digest,'performance test-first changed '+name)
 old=read(OLD)['source_sha256']
 for name in ['games/yuyuhakusho/runtimebridge/schema2_test.go','games/yuyuhakusho/runtimebridge/testdata/schema1-save-baseline.json']:
  if name in old:require(sha(ROOT/name)==old[name],'old frozen test changed '+name)
 return m

def environment():
 env={k:v for k,v in os.environ.items()if k in ['PATH','HOME','LANG','LC_ALL','LC_CTYPE']}
 env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 return env

def flows(kind):
 frozen();base_sources=sources();env=environment();command=['go','test','-json','-count=1','-timeout=45m']
 if kind=='wasm':
  goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',goroot+'/lib/wasm/go_js_wasm_exec'];env.update(GOOS='js',GOARCH='wasm')
 command+=['./games/yuyuhakusho/runtimebridge','-run','^TestSchema2FrozenWholeTrajectories$']
 def run(i):
  ee=dict(env,CLIENT_BRIDGE_SHARDS='4',CLIENT_BRIDGE_SHARD=str(i));record=DATA/f'flows-{kind}-{i}.command.json';log=DATA/f'flows-{kind}-{i}.jsonl'
  meta={'command':command,'started_at_utc':datetime.now(timezone.utc).isoformat(),'source_sha256':base_sources,'shard':i,'shards':4,'environment_overrides':{k:ee[k]for k in ['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','CLIENT_BRIDGE_SHARDS','CLIENT_BRIDGE_SHARD']},'complete':False}
  record.write_text(json.dumps(meta,indent=2)+'\n')
  with log.open('w')as f:p=subprocess.run(command,cwd=ROOT,env=ee,stdout=f,stderr=subprocess.STDOUT)
  meta.update(complete=True,ended_at_utc=datetime.now(timezone.utc).isoformat(),exit_code=p.returncode,log_sha256=sha(log),sources_unchanged=sources()==base_sources)
  record.write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps({'suite':kind,'shard':i,'exit_code':p.returncode,'sources_unchanged':meta['sources_unchanged']}),flush=True);return meta
 with concurrent.futures.ThreadPoolExecutor(max_workers=2)as pool:rows=list(pool.map(run,range(4)))
 require(all(r['exit_code']==0 and r['sources_unchanged']for r in rows),'flow verification failed')

def vet():
 frozen();ss=sources();env=environment();command=['go','vet','./games/yuyuhakusho/runtimebridge'];log=DATA/'vet.txt'
 with log.open('w')as f:p=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 meta={'command':command,'source_sha256':ss,'exit_code':p.returncode,'sources_unchanged':ss==sources(),'log_sha256':sha(log)};(DATA/'vet.command.json').write_text(json.dumps(meta,indent=2)+'\n');require(meta['exit_code']==0 and meta['sources_unchanged'],'vet failed')

def report():
 frozen();ss=sources();expected={r['configuration']['name']for r in read(ROOT/'games/yuyuhakusho/renderstate/testdata/bridge-reference.json')['rows']};suites={}
 for kind in ['native','wasm']:
  names=[]
  for i in range(4):
   p=DATA/f'flows-{kind}-{i}.jsonl';meta=read(DATA/f'flows-{kind}-{i}.command.json');require(meta['complete']and meta['exit_code']==0 and meta['sources_unchanged']and meta['source_sha256']==ss,'flow source metadata differs')
   require(sha(p)==meta['log_sha256'],'flow log changed');rows=[json.loads(x)for x in p.read_text().splitlines()];require(not any(r.get('Action')=='fail'for r in rows),'flow failure');prefix='TestSchema2FrozenWholeTrajectories/';names+=[r['Test'][len(prefix):]for r in rows if r.get('Action')=='pass'and r.get('Test','').startswith(prefix)]
  require(len(names)==49 and set(names)==expected,'frozen49 coverage differs');suites[kind]={'complete':True,'trajectories':49,'reference_frames':203428}
 for kind in ['native','wasm']:
  p=DATA/f'final-{kind}.jsonl';meta=read(DATA/f'final-{kind}.command.json');require(meta['exit_code']==0,'targeted test failed');rows=[json.loads(x)for x in p.read_text().splitlines()];require(not any(r.get('Action')=='fail'for r in rows),'targeted failure')
  for name,digest in meta['source_sha256'].items():require(ss[name]==digest,'targeted source changed '+name)
  suites['targeted_'+kind]={'pass':True,'top_level_tests':sum(r.get('Action')=='pass'and'Test'in r and'/'not in r['Test']for r in rows)}
 for name in ['baseline-native','baseline-wasm','optimized-native','optimized-wasm']:
  benchmark=read(DATA/(name+'.command.json'));require(benchmark['exit_code']==0,'benchmark failed '+name)
  session='games/yuyuhakusho/runtimebridge/session.go'
  if name.startswith('baseline-'):require(benchmark['source_sha256'][session]==read(OLD)['source_sha256'][session],'baseline does not bind original production')
  else:
   for source in [session,'games/yuyuhakusho/runtimebridge/performance.go']:require(benchmark['source_sha256'][source]==ss[source],'optimized benchmark production changed')
 vm=read(DATA/'vet.command.json');require(vm['exit_code']==0 and vm['sources_unchanged']and vm['source_sha256']==ss,'vet source changed')
 artifacts={str(p.relative_to(ROOT)):sha(p)for p in DATA.iterdir()if p.is_file()and p.name!='revision-manifest.json'}
 result={'schema':1,'revision':'performance-v1','pass':True,'at_utc':datetime.now(timezone.utc).isoformat(),'base_freeze':str(OLD.relative_to(ROOT)),'base_freeze_sha256':sha(OLD),'source_sha256':ss,'frozen_tests':read(DATA/'test-first.json'),'fixture_correction':read(DATA/'scene-fixture-lineage.json'),'suites':suites,'artifact_sha256':artifacts,'scope':'typed deep fork and canonical no-RGBA advance; actual platform FPS/RPC evidence is separately owned','production_changes':['games/yuyuhakusho/runtimebridge/performance.go','games/yuyuhakusho/runtimebridge/session.go (Preview uses typed Fork only)']}
 (DATA/'revision-manifest.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'pass':True,'sources':len(ss),'suites':suites},indent=2))

if __name__=='__main__':
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('suite',choices=['native','wasm','vet','report']);a=p.parse_args()
 if a.suite in['native','wasm']:flows(a.suite)
 elif a.suite=='vet':vet()
 else:report()
