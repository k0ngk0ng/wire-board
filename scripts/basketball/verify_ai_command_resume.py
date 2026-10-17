"""Isolated original A8C1 resume function with actor-index and command-status cases."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:];rows=[]
 for actor,tactic,holder,mode,entered,status in itertools.product(range(4),range(6),range(7),[21,22],[False,True],[0,127,128]):
  cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xa8c1;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  statuses=[30,31,32,33];statuses[actor]=status;cpu.memory[0x657:0x65b]=statuses
  cpu.memory[0x672+actor]=mode|(128 if entered else 0);cpu.memory[0x666+actor//2]=tactic;cpu.memory[0x53f]=holder
  for step in range(200):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000
  rows.append(dict(actor=actor,tactic=tactic,holder=holder,mode=mode,entered=entered,statuses=statuses,afterMode=cpu.memory[0x672+actor],afterStatuses=list(cpu.memory[0x657:0x65b])))
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-command-resume-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 A8C1 including original 9B85/9C6D',rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'ai-command-resume-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
