"""Controlled original 84C3 ball/prop responses, preserving all canonical fields."""
import gzip,hashlib,itertools,json
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,put,MPU
from extract_world_actions import decode_world_action_state
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
rows=[]
cases=list(itertools.product([0,2,3,8,10,13,15,21],[0,1,6,7],[0,255,256,-256],[70*256+255,71*256],[0,255,256,383,-1,-256],[4,5]))
for action,prop_action,height,depth,gravity,target in cases:
 r=bytearray(base)
 r[0x42]=target;r[0x518]=action|128;r[0x512+target]=prop_action|128;r[0x53f]=0 if action==2 else 6
 r[0x56b]=1;r[0x513]=0x50|128;r[0x46b+target]=128;r[0x561]=2;r[0x6a8]=3
 put(r,height,0xa9+target,0xb6+target,0xc3+target);put(r,depth,0x8f+target,0x9c+target);put(r,gravity,0x4cc,0x4d9)
 # Alternate strict/equal integer comparisons while preserving distinct fractions.
 x=(200 if len(rows)&1 else 199)*256+19
 put(r,x,0x6e,0x7b,0x88);put(r,200*256+201,0x68+target,0x75+target,0x82+target)
 m=Memory(prg,r);m[0x8000:0xc000]=prg[5*16384:6*16384]
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0x84c3);cpu.sp=255;cpu.stPushWord(0x6fff)
 for n in range(5000):
  if cpu.pc==0x7000:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 rows.append(dict(candidate=target,before=decode_world_action_state(before),after=decode_world_action_state(bytes(m[:2048]))))
p=ROOT/'games/basketball/engine/testdata/ball-prop-contact.json.gz';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);p.write_bytes(packed)
p.with_name('ball-prop-contact-source.json').write_text(json.dumps(dict(scope='controlled original bank5 84C3 instruction calls, not natural-match sampling',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n')
print(len(rows),'original ball/prop calls')
