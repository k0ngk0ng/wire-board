"""Original action16 BB78 calls and uninterrupted trajectories, no patched ROM."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,initial as ball18_initial,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def initial(left=False,entered=False,x=200,vx=3,height=60,vz=128,gravity=64,cursor=0,variant='normal'):
 r=ball18_initial(left,entered,x=x,vx=vx,height=height,variant=variant)
 r[0x518]=16|(128 if entered else 0);r[0x44a]=cursor
 put(r,vz,0x4b2,0x4bf);put(r,gravity,0x4cc,0x4d9)
 return r

def run(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];mid=None;motion=None
 for steps in range(30000):
  if cpu.pc==0xbb8e and mid is None:mid=bytes(m[:2048])
  if cpu.pc==0xbb94 and motion is None:motion=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xe14c:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 after=bytes(m[:2048])
 return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(mid) if mid else None,afterMotion=decode_world_action_state(motion) if motion else None,after=decode_world_action_state(after),context=context(before)['actions'],sounds=sounds)

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 calls=[]
 for left,variant in itertools.product([False,True],['normal','special','owner-six']):
  calls.append(dict(label=f'entry-{left}-{variant}',**run(SoundMemory(PRG,initial(left,variant=variant)))))
 for left,x,vx,vz,cursor in itertools.product([False,True],[79,80,81,446,447,448],[-3,0,3],[-2305,-2048,-1,0,1024],[0,1]):
  m=SoundMemory(PRG,initial(left,True,x=x-vx,vx=vx,vz=vz,gravity=0,cursor=cursor))
  record=run(m)
  assert record['afterMotion']['ball']['body']['position']['x']>>8==x, 'boundary must be tested after motion'
  calls.append(dict(label=f'boundary-{left}-{x}-{vx}-{vz}-{cursor}',**record))
 for left,variant in itertools.product([False,True],['ground','fractional-vx']):
  m=SoundMemory(PRG,initial(left,True,x=470 if not left else 40,variant=variant))
  if variant=='ground':put(m,0,0xaf,0xbc,0xc9);put(m,0,0x4b2,0x4bf);put(m,0,0x4cc,0x4d9)
  if variant=='fractional-vx':put(m,255,0x47e,0x48b)
  calls.append(dict(label=f'{left}-{variant}',**run(m)))
 chains=[]
 for left,height in itertools.product([False,True],[30,100,210]):
  m=SoundMemory(PRG,initial(left,height=height));records=[]
  for tick in range(500):
   records.append(run(m))
   if m[0x518]&127==6:break
  else:raise RuntimeError(('did not land',left,height))
  chains.append(dict(left=left,height=height,records=records))
 out=ROOT/'games/basketball/engine/testdata'
 packed=gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'ball16.json.gz').write_bytes(packed)
 meta=dict(entry='bank6 BB78 via original E149..E14C dispatch',scope='controlled isolated calls and uninterrupted original trajectories; not natural matches',calls=len(calls),chains=[len(x['records']) for x in chains],romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
 (out/'ball16-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
