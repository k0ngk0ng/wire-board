"""Bounded pure arithmetic DAC-only prototype against retained original PCM.

This research model receives original timestamps only as a layer oracle. It
cannot be shipped as a game event schedule or claim independent cadence.
"""
from pathlib import Path
import argparse,gzip,json,re,struct,wave
from reference import ROOT,sha

MASK=(1<<52)-1

def kernel():
 path=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/sound/blip_buf.c';text=path.read_text()
 body=text.split('static short const bl_step [phase_count + 1] [half_width] =',1)[1].split('};',1)[0]
 values=list(map(int,re.findall(r'-?\d+',body)));assert len(values)==264
 return [values[i:i+8]for i in range(0,264,8)],sha(path.read_bytes())

def wrap32(value):return(value+(1<<31))%(1<<32)-(1<<31)
class Prototype:
 def __init__(self,initial,coeff):
  self.origin=initial['master_origin'];self.time=0;self.factor=initial['resampler']['factor'];self.value=initial['dac_value'];self.enabled=initial['dac_enabled'];self.pan=0
  for mask,bit in zip(initial['dac_pan_masks'],[128,64]):
   if mask:self.pan|=bit
  self.previous=initial['previous_chip_output'][:];self.integrator=initial['resampler']['integrator'][:];self.tail=[x[:]for x in initial['resampler']['tail']];self.coeff=coeff
  assert initial['fm_timing']['fm_cycles_ratio']==1008 and(initial['master_origin']+initial['fm_timing']['fm_cycles_start'])%1008==0
  assert(initial['master_origin']*self.factor+self.factor//2)&MASK==initial['resampler']['offset']
 def level(self):
  raw=(self.value-128)*64 if self.enabled else 0
  result=[]
  for bit in[128,64]:
   on=bool(self.pan&bit);value=768+(raw if on else 0)
   if raw<0:value-=224 if on else 256
   result.append(value)
  return result
 def render(self,events,end):
  start=self.time;offset=((self.origin+start)*self.factor+self.factor//2)&MASK
  count=(offset+(end-start)*self.factor)>>52;buf=[self.tail[ch][:]+[0]*(count+20)for ch in range(2)]
  tick=((self.origin+start+1007)//1008)*1008-self.origin
  def advance(to):
   nonlocal tick
   while tick<to:
    current=self.level();delta=[current[ch]-self.previous[ch]for ch in range(2)];self.previous=current
    if delta[0]or delta[1]:
     fixed=(((tick-start)*self.factor+offset)>>32)&0xffffffff;phase=fixed>>15&31;interp=fixed&32767;pos=fixed>>20
     for ch in range(2):
      part=delta[ch]*interp>>15;main=delta[ch]-part
      for i in range(8):
       buf[ch][pos+i]=wrap32(buf[ch][pos+i]+self.coeff[phase][i]*main+self.coeff[phase+1][i]*part)
       buf[ch][pos+8+i]=wrap32(buf[ch][pos+8+i]+self.coeff[32-phase][7-i]*main+self.coeff[31-phase][7-i]*part)
    tick+=1008
  for e in events:
   assert start<=e['cycle']<=end;(advance(e['cycle']))
   if e['kind']=='data':self.value=e['value']
   elif e['kind']=='enable':self.enabled=bool(e['value']&128)
   elif e['kind']=='pan':self.pan=e['value']&192
   else:raise AssertionError(e)
  advance(end);result=[]
  for i in range(count):
   for ch in range(2):
    value=max(-32768,min(32767,self.integrator[ch]>>15));self.integrator[ch]=wrap32(self.integrator[ch]+buf[ch][i]);self.integrator[ch]=wrap32(self.integrator[ch]-(value<<6));result.append(value)
  self.tail=[buf[ch][count:count+18]for ch in range(2)];self.time=end
  return struct.pack('<'+'h'*len(result),*result)

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--capture',type=Path,required=True);parser.add_argument('--config',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
 capture=args.capture.resolve();config=args.config.resolve();out=args.out.resolve();assert all(p.is_relative_to(ROOT)for p in[capture,config,out]);out.mkdir(parents=True,exist_ok=True)
 cfg=json.loads(config.read_text());assert cfg['configuration']['filter']==0 and cfg['configuration']['ym2612']==0 and cfg['configuration']['hq_fm']==1 and cfg['configuration']['fm_preamp']==100 and cfg['configuration']['sample_rate']==44100 and cfg['configuration']['mono']==0
 coeff,source=kernel();summary=json.loads((capture/'capture.json').read_text());reports=[]
 for case in summary['cases']:
  body=json.loads(gzip.decompress((capture/(case['name']+'-events.json.gz')).read_bytes()));events=[]
  for raw in body['events']:
   if raw['kind']==1:kind='data'
   elif raw['kind']==2 and raw['address']==0x2b:kind='enable'
   elif raw['kind']==2 and raw['address']==0x1b6:kind='pan'
   else:continue
   events.append({'cycle':raw['frame']*896040+raw['cycles_before'],'kind':kind,'value':raw['data'],'source_frame':raw['frame'],'source_cycle':raw['cycles_before']})
  assert all(a['cycle']<=b['cycle']for a,b in zip(events,events[1:]))
  model=Prototype(cfg['initial_state'],coeff);chunks=[];position=0;counts=[]
  for frame in range(case['frames']):
   end=(frame+1)*896040;subset=[]
   while position<len(events)and events[position]['cycle']<=end:subset.append(events[position]);position+=1
   block=model.render(subset,end);chunks.append(block);counts.append(len(block)//4)
  assert position==len(events);pcm=b''.join(chunks)
  with wave.open(str(capture/(case['name']+'-original.wav')),'rb')as wav:reference=wav.readframes(wav.getnframes())
  assert len(pcm)==len(reference)and counts==[r['stereo_frames']for r in body['rows']]
  a=struct.unpack('<'+'h'*(len(pcm)//2),pcm);b=struct.unpack('<'+'h'*(len(reference)//2),reference);different=[i for i,(x,y)in enumerate(zip(a,b))if x!=y]
  path=out/(case['name']+'-prototype.wav')
  with wave.open(str(path),'wb')as wav:wav.setnchannels(2);wav.setsampwidth(2);wav.setframerate(44100);wav.writeframes(pcm)
  result={'name':case['name'],'stereo_frames':len(pcm)//4,'different_sample_values':len(different),'max_absolute_error':max((abs(x-y)for x,y in zip(a,b)),default=0),'first_differences':[[i,a[i],b[i]]for i in different[:12]],'pcm_sha256':sha(pcm),'reference_pcm_sha256':sha(reference),'raw_beyond_frame_end_events':[e for e in events if e['source_cycle']>=896040]};reports.append(result);print(json.dumps(result),flush=True)
 report={'pass':all(r['different_sample_values']==0 for r in reports),'scope':'PureDAC arithmetic prototype with given raw oracle event times; no native cadence generation or fullFM/PSG','runtime_configuration_sha256':sha(config.read_bytes()),'kernel_source_sha256':source,'prototype_sha256':sha(Path(__file__).read_bytes()),'cases':reports}
 (out/'result.json').write_text(json.dumps(report,indent=2)+'\n')
 if not report['pass']:raise SystemExit(1)
if __name__=='__main__':main()
