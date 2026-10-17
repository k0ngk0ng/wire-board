"""Execute original B4EE in an isolated CPU harness; this is not a natural match."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
 assert hashlib.sha256(rom).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:]
 records=[];maximum_steps=0
 for actor,grade,zone,action,entered,human,character,seed in itertools.product(range(4),range(8),[0x50,0x60,0x62,0x65],[0x10,0x12,0x14,0x16],[False,True],[False,True],[0,1],[0x12345678,0]):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xb4ee;cpu.x=actor;cpu.a=0;cpu.y=0;cpu.p=0x20
  cpu.memory[0x539:0x53d]=[128]*4
  if human:cpu.memory[0x539+(actor^1)]=0
  for addr,value in [(0x512,action|(128 if entered else 0)),(0x52c,255),(0x4ec,64),(0x672,134),(0x676,17),(0x66e,9),(0x682,7),(0x67e,32),(0x67a,2),(0x619,grade),(0x61d,3),(0x66a,zone),(0x3e3,character),(0x572,8),(0x576,0)]:cpu.memory[addr+actor]=value
  cpu.memory[0x48:0x4c]=list(seed.to_bytes(4,'big'))
  cpu.stPushWord(0x6fff)
  for steps in range(1000):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000,(actor,grade,zone,action,cpu.pc)
  maximum_steps=max(maximum_steps,steps)
  fields=dict(action=0x512,heading=0x4ec,mode=0x672,nextMode=0x676,counter=0x66e,parameter=0x682,destination=0x67e,target=0x67a,nextAction=0x52c,shotCycle=0x572,jumpPhase=0x576)
  after={k:cpu.memory[a+actor] for k,a in fields.items()};after['random']=int.from_bytes(bytes(cpu.memory[0x48:0x4c]),'big')
  records.append(dict(actor=actor,grade=grade,zone=zone,action=action,entered=entered,human=human,character=character,seed=seed,after=after))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'ai-shot-plan7-function.json.gz').write_bytes(packed)
 source=dict(source_is_natural_execution=False,isolated_original_function_execution=True,entry='bank5 B4EE',termination='sentinel return 7000; no original function patched or stubbed',records=len(records),maximum_instructions=maximum_steps,rom_sha256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest())
 (out/'ai-shot-plan7-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
