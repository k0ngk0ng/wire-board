"""Original result/award/continue source oracle, without whole-machine recapture.
Gameplay instructions are unchanged. Explicit display/sound calls are skipped;
records stop before actor rendering or machine reset and state that boundary.
"""
import argparse,base64,gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
OUT=ROOT/'games/basketball/engine/testdata'
TRANSITION_ROWS=json.loads((OUT/'match-transition.json').read_text())
TRANSITION_RAW=gzip.decompress((OUT/'match-transition.ram.gz').read_bytes())
FIRST_RESULT=next(i for i,r in enumerate(TRANSITION_ROWS) if r['kind']=='result')
BASE=TRANSITION_RAW[FIRST_RESULT*4096:FIRST_RESULT*4096+2048]
PRESENTATION={0xc000:'sound request',0xc021:'palette fade/wait',0xc024:'background upload',0xc036:'message upload',0xbcf0:'result scoreboard glyph upload/NMI wait'}
class RecordedMemory(Memory):
 def __init__(self,ram):
  self.writes=[];super().__init__(PRG,ram)
 def __setitem__(self,a,v):
  if isinstance(a,int):
   self.writes.append([a,v])
   if 0x2000<=a<=0x2007 or a==0xa000:list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
def progress(r):
 signed=lambda v:v if v<128 else v-256
 return dict(mode=r[0x58d],period=r[0x58c],winBits=list(r[0x6d2:0x6d4]),rewardSet=r[0x6d4],inventory=[signed(v) for v in r[0xd3:0xe3]],equipment=[[signed(v) for v in r[0x6e4+i*3:0x6e7+i*3]] for i in range(4)])
def snapshot(r):
 return dict(progress=progress(r),scene=r[0x588],phase=r[0x589]&127,entered=bool(r[0x589]&128),firstTeamLost=bool(r[0x3c5]&128),opponent=r[0x587],court=r[0x58b],random=int.from_bytes(r[0x48:0x4c],'big'),characters=list(r[0x3e3:0x3e7]),controllerAssignments=[v if v<128 else v-256 for v in r[0x4e8:0x4ec]],resultTimer=r[0x3c3],scores=[r[0x166]+256*r[0x168],r[0x167]+256*r[0x169]],camera=list(r[0x397:0x39f]))
def call(r,bank,entry,stops,kind):
 m=RecordedMemory(r);m[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384];cpu=MPU(memory=m,pc=entry);cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff);rng=0;omitted=[]
 for _ in range(150000):
  if cpu.pc in stops:break
  if cpu.pc==0xf9c3:rng+=1
  if cpu.pc in PRESENTATION:
   omitted.append(dict(entry=f'{cpu.pc:04X}',kind=PRESENTATION[cpu.pc],a=cpu.a));cpu.pc=(cpu.stPopWord()+1)&65535
  else:cpu.step()
 else:raise RuntimeError((kind,hex(cpu.pc)))
 after=bytearray(m[:2048])
 if kind=='reward-entry':assert not any(0xe3<=a<=0xf4 for a,v in m.writes),'reward wrote beyond inventory16'
 watched=[[f'{a:04X}',v] for a,v in m.writes if 0xd3<=a<0xe3 or 0x6e4<=a<0x6f0 or a in [0x58c,0x58d,0x6d2,0x6d3,0x6d4,0x588,0x589]]
 rec=dict(kind=kind,entry=f'bank{bank}:{entry:04X}',stop=f'{cpu.pc:04X}',before=snapshot(r),after=snapshot(after),rawBefore=base64.b64encode(r).decode(),rawAfter=base64.b64encode(after).decode(),writes=watched,inventoryWrites=[[f'{a:04X}',v] for a,v in m.writes if 0xd3<=a<0xe3],equipmentWrites=[[f'{a:04X}',v] for a,v in m.writes if 0x6e4<=a<0x6f0],rngCalls=rng,omittedPresentationCalls=omitted)
 return after,rec

