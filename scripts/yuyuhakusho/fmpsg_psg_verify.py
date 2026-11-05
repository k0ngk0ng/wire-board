"""Re-run the retained original PSG edge/state oracle in native/race/Wasm."""
from pathlib import Path
import argparse,hashlib,json,os,shutil,subprocess

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/fmpsg-psg'
PACKAGE='./games/yuyuhakusho/audio/chips/psg'

def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','race','wasm','vet','all'],default='all');a=p.parse_args();OUT.mkdir(parents=True,exist_ok=True)
 go=shutil.which('go');node=shutil.which('node');assert go and node
 env={key:os.environ[key]for key in ['HOME','LANG','LC_ALL','LC_CTYPE','TZ']if key in os.environ}
 env.update(PATH=str(Path(go).parent)+':'+str(Path(node).parent)+':/usr/bin:/bin',TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 subprocess.run([str(Path(go).parent/'gofmt'),'-w',str(ROOT/'games/yuyuhakusho/audio/chips/psg/psg.go'),str(ROOT/'games/yuyuhakusho/audio/chips/psg/psg_test.go')],cwd=ROOT,env=env,check=True)
 source={str(p.relative_to(ROOT)):digest(p)for p in sorted((ROOT/'games/yuyuhakusho/audio/chips/psg').rglob('*'))if p.is_file()}
 modes=['native','race','wasm','vet']if a.mode=='all'else[a.mode]
 for mode in modes:
  runenv=dict(env);cmd=[go,'vet',PACKAGE]if mode=='vet'else[go,'test','-json','-count=1']
  if mode=='race':cmd.append('-race')
  if mode=='wasm':
   runenv.update(GOOS='js',GOARCH='wasm');goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=runenv,text=True).strip();cmd.extend(['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')])
  if mode!='vet':cmd.append(PACKAGE)
  log=OUT/(mode+('.log'if mode=='vet'else'.jsonl'))
  with log.open('w')as f:result=subprocess.run(cmd,cwd=ROOT,env=runenv,stdout=f,stderr=subprocess.STDOUT)
  rows=[]if mode=='vet'else[json.loads(s)for s in log.read_text().splitlines()if s.startswith('{')]
  report={'pass':result.returncode==0,'mode':mode,'command':cmd,'exit_code':result.returncode,'source_sha256':source,'environment':runenv,'log_sha256':digest(log),'top_level_pass':sum(r.get('Action')=='pass'and'/'not in r.get('Test','')for r in rows if r.get('Test')),'subtest_pass':sum(r.get('Action')=='pass'and'/'in r.get('Test','')for r in rows if r.get('Test')),'producer_sha256':digest(Path(__file__))}
  (OUT/(mode+'-command.json')).write_text(json.dumps(report,indent=2)+'\n')
  print(json.dumps({k:v for k,v in report.items()if k not in ['source_sha256','environment']},indent=2),flush=True)
  if result.returncode:raise SystemExit(result.returncode)

if __name__=='__main__':main()
