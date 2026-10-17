"""Verify Court2-specific BDD2 counterexamples using actual music61+shot30 work bytes."""
import hashlib,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
out=ROOT/'.local/basketball/rea/special-turn-alias'
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
audio=json.loads((out/'music-shot-work.json').read_text());track=[r for r in audio if r['music']==61]
columns=json.loads((ROOT/'.local/basketball/courts/2/stream-columns.json').read_text())['initialization']
column=next(r for r in columns if r['column']==47);prefix=[255,192+(47&31),column['tiles'][0][0]]
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
rows=[]
for x,works in [(280,[54,132]),(380,[54,179])]:
 for work in works:
  source=next(r for r in track if r['soundWork']==work)
  m=Memory(prg,base);m[0xff]=work;m[0x100:0x103]=prefix;m[0x518]=4;m[0x55d]=8
  put(m,x*256,0x6e,0x7b,0x88);put(m,8*256,0x47e,0x48b);put(m,0,0x498,0x4a5)
  cpu=MPU(memory=m,pc=0xbdd2);cpu.x=6;cpu.sp=255;cpu.stPushWord(0x6fff)
  for n in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  else:raise RuntimeError(hex(cpu.pc))
  rows.append(dict(court=2,x=x,selectedHoop=8,velocityX=8,audioSource=source,graphicsPrefix=prefix,lastUploadColumn=47,afterAction=m[0x518]))
for x in (280,380):assert {r['afterAction'] for r in rows if r['x']==x}=={4,12}
report=dict(scope='original BDD2 calls; actual Court2 music61/shot30 driver work values and original initial upload column47 prefix; not a natural whole-match trace',romSHA256=hashlib.sha256(rom).hexdigest(),audioSourceSHA256=hashlib.sha256((out/'music-shot-work.json').read_bytes()).hexdigest(),cases=rows,conclusion='Both valid Court2 device regions admit a different next action for reachable audio work values; zero/fixed-offset substitution is invalid.')
(out/'court2-counterexamples.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
