"""Research typed dual-player semantic model; no CPU/ROM interpretation.

Costs are static sums of the pinned firmware's declared semantic blocks. Native
PC and future timestamps are never inputs to this model.
"""
from pathlib import Path
import argparse,gzip,json
from reference import ROOT,sha
DELTA=[0,1,2,4,8,16,32,64,-128,-1,-2,-4,-8,-16,-32,-64]
class Dual:
 def __init__(self,samples):
  self.time=0;self.slot=1;self.done=False;self.events=[]
  self.voices=[{'payload':bytes.fromhex(s['encoded_hex'])if s else b'','id':s['id']if s else None,'consumed':0,'fetched':0,'produced':0,'enabled':False,'active':bool(s),'accumulator':128}for s in samples]
 def cost(self,t):self.time+=t*15
 def emit(self,kind,value=None,**other):self.events.append({'cycle':self.time,'kind':kind,**({'value':value}if value is not None else{}),**other})
 def step(self):
  assert not self.done;self.events=[];self.cost(45);output=[]
  for index,v in enumerate(self.voices):
   enabled=v['enabled']
   if enabled:
    encoded=v['payload'][v['consumed']];high=(v['accumulator']+DELTA[encoded>>4])&255;low=(high+DELTA[encoded&15])&255;v['accumulator']=low;v['consumed']+=1
    if index!=self.slot and (v['consumed']&255)==(v['produced']&255):v['enabled']=False
   else:high=low=128
   output.append((high,low));base=(137 if index==0 else 168)if self.slot==0 else(171 if index==0 else 134)
   self.cost(base+(0 if enabled else 5))
  self.cost(52);self.emit('dac',max(0,min(255,output[0][0]+output[1][0]-128)))
  current=self.voices[self.slot];other=self.voices[1-self.slot];self.cost(41)
  if current['fetched']<len(current['payload']):
   self.cost(28);gap=(current['consumed']-current['produced'])&255
   if gap==0 or gap>=192:
    self.cost(44+143)
    for _ in range(4):
     self.cost(16);self.emit('bank',slot=self.slot,offset=current['fetched']);self.cost(3);current['fetched']+=1
    current['produced']=min(current['fetched'],len(current['payload']))
    self.cost(10+(56 if current['fetched']>len(current['payload']) else 57))
   else:
    current['enabled']=True;self.cost(17+27+60+214)
  else:
   self.cost(109)
   if not current['active']:self.cost(71+27+139)
   else:
    self.cost(28)
    if(current['consumed']&255)!=(current['produced']&255):self.cost(57+139)
    else:
     current['active']=False;current['enabled']=False;self.cost(43+27)
     if not other['active']:
      self.cost(106);self.emit('pan',0);self.cost(77);self.emit('enable',0);self.cost(49);self.done=True;return self.events
     self.cost(139)
  self.cost(52);self.emit('dac',max(0,min(255,output[0][1]+output[1][1]-128)));self.cost(38);self.slot=1-self.slot;return self.events
 def run(self):
  result=[]
  for _ in range(40000):
   result.extend(self.step())
   if self.done:return result
  raise AssertionError('player bound exceeded')
class Single(Dual):
 def __init__(self,samples):
  self.time=0;self.done=False;self.events=[];self.sample=samples[0];self.payload=bytes.fromhex(self.sample['encoded_hex']);self.consumed=0;self.fetched=128;self.accumulator=128;self.dummy=0;self.tail=False;rate=self.sample['rate'];self.delay=13*(rate or 256)-5
 def step(self):
  assert not self.done;self.events=[];encoded=self.payload[self.consumed];high=(self.accumulator+DELTA[encoded>>4])&255;low=(high+DELTA[encoded&15])&255;self.accumulator=low;self.consumed+=1
  self.cost(110);self.emit('dac',high)
  if self.tail:
   self.cost(183+self.delay);self.dummy+=9;self.emit('dac',low);self.cost(51+self.delay);self.dummy+=1
   if self.consumed==len(self.payload):
    self.cost(106);self.emit('pan',0);self.cost(77);self.emit('enable',0);self.cost(49);self.done=True
   else:self.cost(27)
  else:
   gap=(self.consumed-self.fetched)&255
   if gap==0 or gap&128:
    self.cost(79+self.delay+10)
    for _ in range(2):
     self.cost(16);self.emit('bank',slot=0,offset=self.fetched);self.cost(3);self.fetched+=1
    self.cost(11+51)
   else:
    self.cost(74+self.delay);self.emit('dummy',slot=0,offset=self.dummy);self.cost(3+40+8+10+51);self.dummy+=4
   self.emit('dac',low);self.cost(78+self.delay)
   if self.fetched>=len(self.payload):
    self.tail=True
    if self.fetched>len(self.payload):self.dummy+=1
  return self.events
 def run(self):
  result=[]
  for _ in range(40000):
   result.extend(self.step())
   if self.done:return result
  raise AssertionError('single player bound exceeded')

def original(body):
 source=[]
 for e in body['events']:
  cycle=e['frame']*896040+e['cycles_before']
  if e['kind']==1:source.append({'cycle':cycle,'kind':'dac','value':e['data']})
  elif e['kind']==2:source.append({'cycle':cycle,'kind':'pan'if e['address']==0x1b6 else'enable','value':e['data']})
  elif e['kind']==3:
   address=e['bank']+(e['address']&0x7fff)
   if body['case']['mode']=='single' and e['pc']==0x149:
    source.append({'cycle':cycle,'kind':'dummy','slot':0,'offset':(e['address']-body['samples'][0]['source']['z80_header'])&65535});continue
   choices=[(i,address-s['source']['sample_address'])for i,s in enumerate(body['samples'])if s and s['source']['sample_address']<=address<s['source']['sample_address']+s['length']+4]
   assert len(choices)==1,(address,choices);slot,offset=choices[0];source.append({'cycle':cycle,'kind':'bank','slot':slot,'offset':offset})
 return source

def main():
 p=argparse.ArgumentParser();p.add_argument('--capture',required=True,type=Path);p.add_argument('--out',required=True,type=Path);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)
 report=[]
 for path in sorted(capture.glob('*.json.gz')):
  body=json.loads(gzip.decompress(path.read_bytes()));model=(Dual if body['case']['mode']=='dual' else Single)(body['samples']);actual=model.run();expected=original(body);mismatch=next(({'index':i,'actual':x,'expected':y}for i,(x,y)in enumerate(zip(actual,expected))if x!=y),None)
  row={'name':body['case']['name'],'source_sha256':sha(path.read_bytes()),'expected_events':len(expected),'actual_events':len(actual),'first_mismatch':mismatch,'pass':mismatch is None and len(expected)==len(actual),'final_state':([{k:v for k,v in s.items()if k!='payload'}for s in model.voices] if hasattr(model,'voices') else {k:v for k,v in vars(model).items()if k not in ['payload','sample','events']}),'end_cycle':model.time};report.append(row);print(json.dumps(row),flush=True)
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps({'pass':all(r['pass']for r in report),'cases':report,'scope':'Typed single/dual player with static semantic block costs; initialized private state and no external bus hold','model_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n')
 if not all(r['pass']for r in report):raise SystemExit(1)
if __name__=='__main__':main()
