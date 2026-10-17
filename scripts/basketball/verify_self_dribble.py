"""Original self-indexed dribble, initialized from the exact runtime failure."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,MPU,put
from encode_world_probe import encode_world_probe
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def run(r,entry,exit):
 m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=entry);cpu.x=6;calls=[];sounds=[];writes=[]
 for _ in range(500000):
  if cpu.pc==exit:break
  if cpu.pc==0xe18b:calls.append(cpu.x)
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  pc=cpu.pc;old=[m[a] for a in [0x53f,0x518,0x56a]]
  cpu.step()
  for a,v in zip([0x53f,0x518,0x56a],old):
   if m[a]!=v:writes.append(dict(pc=pc,address=a,before=v,after=m[a]))
 else:raise RuntimeError(hex(cpu.pc))
 return bytes(m[:2048]),calls,sounds,writes

def main():
 source=ROOT/'.local/basketball/roster-rotation-special-balls-20261018/case-04/before.json';snapshot=json.loads(source.read_text());patch,_=encode_world_probe(snapshot);ram=bytearray(BASE)
 for a,v in patch.items():ram[a]=v
 prepared,_,_,_=run(ram,0xd314,0xd345)
 after,calls,sounds,writes=run(prepared,0xd345,0xd348)
 stage=dict(before=decode_world_action_state(prepared),after=decode_world_action_state(after),context=context(prepared)['actions'],calls=calls,sounds=sounds,writes=writes)
 # Original scheduler visits players 0..5 before dispatching the ball.
 ball_origin,_,_,_=run(prepared,0xd345,0xe149)
 assert ball_origin[0x53f]==6 and ball_origin[0x518]&127==1
 rows=[]
 for entered,left,elapsed,vz in itertools.product([False,True],[False,True],[0,254],[-2049,0,3968]):
  r=bytearray(ball_origin);r[0x518]=1|(128 if entered else 0);r[0x471]=128 if left else 0;r[0x44a]=0;r[0x43d]=elapsed;put(r,vz,0x4b2,0x4bf)
  final,_,sound,_=run(r,0xe149,0xe14c)
  rows.append(dict(before=decode_world_action_state(r),after=decode_world_action_state(final),context=context(r)['actions'],sounds=sound))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,stage=stage),separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'self-dribble.json.gz').write_bytes(packed);raw=source.read_bytes();(out/'self-dribble-runtime-before.json.gz').write_bytes(gzip.compress(raw,mtime=0))
 meta=dict(scope='controlled original D314 preparation and D345 whole action phase from exact failing Go state; original E149 self-dribble calls; not natural original full match',calls=len(rows),worldCalls=calls,romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),runtimeSHA256=hashlib.sha256(raw).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
 (out/'self-dribble-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
