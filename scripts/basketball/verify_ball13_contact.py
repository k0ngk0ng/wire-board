"""Original ball13 contact geometry at animation-reachable cursor values."""
import gzip,hashlib,json
from pathlib import Path
from verify_pass_floor_original import ROOT,MPU
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];rows=[]
# B9A4/BEB6 has ordinary cursors0..2 and turn-tail cursors4..6.
for entered in [False,True]:
 for frame in [0,1,2,4,5,6]:
  cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:]
  cpu.x=6;cpu.pc=0x8318;cpu.p=0x20;cpu.sp=255
  cpu.memory[0x3dc]=8;cpu.memory[0x518]=13|(128 if entered else 0);cpu.memory[0x44a]=frame;cpu.stPushWord(0x6fff)
  for steps in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  pointer=cpu.WordAt(0x2c)
  rows.append(dict(action=13|(128 if entered else 0),frame=frame,pointer=pointer,shape=list(cpu.memory[pointer:pointer+8]),flags=cpu.memory[0x938b+13]))
out=ROOT/'games/basketball/engine/testdata';payload=(json.dumps(rows,indent=2)+'\n').encode();(out/'ball13-contact.json').write_bytes(payload)
path=ROOT/'.local/basketball/team4-after-handler17/native/court1-teams0-4-characters0-1-0-1/before.json';before=path.read_bytes();(out/'ball13-contact-runtime-before.json.gz').write_bytes(gzip.compress(before,mtime=0))
meta=dict(scope='12 original bank5 8318 calls at 6 reachable bank6 ball13 animation cursors; no generic out-of-bounds acceptance',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),beforeSHA256=hashlib.sha256(before).hexdigest())
(out/'ball13-contact-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
