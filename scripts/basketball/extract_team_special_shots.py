"""Execute original ball12/14/15 instructions and retain single-call/continuous controlled oracles."""
import gzip,hashlib,json
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,put,MPU
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
class SoundMemory(Memory):
 def __setitem__(self,a,v):
  if isinstance(a,int) and 0x4000<=a<=0x4015:
   list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]

def initial(action,left=False,entered=False,variant='normal'):
 r=bytearray(base)
 for a,v in {0x518:action|(128 if entered else 0),0x525:128,0x471:128 if left else 0,0x43d:0,0x44a:0,0x457:0,0x532:255,0x5fd:128,0x648:64,0x511:13,0x53f:0 if not entered else 6,0x564:128 if left else 0,0x561:2,0x3dc:8,0x512:155,0x58d:128,0x554:0,0x55d:7 if left else 8,0x58b:1,0x55a:0,0x6d5:3}.items():r[a]=v
 put(r,200*256+113,0x6e,0x7b,0x88);put(r,40*256+71,0x95,0xa2);put(r,60*256+17,0xaf,0xbc,0xc9)
 put(r,(-3 if left else 3)*256+35,0x47e,0x48b);put(r,73,0x498,0x4a5);put(r,128,0x4b2,0x4bf);put(r,64,0x4cc,0x4d9)
 put(r,0,0xa9,0xb6,0xc3)
 if variant=='counter-zero':r[0x511]=0
 if variant=='tail':r[0x44a]=2;r[0x43d]=3
 if variant=='tail-end':r[0x44a]=4;r[0x43d]=254
 if variant=='ground':put(r,0,0xaf,0xbc,0xc9);put(r,0,0x4b2,0x4bf);put(r,0,0x4cc,0x4d9)
 if variant=='cross':put(r,(96 if left else 430)*256+113,0x6e,0x7b,0x88)
 if variant=='reflect':r[0x525]=8;r[0x564]=128
 if variant=='special':r[0x554]=128
 if variant=='owner-six':r[0x53f]=6
 return r

def run(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];mid=None
 for steps in range(30000):
  if cpu.pc in (0xbaf0,0xbb2c,0xb690) and mid is None:mid=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xe14c:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 after=bytes(m[:2048]);return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(mid),after=decode_world_action_state(after),shakeBefore=before[0x6d5],shakeAfter=after[0x6d5],context=context(before)['actions'],sounds=sounds)
rows=[]
for a in (12,14,15):
 for left in (False,True):
  for entered in (False,True):
   for v in ('normal','ground','cross','reflect','special','owner-six')+ (('counter-zero','tail','tail-end') if a==15 else ()):
    m=SoundMemory(prg,initial(a,left,entered,v));rows.append(dict(action=a,left=left,entered=entered,variant=v,**run(m)))
chains=[]
for a in (14,15):
 for left in (False,True):
  m=SoundMemory(prg,initial(a,left));records=[]
  for i in range(320):
   records.append(run(m))
   if m[0x518]&127==6:break
   assert m[0x518]&127 in (12,14,15),(a,left,i,m[0x518])
  else:raise RuntimeError(('chain did not finish',a,left))
  chains.append(dict(action=a,left=left,records=records))
contacts=[]
for a in (14,15):
 for left in (False,True):
  for candidate in (0,3):
   for attack in (False,True):
    for court in (1,2):
     ram=initial(a,left,True);ram[0x42]=candidate;ram[0x58b]=court
     ram[0x512+candidate]=(96 if attack else 0)|128;ram[0x57a+candidate]=3
     ram[0x46b+candidate]=128 if left else 0;ram[0x9c+candidate]=76
     m=SoundMemory(prg,ram);m[0x8000:0xc000]=prg[5*16384:6*16384]
     before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0x84c3);cpu.sp=255;cpu.stPushWord(0x6fff);sounds=[]
     for i in range(10000):
      if cpu.pc==0xfbcc:sounds.append(cpu.a)
      if cpu.pc==0x7000:break
      cpu.step()
     else:raise RuntimeError(('contact did not return',hex(cpu.pc)))
     contacts.append(dict(action=a,left=left,candidate=candidate,attack=attack,before=decode_world_action_state(before),after=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds))
out=ROOT/'games/basketball/engine/testdata';payload=(json.dumps(dict(calls=rows,chains=chains,contacts=contacts),separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);p=out/'team-special-shots.json.gz';p.write_bytes(packed)
p.with_name('team-special-shots-source.json').write_text(json.dumps(dict(scope='controlled original E149..E14C calls; chained stages do not inject later source snapshots; no natural-match claim',calls=len(rows),contactCalls=len(contacts),chains=[len(c['records']) for c in chains],romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n');print(len(rows),'calls; chains', [len(c['records']) for c in chains])
