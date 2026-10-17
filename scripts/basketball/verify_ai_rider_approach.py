"""Isolated original AA2C, its support approach, facing and proximity helpers."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];cpu.memory[0xc000:]=prg[-16384:];rows=[]
 cases=[]
 for actor,entered,action,dx,dz,air,zone in itertools.product(range(4),[False,True],[0,1,2,61],[-300,-9,-8,0,8,9,300],[-9,-4,-3,0,7,8,9],[False,True],[0x30,0x35]):
  cases.append(dict(actor=actor,entered=entered,action=action,dx=dx,dz=dz,air=air,zone=zone,grade=3,phase=0,speed=1,holder=6))
 for actor,entered,grade,phase,speed,holder_kind,dx in itertools.product(range(4),[False,True],[0,3,4,7],[-1,0],[-1,0,1],range(3),[-10,0,10]):
  cases.append(dict(actor=actor,entered=entered,action=4,dx=dx,dz=0,air=True,zone=0x30,grade=grade,phase=phase,speed=speed,holder=[actor,actor^1,6][holder_kind]))
 for actor,entered,holder_kind,dx,dz in itertools.product(range(4),[False,True],range(3),[-257,-16,-15,0,15,16,257],[-9,-8,-7,0,7,8,9]):
  cases.append(dict(actor=actor,entered=entered,action=63,dx=dx,dz=dz,air=True,zone=0x30,grade=3,phase=0,speed=1,holder=[actor,actor^1,6][holder_kind]))
 for case in cases:
  actor=case['actor'];cpu.memory[:2048]=[0]*2048;cpu.sp=255;cpu.pc=0xaa2c;cpu.x=actor;cpu.p=0x20;cpu.stPushWord(0x6fff)
  positions=[dict(x=200,depth=40) for _ in range(7)];positions[actor]=dict(x=(200+case['dx'])&65535,depth=(40+case['dz'])&255)
  for seat,position in enumerate(positions):cpu.memory[0x75+seat]=position['x']&255;cpu.memory[0x82+seat]=position['x']>>8;cpu.memory[0x9c+seat]=position['depth']
  for address,value in [(0x672,29|(128 if case['entered'] else 0)),(0x512,case['action']|128),(0x4ec,32),(0x46b,128 if actor&1 else 0),(0x67e,7),(0x67a,(actor^1) if case['entered'] else (actor^2)),(0x619,case['grade']),(0x576,case['phase']&255),(0x4b9,case['speed']&255),(0x5f7,128 if case['air'] else 0),(0x66a,case['zone']),(0x478,0xd2),(0x485,4),(0x492,0xa7),(0x49f,0xfe)]:cpu.memory[address+actor]=value
  cpu.memory[0x53f]=case['holder']
  for steps in range(300):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000,case
  after={k:cpu.memory[a+actor] for k,a in dict(mode=0x672,action=0x512,heading=0x4ec,facing=0x46b,destination=0x67e,target=0x67a,phase=0x576).items()}
  after['vx']=cpu.memory[0x478+actor]+256*cpu.memory[0x485+actor];after['vz']=cpu.memory[0x492+actor]+256*cpu.memory[0x49f+actor]
  rows.append(dict(case,positions=positions,after=after))
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'ai-rider-approach-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 AA2C including ADA4/B05C/B1E9',rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'ai-rider-approach-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
