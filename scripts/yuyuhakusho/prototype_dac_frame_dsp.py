"""Explicit reference-frame DSP prototype; retains raw event cycle values.

The reference FM updater can generate past frameEnd, but its frame flusher
submits only chip ticks before frameEnd and discards lookahead. This is not a
claim of physical YM2612 hardware behavior or independent event generation.
"""
from pathlib import Path
import argparse,gzip,json,struct,wave
from prototype_dac_dsp import Prototype,kernel,wrap32,MASK
from reference import ROOT,sha

class FramePrototype(Prototype):
 def render(self,events,end):
  start=self.time;offset=((self.origin+start)*self.factor+self.factor//2)&MASK;count=(offset+(end-start)*self.factor)>>52
  buf=[self.tail[ch][:]+[0]*(count+20)for ch in range(2)];tick=((self.origin+start+1007)//1008)*1008-self.origin;discarded=0
  def advance(to):
   nonlocal tick,discarded
   while tick<to:
    current=self.level()
    if tick<end:
     delta=[current[ch]-self.previous[ch]for ch in range(2)];self.previous=current
     if delta[0]or delta[1]:
      fixed=(((tick-start)*self.factor+offset)>>32)&0xffffffff;phase=fixed>>15&31;interp=fixed&32767;position=fixed>>20
      for ch in range(2):
       part=delta[ch]*interp>>15;main=delta[ch]-part
       for i in range(8):
        buf[ch][position+i]=wrap32(buf[ch][position+i]+self.coeff[phase][i]*main+self.coeff[phase+1][i]*part)
        buf[ch][position+8+i]=wrap32(buf[ch][position+8+i]+self.coeff[32-phase][7-i]*main+self.coeff[31-phase][7-i]*part)
    else:discarded+=1
    tick+=1008
  for event in events:
   assert event['cycle']>=start
   advance(event['cycle'])
   if event['kind']=='data':self.value=event['value']
   elif event['kind']=='enable':self.enabled=bool(event['value']&128)
   elif event['kind']=='pan':self.pan=event['value']&192
   else:raise AssertionError(event)
  advance(end);result=[]
  for i in range(count):
   for ch in range(2):
    value=max(-32768,min(32767,self.integrator[ch]>>15));self.integrator[ch]=wrap32(self.integrator[ch]+buf[ch][i]-(value<<6));result.append(value)
  self.tail=[buf[ch][count:count+18]for ch in range(2)];self.time=end;self.discarded=discarded
  return struct.pack('<'+'h'*len(result),*result)

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--config',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();config=a.config.resolve();out=a.out.resolve();assert all(x.is_relative_to(ROOT)for x in[capture,config,out]);out.mkdir(parents=True,exist_ok=True)
 cfg=json.loads(config.read_text());assert cfg['configuration']['filter']==0 and cfg['configuration']['hq_fm']==1 and cfg['configuration']['ym2612']==0 and cfg['configuration']['fm_preamp']==100 and cfg['configuration']['sample_rate']==44100
 coeff,source=kernel();cases=[];manifest=json.loads((capture/'capture.json').read_text())
 for case in manifest['cases']:
  body=json.loads(gzip.decompress((capture/(case['name']+'-events.json.gz')).read_bytes()));byframe={};raw_overruns=[]
  for raw in body['events']:
   if raw['kind']==1:kind='data'
   elif raw['kind']==2 and raw['address']==0x2b:kind='enable'
   elif raw['kind']==2 and raw['address']==0x1b6:kind='pan'
   else:continue
   event={'cycle':raw['frame']*896040+raw['cycles_before'],'kind':kind,'value':raw['data'],'source_frame':raw['frame'],'source_cycle':raw['cycles_before']};byframe.setdefault(raw['frame'],[]).append(event)
   if raw['cycles_before']>=896040:raw_overruns.append(event)
  model=FramePrototype(cfg['initial_state'],coeff);blocks=[];frames=[]
  for frame in range(case['frames']):
   raw=model.render(byframe.get(frame,[]),(frame+1)*896040);assert len(raw)//4==body['rows'][frame]['stereo_frames'];blocks.append(raw);frames.append({'frame':frame,'discarded_lookahead_chip_samples':model.discarded,'pcm_sha256':sha(raw),'stereo_frames':len(raw)//4})
  pcm=b''.join(blocks)
  with wave.open(str(capture/(case['name']+'-original.wav')),'rb')as wav:reference=wav.readframes(wav.getnframes())
  assert len(pcm)==len(reference)
  x=struct.unpack('<'+'h'*(len(pcm)//2),pcm);y=struct.unpack('<'+'h'*(len(reference)//2),reference);different=[i for i,(j,k)in enumerate(zip(x,y))if j!=k]
  result={'name':case['name'],'stereo_frames':len(pcm)//4,'different_sample_values':len(different),'maximum_absolute_error':max((abs(j-k)for j,k in zip(x,y)),default=0),'pcm_sha256':sha(pcm),'reference_pcm_sha256':sha(reference),'raw_overruns':raw_overruns,'frames':frames,'first_difference_indices':different[:12]};cases.append(result);print(json.dumps({k:v for k,v in result.items()if k!='frames'}),flush=True)
  with wave.open(str(out/(case['name']+'-prototype.wav')),'wb')as wav:wav.setnchannels(2);wav.setsampwidth(2);wav.setframerate(44100);wav.writeframes(pcm)
 r={'pass':all(c['different_sample_values']==0 for c in cases),'scope':'Explicit pinned reference-frame flush prototype from raw native input groups; not physicalhardware or independent cadence','raw_event_cycle_values_modified':False,'lookahead_policy':'Advance to original raw write cycles then apply register writes; flush/resample only chipTick<frameEnd, discard computedfuturechip samples, keep lastflushed output separately from currentDAC register','configuration_path':str(config.relative_to(ROOT)),'configuration_sha256':sha(config.read_bytes()),'coefficient_source_sha256':source,'prototype_sha256':sha(Path(__file__).read_bytes()),'cases':cases}
 (out/'result.json').write_text(json.dumps(r,indent=2)+'\n')
 if not r['pass']:raise SystemExit(1)
if __name__=='__main__':main()
