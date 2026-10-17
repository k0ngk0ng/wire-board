"""Original carried/hidden/throw object actions with explicit isolated inputs."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];base=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048];rows=[]
 for action,entered,attached,pose,facing in itertools.product([1,3,6,14,15],[False,True],[False,True],[0,5,25,75],[0,128]):
  if action==3 and entered and attached:continue
  r=bytearray(base);actor=5
  for i in [0,1,5]:
   put(r,300*256+31+i,0x68+i,0x75+i,0x82+i);put(r,40*256+17+i,0x8f+i,0x9c+i);put(r,(32 if i<4 else 48)*256+19+i,0xa9+i,0xb6+i,0xc3+i)
   for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+i,b+i)
  r[0x512]=0xa7;r[0x513]=0xa0;r[0x5b]=pose|facing;r[0x5c]=pose|facing;r[0x46b]=facing;r[0x46c]=facing
  for a,v in {0x517:action|(128 if entered else 0),0x524:128,0x531:3 if action==14 else 2 if action==15 else 255,0x43c:0,0x449:3 if action==3 and entered else 0,0x456:0,0x470:0,0x5fc:128,0x647:128 if attached else 0,0x60:0,0x53e:255 if action==3 and entered else 0,0x3d6:0,0x3d7:0,0x3db:14,0x56b:1,0x56a:3,0x58b:1}.items():r[a]=v
  put(r,33,0x478+5,0x485+5);put(r,17,0x492+5,0x49f+5)
  memory=Memory(prg,r);cpu=MPU(memory=memory);cpu.pc={1:0xb05d,3:0xb0b1,6:0xb164,14:0xb42b,15:0xb453}[action];cpu.x=5;cpu.p=0x20;cpu.sp=255;cpu.stPushWord(0x6fff);before=decode_world_action_state(memory)
  for steps in range(30000):
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000,hex(cpu.pc)
  rows.append(dict(action=action,before=before,after=decode_world_action_state(memory)))
 accepted=[]
 memory=Memory(prg,bytearray(base));cpu=MPU(memory=memory)
 for action,request,entered in itertools.product(range(16),range(16),(0,128)):
  memory[0x517]=action|entered;memory[0x524]=request;cpu.pc,cpu.x,cpu.p=0xe18b,5,0x20
  for tick in range(500):
   if cpu.pc==0xe1ed:break
   cpu.step()
  assert cpu.pc==0xe1ed
  accepted.append(dict(action=action,request=request,entered=entered!=0,nextAction=memory[0x517],nextRequest=memory[0x524]))
 (ROOT/'games/basketball/engine/testdata/auxiliary-request-probes.json').write_text(json.dumps(accepted,separators=(',',':'))+'\n')
 p=ROOT/'games/basketball/engine/testdata/auxiliary-carry-probes.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name('auxiliary-carry-probes-source.json').write_text(json.dumps(dict(scope='isolated original bank6 full handlers 1/3/6/14/15 with original motion; not natural-gameplay proof',records=len(rows),requestRecords=len(accepted),requestSHA256=hashlib.sha256((ROOT/'games/basketball/engine/testdata/auxiliary-request-probes.json').read_bytes()).hexdigest(),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print(len(rows))
if __name__=='__main__':main()
