"""Isolated original B471 matrix, separate from natural-match captures."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest()
 assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:]
 records=[]
 for actor,grade,mount,zone,action,entered in itertools.product(range(4),range(8),[0,1,2,128,255],[0,5,0x55,0x60,0x61,0x65,0x70,0x75],[16,18],[False,True]):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xb471;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  mounts=[0]*6;index=1 if actor<2 else 0;mounts[index]=mount;mounts[index^1]=255 if mount==0 else 0
  cpu.memory[0x6f0:0x6f6]=mounts
  for address,value in [(0x512,action|(128 if entered else 0)),(0x4ec,32),(0x672,134),(0x676,8),(0x682,3),(0x66e,7),(0x619,grade),(0x66a,zone)]:cpu.memory[address+actor]=value
  for steps in range(100):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  records.append(dict(actor=actor,grade=grade,mounts=mounts,zone=zone,action=action,entered=entered,after={k:cpu.memory[a+actor] for k,a in dict(action=0x512,heading=0x4ec,mode=0x672,plan=0x676,parameter=0x682,counter=0x66e).items()}))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-shot-plan3-function.json.gz').write_bytes(packed)
 source=dict(source_is_natural_execution=False,isolated_original_function_execution=True,records=len(records),entry='bank5 B471',rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest())
 (out/'ai-shot-plan3-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
