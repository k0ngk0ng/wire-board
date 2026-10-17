"""Explicit original $68 / auxiliary 8..11 function probes and controller chains."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_inputs import decode_world_input_state

def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];raw=(ROOT/'.local/nes-basket/runs/court-3-reference/ram.bin').read_bytes();base=raw[1500*2048:1501*2048]
 def invoke(m,address,actor):
  m[0x8000:0xc000]=prg[6*16384:7*16384];cpu=MPU(memory=m);cpu.pc,cpu.x,cpu.p=address,actor,0x20;cpu.sp=255;cpu.stPushWord(0x6fff);sounds=[]
  for tick in range(100000):
   if cpu.pc==0x7000:return sounds
   if cpu.pc in (0xc000,0xfbcc):
    if cpu.pc==0xfbcc:sounds.append(cpu.a)
    cpu.pc=cpu.stPopWord()+1
   else:cpu.step()
  raise RuntimeError(hex(cpu.pc))
 def config(r):
  return dict(courtID=r[0x58b],controls=list(r[0x539:0x53d]),characters=list(r[0x3e3:0x3e7]),teams=list(r[0x586:0x588]),runGrades=list(r[0x615:0x619]),jumpGrades=list(r[0x619:0x61d]),shotSkills=list(r[0x61d:0x621]),recoveryGrades=list(r[0x621:0x625]),equipment=r[0x6e4],enhancementEnabled=r[0x58d]<128,enhancements=[r[0x6e5+3*r[0x3e3+j]] for j in range(4)],otherObjectArcFlags=list(r[0x6e9:0x6f0]))
 def setup(actor=0,held=5,team=0,char=0,face=0):
  r=bytearray(base);r[0x586+actor//2]=team;r[0x3e3+actor]=char
  for o in range(7):
   for a,v in {0x512:128,0x51f:128,0x52c:255,0x437:0,0x444:0,0x451:0,0x46b:face,0x5f7:0,0x642:64,0x5b:0,0x4ec:255}.items():r[a+o]=v
   put(r,(260+o*6)*256+37,0x68+o,0x75+o,0x82+o);put(r,40*256+17,0x8f+o,0x9c+o);put(r,0,0xa9+o,0xb6+o,0xc3+o)
   for a,b in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,a+o,b+o)
  r[0x53d]=actor if held==4 else 255;r[0x53e]=actor if held==5 else 255;r[0x53f]=6;r[0x56b]=255;r[0x569]=255;r[0x56f]=actor;r[0x570]=held
  for i in range(4):r[0x539+i]=i;r[0x4e4+i]=0;r[0x550+i]=255;r[0x4e8+i]=i
  r[0x512+held]=129;r[0x642+held]=actor|192
  put(r,6*256 if face==0 else -6*256,0x478+actor,0x485+actor)
  return r
 rows=[]
 for team,char,actor,held,face in itertools.product(range(8),range(4),(0,2),(4,5),(0,128)):
  role=team*4+char;special=role in (0,1,4,16);release=role==28
  phases=[(False,0,0,0,0),(True,0,0,3,0)]
  if special:phases += [(True,c,0,l,0) for c in range(5) for l in (3,4)]
  if release:phases += [(True,c,0,0,b) for c in range(5) for b in (0,128)]
  for entered,cursor,elapsed,loops,pressed in phases:
   r=setup(actor,held,team,char,face);r[0x512+actor]=0x68|(128 if entered else 0);r[0x444+actor]=cursor;r[0x437+actor]=elapsed;r[0x451+actor]=loops;r[0x52c+actor]=0;r[0x57a+actor]=12 if special else 63 if release else 255;r[0x4e4+actor]=pressed
   m=Memory(prg,r);before=decode_world_action_state(m);sounds=invoke(m,0xe18b,actor)
   rows.append(dict(actor=actor,before=before,after=decode_world_action_state(m),context=config(r),pressed=list(r[0x4e4:0x4e8]),sounds=sounds))
 aux=[]
 for action,obj,owner,face,entered in itertools.product((8,9,10,11),(4,5),(0,2,6,7),(0,128),(False,True)):
  phases=[(0,0,3)] if not entered else [(0,0,3),(1,0,4)]
  if action==8 and entered:phases += [(3,2,0),(4,1,0)]
  if action==10 and entered:phases += [(c,0,l) for c in range(5) for l in (3,4)]
  for cursor,elapsed,loops in phases:
   for height,grav in [(32,256),(64,-256)]:
    r=setup(0,obj,0,0,face);r[0x539+obj]=owner;r[0x512+obj]=action|(128 if entered else 0);r[0x444+obj]=cursor;r[0x437+obj]=elapsed;r[0x451+obj]=loops;r[0x52c+obj]=1;r[0x642+obj]=0;r[0x5f7+obj]=128;r[0x451+owner]=3;r[0x5b+owner]=128 if face==0 else 0
    put(r,height*256+29,0xa9+obj,0xb6+obj,0xc3+obj);put(r,grav,0x4c6+obj,0x4d3+obj)
    m=Memory(prg,r);before=decode_world_action_state(m);sounds=invoke(m,0xe18b,obj)
    aux.append(dict(actor=obj,before=before,after=decode_world_action_state(m),context=config(r),sounds=sounds))
 # Edge aliases: ownership selection, attached actors, stale requests,
 # non-default global object links, high-byte preservation, and device mouths.
 for (team,char),held,link,request in itertools.product([(0,0),(0,2),(7,0)],(4,5),(0,130,135),(128,0x22)):
  r=setup(0,held,team,char);r[0x512]=0x68;r[0x642]=link;r[0x51f]=request
  # Keep only requests rejected in68 so the tested role dispatch is entered.
  m=Memory(prg,r);before=decode_world_action_state(m);sounds=invoke(m,0xe18b,0)
  # A request may be accepted by E18B; retain only calls which actually entered68.
  if request<128 and m[0x51f]>=128:continue
  rows.append(dict(actor=0,before=before,after=decode_world_action_state(m),context=config(r),pressed=list(r[0x4e4:0x4e8]),sounds=sounds))
 for action,obj,leader,target in itertools.product((9,11),(4,5),(0,2,255),(0,4,5,6)):
  r=setup(0,obj);r[0x512+obj]=action;r[0x642+obj]=0;r[0x56f]=leader;r[0x570]=target
  put(r,75*256+193,0xa9+obj,0xb6+obj,0xc3+obj);r[0x5f7+obj]=128
  m=Memory(prg,r);before=decode_world_action_state(m);sounds=invoke(m,0xe18b,obj)
  aux.append(dict(actor=obj,before=before,after=decode_world_action_state(m),context=config(r),sounds=sounds))
 # Single-initial-state input chain: start with held prop running, release
 # direction then press B. Inputs remain raw original controller buttons.
 controllers=[]
 for team,char in [(0,0),(2,0),(7,0)]:
  for actor,held,pickup in [(0,4,False),(2,5,False),(0,4,True),(2,5,True)]:
   r=setup(actor,held,team,char);r[0x512+actor]=0xa2;r[0x4ec+actor]=64;r[0x52c+actor]=255
   for a in (0x4f9,0x4fd,0x501,0x505,0x509,0x50d):r[a+actor]=0
   if pickup:
    r[0x512+actor]=128;r[0x512+held]=128;r[0x539+held]=255;r[0x642+held]=64;r[0x4ec+actor]=255
    put(r,0,0x478+actor,0x485+actor)
    put(r,(260+actor*6)*256+37,0x68+held,0x75+held,0x82+held)
    other=held^1
    put(r,440*256,0x68+other,0x75+other,0x82+other)
   m=Memory(prg,r);last=0;chain=[]
   for tick in range(70 if pickup else 55):
    button=64 if tick==0 else 128 if team==7 and tick==12 else 0
    if pickup:button=128 if tick<4 else 1 if tick==8 or 10<=tick<=13 else 64 if tick==14 else 128 if team==7 and tick==27 else 0
    m[0:8]=[0]*8;m[actor]=button;m[4+actor]=button&~last;last=button
    ib=bytes(m[:2048]);invoke(m,0x805c,actor);ia=bytes(m[:2048]);boundaries=[]
    for obj in (actor,held):
     before=decode_world_action_state(m);sounds=invoke(m,0xe18b,obj);boundaries.append(dict(actor=obj,before=before,after=decode_world_action_state(m),sounds=sounds))
    context=dict(world=config(ib),frames=[dict(held=ib[j],pressed=ib[j+4]) for j in range(4)],difficulty=ib[0x58c],scoreDifference=ib[0x696],cachedZones=list(ib[0x66a:0x66e]),objectKinds=list(ib[0x3d6:0x3e3]),hoopLinks=list(ib[0x649:0x64f]),humanTactics=list(ib[0x666:0x668]))
    chain.append(dict(tick=tick,actor=actor,object=held,before=decode_world_input_state(ib),after=decode_world_input_state(ia),afterActions=decode_world_input_state(m),pickupBefore=list(ib[0x634:0x638]),pickupAfter=list(ia[0x634:0x638]),context=context,boundaries=boundaries))
   controllers.append(chain)
 out=ROOT/'games/basketball/engine/testdata'
 for name,data in [('auxiliary-running-players',rows),('auxiliary-running-objects',aux),('auxiliary-running-controller',controllers)]:
  p=out/(name+'.json.gz');p.write_bytes(gzip.compress((json.dumps(data,separators=(',',':'))+'\n').encode(),mtime=0));p.with_name(name+'-source.json').write_text(json.dumps(dict(scope='original explicit Court3 function probes and continuous controller chains; not full natural matches',records=len(data),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
 print('players',len(rows),'objects',len(aux),'controller chains',len(controllers))
if __name__=='__main__':main()
