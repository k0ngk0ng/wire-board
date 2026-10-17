"""Original E165/8E9D dunk variants, explicitly configured calls and chains."""
import gzip,hashlib,itertools,json
from verify_ai_mode5_plan11 import ROOT,ROM,PRG,start,machine
from verify_pass_floor_original import put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context


def setup(actor,action,mount,raised):
 r=start(actor);r[0x512+actor]=action;r[0x51f+actor]=128;r[0x53f]=actor;r[0x518]=130;r[0x61]=7
 r[0x642+actor]=64;r[0x539+actor]=128;r[0x58d]=128;r[0x56e]=mount;r[0x539+mount]=mount|(128 if raised else 0);r[0x577+mount]=3
 r[0x547]=128;r[0x569]=255;r[0x53d]=255;r[0x53e]=255;r[0xe4]=0
 r[0x437+actor]=7;r[0x444+actor]=1;r[0x451+actor]=9;r[0x52c+actor]=255
 r[0x3e3:0x3e7]=bytes([0,1,0,1]);r[0x5f7+actor]=128
 x=220 if actor<2 else 300
 put(r,x*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+17,0x8f+actor,0x9c+actor);put(r,100*256+7,0xa9+actor,0xb6+actor,0xc3+actor)
 put(r,x*256+59,0x6e,0x7b,0x88)
 for addr in [0x478,0x492,0x4ac,0x4c6]:put(r,371,addr+actor,addr+13+actor)
 return machine(r)


def call(cpu,actor):
 cpu.memory[0x8000:0xc000]=PRG[6*16384:7*16384];cpu.pc,cpu.x,cpu.p=0xe165,actor,0x20
 before=bytes(cpu.memory[:2048]);sounds=[]
 for n in range(20000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
  else:cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(actor=actor,before=decode_world_action_state(before),after=decode_world_action_state(bytes(cpu.memory[:2048])),context=context(before)['actions'],sounds=sounds)


def main():
 rows=[];chains=[]
 for actor,action,mount,raised in itertools.product(range(4),[0x58,0x59,0x5b],range(7,13),[False,True]):
  rows.append(call(setup(actor,action,mount,raised),actor))
 for actor,action,support in itertools.product(range(4),[0x58,0x59,0x5b],[1,4]):
  cpu=setup(actor,action,7,False);cpu.memory[0x642+actor]=128|support
  rows.append(call(cpu,actor))
 for actor,action,cursor in itertools.product(range(4),[0x58,0x59,0x5b],range(5)):
  cpu=setup(actor,action,7,False);cpu.memory[0x512+actor]|=128;cpu.memory[0x444+actor]=cursor;cpu.memory[0x437+actor]=[2,1,254,254,254][cursor] if action==0x59 else [4,5,254,254,254][cursor];cpu.memory[0x451+actor]=0
  rows.append(call(cpu,actor))
 for actor,action in itertools.product(range(4),[0x58,0x59,0x5b]):
  cpu=setup(actor,action,7 if actor>=2 else 8,False);chain=[]
  for tick in range(160):
   chain.append(call(cpu,actor))
   if cpu.memory[0x512+actor]&127 not in (0x58,0x59,0x5b):break
  else:raise RuntimeError('dunk did not land')
  chains.append(chain)
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0);(out/'dunk-variants.json.gz').write_bytes(packed)
 before=ROOT/'.local/basketball/team7-after-ball18/native/court1-teams0-7-characters0-1-0-1/before.json';(out/'dunk-variants-runtime-before.json.gz').write_bytes(gzip.compress(before.read_bytes(),mtime=0))
 source=dict(calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),naturalGameplay=False,originalInstructionsUnchanged=True,entry='fixed E165..E168 / bank6 8E9D',soundScope='FBCC sink',ROM_SHA256=hashlib.sha256(ROM).hexdigest(),SHA256=hashlib.sha256(packed).hexdigest(),runtimeBeforeSHA256=hashlib.sha256(before.read_bytes()).hexdigest());(out/'dunk-variants-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))

if __name__=='__main__':main()
