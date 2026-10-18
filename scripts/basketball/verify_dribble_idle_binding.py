"""Original907E initial ownership write, including the retained whole-world first difference."""
import gzip,hashlib,itertools,json
from verify_self_held_ball import ROOT,ROM,PRG,SoundMemory,MPU,put,run as run_world
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
from extract_world_loop import loop_state

def call(r,actor):
 m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0x907e);cpu.x=actor;beforeMotion=None;bound=False
 for _ in range(20000):
  if cpu.pc==0x9089:bound=True
  if cpu.pc==0x90ad:beforeMotion=bytes(m[:2048])
  if cpu.pc==0x90b0:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 assert beforeMotion is not None
 return dict(actor=actor,bound=bound,before=decode_world_action_state(r),beforeMotion=decode_world_action_state(beforeMotion),after=decode_world_action_state(bytes(m[:2048])),context=context(r)['actions'])

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 source=ROOT/'.local/nes-basket/runs/self-held-mode128-observed/world-after.ram';trace=source.read_bytes();base=trace[5*2048:6*2048]
 assert base[0x512]==16 and base[0x53f]==6
 rows=[]
 for actor,entered,holder,x,ball in itertools.product(range(4),[False,True],[0,1,2,3,6],[30,200,480],[1,2,10]):
  r=bytearray(base);r[0x512+actor]=16|(128 if entered else 0);r[0x51f+actor]=128;r[0x444+actor]=0;r[0x437+actor]=0;r[0x451+actor]=0;r[0x52c+actor]=255;r[0x53f]=holder;r[0x518]=ball|128;r[0x642+actor]=0;r[0x569]=255
  put(r,x*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+29,0x8f+actor,0x9c+actor)
  rows.append(call(r,actor))
 prepared,_,_,_,_=run_world(base,0xd314,0xd345)
 after,calls,sounds,_,writes=run_world(prepared,0xd345,0xd348)
 stage=dict(before=decode_world_action_state(prepared),after=decode_world_action_state(after),context=context(prepared)['actions'],calls=calls,sounds=sounds,writes=writes)
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,stage=stage),separators=(',',':'))+'\n').encode(),mtime=0);(out/'dribble-idle-binding.json.gz').write_bytes(packed)
 worlds=[loop_state(trace[i*2048:(i+1)*2048],3539+i) for i in range(len(trace)//2048)];worldpacked=gzip.compress((json.dumps(worlds,separators=(',',':'))+'\n').encode(),mtime=0);(out/'self-held-original-worlds.json.gz').write_bytes(worldpacked)
 meta=dict(scope='original907E entries from retained first differing world5; controlled original full phase; original full-machine trace after one alignment',records=len(rows),entry='bank6 907E',bindWrite='9089 STX053F',beforeMotion='90AD',worldRecords=len(worlds),firstDifferentIndexBeforeFix=6,romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),traceSHA256=hashlib.sha256(trace).hexdigest(),worldFixtureSHA256=hashlib.sha256(worldpacked).hexdigest())
 (out/'dribble-idle-binding-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
