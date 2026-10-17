"""Configured original rider→jump calls and uninterrupted detached jump chains.

Failure inventory: rejecting accepted 3D→04; detaching after EEFF instead of
before; clearing a support's link; losing fractions/velocity; negative jump phase
skipping motion incorrectly; Court5 device writes and restore drift.
"""
import gzip, hashlib, itertools, json
from verify_pass_floor_original import ROOT, Memory, MPU, put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
from verify_court_profiles import image, EXPECTED
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
assert hashlib.sha256(rom).hexdigest()==EXPECTED
base=image()

def setup(court, computer, phase, support, heading):
 r=bytearray(base[:2048]); actor=0
 r[0x58b]=court;r[0x512]=0xbd;r[0x51f]=4;r[0x539]=128 if computer else 0
 r[0x642]=128|support;r[0x5f7]=128;r[0x576]=phase;r[0x4ec]=heading
 r[0x569]=255;r[0x540:0x546]=[7,8,9,10,11,12]
 r[0x437]=1;r[0x444]=0;r[0x451]=0;r[0x52c]=255;r[0x53f]=6;r[0x53d]=255;r[0x53e]=255
 r[0x3d6:0x3da]=[0,0,1,1];r[0x3da:0x3dc]=[12,11];r[0x586:0x588]=[0,1]
 r[0x3e3:0x3e7]=[0,1,0,1];r[0x6d7:0x6e4]=[0]*13;r[0x58d]=0;r[0x6e4:0x6ed]=[255]*9
 put(r,189*256+37,0x68,0x75,0x82);put(r,73*256+19,0x8f,0x9c);put(r,32*256+23,0xa9,0xb6,0xc3)
 put(r,832,0x478,0x485);put(r,448,0x492,0x49f);put(r,0,0x4ac,0x4b9);put(r,0,0x4c6,0x4d3)
 return r

def decoded(r):
 w=decode_world_action_state(r)
 for i in range(2):w["auxiliary"][i]["kind"]=r[0x3da+i]
 return w

def call(m):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=0;sounds=[];motion=[]
 for n in range(20000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xeeff:motion.append(m[0x642])
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1;continue
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(actor=0,before=decoded(before),after=decoded(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds,motionAttachments=motion)
rows=[]
for c,ai,p,s,h,variant in itertools.product((0,2,5),(False,True),(0,255),(1,4),(0,64,192,255),range(3)):
 m=Memory(rom[16:16+131072],setup(c,ai,p,s,h));m[0x646]=128 if variant==2 else 0;m[0x5fb]=128 if variant==1 else 0;rows.append(call(m))
chains=[]
for c,ai,p in itertools.product((0,5),(False,True),(0,255)):
 m=Memory(rom[16:16+131072],setup(c,ai,p,1,64));chain=[]
 for tick in range(50):
  chain.append(call(m))
  if (m[0x512]&127)==6:break
 else:raise RuntimeError('jump did not land')
 chains.append(chain)
riders=[]
for c,kind,air,attached in itertools.product((0,3,5),(10,11,12,13,14,15),(False,True),(False,True)):
 m=Memory(rom[16:16+131072],setup(c,True,0,4,64));m[0x51f]=128;m[0x3da]=kind;m[0x5f7]=128 if air else 0;m[0x642]=132 if attached else 4
 riders.append(call(m))
runs=[]
for c,ai,entered,support,action in itertools.product((0,2,5),(False,True),(False,True),(1,4),(2,15)):
 m=Memory(rom[16:16+131072],setup(c,ai,0,support,64));m[0x512]=action|(128 if entered else 0);m[0x51f]=128
 runs.append(call(m))
p=ROOT/'games/basketball/engine/testdata/rider-jump.json.gz'
payload=gzip.compress(json.dumps(dict(calls=rows,chains=chains,riders=riders,runs=runs),separators=(',',':')).encode(),mtime=0);p.write_bytes(payload)
meta=dict(source_is_original_execution=True,natural_gameplay=False,entry='fixed E165/E18B through E168',romSHA256=EXPECTED,fixtureSHA256=hashlib.sha256(payload).hexdigest(),calls=len(rows),riderCalls=len(riders),runCalls=len(runs),chains=len(chains),chainCalls=sum(map(len,chains)),motionAttachments=sorted(set(v for r in rows for v in r['motionAttachments'])),scope='configured original rider jump requests, continuous once-initialized jumps; FBCC sound sink only; no CPU emulator in runtime')
p.with_name('rider-jump-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
