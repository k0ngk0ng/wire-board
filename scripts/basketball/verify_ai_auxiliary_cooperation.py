"""Original bank5 modes13/3/14, explicit bounded state matrices."""
import gzip,hashlib,itertools,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:16+131072]

def initial(actor,mode,entered=False):
 return dict(actor=actor,mode=mode|(128 if entered else 0),plan=14,action=130,heading=32,facing=128,destination=71,target=2,status=129,message=7,owners=[255,255],auxiliaryActions=[0,0],heights=[0,0],positions=[dict(x=(200+20*i)*256+17,depth=40*256+33,height=0) for i in range(6)],link=64,holder=6,playerActions=[128]*4)

def execute(row,capture=False):
 c=MPU();c.memory[0x8000:]=PRG[5*16384:6*16384]+PRG[-16384:];a=row['actor'];m=c.memory
 for base,key in [(0x672,'mode'),(0x676,'plan'),(0x512,'action'),(0x4ec,'heading'),(0x46b,'facing'),(0x67e,'destination'),(0x67a,'target'),(0x657,'status'),(0x6c6,'message')]:m[base+a]=row[key]
 for i in range(4):
  if i!=a:m[0x512+i]=row['playerActions'][i]
 for i in range(2):
  m[0x53d+i]=row['owners'][i];m[0x516+i]=row['auxiliaryActions'][i];h=row['heights'][i];m[0xad+i]=h&255;m[0xba+i]=(h>>8)&255;m[0xc7+i]=(h>>16)&255
 for i,p in enumerate(row['positions']):
  for value,addrs in [(p['x'],[0x68+i,0x75+i,0x82+i]),(p['depth'],[0x8f+i,0x9c+i])]:
   for j,addr in enumerate(addrs):m[addr]=(value>>(8*j))&255
 m[0x53f]=row['holder']&255;m[0x642+(a^1)]=row['link']
 before=bytes(m[:2048])
 c.pc=0xa05d if capture else {3:0xa187,13:0xa557,14:0xa599}[row['mode']&127];c.x=a;c.sp=255;c.stPushWord(0x6fff)
 for _ in range(2000):
  if c.pc==0x7000:break
  c.step()
 else:raise RuntimeError(hex(c.pc))
 if capture:
  from extract_world_inputs import decode_world_input_state
  from extract_world_preparation import context
  ctx=dict(world=context(before)['actions'],difficulty=before[0x58c],scoreDifference=before[0x696],cachedZones=list(before[0x66a:0x66e]),objectKinds=list(before[0x3d6:0x3e3]),hoopLinks=list(before[0x649:0x64f]),humanTactics=list(before[0x666:0x668]))
  return dict(actor=a,before=decode_world_input_state(before),after=decode_world_input_state(bytes(m[:2048])),context=ctx)
 out={key:m[base+a] for base,key in [(0x672,'mode'),(0x676,'plan'),(0x512,'action'),(0x4ec,'heading'),(0x46b,'facing'),(0x67e,'destination'),(0x67a,'target'),(0x657,'status'),(0x6c6,'message')]};out['owners']=list(m[0x53d:0x53f]);return dict(row,after=out)

def main():
 rows=[]
 for a,entered,owners,action,plan in itertools.product(range(4),[False,True],range(3),[0,32,160,78],[14,22,142]):
  r=initial(a,3,entered);r.update(owners=[[a,255],[255,a],[255,255]][owners],action=action,plan=plan);rows.append(execute(r))
 for a,entered,eligibility,dx,dz in itertools.product(range(4),[False,True],range(6),[-300,-9,-8,0,8,9,300],[-9,-8,-7,0,7,8,9]):
  r=initial(a,13,entered);r['positions'][a]=dict(x=(280+dx)*256+17,depth=((40+dz)&255)*256+33,height=0)
  if eligibility==1:r['owners']=[1,255]
  if eligibility==2:r['owners']=[1,2]
  if eligibility==3:r['auxiliaryActions']=[1,0]
  if eligibility==4:r['heights']=[256,0]
  if eligibility==5:r['heights']=[255,256]
  rows.append(execute(r))
 for a,entered,owners,link,holder,actions in itertools.product(range(4),[False,True],range(4),[64,128|4,128|5,128|3],[-1,0,1,2,3,4,5,6],range(4)):
  r=initial(a,14,entered);r['owners']=[[255,255],[a,255],[255,a],[a^1,a^2]][owners];r['link']=link;r['holder']=holder
  enemy=(a&2)^2;r['playerActions'][enemy]=74 if actions&1 else 0;r['playerActions'][enemy+1]=74 if actions&2 else 0
  rows.append(execute(r))
 for a,entered,destination,dx,dz in itertools.product(range(4),[False,True],[16,79,80],[-300,-256,-255,-16,0,16,255,256,300],[-9,-8,-7,0,7,8,9]):
  r=initial(a,14,entered);r['owners']=[a,255];r['destination']=destination;target=a^2;r['positions'][a]=dict(x=r['positions'][target]['x']+dx*256,depth=((40+dz)&255)*256+33,height=0);rows.append(execute(r))
 worlds=[]
 for a,entered,available in itertools.product(range(4),[False,True],[False,True]):
  r=initial(a,13,entered);r['positions'][a]=dict(r['positions'][4]);r['owners']=[255,255] if available else [0,1];worlds.append(execute(r,True))
 out=ROOT/'games/basketball/engine/testdata';wp=gzip.compress((json.dumps(worlds,separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-auxiliary-world-input.json.gz').write_bytes(wp)
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-auxiliary-cooperation.json.gz').write_bytes(packed)
 meta=dict(scope='isolated unmodified original handlers, not natural menu or complete world evidence',entries={'3':'bank5 A187','13':'bank5 A557','14':'bank5 A599'},records=len(rows),worldInputCalls=len(worlds),worldInputSHA256=hashlib.sha256(wp).hexdigest(),romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest());(out/'ai-auxiliary-cooperation-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