def set_progress(r,p):
 r[0x58d]=p['mode'];r[0x58c]=p['period'];r[0x6d2:0x6d4]=bytes(p['winBits']);r[0x6d4]=p['rewardSet'];r[0xd3:0xe3]=bytes(v&255 for v in p['inventory']);r[0x6e4:0x6f0]=bytes(v&255 for row in p['equipment'] for v in row)
def starting(mode,reward,opponent,period,shape):
 bit=1<<(opponent-1)
 wins={0:[0,0],2:[bit,0],24:[127^bit,127^bit],26:[127,127^bit]}[period]
 p=dict(mode=mode,period=period,winBits=wins,rewardSet=reward,inventory=[-1]*16,equipment=[[-1]*3 for _ in range(4)])
 if shape=='duplicates':p['inventory'][:3]=[18,18,1]
 if shape=='equipped':p['inventory'][:2]=[5,5];p['equipment'][0]=[1,14,26];p['equipment'][2]=[3,-1,24]
 if shape=='last-slot':p['inventory'][:15]=[18]*15
 if shape=='full':p['inventory']=[18]*16
 return p

def settlement(mode,reward,opponent,period,shape,won):
 p=starting(mode,reward,opponent,period,shape);r=bytearray(BASE);set_progress(r,p)
 r[0x588]=1;r[0x589]=6;r[0x58c]=period+2;r[0x586:0x588]=bytes([0,opponent]);r[0x58b]=opponent if period&2==0 else 0
 r[0x4e8:0x4ec]=bytes([0,255 if mode==0 else 1,255,255]);r[0x166:0x16a]=bytes([20,10,0,0] if won else [10,20,0,0]);r[0x3e3:0x3e7]=bytes([0,1,0,3]);r[4]=0;r[0x48:0x4c]=bytes.fromhex('12345678')
 before=snapshot(r);entered,a=call(r,4,0xba40,{0xbc96},'result-entry')
 entered[4]=0x10
 confirmed,b=call(entered,4,0xba40,{0xbc96},'result-confirm')
 assert confirmed[0x589] in (0,7)
 next_stage='defeat' if confirmed[0x3c5]&128 else 'complete' if confirmed[0x58c]==28 else 'prepare'
 phases=[a,b];after=confirmed;offered=-1;stored=False;repeat=None
 if next_stage=='prepare':
  assert confirmed[0x588]==1 and confirmed[0x589]==7
  after,c=call(confirmed,4,0xbd8e,{0xbe63},'reward-entry');phases.append(c)
  offered=after[0x18];stored=bool(c['inventoryWrites'])
  assert c['rngCalls']==0 and after[0x48:0x4c]==confirmed[0x48:0x4c]
  # One entered call is captured separately; it represents presentation time,
  # not a second invocation of the platform's pure settlement operation.
  again=bytearray(after);again[4]=0
  repeated,d=call(again,4,0xbd8e,{0xbe63},'reward-entered-repeat');repeat=d
  assert not d['inventoryWrites'] and progress(repeated)==progress(after)
 else:
  assert confirmed[0x588]==7 and confirmed[0x589]==0
 assert progress(after)['equipment']==p['equipment']
 if not won:assert progress(after)==p
 return dict(kind='settlement',label=f'{mode}-{reward}-{opponent}-{period}-{shape}-{won}',startingProgress=p,confirmedProgress=progress(confirmed),opponent=opponent,won=won,nextStage=next_stage,rewardItem=offered,rewardStored=stored,capacityScope='controlled-abnormal-capacity' if shape in ['last-slot','full'] else 'controlled-state-not-earned-win-claim',inventoryShape=shape,before=before,confirmed=snapshot(confirmed),after=snapshot(after),phases=phases,enteredRepeat=repeat)

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 cases=[]
 for mode,reward,opponent,period,shape in itertools.product([0,1],range(4),range(1,8),[0,2],['empty','duplicates','equipped','last-slot','full']):cases.append(settlement(mode,reward,opponent,period,shape,True))
 for mode,reward,opponent,period,shape in itertools.product([0,1],[0,3],[1,7],[0,2,24,26],['empty','duplicates','equipped','full']):cases.append(settlement(mode,reward,opponent,period,shape,False))
 for mode,reward,opponent,shape in itertools.product([0,1],[0,3],[1,7],['empty','duplicates','equipped','full']):cases.append(settlement(mode,reward,opponent,26,shape,True))
 loss_path=ROOT/'.local/nes-basket/runs/campaign-audit-loss-continue/ram.bin';loss_data=loss_path.read_bytes();before=loss_data[(24000-1)*2048:24000*2048];after=loss_data[(26400-1)*2048:26400*2048]
 natural=dict(run='campaign-audit-loss-continue',beforeFrame=24000,afterFrame=26400,traceSHA256=hashlib.sha256(loss_data).hexdigest(),before=snapshot(before),after=snapshot(after),scope='retained normal loss/continue controller run; prior24000 RAM/PNG matches old reference')
 choices=[]
 for choice,shape in itertools.product(['continue','quit'],['empty','equipped','full']):
  r=bytearray(before)
  p=starting(0,3,1,2,shape);set_progress(r,p);r[0x588]=7;r[0x589]=139;r[0x3c4]=128;r[0x3c3]=0;r[0x79]=32;r[4]=0x80|(1 if choice=='quit' else 0)
  final,row=call(r,3,0xbe39,{0xbf35,0xf79d},choice)
  assert progress(final)==p
  assert row['stop']==('F79D' if choice=='quit' else 'BF35')
  choices.append(row)
 prior=json.loads(gzip.decompress((ROOT/'games/basketball/campaign/testdata/reward-calls.json.gz').read_bytes()))
 lookup={(r['rewardSet'],r['opponent'],r['postMatchPeriod']):r['item'] for r in prior}
 for c in cases:
  assert all(not phase['equipmentWrites'] for phase in c['phases'])
  if c['nextStage']=='prepare':
   assert c['rewardItem']==lookup[c['startingProgress']['rewardSet'],c['opponent'],c['confirmedProgress']['period']]
   first=next((i for i,v in enumerate(c['confirmedProgress']['inventory']) if v<0),None)
   writes=c['phases'][-1]['inventoryWrites']
   assert writes==([] if first is None else [[f'{0xd3+first:04X}',c['rewardItem']]])
  else:assert c['rewardItem']==-1 and not c['rewardStored'] and c['after']['progress']['inventory']==c['startingProgress']['inventory']
 doc=dict(cases=cases,choices=choices,naturalLossContinue=natural)
 data=gzip.compress((json.dumps(doc,separators=(',',':'))+'\n').encode(),mtime=0);path=OUT/'campaign-settlement-cases.json.gz';path.write_bytes(data) if not args.check else None
 if args.check:assert path.read_bytes()==data,'settlement fixture is not reproducible'
 meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),resultBase=dict(fixture='match-transition.ram.gz',record=FIRST_RESULT,frame=TRANSITION_ROWS[FIRST_RESULT]['frame'],sourceSHA256=hashlib.sha256((OUT/'match-transition.ram.gz').read_bytes()).hexdigest()),reuseRewardFixture=dict(path='games/basketball/campaign/testdata/reward-calls.json.gz',sha256=hashlib.sha256((ROOT/'games/basketball/campaign/testdata/reward-calls.json.gz').read_bytes()).hexdigest()),cases=len(cases),byStage={s:sum(c['nextStage']==s for c in cases) for s in ['prepare','defeat','complete']},choices=len(choices),fullInventoryRewardCases=sum(c['inventoryShape']=='full' and c['nextStage']=='prepare' for c in cases),fixtureSHA256=hashlib.sha256(data).hexdigest(),scope='controlled original result-confirm and reward-entry logical mutations; presentation calls explicitly omitted; repeated-entered sample kept separate; quit stops before machine reset',unchangedOriginalInstructions=True,presentationCalls={f'{k:04X}':v for k,v in PRESENTATION.items()},reusedREA=['ev_7945a86e38a813fa472f9a63045cf673ae9f0c3702f1a79b6c8a17e20eebeefb','ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd'],normalLossContinueSHA256=natural['traceSHA256'])
 if not args.check:(OUT/'campaign-settlement-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
