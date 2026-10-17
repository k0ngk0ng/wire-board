"""Original B100 isolated alias88 oracle and original F4/DA initialization blocks."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 prg=rom[16:16+131072];cpu=MPU();cpu.memory[0xc000:]=prg[-16384:]
 cpu.memory[:256]=[0xaa]*256;cpu.pc=0xfa8c;cpu.x=0x5b;cpu.sp=255;cpu.stPushWord(0x6fff)
 for n in range(2000):
  if cpu.pc==0x7000:break
  cpu.step()
 assert cpu.pc==0x7000 and cpu.memory[0xf4]==0 and cpu.memory[0xda]==0
 cpu.memory[0x8000:0xc000]=prg[3*16384:4*16384];cpu.pc=0xa0d3
 for n in range(200):
  if cpu.pc==0xa0df:break
  cpu.step()
 assert cpu.pc==0xa0df and cpu.memory[0xd3:0xe3]==[255]*16 and cpu.memory[0xf4]==0
 initialization=dict(zero_page='fixed FA8C with X=5B, as called by C9B6',list='bank3 A0D3..A0DF, before equipment initialization',depth88=cpu.memory[0xf4],xHigh88=cpu.memory[0xda])
 cpu.memory[0x8000:0xc000]=prg[5*16384:6*16384];rows=[]
 for actor,pose_source,depth,x_high,x_low,ball_x,ball_depth in itertools.product([0,3],[0,1,2,3,6,8,129,255],[0,16,201],[0,1,255],[0,1],[0,1,32,255,256,257,511],[0,8,16,17,32,200,255]):
  cpu.memory[:2048]=[0]*2048;cpu.pc=0xb100;cpu.x=actor;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
  cpu.memory[0x512+((actor&2)^2)]=0xd7;cpu.memory[0x56a]=pose_source;cpu.memory[0xf4]=depth;cpu.memory[0xda]=x_high;cpu.memory[0xcd]=x_low
  cpu.memory[0x7b]=ball_x&255;cpu.memory[0x88]=ball_x>>8;cpu.memory[0xa2]=ball_depth
  probes=[];read_depth=False;read_x=False
  for n in range(200):
   if cpu.pc==0x7000:break
   if cpu.pc==0xb10a:probes.append(cpu.y)
   if cpu.pc==0xb11b:read_depth=True
   if cpu.pc==0xb12c:read_x=True
   cpu.step()
  assert cpu.pc==0x7000 and probes==[((actor&2)^2),88]
  target=cpu.memory[0x67a+actor] if cpu.p&1 else -1
  rows.append(dict(actor=actor,poseSource=pose_source,depth=depth,xHigh=x_high,xLow=x_low,ballX=ball_x,ballDepth=ball_depth,target=target,readDepth=read_depth,readX=read_x))
 payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'pursuit-alias88-function.json.gz').write_bytes(packed)
 source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,records=len(rows),entry='bank5 B100, rejected opponent action57 then candidate88',initialization=initialization,rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest());(out/'pursuit-alias88-function-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
