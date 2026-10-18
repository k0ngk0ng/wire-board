"""Unmodified original full 8BBD calls; failure inventory precedes implementation.

All initial states are declared controlled states, never natural-game claims.
"""
import base64,gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def initial(actor=0,action=22,entered=False,pants=255,shoes=255,character=0,
            mode=0,team=0,zone=0x40,depth=40,x=100,held=64,pressed=0,
            computer=False,holder=True,link=64,flag=77,seed=0x12345678,
            ending=False,moving=False):
 r=bytearray(BASE)
 r[0x6e4:0x6f0]=bytes([255]*12)
 for addr,value in {0x512+actor:action|(128 if entered else 0),0x554+actor:flag,
  0x51f+actor:128,0x46b+actor:0,0x4ec+actor:64,0x437+actor:4 if ending else 0,
  0x444+actor:0,0x451+actor:0,0x52c+actor:action&0x70 if ending else 255,
  0x5f7+actor:0,0x642+actor:64,0x642+(actor^1):link,
  0x4e0+actor:held,0x4e4+actor:pressed,0x58b:1,0x58d:mode,
  0x586+actor//2:team,0x3e3+actor:character,0x66a+actor:zone,
  0x539+actor:128 if computer else actor,0x53f:actor if holder else actor^1,
  0x6e5+3*character:pants,0x6e6+3*character:shoes}.items():r[addr]=value
 r[0x48:0x4c]=seed.to_bytes(4,'big')
 put(r,x*256+17,0x68+actor,0x75+actor,0x82+actor)
 put(r,depth*256+33,0x8f+actor,0x9c+actor)
 put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
 for lo,hi in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,lo+actor,hi+actor)
 if moving:put(r,300,0x478+actor,0x485+actor)
 return r

def run(label,**kwargs):
 r=initial(**kwargs);actor=kwargs.get('actor',0)
 m=SoundMemory(PRG,r);c=MPU(memory=m,pc=0x8bbd);c.x=actor;c.sp=255;c.stPushWord(0x6fff)
 snapshots={};path=[];rng=0;sounds=[]
 for count in range(20000):
  if c.pc==0x7000:break
  path.append(c.pc)
  if c.pc in [0x8bda,0x8be0,0xee52,0x8bea]:snapshots[c.pc]=bytes(m[:2048])
  if c.pc==0xf9c3:rng+=1
  if c.pc==0xfbcc:sounds.append(c.a)
  c.step()
 else:raise RuntimeError((label,hex(c.pc)))
 after=bytes(m[:2048]);ctx=context(r)['actions'];ctx['cachedZones']=list(r[0x66a:0x66e])
 row=dict(label=label,actor=actor,inputs=kwargs,before=decode_world_action_state(r),
  prepared=decode_world_action_state(snapshots[0x8bda]),
  afterAnimation=decode_world_action_state(snapshots[0x8be0]),
  beforeMotion=decode_world_action_state(snapshots[0x8bea]),after=decode_world_action_state(after),
  context=ctx,held=list(r[0x4e0:0x4e4]),pressed=list(r[0x4e4:0x4e8]),
  shoes=[r[0x6e6+3*r[0x3e3+i]] for i in range(4)],rngCalls=rng,sounds=sounds,
  initialRAM=base64.b64encode(r).decode(),changedRAM=[[i,after[i]] for i in range(2048) if after[i]!=r[i]],path=path)
 return row

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[]
 # Each distinct mode/identity/input/rider gate, all source aliases and phases.
 cases=[{},dict(held=0),dict(held=8),dict(held=72),dict(computer=True),dict(holder=False),
  dict(mode=128),dict(mode=1),dict(team=1),dict(link=128),dict(link=129),dict(link=144),
  dict(pressed=128),dict(pressed=64),dict(pressed=192),dict(ending=True,entered=True),
  dict(moving=True,entered=True)]
 for actor,action,entered,equip,case in itertools.product(range(4),[6,22,38,54,70],[False,True],[(255,255),(11,255),(15,255),(255,30),(11,30),(15,30)],cases):
  args=dict(actor=actor,action=action,entered=entered,pants=equip[0],shoes=equip[1],character=actor,
            zone=0x50 if equip[0]==15 else 0x40,x=180 if actor<2 else 100)
  args.update(case)
  if 'link' in case:args['link']=(case['link']&0xf0)|actor if case['link']!=129 else 128|(actor^1)
  rows.append(run('gates',**args))
 # Full ordinary fallback boundary matrix includes high-X-byte alias behavior.
 for actor,held,x,depth,flag in itertools.product(range(4),[64,72],[64,65,128,129,143,144,207,208,399,400,463,464],[24,25,57,58],[0,77,128]):
  rows.append(run('ordinary',actor=actor,character=(actor+1)%4,entered=True,held=held,x=x,depth=depth,flag=flag))
 for actor,pants,zone,depth,x in itertools.product(range(4),[11,15,10,20,255],[0x30,0x40,0x4f,0x50,0x5f],[24,25,57,58],[100,180]):
  rows.append(run('pants',actor=actor,character=(actor+2)%4,pants=pants,zone=zone,depth=depth,x=x,held=72,entered=True))
 for actor,pants,seed in itertools.product(range(4),[11,15,255],[0,0x02345678,0x04345678,0x06345678,0xffffffff]):
  rows.append(run('shoe-rng',actor=actor,character=(actor+1)%4,pants=pants,shoes=30,seed=seed,zone=0x40,depth=24,held=72))
 # Independent branch assertions make probe mistakes visible before Go exists.
 for row in rows:
  i=row['actor'];r=base64.b64decode(row['initialRAM']);enabled=r[0x58d]<128 and r[0x586+i//2]==0
  gate=r[0x4e0+i]&64 and r[0x539+i]<128 and r[0x53f]==i and r[0x642+(i^1)]!=(128|i)
  assert row['rngCalls']==int(bool(enabled and gate and row['shoes'][i]==30)),row['inputs']
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'landing-equipment.json.gz').write_bytes(packed)
 summary=dict(records=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),scope='declared controlled original full 8BBD calls; all instruction paths retained; no natural-game claim',stages=['8BDA before animation','8BE0 after animation/8B90','8BEA before EEFF','7000 complete return'],rngCalls=sum(r['rngCalls'] for r in rows),status='original probe passed')
 (out/'landing-equipment-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))
if __name__=='__main__':main()
