"""Original 3E/12/15 preparation on the positive-X/negative-depth diagonal."""
import gzip,hashlib,json
from verify_pass_floor_original import ROOT,Memory,put,MPU
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
class SoundMemory(Memory):
 def __setitem__(self,a,v):
  if isinstance(a,int) and 0x4000<=a<=0x4015:list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
rows=[]
for kind,actor,action,entry,end in [('player',0,62,0x9e3d,0x9e99),('player',1,62,0x9e3d,0x9e99),('ball',6,12,0xb65f,0xb690),('ball',6,15,0xbb09,0xbb2c)]:
 for fraction in [0,1,64,105,255]:
  r=bytearray(base)
  r[0x512+actor]=action;r[0x642+actor]=64;r[0x46b+actor]=0;r[0x58d]=128;r[0x554]=0;r[0x554+actor]=0
  r[0x53f]=0;r[0x539]=0;r[0x53a]=0;r[0x55d]=8;r[0x4e0]=0;r[0x4e1]=0;r[0x572+actor]=0
  r[0x48:0x4c]=[0x12,0x34,0x56,0x78]
  put(r,32*256,0xa9,0xb6,0xc3)
  put(r,420*256+117,0x68+actor,0x75+actor,0x82+actor);put(r,87*256+43,0x8f+actor,0x9c+actor);put(r,32*256+17,0xa9+actor,0xb6+actor,0xc3+actor)
  r[0x69]=fraction
  if kind=='ball':r[0x512]=0x9b
  m=SoundMemory(prg,r);before=bytes(m[:2048]);cpu=MPU(memory=m,pc=entry);cpu.x=actor;sounds=[]
  for n in range(30000):
   if cpu.pc==end:break
   if cpu.pc==0xfbcc:sounds.append(cpu.a)
   cpu.step()
  else:raise RuntimeError(hex(cpu.pc))
  rows.append(dict(kind=kind,actor=actor,action=action,secondPlayerX=m[0x69]+256*m[0x76]+65536*m[0x83],before=decode_world_action_state(before),prepared=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds))
p=ROOT/'games/basketball/engine/testdata/special-launch-context.json.gz';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);p.write_bytes(packed)
p.with_name('special-launch-context-source.json').write_text(json.dumps(dict(scope='controlled original preparation functions, ending before animation and motion; original byte69 sampled at launch',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n');print(len(rows),'special diagonal preparations')
