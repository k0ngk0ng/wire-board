"""Isolated original A95F team-support strategy; no replacement movement or AI."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:];rows=[]
 for actor,entered,action,action_entered,human,link_kind,owner_kind,zone in itertools.product(range(4),[False,True],[0,1,2],[False,True],[False,True],range(5),range(4),[0x20,0x25,0x30,0x35,0x40,0x45,0x50,0x75]):
  link=[0,64,128|actor,128|(actor^1),128|4][link_kind];holder=[actor,actor^1,actor^2,6][owner_kind]
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xa95f;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x539:0x53d]=[128]*4
  if human:cpu.memory[0x539+(actor^1)]=0
  for address,value in [(0x672,28|(128 if entered else 0)),(0x512,action|(128 if action_entered else 0)),(0x4ec,32),(0x66a,zone)]:cpu.memory[address+actor]=value
  cpu.memory[0x642+(actor^1)]=link;cpu.memory[0x53f]=holder
  for steps in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  rows.append(dict(actor=actor,entered=entered,action=action,actionEntered=action_entered,human=human,link=link,holder=holder,zone=zone,after={k:cpu.memory[a+actor] for k,a in dict(mode=0x672,action=0x512,heading=0x4ec).items()}))
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-team-support-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 A95F including B1D5/B227',rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'ai-team-support-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
