"""Unmodified bank 6 BBB9: controlled calls and uninterrupted ball chains."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,MPU,initial,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def state(left=False,entered=False,latch=128,counter=6,vz=128,variant='normal'):
 r=initial(left,entered,latch,counter,variant=variant)
 r[0x518]=19|(128 if entered else 0)
 put(r,vz,0x4b2,0x4bf)
 if variant=='last-frame':r[0x44a]=2;r[0x43d]=1
 return r

def run(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];mid=None;motion=None
 for steps in range(30000):
  if cpu.pc==0xbbd1 and mid is None:mid=bytes(m[:2048])
  if cpu.pc==0xbbd7 and motion is None:motion=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xe14c:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 after=bytes(m[:2048]);return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(mid) if mid else None,afterMotion=decode_world_action_state(motion) if motion else None,after=decode_world_action_state(after),context=context(before)['actions'],sounds=sounds)

def main():
 calls=[]
 for left,entered,latch,counter,vz in itertools.product([False,True],[False,True],[0,127,128,255],[0,1,6,128,255],[-32768,-2303,-2048,-2047,-1793,-1792,-1,0,128,2048]):
  calls.append(dict(label=f'{left}-{entered}-{latch}-{counter}-{vz}',**run(SoundMemory(PRG,state(left,entered,latch,counter,vz)))))
 for left,entered,variant in itertools.product([False,True],[False,True],['ground','special','owner-six','last-frame']):
  calls.append(dict(label=f'{left}-{entered}-{variant}',**run(SoundMemory(PRG,state(left,entered,variant=variant)))))
 chains=[]
 for left,entered,latch in itertools.product([False,True],[False,True],[0,128]):
  m=SoundMemory(PRG,state(left,entered,latch));records=[]
  for tick in range(500):
   records.append(run(m))
   if m[0x518]&127==6:break
  else:raise RuntimeError(('chain did not land',left,entered,latch,m[0x518]))
  chains.append(dict(left=left,entered=entered,latch=latch,records=records))
 out=ROOT/'games/basketball/engine/testdata'
 packed=gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'ball19.json.gz').write_bytes(packed)
 meta=dict(entry='bank6 BBB9 via original E149..E14C dispatch',scope='controlled original calls and uninterrupted chains; no world injection within chain; not natural-match evidence',calls=len(calls),chains=[len(c['records']) for c in chains],romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
 (out/'ball19-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
