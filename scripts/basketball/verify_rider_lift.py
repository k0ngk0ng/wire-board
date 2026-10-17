"""Original action07 support lift: entry facing, trigger, full actor loop, FF alias.
Failure inventory: missing handler; lost/late linked-actor3F trigger; wrong facing
on X-page boundaries; attached return uses idle; FF treated as a player index or
ignored instead of writing0611; animation finishes early; invalid restores.
"""
import gzip, hashlib, itertools, json
from verify_pass_floor_original import ROOT, Memory, MPU, put
from verify_court_profiles import EXPECTED
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==EXPECTED
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
def setup(actor,entered,cursor,linked,attached,left,court):
 r=bytearray(base);r[0x58b]=court;r[0x512+actor]=7|(128 if entered else 0);r[0x51f+actor]=128
 r[0x539:0x53d]=[128]*4;r[0x550+actor]=linked;r[0x642+actor]=(128|(actor^1)) if attached else 64
 r[0x437+actor]=0;r[0x444+actor]=cursor;r[0x451+actor]=0;r[0x52c+actor]=61 if attached else 0
 r[0x53f]=6;r[0x53d:0x53f]=[255]*2;r[0x540:0x546]=list(range(7,13));r[0x569]=255
 r[0x5f7+actor]=128 if attached else 0;r[0x6d7:0x6e4]=[0]*13
 put(r,256*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+19,0x8f+actor,0x9c+actor);put(r,32*256+23 if attached else 0,0xa9+actor,0xb6+actor,0xc3+actor)
 put(r,(255 if left else 257)*256,0x6e,0x7b,0x88)
 put(r,832,0x478+actor,0x485+actor);put(r,-192,0x492+actor,0x49f+actor);put(r,0,0x4ac+actor,0x4b9+actor);put(r,0,0x4c6+actor,0x4d3+actor)
 return r
def call(m,actor):
 b=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=actor;sounds=[]
 for n in range(20000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1;continue
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(actor=actor,before=decode_world_action_state(b),after=decode_world_action_state(bytes(m[:2048])),context=context(b)['actions'],sounds=sounds)
rows=[]
for actor,entered,cursor,noLink,attached,left,court in itertools.product(range(4),(False,True),range(3),(False,True),(False,True),(False,True),(0,2,5)):
 m=Memory(rom[16:16+131072],setup(actor,entered,cursor,255 if noLink else actor^1,attached,left,court));rows.append(call(m,actor))
chains=[]
for actor,noLink,attached in itertools.product(range(4),(False,True),(False,True)):
 m=Memory(rom[16:16+131072],setup(actor,False,0,255 if noLink else actor^1,attached,False,5));chain=[]
 for tick in range(30):
  chain.append(call(m,actor))
  if (m[0x512+actor]&127)!=7:break
 else:raise RuntimeError('lift never ended')
 chains.append(chain)
p=ROOT/'games/basketball/engine/testdata/rider-lift.json.gz';packed=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0);p.write_bytes(packed)
meta=dict(source_is_original_execution=True,natural_gameplay=False,entry='E165/E18B through92F9..9331',calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),romSHA256=EXPECTED,fixtureSHA256=hashlib.sha256(packed).hexdigest(),scope='configured source single-actor lift and continuous lift animation; no complete linked target3F action claim; FBCC sound sink only',alias='linkedActor FF writes3F to0611 (actor0 endurance)')
p.with_name('rider-lift-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
