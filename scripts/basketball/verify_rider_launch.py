"""Original 3F ball-target throw and continuous 07 support→3F rider chain.
Failures: treating3F as3E basket/equipment targeting; fractional target retained;
wrong shared RNG, source pose or attachment; wrong32px arc; losing source1 X
alias in FD91; ignoring lift→rider calls; JSON recovery differs.
"""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from verify_court_profiles import EXPECTED
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==EXPECTED
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
def setup(actor,entered,attached,court,left,high):
 r=bytearray(base);r[0x58b]=court;r[0x512+actor]=63|(128 if entered else 0);r[0x51f+actor]=128
 r[0x539:0x53d]=[128]*4;r[0x642:0x646]=[64]*4;r[0x642+actor]=(128|(actor^1)) if attached else 64
 r[0x437+actor]=0;r[0x444+actor]=0;r[0x451+actor]=0;r[0x52c+actor]=255
 r[0x53f]=6;r[0x53d:0x53f]=[255]*2;r[0x540:0x546]=list(range(7,13));r[0x569]=255
 r[0x5f7+actor]=128;r[0x6d7:0x6e4]=[0]*13;r[0x58d]=0;r[0x6e4:0x6ed]=[255]*9
 put(r,256*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+19,0x8f+actor,0x9c+actor);put(r,32*256+23,0xa9+actor,0xb6+actor,0xc3+actor)
 put(r,(80 if left else 400)*256+47,0x6e,0x7b,0x88);put(r,48*256+39,0x95,0xa2);put(r,high*256+61,0xaf,0xbc,0xc9)
 put(r,832,0x478+actor,0x485+actor);put(r,-192,0x492+actor,0x49f+actor);put(r,0,0x4ac+actor,0x4b9+actor);put(r,0,0x4c6+actor,0x4d3+actor)
 return r

def call(m,actor):
 b=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=actor;sounds=[]
 for n in range(30000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1;continue
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(actor=actor,before=decode_world_action_state(b),after=decode_world_action_state(bytes(m[:2048])),context=context(b)['actions'],sounds=sounds)
rows=[]
for actor,entered,attached,court,left,high in itertools.product(range(4),(False,True),(False,True),(0,2,5),(False,True),(0,88,280)):
 m=Memory(rom[16:16+131072],setup(actor,entered,attached,court,left,high));rows.append(call(m,actor))
chains=[]
for court,left in itertools.product((0,5),(False,True)):
 m=Memory(rom[16:16+131072],setup(0,True,True,court,left,88));m[0x512]=0xbd;m[0x513]=7;m[0x520]=128;m[0x551]=0;m[0x550]=255
 m[0x438]=0;m[0x445]=0;m[0x452]=0;m[0x52d]=255;m[0x5f8]=0
 put(m,256*256,0x69,0x76,0x83);put(m,40*256,0x90,0x9d);put(m,0,0xaa,0xb7,0xc4)
 for a in (0x479,0x486,0x493,0x4a0,0x4ad,0x4ba,0x4c7,0x4d4):m[a]=0
 chain=[];seen=False
 for tick in range(120):
  chain.append(call(m,1));chain.append(call(m,0));seen|=(m[0x512]&127)==63
  if seen and (m[0x512]&127)==0:break
 else:raise RuntimeError('linked throw did not land')
 chains.append(chain)
p=ROOT/'games/basketball/engine/testdata/rider-launch.json.gz';packed=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0);p.write_bytes(packed)
meta=dict(source_is_original_execution=True,natural_gameplay=False,entry='E165/E18B→9E3D action3F;07 support/rider coupled loops',calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),romSHA256=EXPECTED,fixtureSHA256=hashlib.sha256(packed).hexdigest(),scope='configured unchanged original instructions; continuous support/rider initialized once; FBCC sound sink only')
p.with_name('rider-launch-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
