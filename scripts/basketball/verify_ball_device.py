"""Original owner-six shot and stuck-rim ball instruction probes.
Runs unmodified original routines and mapped MMC3 writes. No natural-match claim.
"""
import gzip,hashlib,itertools,json
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,MPU,ball,put

def decode(r):
 d=ball(r);d.update(unownedSpecial=r[0x55a],target=dict(x=(r[0x69f]+256*r[0x6a0])*256,depth=r[0x6a1]*256,height=(r[0x6a2]+256*r[0x6a3])*256),arc=r[0x6a4]*256,landing=dict(x=(r[0x699]+256*r[0x69a])*256,depth=r[0x698]*256,height=0))
 return d

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();digest=hashlib.sha256(rom).hexdigest();assert digest=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 ram=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048];records=[]
 def run(r,entry,kind):
  mem=Memory(rom[16:16+131072],r);cpu=MPU(memory=mem);cpu.sp=255;cpu.stPushWord(0x6fff);cpu.pc,cpu.x,cpu.p=entry,6,0x20;before=decode(mem)
  for steps in range(30000):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000,(kind,hex(cpu.pc))
  records.append(dict(kind=kind,before=before,after=decode(mem),hoopHeight=(r[0xb6+r[0x55d]]+256*r[0xc3+r[0x55d]])*256,mountFlags=list(r[0x6f0:0x6f6]),instructions=steps))
 def base():
  r=bytearray(ram)
  for a,v in {0x525:255,0x43d:0,0x44a:0,0x457:0,0x532:255,0x5fd:128,0x648:0,0x53f:6,0x564:0,0x561:2,0x3dc:8,0x589:129,0x58d:0,0x55a:0}.items():r[a]=v
  put(r,200*256+113,0x6e,0x7b,0x88);put(r,40*256+71,0x95,0xa2);put(r,30*256+91,0xaf,0xbc,0xc9)
  put(r,0,0x47e,0x48b);put(r,0,0x498,0x4a5);put(r,0,0x4b2,0x4bf);put(r,0,0x4cc,0x4d9)
  return r
 for height,facing,boost,flags in itertools.product([0,55,56,95,96,151,152,191,192,271,272],[0,128],[0,128],[1,3]):
  r=base();r[0x518]=4;r[0x471]=facing;r[0x55a]=boost;r[0x6f0:0x6f6]=bytes([flags]*6);put(r,height*256+91,0xaf,0xbc,0xc9);run(r,0xb65f,'owner6')
 for side,height in itertools.product([0,1],[80,176,272]):
  r=base();r[0x518]=9;r[0x471]=128 if side==0 else 0;r[0x55d]=7+side;put(r,height,0xb6+7+side,0xc3+7+side);put(r,(62 if side==0 else 462)*256+113,0x6e,0x7b,0x88);run(r,0xb7ee,'stuck-entry')
 for side,x,count,vx,cursor in itertools.product([0,1],[53,54,61,62,69,70],[0,1,2,96],[-384,-1,0,256],range(5)):
  r=base();r[0x518]=137;r[0x471]=128 if side==0 else 0;r[0x511]=count;r[0x44a]=cursor;r[0x55d]=7+side;put(r,(x if side==0 else x+400)*256+113,0x6e,0x7b,0x88);put(r,94*256+91,0xaf,0xbc,0xc9);put(r,vx,0x47e,0x48b);run(r,0xb7ee,'stuck-update')
 clearMemory=Memory(rom[16:16+131072],bytes([165])*2048);clearCPU=MPU(memory=clearMemory);clearCPU.sp=255;clearCPU.stPushWord(0x6fff);clearCPU.pc=0xf8bd;clearCPU.a,clearCPU.x,clearCPU.y=0,2,7
 for clearSteps in range(20000):
  if clearCPU.pc==0x7000:break
  clearCPU.step()
 assert clearCPU.pc==0x7000 and all(v==0 for v in clearMemory[0x200:0x800])
 clearProof=dict(entry='fixed F8BD with C9CB setup A=0 X=2 Y=7',initialFill=165,clearedRange='0200..07FF',unownedSpecialAfter=clearMemory[0x55a],instructions=clearSteps)
 out=ROOT/'games/basketball/engine/testdata';p=out/'ball-device.json.gz';p.write_bytes(gzip.compress((json.dumps(records,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name('ball-device-source.json').write_text(json.dumps(dict(scope='isolated original instruction execution; full action routines including motion, no natural-match claim',records=len(records),romSHA256=digest,sha256=hashlib.sha256(p.read_bytes()).hexdigest(),entry=['bank6:B65F','bank6:B7EE'],coldClear=clearProof),indent=2)+'\n');print(len(records))
if __name__=='__main__':main()
