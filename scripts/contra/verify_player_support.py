"""Verify the exact-player-masked stone recognizer and optionally replay inputs."""
import argparse,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence';BASE=ROOT/'.local/contra-player-support'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--replay',action='store_true');parser.add_argument('--label',default='verify');a=parser.parse_args();assert a.label.replace('-','').isalnum()
 tmp=BASE/'tmp';tmp.mkdir(parents=True,exist_ok=True);env={**os.environ,'GOCACHE':str(ROOT/'.local/go-build'),'GOMODCACHE':str(ROOT/'.local/go-mod'),'GOTMPDIR':str(tmp),'TMPDIR':str(tmp),'GOTOOLCHAIN':'local'}
 files=sorted((ROOT/'games/contra/engine').glob('bot_vision*.go'))+[ROOT/'games/contra/engine/testdata/bot-waterfall-player-covered-stone.png',ROOT/'games/contra/engine/testdata/bot-waterfall-partial-player-stone.png',ROOT/'scripts/contra/replay_player_support.go']
 before={str(p.relative_to(ROOT)):sha(p)for p in files};command=['go','test','-race','./games/contra/engine','-run','^TestBotPlayerCovered','-count=1','-json'];log=E/'bot-player-covered-supports-bound-race.jsonl'
 with log.open('w')as f:r=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 after={str(p.relative_to(ROOT)):sha(p)for p in files};changed=[p for p in before if before[p]!=after[p]]
 result={'passed':r.returncode==0 and not changed,'command':command,'exitCode':r.returncode,'sourceChangedDuringCheck':changed,'sourceHashes':before,'logSHA256':sha(log),'scope':'Only fixed moving-stone art may omit opaque points of an accepted exact player. No World/OnEnemy input, generic enemy partial matching, policy/nav or Session change.','regressionLog':'bot-player-covered-supports-race.jsonl','regressionLogSHA256':sha(E/'bot-player-covered-supports-race.jsonl')}
 if a.replay:
  out=BASE/('recheck-'+a.label);assert not out.exists(),'use a new replay label'
  replay=['go','run','scripts/contra/replay_player_support.go','games/contra/engine/testdata/player-support-replay',str(out),'25924','25928'];subprocess.run(replay,cwd=ROOT,env=env,check=True)
  observed=json.loads((out/'observations.json').read_text());actual={};expect={25924:[(168,127)],25925:[(169,127)],25926:[(170,127)],25927:[],25928:[(171,127)]}
  for row in observed:
   stones=[(e['X'],e['Y'])for e in row['observation']['Enemies']or[]if e['Kind']==16 and e['Sprite']==72];assert stones==expect[row['frame']],(row['frame'],stones);actual[row['frame']]=stones
  assert (out/'frame-25926.png').read_bytes()==(ROOT/'games/contra/engine/testdata/bot-waterfall-player-covered-stone.png').read_bytes()
  result['replay']={'command':replay,'sourcePadsSHA256':sha(ROOT/'games/contra/engine/testdata/player-support-replay/pads.bin.gz'),'declaredLifeGrantsSHA256':sha(ROOT/'games/contra/engine/testdata/player-support-replay/test-life-refills.json'),'supportPositions':actual,'observationsSHA256':sha(out/'observations.json'),'fixturePixelsIdentical':True,'limitation':'Diagnostic replay retains the original declared life assistance; this is not a natural standard-lives campaign or policy win.'}
 (E/'bot-player-covered-supports-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'passed':result['passed'],'changed':changed,'replayed':a.replay}));tmp.rmdir();raise SystemExit(r.returncode or bool(changed))
if __name__=='__main__':main()
