"""Unmodified original bank 6 BCAA oracle: controlled calls and continuous chains."""
import gzip,hashlib,itertools,json
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
PRG=ROM[16:16+131072]
BASE=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
class SoundMemory(Memory):
 def __setitem__(self,a,v):
  if isinstance(a,int) and 0x4000<=a<=0x4015: list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
def initial(left=False,entered=False,latch=128,counter=6,x=200,vx=3,height=60,seed=0x12345678,variant='normal'):
 r=bytearray(BASE)
 for a,v in {0x518:18|(128 if entered else 0),0x525:128,0x471:128 if left else 0,0x43d:0,0x44a:0,0x457:0,0x532:255,0x5fd:128,0x648:64,0x511:counter,0x53f:0 if not entered else 6,0x564:0,0x561:2,0x3dc:8,0x512:155,0x58d:128,0x554:0,0x55d:7 if left else 8,0x58b:1,0x55a:0,0x571:latch}.items():r[a]=v
 put(r,x*256+113,0x6e,0x7b,0x88);put(r,40*256+71,0x95,0xa2);put(r,height*256+17,0xaf,0xbc,0xc9)
 put(r,vx*256,0x47e,0x48b);put(r,73,0x498,0x4a5);put(r,128,0x4b2,0x4bf);put(r,64,0x4cc,0x4d9)
 put(r,0,0xa9,0xb6,0xc3);r[0x48:0x4c]=seed.to_bytes(4,'big')
 if variant=='tail':r[0x44a]=7;r[0x43d]=0
 if variant=='tail-end':r[0x44a]=9;r[0x43d]=254
 if variant=='primary-end':r[0x44a]=5;r[0x43d]=254
 if variant=='ground':put(r,0,0xaf,0xbc,0xc9);put(r,0,0x4b2,0x4bf);put(r,0,0x4cc,0x4d9)
 if variant=='special':r[0x554]=128
 if variant=='owner-six':r[0x53f]=6
 if variant=='fractional-vx':put(r,255,0x47e,0x48b)
 return r
def run(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];mid=None;motion=None
 for steps in range(30000):
  if cpu.pc==0xbcc2 and mid is None:mid=bytes(m[:2048])
  if cpu.pc==0xbcc8 and motion is None:motion=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xe14c:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 after=bytes(m[:2048]);return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(mid) if mid else None,afterMotion=decode_world_action_state(motion) if motion else None,after=decode_world_action_state(after),context=context(before)['actions'],sounds=sounds)
def main():
 calls=[]
 for left,entered in itertools.product([False,True],[False,True]):
  for variant in ['normal','ground','special','owner-six']:
   m=SoundMemory(PRG,initial(left,entered,variant=variant));calls.append(dict(label=f'{left}-{entered}-{variant}',**run(m)))
 for left,latch,counter,seed in itertools.product([False,True],[0,1,127,128,255],[0,1,6,127,128,255],[0x00345678,0x02345678,0x04345678,0x06345678]):
  x=50 if left else 470;vx=-3 if left else 3
  m=SoundMemory(PRG,initial(left,True,latch,counter,x,vx,seed=seed));calls.append(dict(label=f'cross-{left}-{latch}-{counter}-{seed}',**run(m)))
 for left,variant,x,vx in itertools.product([False,True],['tail','tail-end','primary-end','fractional-vx','ground'],[47,48,49,446,447,448],[-3,0,3]):
  m=SoundMemory(PRG,initial(left,True,x=x,vx=vx,variant=variant));calls.append(dict(label=f'{left}-{variant}-{x}-{vx}',**run(m)))
 chains=[]
 for left,seed in itertools.product([False,True],[0x00345678,0x02345678,0x04345678,0x06345678]):
  m=SoundMemory(PRG,initial(left,seed=seed));records=[]
  for tick in range(500):
   records.append(run(m))
   if m[0x518]&127==6:break
  else:raise RuntimeError(('chain did not land',left,seed,m[0x518]))
  chains.append(dict(left=left,seed=seed,records=records))
 output=ROOT/'games/basketball/engine/testdata'
 packed=gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
 (output/'ball18.json.gz').write_bytes(packed)
 path=ROOT/'.local/basketball/team7-after-contact-tails/native/court1-teams0-7-characters0-1-0-1/before.json'
 before=path.read_bytes();(output/'ball18-runtime-before.json.gz').write_bytes(gzip.compress(before,mtime=0))
 meta=dict(entry='bank6 BCAA via original E149..E14C dispatch',scope='controlled original calls, uninterrupted chains, separate real Go runtime failure',calls=len(calls),chains=[len(c['records']) for c in chains],romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),runtimeBefore=str(path.relative_to(ROOT)),runtimeBeforeSHA256=hashlib.sha256(before).hexdigest())
 (output/'ball18-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
