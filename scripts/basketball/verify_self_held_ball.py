"""Original B585 self-indexing and full action phase from exact Mode128 failure."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,MPU,put
from encode_world_probe import encode_world_probe
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def run(r,entry,exit):
 m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=entry);cpu.x=6;calls=[];sounds=[];points={};writes=[]
 for _ in range(500000):
  if cpu.pc==exit:break
  if cpu.pc==0xe18b:calls.append(cpu.x)
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc in (0xb585,0xb5ba,0xb5bd,0xb5c0,0xb5df) and cpu.pc not in points:points[cpu.pc]=bytes(m[:2048])
  pc=cpu.pc;actor=cpu.x;old=[m[a] for a in (0x53f,0x518,0x56a,0x561)]
  cpu.step()
  for a,v in zip((0x53f,0x518,0x56a,0x561),old):
   if m[a]!=v:writes.append(dict(pc=pc,actor=actor,address=a,before=v,after=m[a]))
 else:raise RuntimeError(hex(cpu.pc))
 return bytes(m[:2048]),calls,sounds,points,writes

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 source=ROOT/'.local/basketball/preparation-after-source-plan12-20261018/court1-roster0-all-computer/before.json';snapshot=json.loads(source.read_text());patch,_=encode_world_probe(snapshot);ram=bytearray(BASE)
 assert snapshot['videoFrames']==21719 and snapshot['config']['match']['Mode']==128
 for a,v in patch.items():ram[a]=v
 prepared,_,_,_,_=run(ram,0xd314,0xd345)
 after,calls,sounds,points,writes=run(prepared,0xd345,0xd348)
 assert points[0xb585][0x53f]==6 and points[0xb585][0x518]==2
 stage=dict(before=decode_world_action_state(prepared),after=decode_world_action_state(after),context=context(prepared)['actions'],calls=calls,sounds=sounds,writes=writes)
 origin=points[0xb585];rows=[]
 for entered,left,x,height,vx,vz in itertools.product([False,True],[False,True],[2,66,260,510],[0,73,260],[-3,0,3],[-3,0,3]):
  r=bytearray(origin);r[0x518]=2|(128 if entered else 0);r[0x471]=128 if left else 0;r[0x44a]=0;r[0x43d]=254;r[0x561]=3
  put(r,x*256+228,0x6e,0x7b,0x88);put(r,height*256+64,0xaf,0xbc,0xc9);put(r,vx*256+29,0x47e,0x48b);put(r,vz*256+41,0x4b2,0x4bf)
  final,_,sound,mids,_=run(r,0xe149,0xe14c)
  rows.append(dict(before=decode_world_action_state(r),prepared=decode_world_action_state(mids[0xb5ba]),afterMotion=decode_world_action_state(mids[0xb5c0]),after=decode_world_action_state(final),context=context(r)['actions'],sounds=sound))
 tails=[]
 for action,pose,left in itertools.product([0,2,4,6,14,24],[0,30,59,63],[False,True]):
  r=bytearray(points[0xb5c0]);r[0x518]=action|128;r[0x61]=pose|(128 if left else 0)
  final,_,sound,_,_=run(r,0xb5c0,0xb5f3)
  tails.append(dict(before=decode_world_action_state(r),after=decode_world_action_state(final)))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,tails=tails,stage=stage),separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'self-held-ball.json.gz').write_bytes(packed);raw=source.read_bytes();(out/'self-held-ball-runtime-before.json.gz').write_bytes(gzip.compress(raw,mtime=0))
 meta=dict(scope='controlled original preparation and full13-object action stage from exact Mode128 failure; independent original B585 self-indexed calls',calls=len(rows),tailCalls=len(tails),worldCalls=calls,romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),runtimeSHA256=hashlib.sha256(raw).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),writes=writes)
 (out/'self-held-ball-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
