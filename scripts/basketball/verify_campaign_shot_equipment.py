"""Shirt1 full original player releases and complete world action phases.
Controlled calls use recorded RAM and original ROM instructions. No FCEUX
capture is started, and no controlled shot is labelled natural gameplay.
"""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
HOME=ROOT/'.local/nes-basket/runs/campaign-audit-equipped-home-match/ram.bin'
HOME_DATA=HOME.read_bytes();BASE=HOME_DATA[(4345-1)*2048:4345*2048]
NY=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
assert hashlib.sha256(HOME_DATA).hexdigest()=='c7c6cffa803c4d996d86cd7b8968c5788fc953f1545f3d9d070dce96e42b7173'
assert BASE[0x6e4]==1 and BASE[0x58d]==0 and BASE[0x58c]==2
class SoundMemory(Memory):
 def __setitem__(self,a,v):
  if isinstance(a,int) and 0x4000<=a<=0x4015:list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
def initial(actor=0,char=0,air=False,mode=0,item=1,cycle=3,zone=0x31,entered=False,seed=0x12345678,team=0,home=False):
 r=bytearray(BASE if home else NY);r[0x58b]=0 if home else 1;r[0x58d]=mode;r[0x586+actor//2]=team
 r[0x3e3+actor]=char;r[0x3e3+(actor^1)]=(char+1)&3
 r[0x6e4:0x6f0]=bytes([255]*12);r[0x6e4+char*3]=item
 for a,v in {0x512+actor:(28 if air else 27)|(128 if entered else 0),0x51f+actor:128,0x46b+actor:128 if actor>=2 else 0,0x4ec+actor:192 if actor>=2 else 64,0x437+actor:0,0x444+actor:0,0x451+actor:0,0x52c+actor:255,0x5f7+actor:128 if air else 0,0x642+actor:64,0x4e0+actor:0,0x4e4+actor:0,0x572+actor:cycle&255,0x554+actor:128 if entered else 0,0x66a+actor:zone,0x53f:actor,0x518:130,0x525:128,0x648:64,0x511:2,0x564:0,0x569:255,0x56b:255}.items():r[a]=v
 put(r,200*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+33,0x8f+actor,0x9c+actor);put(r,60*256 if air else 0,0xa9+actor,0xb6+actor,0xc3+actor)
 for lo,hi in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,lo+actor,hi+actor)
 put(r,208*256+17,0x6e,0x7b,0x88);put(r,40*256+33,0x95,0xa2);put(r,(78 if air else 18)*256,0xaf,0xbc,0xc9)
 r[0x48:0x4c]=seed.to_bytes(4,'big')
 return r

def run(r,actor,world=False):
 m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0xd345 if world else 0xe165);cpu.x=actor;sounds=[];calls=[];release=None;rngCalls=0
 stop=0xd348 if world else 0xe168
 for _ in range(150000):
  if cpu.pc==0xe18b:calls.append(cpu.x)
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0xf9c3:rngCalls+=1
  if cpu.pc==0xe168 and release is None:release=decode_world_action_state(bytes(m[:2048]))
  if cpu.pc==stop:break
  cpu.step()
 else:raise RuntimeError(('did not return',actor,world,hex(cpu.pc)))
 ctx=context(r)['actions'];ctx.update(cachedZones=list(r[0x66a:0x66e]),ballShotInputButtons=r[0x4e6])
 return dict(actor=actor,before=decode_world_action_state(r),after=decode_world_action_state(bytes(m[:2048])),context=ctx,calls=calls,sounds=sounds,rngCalls=rngCalls,afterFirstPlayer=release)
def main():
 player=[]
 for actor,char,air,mode,cycle,zone in itertools.product([0,1],range(4),[False,True],[0,1,128],[-1,0,1,2,3,4,5,6,7,8],[0x0f,0x10,0x31,0x35,0x5f,0x60]):
  r=initial(actor,char,air,mode,cycle=cycle,zone=zone)
  player.append(dict(label=f'item1-{actor}-{char}-{air}-{mode}-{cycle}-{zone}',**run(r,actor)))
 for actor,char,air,mode,seed in itertools.product([0,2],range(4),[False,True],[0,1,128],[0,0x12345678,0x80000000,0xffffffff]):
  r=initial(actor,char,air,mode,item=7,cycle=2 if air else 3,seed=seed,team=0 if actor<2 else 1)
  player.append(dict(label=f'rng-control-{actor}-{char}-{air}-{mode}-{seed}',**run(r,actor)))
 for actor,char,air,item in itertools.product(range(4),range(4),[False,True],[1,255]):
  r=initial(actor,char,air,item=item,cycle=2 if air else 3,entered=True,team=0 if actor<2 else 1)
  player.append(dict(label=f'entered-{actor}-{char}-{air}-{item}',**run(r,actor)))
 for actor,char,air,mode in itertools.product([2,3],range(4),[False,True],[0,1,128]):
  r=initial(actor,char,air,mode,item=1,cycle=2 if air else 3,team=1)
  player.append(dict(label=f'opponent-item1-{actor}-{char}-{air}-{mode}',**run(r,actor)))
 world=[]
 for home,actor,char,air,mode,item in itertools.product([False,True],[0,1],[0,3],[False,True],[0,1,128],[1,7]):
  r=initial(actor,char,air,mode,item=item,cycle=2 if air else 3,home=home)
  world.append(dict(label=f'world-{home}-{actor}-{char}-{air}-{mode}-{item}',**run(r,actor,True)))
 out=ROOT/'games/basketball/engine/testdata';files={}
 for name,rows in [('campaign-shot-player',player),('campaign-shot-world',world)]:
  data=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);p=out/(name+'.json.gz');p.write_bytes(data);files[p.name]=dict(records=len(rows),sha256=hashlib.sha256(data).hexdigest())
 meta=dict(scope='controlled complete E165..E168 player releases and D345..D348 world action phases; no natural-shot claim',romSHA256=hashlib.sha256(ROM).hexdigest(),normalEquippedMenuSource=dict(path=str(HOME.relative_to(ROOT)),frame=4345,fullTraceSHA256=hashlib.sha256(HOME_DATA).hexdigest(),frameSHA256=hashlib.sha256(BASE).hexdigest()),newYorkBaseSHA256=hashlib.sha256(NY).hexdigest(),sourceInstructionsUnchanged=True,outputs=files,reusedREA='ev_22e089841ea9f9c13f2d325c435946a52e8700cd2ee6a265d6caa7c02c223f6e')
 (out/'campaign-shot-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
