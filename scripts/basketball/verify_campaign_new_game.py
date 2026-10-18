"""Export existing original new-game evidence into a small typed-factory oracle.
This does not recapture gameplay or regenerate expected values with Go's RNG.
"""
import argparse,gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
SOURCE=ROOT/'.local/basketball/rea/campaign-new-game/calls.json'
EXPECTED='08836c0a95cf13a9319f65506efd721f57b3bab34adfc360db67773ac7342c03'
def signed(v):return v if v<128 else v-256
def progress(s,mode,reward):
 return dict(mode=mode,period=s['period'],winBits=s['wins'],rewardSet=reward,inventory=[signed(v) for v in s['inventory']],equipment=[[signed(v) for v in s['equipment'][i:i+3]] for i in range(0,12,3)])
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');a=parser.parse_args()
 raw=SOURCE.read_bytes();assert hashlib.sha256(raw).hexdigest()==EXPECTED
 d=json.loads(raw);empty=d['coldClearThenEmptyInitialization'][-1]['after'];assert empty['period']==0 and empty['wins']==[0,0] and empty['inventory']==[255]*16 and empty['equipment']==[255]*12
 modes=[r['after']['mode'] for r in d['menuChoiceCalls'] if 'selectedPlayers' in r];assert modes==[0,1]
 rows=[]
 for index,r in enumerate(d['rewardEntryCalls']):
  if r['entered']:assert len(r['randomCalls'])==0;continue
  assert len(r['randomCalls'])==1
  for mode in modes:
   rows.append(dict(sourceCall=index,mode=mode,randomBeforeRewardDraw=r['before']['random'],expected=dict(progress=progress(empty,mode,r['after']['rewardSet']),random=r['after']['random']),scope='composition of original empty initialization, A205 reward draw and independent source Mode0/1 selection; not a continuous natural menu trace'))
 assert len(rows)==64
 natural=d['naturalBootBoundaries'][-1];calls=natural['call']['randomCalls'];assert len(calls)==2
 edge=dict(beforeVideoFrame=natural['beforeFrame'],afterVideoFrame=natural['afterFrame'],outerBefore=calls[0]['state'],randomBeforeRewardDraw=calls[1]['state'],expected=dict(progress=progress(natural['observed'],0,natural['observed']['rewardSet']),random=natural['observed']['random']),scope=natural['scope'])
 fixture=dict(calls=rows,naturalBoundary=edge)
 data=gzip.compress((json.dumps(fixture,separators=(',',':'))+'\n').encode(),mtime=0)
 out=ROOT/'games/basketball/engine/testdata';target=out/'campaign-new-game-calls.json.gz'
 meta=dict(records=len(rows),source=str(SOURCE.relative_to(ROOT)),sourceSHA256=EXPECTED,fixtureSHA256=hashlib.sha256(data).hexdigest(),factoryBoundary='already initialized random register immediately before A205 inner draw; excludes CABC outer draw',normalModes=modes,reusedREA=['ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd','ev_efaf6f5b71e4d47395c79ea8a9a16de695c11c8f081322e086bd9b7b108fdc1c'],bootSeedScope='bootstrap RAM dependency deliberately outside factory; no hardcoded observed seed',normalBoundaryLimitation=d['navigationIdentity']['limitation'])
 if a.check:assert target.read_bytes()==data
 else:
  target.write_bytes(data);target.with_name('campaign-new-game-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
