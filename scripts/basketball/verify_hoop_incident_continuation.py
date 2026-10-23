"""Replay the captured incident to its original confirmed result, including save/restore.

Uses the shipped native runtime bridge; no network or game-state overrides after
restore. Only the first physical controller and intermission START are sampled.
Run from the repository root after sourcing scripts/env.sh.
"""
import argparse,gzip,hashlib,json,subprocess,time
from pathlib import Path
root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser();parser.add_argument('--controls',action='store_true');args=parser.parse_args()
out=root/'.local/basketball-incident-20261023';out.mkdir(parents=True,exist_ok=True)
variant='controls' if args.controls else 'idle'
initial=json.loads(gzip.decompress((root/'games/basketball/engine/testdata/hoop-air-throw-runtime.json.gz').read_bytes()))
process=subprocess.Popen(['go','run','./games/basketball/cmd/runtime-stdio'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True,cwd=root)
def request(r):
 process.stdin.write(json.dumps(r)+'\n');process.stdin.flush();return json.loads(process.stdout.readline())
report={'initialFrame':initial['videoFrames'],'samples':[],'status':'running','controls':args.controls,'restores':0}
digest=hashlib.sha256()
try:
 r=request({'op':'restore','checkpoint':{'version':1,'state':initial}});assert r['ok'],r
 state=r['data']['state'];session=r['data']['session'];start=time.time();intermission=False
 for n in range(1800):
  held=[0,0,0,0];ticks=30
  if args.controls:held[0]=[1,1|128,1,0,2,2|64,2,0,0,192,0,128][n%12]
  if state.get('awaitingNextPeriod') and state.get('waitVideoFrames',0)==0:held[0]=16;ticks=1;intermission=True
  before=state
  r=request({'op':'advance','session':session,'held':held,'ticks':ticks})
  if not r['ok']:
   (out/('continuation-'+variant+'-failure-state.json')).write_text(json.dumps(before));report.update(status='failed',error=r['error'],priorFrame=before['videoFrames'],ticks=ticks,held=held);break
  state=r['data']['state']
  replica=request({'op':'restore','checkpoint':{'version':1,'state':before}});assert replica['ok'],replica
  replica_id=replica['data']['session']
  repeated=request({'op':'advance','session':replica_id,'held':held,'ticks':ticks})
  assert repeated['ok'] and repeated['data']['state']==state, 'Restoring the exact previous state changed subsequent rules'
  assert request({'op':'dispose','session':replica_id})['ok']
  report['restores']+=1;digest.update(json.dumps(state,sort_keys=True,separators=(',',':')).encode())
  outcome=state.get('outcome');playing=state.get('playing')
  if n%20==0:report['samples'].append({'frame':state['videoFrames'],'outcome':outcome,'period':playing['match']['Period'] if playing else None})
  if outcome and outcome.get('confirmed'):
   report.update(status='passed',finalFrame=state['videoFrames'],outcome=outcome,intermissionConfirmed=intermission);break
 else:report.update(status='limit',finalFrame=state['videoFrames'])
 report['elapsedSeconds']=round(time.time()-start,3);report['stateSHA256']=digest.hexdigest()
 (out/('continuation-'+variant+'.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report));assert report['status']=='passed',report
finally:process.stdin.close();process.wait(timeout=10)
