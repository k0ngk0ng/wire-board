"""Unchanged original76 driver timeline; retained PCM is never rewritten."""
import csv,gzip,hashlib,json,math,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/music76-source';OUT.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(ROOT/'scripts/basketball'))
from verify_pass_floor_original import Memory,MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
BASE=ROOT/'.local/basketball/audio/export'
entry=next(r for r in json.load(open(BASE/'ending-music-manifest.json'))['tracks']if r['id']==76)
files=entry['captures'][0]
for k,p in files.items():
 a,b=BASE/p,BASE/entry['captures'][1][k]
 assert hashlib.file_digest(a.open('rb'),'sha256').digest()==hashlib.file_digest(b.open('rb'),'sha256').digest()
raw=(BASE/files['driver']).read_bytes();cursors=[int(r['samples'])for r in csv.DictReader((BASE/files['samples']).open())]
jumps=[{k:int(v)for k,v in r.items()}for r in csv.DictReader((BASE/files['jumps']).open())]
main=[r for r in jumps if r['channel']==0];a,b=main[:2]
phase=lambda step:raw[(step-1)*231+6]
ticks=((b['step']-a['step'])*140+phase(a['step'])-phase(b['step']))//256
period=ticks*256//math.gcd(ticks*256,140)
class M(Memory):
 def __init__(self):super().__init__(PRG,bytes(2048));self[0x8000:0xc000]=PRG[:16384]
 def __setitem__(self,a,v):
  if isinstance(a,int)and 0x4000<=a<=0x4017:return list.__setitem__(self,a,v)
  super().__setitem__(a,v)
def call(m,pc,arg=0):
 cpu=MPU(memory=m,pc=pc);cpu.a=arg;cpu.sp=255;cpu.stPushWord(0x6fff)
 for n in range(30000):
  if cpu.pc==0x7000:return
  cpu.step()
 raise RuntimeError(hex(cpu.pc))
m=M();call(m,0x8000,0);call(m,0x8000,76)
states=bytearray();matches=[];changes=[];lastTempo=None;matched=0
for step in range(4,45001):
 call(m,0x8003);state=bytes(m[0x700:0x7e7]);states.extend(state)
 if step<=20000:
  expected=raw[(step-1)*231:step*231]
  assert expected==state,('retained source drift',step,[(hex(0x700+i),x,y)for i,(x,y)in enumerate(zip(expected,state))if x!=y]);matched+=1
 if state[5]!=lastTempo:changes.append(dict(step=step,tempo=state[5],phase=state[6]));lastTempo=state[5]
 start=step-period
 if start>=4289 and state==states[(start-4)*231:(start-3)*231]:matches.append((start,step))
 if step%10000==0:print('source step',step,flush=True)
assert matches
first=matches[0]
(OUT/'driver-extension.bin.gz').write_bytes(gzip.compress(states,mtime=0))
report=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),captureFiles={k:dict(path=p,sha256=hashlib.file_digest((BASE/p).open('rb'),'sha256').hexdigest())for k,p in files.items()},bothCapturesIdentical=True,retainedMatchedStates=matched,replayedSourceSteps=[4,45000],tempoChanges=changes,sequenceTicks=ticks,phasePeriodFrames=period,sequenceLoops=period*140//(ticks*256),allMusicChannelsJumpTogether=all(sorted(x['channel']for x in jumps if x['step']==r['step'])==[0,1,2,3]for r in main),firstFullDriverRepeat=first,repeatBoundariesVerified=len(matches),driverRange='0700..07E6 all231bytes',firstRepeatStateSHA256=hashlib.sha256(states[(first[0]-4)*231:(first[0]-3)*231]).hexdigest(),extensionSHA256=hashlib.sha256(states).hexdigest(),introTransitionFrame=dict(step=4289,beforeSample=cursors[4288],afterSample=cursors[4289],beforeSeconds=cursors[4288]/44100,afterSeconds=cursors[4289]/44100),existingPCMEnd=dict(step=20000,samples=cursors[20000]),pcmCoverage='retained PCM does not span the first complete driver-phase repeat; no new PCM capture or derivative created',limitations=['isolated original driver, not whole-console or live gameplay','APU writes accepted but not rendered by this probe','full driver-state repeat does not prove raw PCM oscillator phase equality','tempo-command onset within the recorded frame is not identified to one PCM sample'])
(OUT/'summary.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
