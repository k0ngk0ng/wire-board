"""Original bank5 geometry reads for the reachable action68/variant63 tail."""
from pathlib import Path
import hashlib,json,sys
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
prg=rom[16:16+131072];rows=[]
for kind in range(8):
 for frame in range(5):
  cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:]
  cpu.x=1;cpu.pc=0x8318;cpu.p=0x20;cpu.sp=255
  cpu.memory[0x3d7]=kind;cpu.memory[0x513]=0xe8;cpu.memory[0x57b]=63;cpu.memory[0x445]=frame;cpu.stPushWord(0x6fff)
  for steps in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  ptr=cpu.WordAt(0x2c);shape=list(cpu.memory[ptr:ptr+8]);flags=cpu.memory[0x9321+104]
  rows.append(dict(kind=kind,action=104,variant=63,frame=frame,pointer=ptr,shape=shape,flags=flags))
out=ROOT/'games/basketball/engine/testdata/running-release-contact.json';out.write_text(json.dumps(rows,indent=2)+'\n')
out.with_name('running-release-contact-source.json').write_text(json.dumps(dict(romSHA256=hashlib.sha256(rom).hexdigest(),fixtureSHA256=hashlib.sha256(out.read_bytes()).hexdigest(),generatorSHA256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),entry='bank5 8318 -> 82E4',tailSource='bank6 AA2B sets animation cursor3; original running attack tail advances cursor4',scope='40 isolated original geometry reads, not natural ROM match evidence',records=len(rows)),indent=2)+'\n')
print(json.dumps(rows[:5]))
