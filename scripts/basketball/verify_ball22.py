"""Original bank6 BD23 ball22, controlled function calls and continuous chains."""
import gzip, hashlib, itertools, json
from verify_ball18 import ROOT, ROM, PRG, BASE, SoundMemory, initial as ball18_initial, MPU, put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def initial(left=False, entered=False, x=200, vx=3, height=60, cursor=4, elapsed=0, loops=0, ground=False):
 r=ball18_initial(left,entered,x=x,vx=vx,height=height)
 r[0x518]=22|(128 if entered else 0)
 r[0x44a]=cursor;r[0x43d]=elapsed;r[0x457]=loops
 if ground:
  put(r,0,0xaf,0xbc,0xc9);put(r,0,0x4b2,0x4bf);put(r,0,0x4cc,0x4d9)
 return r

def run(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe149);cpu.x=6;sounds=[];prepared=None
 for steps in range(30000):
  if cpu.pc==0xbd4f and prepared is None:prepared=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xe14c:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(before=decode_world_action_state(before),prepared=decode_world_action_state(prepared) if prepared else None,after=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds)

def main():
 calls=[]
 for left,entered,height,ground in itertools.product([False,True],[False,True],[0,60,87,88,96,191,192,280],[False,True]):
  calls.append(dict(label=f'entry-{left}-{entered}-{height}-{ground}',**run(SoundMemory(PRG,initial(left,entered,height=height,ground=ground)))))
 for left,x,vx,height in itertools.product([False,True],[55,56,63,64,65,455,462,463,464,471],[-3,0,3],[87,88,100]):
  calls.append(dict(label=f'turn-{left}-{x}-{vx}-{height}',**run(SoundMemory(PRG,initial(left,True,x=x,vx=vx,height=height)))))
 for left,cursor,elapsed,loops,next_action in itertools.product([False,True],[0,1,2,4],[0,254],[0,254,255],[255,6]):
  r=initial(left,True,cursor=cursor,elapsed=elapsed,loops=loops);r[0x532]=next_action
  calls.append(dict(label=f'animation-{left}-{cursor}-{elapsed}-{loops}-{next_action}',**run(SoundMemory(PRG,r))))
 chains=[]
 for left,height in itertools.product([False,True],[0,60,96,192]):
  m=SoundMemory(PRG,initial(left,height=height));rows=[]
  for tick in range(1200):
   rows.append(run(m))
   if m[0x518]&127==6:break
  else:raise RuntimeError(('chain did not land',left,height,m[0x518]))
  chains.append(dict(left=left,height=height,records=rows))
 out=ROOT/'games/basketball/engine/testdata'
 packed=gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'ball22.json.gz').write_bytes(packed)
 meta=dict(entry='original E149..E14C dispatch to bank6 BD23',calls=len(calls),chains=[len(c['records']) for c in chains],romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),scope='controlled original calls and continuous chains, no natural match claim')
 (out/'ball22-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
