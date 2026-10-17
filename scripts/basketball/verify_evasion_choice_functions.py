"""Isolated original evasion choices, including 16-bit teammate-distance boundaries."""
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
 for choice,actor,x,distance,action in itertools.product([2,3,4],range(4),[0,1,32760,65535],[-32768,-257,-96,-95,-1,0,1,95,96,257,32767],[16,18]):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc={2:0xb840,3:0xb87f,4:0xb885}[choice];cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  other=(x-distance)&65535
  for seat,value in [(actor,x),(actor^1,other)]:cpu.memory[0x75+seat]=value&255;cpu.memory[0x82+seat]=value>>8
  for address,value in [(0x512,action|128),(0x4ec,32),(0x672,154),(0x676,8),(0x682,choice),(0x576,3)]:cpu.memory[address+actor]=value
  for step in range(100):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  records.append(dict(choice=choice,actor=actor,x=x,other=other,action=action,after={k:cpu.memory[a+actor] for k,a in dict(action=0x512,heading=0x4ec,mode=0x672,plan=0x676,parameter=0x682,jumpPhase=0x576).items()}))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-evasion-choice-functions.json.gz').write_bytes(packed)
 source=dict(source_is_natural_execution=False,isolated_original_function_execution=True,records=len(records),entries=['bank5 B840','bank5 B87F','bank5 B885'],rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest())
 (out/'ai-evasion-choice-functions-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
