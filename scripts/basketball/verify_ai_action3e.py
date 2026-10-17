"""Isolated original A05D Action3E gate and complete A11E possession route."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:];gates=[];waits=[]
 for mode,entered,actor,holder_kind,action_entered in itertools.product(range(42),[False,True],range(4),range(3),[False,True]):
  holder=[actor,actor^1,6][holder_kind]
  cpu.memory[:2048]=[0]*2048;cpu.pc=0xa05d;cpu.x=actor;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x512+actor]=62|(128 if action_entered else 0);cpu.memory[0x672+actor]=mode|(128 if entered else 0);cpu.memory[0x53f]=holder
  for n in range(100):
   if cpu.pc in (0xa08a,0x7000):break
   cpu.step()
  assert cpu.pc in (0xa08a,0x7000)
  gates.append(dict(mode=mode,entered=entered,actor=actor,holder=holder,actionEntered=action_entered,dispatch=cpu.pc==0xa08a,afterMode=cpu.memory[0x672+actor]))
 for entered,actor,holder_kind,human,carried,action_entered in itertools.product([False,True],range(4),range(3),[False,True],[-1,0,3],[False,True]):
  holder=[actor,actor^1,6][holder_kind]
  cpu.memory[:2048]=[0]*2048;cpu.pc=0xa05d;cpu.x=actor;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x539:0x53d]=[128]*4
  if human:cpu.memory[0x539+(actor^1)]=0
  cpu.memory[0x512+actor]=62|(128 if action_entered else 0);cpu.memory[0x672+actor]=1|(128 if entered else 0);cpu.memory[0x53f]=holder;cpu.memory[0x569]=carried&255
  for n in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000 and cpu.memory[0x569]==carried&255
  waits.append(dict(entered=entered,actor=actor,holder=holder,human=human,carried=carried,actionEntered=action_entered,afterMode=cpu.memory[0x672+actor]))
 payload=(json.dumps(dict(gates=gates,waits=waits),separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-action3e-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,gate_cases=len(gates),complete_possession_cases=len(waits),entry='bank5 A05D; gate stops A08A/return, full mode1 returns through A11E',mode_flags=list(cpu.memory[0x95ef:0x95ef+42]),rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'ai-action3e-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
