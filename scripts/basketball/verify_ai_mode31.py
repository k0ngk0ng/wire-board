"""Isolated, unpatched original AB87 / B383 execution; not natural matches."""
import gzip, hashlib, itertools, json, sys
from pathlib import Path
ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom = (ROOT/'.local/nes-basket/basketball.nes').read_bytes()
 digest = hashlib.sha256(rom).hexdigest()
 assert digest == '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg = rom[16:16+131072]; cpu = MPU()
 cpu.memory[0x8000:0xc000] = prg[5*16384:6*16384]
 cpu.memory[0xc000:] = prg[-16384:]
 rows=[]
 def run(entered, parameter, actor, action, seed, difficulty, horizontal, vertical, phase, airborne, human):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xab87;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x539:0x53d]=[128]*4
  if human:cpu.memory[0x539+(actor^1)]=0
  for address,value in [(0x672,31|(128 if entered else 0)),(0x676,8),(0x66e,9),(0x682,parameter),(0x67e,32),(0x67a,2),(0x512,action|128),(0x4ec,32),(0x619,3),(0x61d,3),(0x3e3,1),(0x66a,0x52),(0x572,8),(0x576,phase&255),(0x4b9,vertical&255),(0x485,horizontal&255),(0x5f7,128 if airborne else 0),(0x52c,255)]:cpu.memory[address+actor]=value
  cpu.memory[0x58c]=difficulty
  cpu.memory[0x48:0x4c]=list(seed.to_bytes(4,'big'))
  for steps in range(500):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000, hex(cpu.pc)
  after={k:cpu.memory[a+actor] for k,a in dict(mode=0x672,plan=0x676,counter=0x66e,parameter=0x682,destination=0x67e,target=0x67a,action=0x512,heading=0x4ec,cycle=0x572,phase=0x576,nextAction=0x52c).items()}
  after['random']=int.from_bytes(bytes(cpu.memory[0x48:0x4c]),'big')
  rows.append(dict(entered=entered,parameter=parameter,actor=actor,action=action,seed=seed,difficulty=difficulty,horizontal=horizontal,vertical=vertical,phase=phase,airborne=airborne,human=human,after=after))
 for actor,horizontal,vertical,difficulty,half in itertools.product(range(4),[-1,0],[-1,0,1],range(0,32,4),[0,7,8,15,16,31,32,47,48,63,64,79,80,95,96,111,112,127]):
  b=half*2; high=(b<<1)&255; second=(((high>>1)&1)^(b>>7))<<1
  run(False,7,actor,62,(high<<24)|(second<<16),difficulty,horizontal,vertical,-1,True,False)
 for parameter,actor,action,horizontal,vertical,phase,airborne,human in itertools.product([1,2,8,9,10],range(4),[62,20,26],[-128,-1,0,127],[-128,-1,0,1,2,3,127],[-1,0],[False,True],[False,True]):
  run(True,parameter,actor,action,0x12345678,4,horizontal,vertical,phase,airborne,human)
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode(); packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'ai-mode31-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 AB87 including original B383 dispatch',rom_sha256=digest,threshold_bytes=list(cpu.memory[0xabc0:0xabd0]),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest())
 (out/'ai-mode31-function-source.json').write_text(json.dumps(source,indent=2)+'\n'); print(json.dumps(source))

if __name__=='__main__':main()
