"""Read-only counter prototype; inputs are external IO/ack plus call order."""
from pathlib import Path
import gzip,json
ROOT=Path(__file__).resolve().parents[2]
FRAME=896040;LINE=3420

class Video:
 def __init__(self,e):
  assert e['vcounter']==223 and e['line_cycle']==FRAME
  self.pos=-1;self.h=e['hcounter'];self.reload=e['reg10'];self.horizontal=bool(e['reg0']&16);self.vertical=bool(e['reg1']&32);self.display=bool(e['reg1']&64)
  self.hp=bool(e['hint_pending']);self.vp=bool(e['vint_pending']);self.edge=False;self.signals=[]
 def start(self):
  assert self.pos==-1;self.pos=0;self.edge=False
  if self.h==0:self.hp=True;self.signals.append(('h',0))
 def vint(self):
  assert self.pos==0 and not self.edge;self.edge=True;self.vp=True;self.signals.append(('v',788))
 def move(self,target):
  if self.pos==-1:self.start()
  if target>0 and not self.edge:self.vint()
  while self.pos<target:
   self.pos+=1
   if self.pos==37:self.h=self.reload
   elif 38<=self.pos<262:
    if self.h==0:
     self.h=self.reload;self.hp=True;self.signals.append(('h',self.pos*LINE))
    else:self.h-=1
 def vc(self):return 223 if self.pos in[-1,262]else 224+self.pos if self.pos<38 else self.pos-38
 def blank(self):return 0<=self.pos<37
 def state(self):
  return {'hcounter':self.h,'reg10':self.reload,'hint_pending':16 if self.hp else 0,'vint_pending':32 if self.vp else 0,'status_irq':(8 if self.blank()else 0)|(128 if self.vp else 0),'vcounter':self.vc()}
 def ack(self):
  if self.vertical and self.vp:self.vp=False
  else:self.hp=False
 def register(self,r,v):
  if r==0:
   assert v&2==0,'HVC latch unsupported';self.horizontal=bool(v&16)
  elif r==1:
   assert v&4 and not v&8,'not NTSC224 mode5';self.vertical=bool(v&32);self.display=bool(v&64)
  elif r==10:self.reload=v
  elif r==12:assert v&1 and not v&6,'not H40 noninterlaced'
 def write(self,width,value):
  assert width in[2,4]
  for word in ([value>>16,value&65535]if width==4 else[value]):
   if word&0xc000==0x8000:self.register((word>>8)&31,word&255)
 def status(self,e):
  bits=self.state()['status_irq']
  if not self.display:bits|=8
  if self.pos==0 and e['cpu_cycle']+e['instruction_cycles']>=788 and not e['z80_irq']:bits|=128
  return bits
 def hvc(self,e):
  v=self.vc()
  if e['cpu_cycle']-self.pos*LINE>=LINE:v=(v+1)%262
  if v>234:v-=262
  return v&255

def run():
 path=ROOT/'games/yuyuhakusho/verification/world-video/native/events.json.gz';payload=json.loads(gzip.decompress(path.read_bytes()));events=[dict(zip(payload['fields'],row))for row in payload['events']]
 cases=[];model=None;last_frame=None;checks=0;reads=0
 for e in events:
  if e['kind']==0:
   if last_frame is None or e['frame']!=last_frame+1:model=Video(e)
   else:
    assert model.pos==262
    model.pos=-1
    assert model.state()=={k:e[k]for k in model.state()}
   last_frame=e['frame'];model.start();continue
  if e['kind']==9:
   model.move(262)
   assert model.state()=={k:e[k]for k in model.state()},('end',e,model.state())
   cases.append({'frame':e['frame'],'signals':model.signals});model.signals=[];continue
  target=e['line_cycle']//LINE;assert 0<=target<262
  model.move(target)
  # A first-slice execution hook cannot begin at or beyond its788 target.
  # Memory IO can overrun, so it must not independently trigger this edge.
  if target==0 and not model.edge and e['kind']in[1,4,5]and e['cpu_cycle']>=788:model.vint()
  if e['kind']==4:model.ack()
  wanted={k:e[k]for k in model.state()};actual=model.state()
  assert actual==wanted,('state',e,actual,wanted)
  assert model.horizontal==bool(e['reg0']&16)and model.vertical==bool(e['reg1']&32)and model.display==bool(e['reg1']&64)
  checks+=1
  if e['kind']==2:model.write(e['width'],e['value'])
  elif e['kind']==3:
   actual=model.status(e);native=e['value']&0x88 if e['width']!=1 or e['address']&1 else 0
   assert actual==native,('status',e,actual,native);reads+=1
  elif e['kind']==6 and not(e['address']&1):
   native=e['value']if e['width']==1 else e['value']>>8
   assert model.hvc(e)==native,('hvc',e,model.hvc(e),native);reads+=1
  elif e['kind']==8:raise AssertionError('Z80 VDP status outside profile')
 return {'pass':True,'checks':checks,'reads':reads,'frames':cases,'scope':'Given original ordered IO and ack; no CPU acceptance or world work scheduling proof'}

if __name__=='__main__':
 result=run();out=ROOT/'games/yuyuhakusho/verification/world-video/prototype-first.json';out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'pass':True,'checks':result['checks'],'reads':result['reads']}))
