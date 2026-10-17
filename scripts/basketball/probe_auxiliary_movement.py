"""Original group20 carry movement and attached A-throw action calls."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];base=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048];rows=[]
 cases=list(itertools.product([0x21,0x22,0x23,0x24,0x25,0x26,0x28,0x29,0x2a,0x2b,0x2c,0x0f],[False,True],range(9),[False,True],[4,5,None],[0,40],[0,7]))
 cases += [(a,e,h,c,o,40,0) for a,e,h,c,o in itertools.product([0x27,0x2b],[False,True],[0,2,6,8],[False,True],[4,5])]
 for action,entered,heading,computer,held,height,grade in cases:
  # Throw requires a held prop; missing ownership is separately observed only
  # on movement/jump/fall paths, where the original has explicit fallbacks.
  if action in (0x28,0x2a,0x2b,0x2c) and held is None:continue
  r=bytearray(base)
  for a,v in {0x512:action|(128 if entered else 0),0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:128,0x5f7:128 if height else 0,0x642:193 if action in (0x27,0x2b) and height==40 and grade==0 and heading in (0,2,6,8) else 64,0x5b:3,0x539:128 if computer else 0,0x4ec:255 if heading==8 else heading*32,0x53d:0 if held==4 else 255,0x53e:0 if held==5 else 255,0x53f:6,0x569:255,0x56b:255,0x554:128,0x58d:128,0x615:grade,0x619:grade,0x572:5,0x6d7:0}.items():r[a]=v
  put(r,300*256+19,0x68,0x75,0x82);put(r,40*256+23,0x8f,0x9c);put(r,height*256,0xa9,0xb6,0xc3)
  put(r,-833,0x478,0x485);put(r,255,0x492,0x49f);put(r,-512 if height else 0,0x4ac,0x4b9);put(r,128 if height else 0,0x4c6,0x4d3)
  put(r,320*256+11,0x69,0x76,0x83);put(r,42*256+13,0x90,0x9d);put(r,32*256+17,0xaa,0xb7,0xc4);r[0x3d7]=0;r[0x5c]=5
  # Two original equipment variants cover EC31's gravity override.
  if action in (0x24,0x25) and grade==7:r[0x58d]=0;r[0x586]=0;r[0x3e3]=0;r[0x6e6]=23
  if action in (0x24,0x25) and not entered and grade==7:r[0x642]=193
  if action==0x0f and held is not None and grade==7:r[0x53f]=0;r[0x518]=129
  if action==0x29 and grade==0:put(r,0,0x4c6,0x4d3)
  memory=Memory(prg,r);cpu=MPU(memory=memory);cpu.pc=0xe18b;cpu.x=0;cpu.p=0x20;cpu.sp=255;cpu.stPushWord(0x6fff);before=decode_world_action_state(memory);sounds=[]
  for steps in range(30000):
   if cpu.pc==0x7000:break
   if cpu.pc in (0xc000,0xfbcc):sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
   else:cpu.step()
  assert cpu.pc==0x7000,(action,hex(cpu.pc))
  rows.append(dict(before=before,after=decode_world_action_state(memory),computer=computer,runGrade=grade,jumpGrade=grade,team=r[0x586],enhancementEnabled=r[0x58d]<128,jumpEnhancement=r[0x6e6+3*r[0x3e3]],sounds=sounds))
 p=ROOT/'games/basketball/engine/testdata/auxiliary-movement.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name('auxiliary-movement-source.json').write_text(json.dumps(dict(scope='isolated original E18B carry movement handlers including original motion; not natural gameplay',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print(len(rows))
if __name__=='__main__':main()
