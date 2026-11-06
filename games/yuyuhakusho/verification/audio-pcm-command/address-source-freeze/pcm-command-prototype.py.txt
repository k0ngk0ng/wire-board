"""Independent typed command semantics with static atomic latency partitions.

The model receives typed commands/windows and immutable audio resources only.
Original PC and future native times are used only by the external comparator.
"""
from pathlib import Path
import argparse,gzip,json
from reference import ROOT,sha
DELTA=[0,1,2,4,8,16,32,64,-128,-1,-2,-4,-8,-16,-32,-64];F=896040
class End(Exception):pass
class Restart(Exception):pass
class Clock:
 def __init__(self,device,inputs,limit):self.d=device;self.time=0;self.inputs=inputs;self.limit=limit;self.cursor=0;self.events=[];self.holds=[]
 def op(self,ticks,fn=None):
  if self.time>=self.limit:raise End()
  while self.cursor<len(self.inputs)and self.time>=self.inputs[self.cursor]['begin']:
   action=self.inputs[self.cursor];self.cursor+=1;grant=self.time;self.time=((action['end']+14)//15)*15;self.holds.append({'grant':grant,'resume':self.time,'input':action})
   if action['kind']=='reset':
    self.events.extend([{'cycle':action['begin'],'kind':'reset','start':action['begin']},{'cycle':action['end'],'kind':'reset','start':action['end']}]);raise Restart()
   self.d.publish(action)
  if self.time>=self.limit:raise End()
  start=self.time;self.time+=ticks*15
  value=fn()if fn else None
  return start,value
 def delay(self,*ticks):
  for t in ticks:self.op(t)
 def read(self,ticks,fn):return self.op(ticks,fn)[1]
 def emit(self,ticks,kind,value=None,resource=None,offset=None,register=None,port=None):
  start,_=self.op(ticks);event={'cycle':self.time,'kind':kind,'start':start}
  if value is not None:event['value']=value()if callable(value)else value
  if resource is not None:event['resource']=resource['id']
  if offset is not None:event['offset']=offset
  if register is not None:event['register']=register
  if port is not None:event['port']=port
  self.events.append(event)
 def address(self,ticks,port,value):
  if self.d.addresses:self.emit(ticks,'address',value,port=port)
  else:self.delay(ticks)
 def bank(self,ticks,kind,source,offset):
  self.emit(ticks,kind,resource=source,offset=offset);frame=self.events[-1]['start']//F;cycle=self.time-frame*F;dma=next((d for d in self.d.dma if d['frame']==frame),{});at=max(cycle,dma.get('end',0))if dma.get('mode',0)<2 else cycle;self.events[-1].update(resume=frame*F+at+45,m68k_delay=((at%7+72)//7)*7);self.time=frame*F+at+45
class Device:
 def __init__(self,body):
  self.addresses=any(e['kind']==11 for e in body['events']);self.chip=any(e['kind']==9 for e in body['events']);self.dma=body['case'].get('dma',[]);self.resources=body['resources'];self.clock=Clock(self,body['case']['inputs'],body['case']['frames']*F);self.rate_request=0;self.start_marker=0;self.mode_flags=0;self.latest_flags=0;self.ordinary_pan=0;self.music_pan=0;self.restore_pan=0;self.global_single=None;self.busy=0;self.single_state=None;self.slots=[self.empty_slot(),self.empty_slot()]
 def publish(self,action):
  if action['kind']=='hold':return
  if action['kind']=='restore-pan':self.restore_pan=action['pan'];return
  s=self.resources[str(action['code'])+action['protocol']]
  if action['protocol']=='music':
   assert not self.start_marker&128 and s['flags']&192>=self.mode_flags&192
   self.start_marker=1;self.music_pan=action['pan'];self.global_single=s;self.rate_request=s['rate'];self.latest_flags=s['flags']
  elif action['target']=='single':self.start_marker=128;self.ordinary_pan=s['ordinary_routing'];self.global_single=s;self.rate_request=s['rate'];self.latest_flags=s['flags']
  else:
   if not self.mode_flags&1:self.rate_request=128;self.start_marker=128
   i=0 if action['target']=='a'else 1;self.slots[i]['busy']=128;self.slots[i]['flags']=s['flags'];self.slots[i]['request']=s;self.latest_flags=s['flags'];self.ordinary_pan=192
 def busyset(self,ticks,value):self.clock.op(ticks,lambda:setattr(self,'busy',value))
 def end(self):
  c=self.clock;c.delay(7);self.busyset(13,128);c.delay(20,20,10);c.address(10,2,0xb6);c.delay(6);pan=c.read(13,lambda:self.restore_pan);c.emit(7,'pan',pan);c.delay(20,20,10);c.address(10,0,0x2b);c.delay(6,4);c.emit(7,'enable',0);c.op(13,lambda:setattr(self,'mode_flags',0));c.op(13,lambda:setattr(self,'start_marker',0));self.busyset(13,0);c.delay(10)
 def check(self,s):
  c=self.clock;c.delay(4,4,10)
  if s['remaining']&0x8000:c.delay(4,10);s['dummy']+=1
  else:
   c.delay(4,10)
   if s['remaining']:
    rate=c.read(13,lambda:self.rate_request);c.delay(4,10)
    if rate:c.delay(10);return 'request',rate
    return 'normal',0
  c.delay(7,4,4,4,4,4);return 'tail',0
 def single_start(self,rate):
  c=self.clock;c.op(13,lambda:None);c.delay(4);c.op(13,lambda:setattr(self,'rate_request',0));source=c.read(20,lambda:self.global_single);flags=c.read(13,lambda:self.latest_flags);c.op(13,lambda:setattr(self,'mode_flags',flags));marker=c.read(13,lambda:self.start_marker);c.delay(4,10)
  pan=c.read(13,lambda:self.ordinary_pan if marker&128 else self.music_pan)
  if not marker&128:c.delay(12)
  c.delay(4);c.read(13,lambda:self.global_single);c.delay(4,13,7);c.read(13,lambda:self.global_single)
  for i in range(8):c.delay(13,4,13 if i<7 else 8)
  c.delay(4);self.busyset(13,0);c.bank(19,'metadata',source,0);c.bank(19,'metadata',source,1);c.delay(10,10)
  fetched=0
  while fetched<128:
   c.read(13,lambda:0);c.delay(4,10)
   for _ in range(4):c.bank(16,'bank',source,fetched);fetched+=1
   c.delay(10)
  c.bank(19,'metadata',source,2);c.bank(19,'metadata',source,3);length=len(bytes.fromhex(source['encoded_hex']));c.delay(7,10)
  if length&255<128:c.delay(4)
  c.delay(4,4,7);self.busyset(13,128);c.delay(20,20,4,10);c.address(10,2,0xb6);c.delay(6);c.emit(7,'pan',pan)
  c.delay(20,20,10);c.address(10,0,0x28);c.delay(6);self.keyoff();c.delay(20,20,6);c.address(10,0,0x2b);c.delay(6);c.emit(10,'enable',128)
  c.delay(10,10);c.read(13,lambda:rate);c.delay(4,4,10,11,10,16,7,4,4);self.busyset(13,0);c.delay(10)
  s={'source':source,'data':bytes.fromhex(source['encoded_hex']),'consumed':0,'fetched':128,'acc':128,'rate':rate,'remaining':(length-128)&65535,'dummy':0};self.single_state=s;mode,newrate=self.check(s)
  return self.single(s,mode,newrate)
 def decode_start(self,s):
  c=self.clock;c.delay(16,4,4,4,13,4);encoded=c.read(7,lambda:s['data'][s['consumed']]);s['consumed']+=1;c.delay(4,4);self.busyset(7,64);c.delay(4);c.op(7,lambda:s.update(acc=(s['acc']+DELTA[encoded>>4])&255));c.delay(4,4);c.address(7,0,0x2a);c.delay(4);c.emit(13,'dac',lambda:s['acc']);self.busyset(7,0);c.delay(4,4);self.wait_rate(s['rate']);c.delay(4,4);c.op(7,lambda:s.update(acc=(s['acc']+DELTA[encoded&15])&255));c.delay(4);return encoded
 def wait_rate(self,rate):
  n=rate or 256
  for i in range(n):self.clock.op(13 if i<n-1 else 8)
 def low(self,s):
  c=self.clock;c.delay(4);self.busyset(7,64);c.delay(4,4,4,4);c.address(7,0,0x2a);c.delay(4);c.emit(13,'dac',lambda:s['acc']);self.busyset(7,0)
 def single(self,s,mode,rate):
  c=self.clock
  while True:
   if mode=='request':return rate
   self.decode_start(s)
   if mode=='tail':
    c.delay(4,4)
    for _ in range(9):c.delay(10);s['dummy']+=1
    self.low(s);c.delay(4);self.wait_rate(s['rate']);c.delay(4,4,4,4,10);s['dummy']+=1;c.delay(4,10)
    if s['consumed']==len(s['data']):self.end();return 0
    rate=c.read(13,lambda:self.rate_request);c.delay(4,10)
    if rate:c.delay(10);return rate
    continue
   c.delay(4);gap=(s['consumed']-s['fetched'])&255
   if gap==0:
    c.delay(12);c.read(13,lambda:0);c.delay(4,12);copy=True
   else:
    c.delay(7,10)
    if gap&128:c.read(13,lambda:0);c.delay(4,7);copy=True
    else:c.bank(19,'dummy',s['source'],s['dummy']);c.delay(10,4,4,10,10,10,10);s['dummy']+=4;copy=False
   if copy:
    c.delay(10)
    for _ in range(2):c.bank(16,'bank',s['source'],s['fetched']);s['fetched']+=1;s['remaining']=(s['remaining']-1)&65535
    c.delay(11)
   self.low(s);c.delay(4);self.wait_rate(s['rate']);c.delay(4,4);mode,rate=self.check(s)
 def empty_slot(self):return {'busy':0,'flags':0,'control':0,'enabled':False,'source':None,'data':b'','acc':128,'consumed':0,'fetched':0,'produced':0,'remaining':0,'request':None}
 def dual_start(self):
  c=self.clock;c.delay(4);c.op(13,lambda:setattr(self,'rate_request',0));flags=c.read(13,lambda:self.latest_flags);c.op(13,lambda:setattr(self,'mode_flags',flags));c.delay(10);c.address(10,2,0xb6);c.delay(4);c.emit(10,'pan',192);c.delay(20,20,10);c.address(10,0,0x28);c.delay(4);self.keyoff();c.delay(4,20,20);c.address(10,0,0x2b);c.delay(4);c.emit(10,'enable',128)
  c.delay(4,10,16,16,10,16,16,7)
  for v in self.slots:c.op(13,lambda v=v:v.update(acc=128))
  c.delay(4)
  for v in self.slots:c.op(13,lambda v=v:v.update(enabled=False,consumed=0))
  for v in self.slots:c.op(13,lambda v=v:v.update(control=0,produced=0,fetched=0))
  self.busyset(13,0);c.delay(10,10,10,4,10);return self.dual()
 def dual_decode(self,index,phase):
  c=self.clock;v=self.slots[index];enabled=c.read(10,lambda:v['enabled']);c.delay(7 if enabled else 12)
  if enabled:
   c.delay(10,4,4);encoded=c.read(7,lambda:v['data'][v['consumed']]);c.delay(7 if index==0 else 4);hi=c.read(7,lambda:(v['acc']+DELTA[encoded>>4])&255);c.delay(4,15,4);lo=c.read(7,lambda:(hi+DELTA[encoded&15])&255);c.delay(4,15,4);c.op(20,lambda:v.update(acc=lo,consumed=v['consumed']+1))
   if index!=phase:
    produced=c.read(13,lambda:v['produced']);c.delay(4);same=(produced&255)==(v['consumed']&255);c.delay(12 if same else 7)
    if same:c.op(13,lambda:v.update(enabled=False))
    else:c.delay(4,4,10)
   else:c.delay(4,4)
   return hi,lo
  if index==phase:
   c.delay(10,4,4,7,15,15)
   if phase==0:c.delay(4,4,7,20,20,10)
   else:c.delay(4,4,4,20,20,10)
  else:
   c.delay(10,7,15,15)
   if index==0:c.delay(7,10,20,20,20,20,10)
   else:c.delay(4,10,20,20,20,20,10)
  return 128,128
 def dual_output(self,value):
  c=self.clock;c.delay(4);self.busyset(7,64);c.delay(19,4);c.address(7,0,0x2a);c.delay(4);c.emit(7,'dac',value)
 def dual_transfer(self,v):
  c=self.clock;c.delay(4);c.op(13,lambda:v.update(enabled=False));c.op(13,lambda:v.update(consumed=0));c.op(13,lambda:v.update(produced=0));c.delay(7);c.op(13,lambda:v.update(acc=128));c.delay(7);c.op(13,lambda:v.update(control=1));self.busyset(13,1);flags=c.read(13,lambda:self.latest_flags);c.op(13,lambda:setattr(self,'mode_flags',flags));c.delay(10);source=c.read(10,lambda:v['request']);c.delay(10,10,10,11,11,11);v.update(source=source,data=bytes.fromhex(source['encoded_hex']),fetched=0,remaining=len(bytes.fromhex(source['encoded_hex'])));c.delay(4);self.busyset(13,0);c.delay(4,20,20,20,20,20,10)
 def dual_footer(self):self.clock.delay(4,4,4,7,10,20,20,20,20,20,10)
 def dual_pending(self,v):
  c=self.clock;busy=c.read(13,lambda:v['busy']);c.delay(4,10)
  if busy&128:c.delay(7);c.op(13,lambda:v.update(busy=64));c.op(13,lambda:v.update(control=64))
  else:c.delay(4,7,10,12)
  c.delay(4,20,20,20,20,20,20,20,20,20,20,10)
 def dual_refill(self,phase):
  c=self.clock;v=self.slots[phase];other=self.slots[1-phase];control=c.read(10,lambda:v['control']);c.delay(10)
  if control&1:
   c.delay(10,4,4,10);gap=(v['consumed']-v['produced'])&255
   if gap==0:c.read(13,lambda:0);c.delay(4,10,7,10);copy=True
   else:
    c.delay(7,10)
    if gap<192:c.delay(7);c.op(13,lambda:v.update(enabled=True));c.delay(7);copy=False
    else:c.read(13,lambda:0);c.delay(4,10);copy=True
   if not copy:self.dual_pending(v);return True
   c.delay(10,10,8,7,4)
   for i in range(8):c.delay(7);c.delay(4)if i<7 else None
   c.delay(10,10)
   for _ in range(4):c.bank(16,'bank',v['source'],v['fetched']);v['fetched']+=1;v['remaining']=(v['remaining']-1)&65535
   c.delay(10)
   if not v['remaining']:
    c.delay(4);c.op(13,lambda:v.update(produced=v['fetched']));c.delay(4);c.op(13,lambda:v.update(control=0));c.delay(7,4,12)
   else:
    c.delay(4,4,10)
    if v['remaining']&32768:c.delay(4,4);c.op(13,lambda:v.update(produced=len(v['data'])));c.delay(4);c.op(13,lambda:v.update(control=0))
    else:c.delay(11,11,4);c.op(13,lambda:v.update(produced=v['fetched']))
   return True
  c.delay(10)
  if control&64:self.dual_transfer(v);return True
  c.delay(4,4,4,10,20,20);busy=c.read(13,lambda:v['busy']);c.delay(4,10)
  if busy&128:
   c.delay(7);c.op(13,lambda:v.update(busy=64));c.op(13,lambda:v.update(control=64));c.delay(4,20,20,20,20,20,20,20,20,20,20,10);return True
  c.delay(10)
  if busy:
   c.delay(10,4,4,10)
   if(v['consumed']&255)==(v['produced']&255):
    c.delay(4);c.op(13,lambda:v.update(enabled=False));c.op(13,lambda:v.update(busy=0));c.op(13,lambda:v.update(flags=0))
   else:c.delay(7,20,20,10);self.dual_footer();return True
  else:c.delay(4,7,10,20,20,10)
  otherbusy=c.read(13,lambda:other['busy']);c.delay(4,10)
  if not otherbusy:self.end();return False
  self.dual_footer();return True
 def dual(self):
  c=self.clock;phase=0
  while True:
   c.delay(7,14,14,10);values=[self.dual_decode(i,phase)for i in range(2)];mix=lambda half:max(0,min(255,values[0][half]+values[1][half]-128));self.dual_output(mix(0));self.busyset(7,0);c.delay(4,10)
   if not self.dual_refill(phase):return 0
   self.dual_output(mix(1));self.busyset(7,0);c.delay(4);rate=c.read(13,lambda:self.rate_request);c.delay(4,10)
   if rate:
    c.delay(4,4)
    for v in self.slots:c.op(13,lambda v=v:v.update(busy=0))
    for v in self.slots:c.op(13,lambda v=v:v.update(flags=0))
    c.delay(4,10);return rate
   phase=1-phase
 def keyoff(self):
  if self.chip:self.clock.emit(10,'ym',6,register=0x28)
  else:self.clock.delay(10)
 def boot(self):
  c=self.clock;c.delay(4,4,4,10,7);self.busyset(13,1);c.delay(20,20,10);c.address(10,2,0xb6);c.delay(6);c.emit(10,'pan',0);c.delay(20,20,10);c.address(10,0,0x28);c.delay(6);self.keyoff();c.delay(20,20,6);c.address(10,0,0x2b);c.delay(6,4);c.emit(7,'enable',0)
  c.delay(13);c.op(13,lambda:setattr(self,'rate_request',0));c.op(13,lambda:setattr(self,'start_marker',0));c.op(13,lambda:setattr(self,'mode_flags',0));c.op(13,lambda:setattr(self,'ordinary_pan',0));c.op(13,lambda:setattr(self,'music_pan',0));c.op(13,lambda:setattr(self,'restore_pan',0))
  for v in self.slots:c.op(13,lambda v=v:v.update(busy=0))
  for v in self.slots:c.op(13,lambda v=v:v.update(flags=0))
  self.busyset(13,0)
 def run(self):
  c=self.clock;rate=0
  need_boot=False
  while True:
   try:
    if need_boot:self.boot();need_boot=False
    while True:
     if not rate:
      rate=c.read(13,lambda:self.rate_request);c.delay(4,10)
      if not rate:continue
     self.busyset(13,rate);c.delay(4,10)
     rate=self.dual_start() if rate&128 else self.single_start(rate)
   except Restart:need_boot=True;rate=0
   except End:break
  return c.events

def original(body):
 result=[];sources=list({s['id']:s for s in body['resources'].values()}.values())
 for e in body['events']:
  if e['kind']not in[1,2,3,9,10,11]:continue
  event={'cycle':e['frame']*F+e['cycles_before']}
  if e['kind']==1:event.update(kind='dac',value=e['data'])
  elif e['kind']==2:event.update(kind='pan'if e['address']==0x1b6 else'enable',value=e['data'])
  elif e['kind']==9:event.update(kind='ym',register=e['address'],value=e['data'])
  elif e['kind']==10:event.update(kind='reset')
  elif e['kind']==11:event.update(kind='address',port=e['address'],value=e['data'])
  else:
   address=e['bank']+(e['address']&32767)
   if e['pc']in[0xa5,0xa8,0xc3,0xc6]:
    candidates=[s for s in sources if s['source']['header_address']<=address<s['source']['header_address']+4];assert len(candidates)==1;(s,)=candidates;event.update(kind='metadata',resource=s['id'],offset=address-s['source']['header_address'])
   elif e['pc']in[0xb7,0xb9,0xbb,0xbd,0x168,0x16a]:
    earlier=next(x for x in reversed(result)if x['kind']=='metadata'and x['offset']==1);s=next(s for s in sources if s['id']==earlier['resource']);event.update(kind='bank',resource=s['id'],offset=address-s['source']['sample_address'])
   elif e['pc']==0x149:
    iy=e['registers'][7];bank=e['bank'];candidates=[s for s in sources if((bank|(iy&32767))-s['source']['header_address'])&65535<32768]
    # The latched single header pointer identifies source even after a later
    # descriptor publication; follow the nearest earlier metadata1 event.
    earlier=next(x for x in reversed(result)if x['kind']=='metadata'and x['offset']==1);s=next(s for s in sources if s['id']==earlier['resource']);event.update(kind='dummy',resource=s['id'],offset=(address-s['source']['header_address'])&65535)
   else:
    candidates=[s for s in sources if s['source']['sample_address']<=address<s['source']['sample_address']+len(bytes.fromhex(s['encoded_hex']))+4];assert len(candidates)==1,(address,candidates);(s,)=candidates;event.update(kind='bank',resource=s['id'],offset=address-s['source']['sample_address'])
  if e['kind']==3:event.update(resume=e['frame']*F+e['z80_after'],m68k_delay=e['m68k_after']-e['m68k_before'])
  result.append(event)
 return result

def main():
 p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--single-only',action='store_true');a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);rows=[]
 for path in sorted(capture.glob('*.json.gz')):
  body=json.loads(gzip.decompress(path.read_bytes()))
  if a.single_only and any(x.get('target')in['a','b']for x in body['case']['inputs']):continue
  model=Device(body);actual=[{k:v for k,v in e.items()if k!='start'}for e in model.run()];expected=original(body);diff=next(({'index':i,'actual':x,'expected':y}for i,(x,y)in enumerate(zip(actual,expected))if x!=y),None);row={'name':body['case']['name'],'pass':not diff and len(actual)==len(expected),'events':len(actual),'expected':len(expected),'difference':diff,'holds':model.clock.holds};rows.append(row);print(json.dumps(row),flush=True)
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps({'pass':all(r['pass']for r in rows),'cases':rows,'model_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n')
 if not all(r['pass']for r in rows):raise SystemExit(1)
if __name__=='__main__':main()
