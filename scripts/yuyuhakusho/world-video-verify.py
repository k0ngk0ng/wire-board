"""Repeat the frozen VDP signal/counter tests without a ROM or emulator."""
import argparse,hashlib,json,os,shutil,subprocess,time
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/world-video'
PACKAGE=ROOT/'games/yuyuhakusho/audio/videoclock'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--mode',choices=['native','wasm','vet'],required=True)
 p.add_argument('--out',type=Path,required=True);a=p.parse_args()
 out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 frozen=json.loads((BASE/'api-test-frozen.json').read_text());assert not frozen['production_exists']
 for row in frozen['artifacts']:
  path=ROOT/row['path']
  if path==PACKAGE/'clock_test.go':path=BASE/'clock-test-v1.go.txt'
  assert sha(path)==row['sha256'],('Frozen pre-implementation source changed',row['path'])
 merge=json.loads((BASE/'merge-test-first.json').read_text())
 assert merge['v1_production_unchanged'] and merge['actual_semantic_red'] and merge['actual_missing_effect_API_red']
 for path,digest in merge['files'].items():assert sha(ROOT/path)==digest,('Merge test-first artifact changed',path)
 assert sha(PACKAGE/'clock_test.go')==merge['frozen_test_sha256']
 assert (PACKAGE/'clock_test.go').read_bytes()==(BASE/'clock-merge-frozen-test.go.txt').read_bytes()
 assert 'undefined: State' in (BASE/'final-tests-missing-api-red.jsonl').read_text()
 fixture=json.loads((BASE/'fixture.json').read_text())
 for key in['fixture','source','capture','producer']:
  row=fixture[key];assert sha(ROOT/row['path'])==row['sha256'],('Native fixture source changed',row['path'])
 files=[PACKAGE/'clock.go',PACKAGE/'clock_test.go',PACKAGE/'testdata/natural.json.gz',Path(__file__),BASE/'fixture.json',BASE/'api-test-frozen.json',BASE/'merge-test-first.json']
 sources={str(q.relative_to(ROOT)):sha(q)for q in files}
 env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2')
 for key in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR']:Path(env[key]).mkdir(parents=True,exist_ok=True)
 go=shutil.which('go');assert go
 cmd=['nice','-n','10',go,'test','-json','-count=1','-timeout=2m']
 if a.mode=='native':cmd.append('-race')
 elif a.mode=='wasm':
  goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip()
  executor=Path(goroot)/'lib/wasm/go_js_wasm_exec';assert executor.is_file()
  cmd+=['-exec',str(executor)];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
 else:cmd=['nice','-n','10',go,'vet']
 cmd.append('./games/yuyuhakusho/audio/videoclock')
 started=time.monotonic();r=subprocess.run(cmd,cwd=ROOT,env=env,capture_output=True,text=True)
 log=out/(a.mode+('.txt'if a.mode=='vet'else'.jsonl'));log.write_text(r.stdout+r.stderr)
 passed=[]
 if a.mode!='vet':
  rows=[json.loads(line)for line in r.stdout.splitlines()if line.startswith('{')]
  passed=[row['Test']for row in rows if row['Action']=='pass'and'Test'in row]
  assert not any(row['Action']=='fail'for row in rows)if r.returncode==0 else True
 assert sources=={q:sha(ROOT/q)for q in sources},'VDP sources changed during verification'
 report={'pass':r.returncode==0,'mode':a.mode,'exit_code':r.returncode,'elapsed_seconds':time.monotonic()-started,'command':cmd,
  'environment':{key:env[key]for key in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if key in env},
  'passed_tests':passed,'source_sha256':sources,'log_sha256':sha(log),
  'native_frames':15,'native_scalar_states':fixture['scalar_states'],'native_reads':fixture['reads'],
  'scope':'Fixed NTSC224 H40 VDP counters/signals given ordered external operations; not CPU acceptance, full status/VDP or independent world/audio scheduling'}
 (out/(a.mode+'-command.json')).write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({'pass':report['pass'],'mode':a.mode,'passed_tests':len(passed),'log_sha256':report['log_sha256']}))
 if r.returncode:print(log.read_text()[-6000:]);raise SystemExit(r.returncode)

if __name__=='__main__':main()
