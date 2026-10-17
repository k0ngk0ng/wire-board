"""Original pickup/A-throw action boundaries from the established explicit setup."""
import gzip,hashlib,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];source=ROOT/'.local/nes-basket/runs/court-3-reference/ram.bin';base=source.read_bytes()[1500*2048:1501*2048];rows=[]
 for spare in (0,0x41):
  r=bytearray(base)
  for actor in (0,5):
   put(r,300*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+17,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
   for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+actor,b+actor)
   for a,v in {0x512:128,0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:0,0x5f7:0,0x642:64,0x5b:0}.items():r[a+actor]=v
  for a,v in {0x53d:255,0x53e:255,0x53f:6,0x569:255,0x56b:255,0x550:255,0x539:0,0x4e8:0,0x4ec:255}.items():r[a]=v
  for a in (0x4f9,0x4fd,0x501,0x505,0x509,0x50d):r[a]=0
  memory=Memory(prg,r);cpu=MPU(memory=memory)
  def call(addr,actor):
   memory[0x8000:0xc000]=prg[6*16384:7*16384];cpu.sp=255;cpu.stPushWord(0x6fff);cpu.pc,cpu.x,cpu.p=addr,actor,0x20
   for _ in range(100000):
    if cpu.pc==0x7000:return
    if cpu.pc==0xc000:cpu.pc=cpu.stPopWord()+1
    else:cpu.step()
   raise RuntimeError(hex(cpu.pc))
  last=0;second=None;released=False
  for tick in range(100):
   if second is None and memory[0x53e]==0 and memory[0x512]&127==32:second=tick+3
   held=128 if tick<4 or second is not None and second<=tick<second+4 else 0
   memory[0:8]=[held,0,0,spare,held&~last,0,0,spare if tick==0 else 0];last=held
   call(0x805c,0)
   for actor in (0,5):
    before=bytes(memory[:2048]);call(0xe18b,actor);after=bytes(memory[:2048])
    rows.append(dict(chain=spare,tick=tick,actor=actor,pickupFlags=before[0x634],before=decode_world_action_state(before),after=decode_world_action_state(after)))
   released|=memory[0x517]&127==3
   if released and memory[0x517]&127==0:break
 out=ROOT/'games/basketball/engine/testdata';p=out/'auxiliary-carry-chain.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name('auxiliary-carry-chain-source.json').write_text(json.dumps(dict(scope='explicit original Court3 pickup and A-throw setup; input E18B handlers and motion execute unchanged; not natural-match proof',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),sourceRAM=str(source),sourceFrame=1501,sourceRAMSHA256=hashlib.sha256(base).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print('auxiliary carry boundaries',len(rows))
if __name__=='__main__':main()
