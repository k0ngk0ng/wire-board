"""Unpatched bank5 personal modes10/11/30, direct functions and real dispatcher."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,BASE
from verify_pass_floor_original import MPU,put
PRG=ROM[16:16+131072]
PROFILES=json.loads((ROOT/'games/basketball/engine/testdata/ai-pursuit-profiles.json').read_text())
ENTRY={10:0xa461,11:0xa476,28:0xa95f,30:0xa208}
class Mem(list):
 def __init__(self,ram):
  super().__init__([0]*65536);self[:2048]=ram;self[0x8000:0xc000]=PRG[5*16384:6*16384];self[0xc000:]=PRG[-16384:];self.writes=[]
 def __setitem__(self,k,v):
  super().__setitem__(k,v)
  if isinstance(k,int) and hasattr(self,'writes'):self.writes.append(k)
def signed(v):return v if v<128 else v-256
def control(r,a):
 return dict(mode=r[0x672+a]&127,entered=bool(r[0x672+a]&128),plan=r[0x676+a],counter=signed(r[0x66e+a]),planParameter=r[0x682+a],trackedObject=signed(r[0x67a+a]),destination=r[0x67e+a])
def pos(r,a):return dict(x=r[0x68+a]+256*r[0x75+a]+65536*r[0x82+a],depth=r[0x8f+a]+256*r[0x9c+a],height=r[0xa9+a]+256*r[0xb6+a]+65536*r[0xc3+a])
def view(r,a):
 return dict(Actor=a,Action=r[0x512+a]&127,ActionEntered=bool(r[0x512+a]&128),Heading=8 if r[0x4ec+a]==255 else r[0x4ec+a]//32,FacingLeft=bool(r[0x46b+a]&128),Position=pos(r,a),HasBall=r[0x53f]==a,Players=[pos(r,i) for i in range(4)],BallPosition=pos(r,6),BallHolder=r[0x53f],Random=int.from_bytes(bytes(r[0x48:0x4c]),'big'),BallAction=r[0x518]&127,BallFacingLeft=bool(r[0x471]&128),BallLanding=dict(x=(r[0x699]+256*r[0x69a])*256,depth=r[0x698]*256),PursuitOptions=next(p['options'] for p in PROFILES if p['team']==r[0x586+a//2] and p['character']==r[0x3e3+a]),CachedZones=list(r[0x66a:0x66e]),Team=r[0x586+a//2],TeamHasHuman=False,Airborne=bool(r[0x5f7+a]&128),TeammateLink=r[0x642+(a^1)])
def origin(actor,mode,entered,action,actionEntered,ball=0,counter=0,distance=0,height=0):
 r=bytearray(BASE)
 for a,v in {0x672+actor:mode|(128 if entered else 0),0x676+actor:8,0x66e+actor:counter,0x682+actor:3,0x67a+actor:2,0x67e+actor:32,0x512+actor:action|(128 if actionEntered else 0),0x4ec+actor:64,0x46b+actor:128,0x518:ball|128,0x471:128,0x53f:(actor^1) if ball in (1,2) else 6,0x5f7+actor:0,0x58b:1}.items():r[a]=v
 r[0x539:0x53d]=bytes([128]*4);r[0x586:0x588]=bytes([0,1]);r[0x3e3:0x3e7]=bytes([0,1,2,3]);r[0x66a:0x66e]=bytes([0x32]*4);r[0x657:0x65b]=bytes([51,52,53,54]);r[0x48:0x4c]=(0x12345678).to_bytes(4,'big')
 for i in range(7):
  put(r,(200+(distance if i!=actor else 0))*256+11,0x68+i,0x75+i,0x82+i);put(r,40*256+17,0x8f+i,0x9c+i);put(r,height*256 if i==6 else 0,0xa9+i,0xb6+i,0xc3+i)
 put(r,200+distance,0x699,0x69a);r[0x698]=40
 return r

def run(r,actor,direct,returnRam=False):
 mode=r[0x672+actor]&127;m=Mem(r);cpu=MPU(memory=m,pc=ENTRY[mode] if direct else 0xa05d);cpu.x=actor;cpu.p=0x20;cpu.sp=255;cpu.stPushWord(0x6fff);ready=False
 for steps in range(3000):
  if cpu.pc==0xa21a:ready=bool(cpu.p&1)
  if cpu.pc==0x7000:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 v=view(m,actor)
 expected=dict(Control=control(m,actor),Action=v['Action'],ActionEntered=v['ActionEntered'],Heading=v['Heading'],FacingLeft=v['FacingLeft'],BindHoop=-1,Random=v['Random'],BindBall=0x53f in m.writes,ReactionReady=ready,ClearCommandStatus=0x657+actor in m.writes)
 if 0x518 in m.writes:expected['Ball']=dict(Change=True,Action=m[0x518]&127)
 assert [pos(m,i) for i in range(7)]==[pos(r,i) for i in range(7)]
 assert all(m[0x657+i]==r[0x657+i] for i in range(4) if i!=actor)
 record=dict(direct=direct,before=control(r,actor),view=view(r,actor),after=expected,statusBefore=list(r[0x657:0x65b]),statusAfter=list(m[0x657:0x65b]))
 return (record,bytes(m[:2048])) if returnRam else record
def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[]
 for actor,mode,entered,action,ae,direct in itertools.product(range(4),[10,11],[False,True],[0,2,3,4,6,15,16,17,18,19,20,23,61,62,72,105],[False,True],[False,True]):
  rows.append(run(origin(actor,mode,entered,action,ae),actor,direct))
 for actor,entered,ball,counter,distance,height in itertools.product(range(4),[False,True],[0,1,2,3,4,5,6,7,10,16,23],[0,1,2,4,255],[0,12,60],[0,32,64,256]):
  rows.append(run(origin(actor,30,entered,2,True,ball,counter,distance,height),actor,False))
 chains=[]
 for actor in range(4):
  r=origin(actor,28,True,0,True);r[0x53f]=actor;r[0x642+(actor^1)]=0x80|actor
  first,r=run(r,actor,False,True);second,r=run(r,actor,False,True);chains.append([first,second])
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(dict(calls=rows,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-command-pass.json.gz').write_bytes(packed)
 meta=dict(scope='controlled original functions plus A05D dispatcher; not natural match or human input chain',records=len(rows),chains=len(chains),romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),entries=ENTRY)
 (out/'ai-command-pass-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
