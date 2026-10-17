"""Original auxiliary action2 throw arcs and uninterrupted landing chains."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];base=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048];rows=[]
 for chain,(object,height,facing,arc,diagonal) in enumerate(itertools.product([4,5],[0,55,56,151,152,271],[0,128],[1,3],[False,True])):
  r=bytearray(base);a=object
  for address,value in {0x512+a:2,0x51f+a:128,0x52c+a:255,0x437+a:0,0x444+a:0,0x451+a:0,0x46b+a:128,0x5f7+a:128,0x642+a:128,0x539+a:0,0x3d6+a:14,0x46b:facing,0x55d:7,0x58d:0,0x589:129,0x58b:1}.items():r[address]=value
  put(r,height*256+7,0xa9,0xb6,0xc3);put(r,(height+32)*256+11,0xa9+a,0xb6+a,0xc3+a)
  put(r,(420 if diagonal else 200)*256+13,0x68+a,0x75+a,0x82+a);put(r,(87 if diagonal else 40)*256+17,0x8f+a,0x9c+a)
  r[0x69]=37;r[0x6f0]=arc
  memory=Memory(prg,r);cpu=MPU(memory=memory)
  for tick in range(300):
   before=decode_world_action_state(memory);cpu.pc,cpu.x,cpu.p=0xb07e,object,0x20;cpu.sp=255;cpu.stPushWord(0x6fff)
   for steps in range(30000):
    if cpu.pc==0x7000:break
    cpu.step()
   assert cpu.pc==0x7000,hex(cpu.pc)
   rows.append(dict(chain=chain,tick=tick,object=object,arcFlags=arc,secondPlayerX=before['actors'][1]['player']['body']['position']['x'],before=before,after=decode_world_action_state(memory)))
   if memory[0x512+object]&127==0:break
   # One complete chain per object/facing; other cases retain the launch only.
   if height!=56 or arc!=1 or diagonal:break
  else:raise RuntimeError('prop did not land')
 p=ROOT/'games/basketball/engine/testdata/auxiliary-lob.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name('auxiliary-lob-source.json').write_text(json.dumps(dict(scope='original bank6 B07E isolated launches plus four continuous landing chains, no natural-match claim',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print(len(rows))
if __name__=='__main__':main()
