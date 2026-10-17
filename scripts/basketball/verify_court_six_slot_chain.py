"""One original goal call followed by uninterrupted original slot calls.
Initial states are all14 recorded real goals. The clock advances; no RNG/reel/
score/progress value is replaced between calls. This is isolated subsystem
execution, not a complete match or an invented winning initial configuration.
"""
import gzip,hashlib,json,sys
from pathlib import Path
from capture_court_animation import state,patches,scores
from verify_pass_floor_original import ROOT,Memory,MPU
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
source=ROOT/'.local/nes-basket/runs/court-six-slot-observed-v2/slot-goals.ram';raw=source.read_bytes();assert len(raw)==14*4096
class Trace(Memory):
 def __init__(self,ram):
  self.active=False;self.queues=[];super().__init__(prg,ram);self[0x8000:0xc000]=prg[4*16384:5*16384]
 def __setitem__(self,a,v):
  super().__setitem__(a,v)
  if self.active and isinstance(a,int) and a in(0x30b,0x32e,0x351):self.queues.append(a)
def call(m,entry,stop):
 cpu=MPU(memory=m,pc=entry);cpu.x=6;cpu.sp=255;cpu.stPushWord(0x6fff);m.queues=[];m.active=True;sounds=[];palette=False
 for n in range(40000):
  if cpu.pc==stop:break
  if cpu.pc in(0xc000,0xfcec):
   if cpu.pc==0xc000:sounds.append(cpu.a)
   else:palette=True
   cpu.pc=cpu.stPopWord()+1;continue
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 m.active=False;return sounds,palette
chains=[];rewards=0
for index in range(14):
 m=Trace(raw[index*4096:index*4096+2048]);before=bytes(m[:2048]);goalSounds,_=call(m,0x8e33,0x7000);afterGoal=bytes(m[:2048]);assert afterGoal[0x6dd:0x6df]==raw[index*4096+2048+0x6dd:index*4096+2048+0x6df]
 rows=[]
 for tick in range(128):
  b=bytes(m[:2048]);sounds,palette=call(m,0x9b95,0x7000);a=bytes(m[:2048]);c=dict(court=6,active=(b[0x589]&127)not in(0,2),videoFrame=b[0x300],cameraX=b[0x397],random=int.from_bytes(b[0x48:0x4c],'big'),scores=scores(b),hoopOwners=list(b[0x540:0x546]),hoopLocks=list(b[0x62d:0x633]))
  out=dict(state=state(a,6),patches=patches(a,m.queues),palette=palette,paletteID=a[0x304]if palette else 0,sounds=sounds,random=int.from_bytes(a[0x48:0x4c],'big'),scores=scores(a))
  rows.append(dict(before=state(b,6),context=c,after=out));rewards+=scores(a)!=scores(b)
  if not any(a[0x6dd:0x6df]):break
  m[0x300]=(m[0x300]+1)&255
 else:raise RuntimeError('original slot did not finish')
 chains.append(dict(goalIndex=index,goalBefore=list(before),goalAfter=list(afterGoal),goalSounds=goalSounds,calls=rows))
p=ROOT/'games/basketball/engine/testdata/court-six-slot-chains.json.gz';packed=gzip.compress(json.dumps(chains,separators=(',',':')).encode(),mtime=0);p.write_bytes(packed)
meta=dict(source_is_original_execution=True,natural_gameplay=False,chains=len(chains),slotCalls=sum(len(c['calls'])for c in chains),rewards=rewards,initial='all14 original observed goal entries; no overwritten RNG/reels/scores',scope='original8E33 goal then repeated9B95, external0300++ only, no intermediate state injection; C000/FCEC observed command sinks',romSHA256=hashlib.sha256(rom).hexdigest(),goalSourceSHA256=hashlib.sha256(raw).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
p.with_name('court-six-slot-chains-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
