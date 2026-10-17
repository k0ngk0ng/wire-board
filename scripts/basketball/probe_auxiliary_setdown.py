"""Original E18B/9BA4 staged set-down calls and a complete input-driven chain."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_inputs import decode_world_input_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];source=ROOT/'.local/nes-basket/runs/court-3-reference/ram.bin';base=source.read_bytes()[1500*2048:1501*2048]
 rows=[];chains=[];controllers=[]
 def invoke(memory,address,actor):
  memory[0x8000:0xc000]=prg[6*16384:7*16384];cpu=MPU(memory=memory);cpu.pc,cpu.x,cpu.p=address,actor,0x20;cpu.sp=255;cpu.stPushWord(0x6fff)
  for tick in range(100000):
   if cpu.pc==0x7000:return
   if cpu.pc in (0xc000,0xfbcc):cpu.pc=cpu.stPopWord()+1
   else:cpu.step()
  raise RuntimeError(hex(cpu.pc))
 phases=[(False,0,0)]+[(True,c,t) for c in (0,1) for t in range(3 if c==0 else 4)]
 for actor,held,facing,(entered,cursor,elapsed),door in itertools.product([0,2],[4,5,None],[0,128],phases,[False,True]):
  r=bytearray(base)
  for a,v in {0x512:0x4f|(128 if entered else 0),0x51f:128,0x52c:0,0x437:elapsed,0x444:cursor,0x451:0,0x46b:facing,0x5f7:0,0x642:64,0x5b:facing|15,0x539:0,0x4ec:255}.items():r[a+actor]=v
  r[0x53d]=actor if held==4 else 255;r[0x53e]=actor if held==5 else 255;r[0x53f]=6;r[0x56b]=255
  for obj in [4,5]:r[0x512+obj]=129;r[0x642+obj]=actor|128;put(r,300*256,0x68+obj,0x75+obj,0x82+obj)
  put(r,(152 if door else 300)*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,(80 if door else 40)*256+19,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
  for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+actor,b+actor)
  memory=Memory(prg,r);before=decode_world_action_state(memory);invoke(memory,0xe18b,actor)
  rows.append(dict(actor=actor,court=3,before=before,after=decode_world_action_state(memory)))
 # Reuse the established explicit pickup setup; only controller samples change
 # after it. B has no direction and therefore requests 4F, not a high throw.
 r=bytearray(base)
 for actor in (0,5):
  put(r,300*256+37,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+17,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
  for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+actor,b+actor)
  for a,v in {0x512:128,0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:0,0x5f7:0,0x642:64,0x5b:0}.items():r[a+actor]=v
 for a,v in {0x53d:255,0x53e:255,0x53f:6,0x569:255,0x56b:255,0x550:255,0x539:0,0x4e8:0,0x4ec:255}.items():r[a]=v
 for a in (0x4f9,0x4fd,0x501,0x505,0x509,0x50d):r[a]=0
 memory=Memory(prg,r);last=0;second=None;dropped=False
 for tick in range(100):
  if second is None and memory[0x53e]==0 and memory[0x512]&127==32:second=tick+3
  held=128 if tick<4 else 64 if second is not None and second<=tick<second+4 else 0
  memory[0:8]=[held,0,0,0,held&~last,0,0,0];last=held
  ib=bytes(memory[:2048]);invoke(memory,0x805c,0);ia=bytes(memory[:2048])
  for actor in (0,5):
   before=decode_world_action_state(memory);invoke(memory,0xe18b,actor);chains.append(dict(tick=tick,actor=actor,pickupFlags=memory[0x634],before=before,after=decode_world_action_state(memory)))
  config=dict(courtID=ib[0x58b],controls=list(ib[0x539:0x53d]),characters=list(ib[0x3e3:0x3e7]),teams=list(ib[0x586:0x588]),runGrades=list(ib[0x615:0x619]),jumpGrades=list(ib[0x619:0x61d]),shotSkills=list(ib[0x61d:0x621]),recoveryGrades=list(ib[0x621:0x625]),equipment=ib[0x6e4],enhancementEnabled=ib[0x58d]<128,enhancements=[ib[0x6e5+3*ib[0x3e3+j]] for j in range(4)],otherObjectArcFlags=list(ib[0x6e9:0x6f0]))
  context=dict(world=config,frames=[dict(held=ib[j],pressed=ib[j+4]) for j in range(4)],difficulty=ib[0x58c],scoreDifference=ib[0x696],cachedZones=list(ib[0x66a:0x66e]),objectKinds=list(ib[0x3d6:0x3e3]),hoopLinks=list(ib[0x649:0x64f]),humanTactics=list(ib[0x666:0x668]))
  controllers.append(dict(tick=tick,before=decode_world_input_state(ib),after=decode_world_input_state(ia),afterActions=decode_world_input_state(memory),pickupBefore=list(ib[0x634:0x638]),pickupAfter=list(ia[0x634:0x638]),jumpEnhancements=[ib[0x6e6+3*ib[0x3e3+j]] for j in range(4)],context=context))
  dropped|=memory[0x512]&127==0x4f
  if dropped and memory[0x53e]==255 and memory[0x517]&127==0 and memory[0x512]&127==0:break
 else:raise RuntimeError('set-down chain did not finish')
 out=ROOT/'games/basketball/engine/testdata'
 for name,data in [('auxiliary-setdown-probes',rows),('auxiliary-setdown-chain',chains),('auxiliary-setdown-controller',controllers)]:
  p=out/(name+'.json.gz');p.write_bytes(gzip.compress((json.dumps(data,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name(name+'-source.json').write_text(json.dumps(dict(scope='original functions from an explicit Court3 setup; not natural-match proof',records=len(data),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
 print('setdown probes',len(rows),'chain boundaries',len(chains))
if __name__=='__main__':main()
