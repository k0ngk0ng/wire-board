"""Isolated original A49E and B383 dispatch, including entry versus persisted plan."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:];rows=[]
 for mode,entered,parameter,actor,action,seed,zone,grade in itertools.product([23,24,25],[False,True],[2,4,5,7],range(4),[16,20],[0,0x12345678],[0x52,0x60],[2,3]):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xa49e;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x539:0x53d]=[128]*4
  human=actor<2
  if human:cpu.memory[0x539+(actor^1)]=0
  for address,value in [(0x672,mode|(128 if entered else 0)),(0x676,8),(0x66e,9),(0x682,parameter),(0x67e,32),(0x67a,2),(0x512,action|128),(0x4ec,32),(0x619,grade),(0x61d,3),(0x3e3,1),(0x66a,zone),(0x572,8),(0x576,0),(0x4b9,1),(0x5f7,128),(0x52c,255)]:cpu.memory[address+actor]=value
  cpu.memory[0x48:0x4c]=list(seed.to_bytes(4,'big'))
  for steps in range(500):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  after={k:cpu.memory[a+actor] for k,a in dict(mode=0x672,plan=0x676,counter=0x66e,parameter=0x682,destination=0x67e,target=0x67a,action=0x512,heading=0x4ec,cycle=0x572,phase=0x576,nextAction=0x52c).items()};after['random']=int.from_bytes(bytes(cpu.memory[0x48:0x4c]),'big')
  rows.append(dict(mode=mode,entered=entered,parameter=parameter,actor=actor,action=action,seed=seed,zone=zone,grade=grade,human=human,after=after))
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-forced-shot-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 A49E including original B383 dispatch',rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'ai-forced-shot-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
