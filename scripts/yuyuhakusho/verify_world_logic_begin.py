"""Reproduce actual input/clock/RNG/feedback work and joint logical World entry."""
import argparse,gzip,hashlib,json,os,shutil,subprocess,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/world-logic-begin'
FRAME=ROOT/'games/yuyuhakusho/verification/world-frame-work'
COMMON='PCMTransferGate|ScheduledDMA|PCMDeviceFork|Segmented|Original|IndependentReset|CompleteChipControl|ConstructionInput|ActualYMAddress|PhysicalOutput|RestoreRequires|OpaqueMetadata|SameSlot|UnscheduledDMA|PendingBus|MixedRuntime|BattleWorldEntry|BattleSamplePrefix|BattleReturn|SoundIRQ|WorldSoundLiveJoint|WorldClock|WorldFlow|WorldDMA|WorldFrame|WorldLogicBegin|WorldObjectsReady|ControllerNormalizationWork|ControllerAgainstOriginal|ControllerBoundaryEdges|ControllerRejects|CallOrderReload|InvalidInputAndSnapshot|M68KRead|PendingIsNot|StatusBoundary|HorizontalRequest|Random'
PACKAGES=['./games/yuyuhakusho/engine','./games/yuyuhakusho/audio/pcmcommands','./games/yuyuhakusho/audio/chips','./games/yuyuhakusho/audio/videoclock','./games/yuyuhakusho/audio/scoreclock']
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def check_record(path,archives):
 first=json.loads(path.read_text());assert first['test_first']and not first['production_exists']
 for name,digest in first['files'].items():
  if name in archives:
   x=archives[name];raw=(ROOT/x['archive']).read_bytes();raw=gzip.decompress(raw)if x.get('compressed',True)else raw
   assert hashlib.sha256(raw).hexdigest()==digest,name
  else:assert sha(ROOT/name)==digest,name
def frozen():
 correction=json.loads((BASE/'context-correction.json').read_text());pause=json.loads((FRAME/'pause-hook-adapter.json').read_text());indices=json.loads((BASE/'objects-index-adapter.json').read_text());archives=dict(correction['frozen_archives']);archives[pause['frozen_path']]={'archive':pause['archive']};archives[indices['frozen_path']]={'archive':indices['archive']}
 for p in [FRAME/'test-first.json',FRAME/'live-test-first.json',BASE/'test-first.json',BASE/'objects-test-first.json']:check_record(p,archives)
 for c in [pause,indices]:assert sha(ROOT/c['frozen_path'])==c['current_sha256']
 for p,digest in correction['current_sha256'].items():assert sha(ROOT/p)==digest,p
 norm=ROOT/'games/yuyuhakusho/verification/controller-normalization-work';result=json.loads((norm/'result.json').read_text());assert result['pass']
 for p,digest in result['manifest_files'].items():assert sha(ROOT/p)==digest,p
 assert 'AdvanceWorldClockWork'in(FRAME/'missing-api-red.jsonl').read_text()
 assert 'undefined: scoreclock.AdvanceBattleWorldLogicBegin'in(BASE/'missing-api-red.jsonl').read_text()
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','native-race','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');a=p.parse_args();frozen()
 if a.audit:
  m=json.loads((BASE/'validation.json').read_text());assert m['pass']
  for group in ['sources','artifacts']:
   for name,digest in m[group].items():assert sha(ROOT/name)==digest,name
  for name,info in m.get('compressed_logs',{}).items():
   raw=gzip.decompress((ROOT/name).read_bytes());assert len(raw)==info['raw_bytes']and hashlib.sha256(raw).hexdigest()==info['raw_sha256']
  print(json.dumps({'pass':True,'sources':len(m['sources']),'artifacts':len(m['artifacts'])}));return
 assert a.mode and a.out;out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
 env=dict(os.environ,TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2');go=shutil.which('go');assert go
 world='ChuWorld'if a.mode!='native-race'else'ChuWorld(FutureSlots|Errors|RuleBundle|StartCells)'
 pattern='^Test('+COMMON+'|'+world+')'
 command=[go,'test','-json','-count=1','-timeout=20m','-run',pattern]
 if a.mode=='native-race':command+=['-race']
 elif a.mode=='wasm':
  goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1',GODEBUG='gccheckmark=1,clobberfree=1')
 command+=PACKAGES
 if a.mode=='vet':command=[go,'vet',*PACKAGES,'./games/yuyuhakusho/audio/session']
 start=time.time()
 with(out/'output.jsonl').open('w')as log:proc=subprocess.run(command,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT)
 record={'command':command,'cwd':str(ROOT),'environment':{k:env[k]for k in ['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH','GODEBUG']if k in env},'exit_code':proc.returncode,'elapsed_seconds':time.time()-start,'output_sha256':sha(out/'output.jsonl')}
 if a.mode!='vet':
  rows=[json.loads(l)for l in(out/'output.jsonl').read_text().splitlines()];record['tests_passed']=sum(r['Action']=='pass'and'Test'in r for r in rows);record['top_level_passed']=sum(r['Action']=='pass'and'Test'in r and '/'not in r['Test']for r in rows);record['failures']=[r for r in rows if r['Action']in['fail','build-fail','skip']]
 (out/'command.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));raise SystemExit(proc.returncode)
if __name__=='__main__':main()
