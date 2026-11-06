"""Autonomous command model through DAC-only reference DSP; research contract.

Native event timestamps never drive the model. A reset preserves original call
order even when its raw timestamp precedes a completed Z80 memory operation.
"""
from pathlib import Path
import argparse,gzip,json,runpy
from reference import ROOT,sha
from prototype_dac_dsp import kernel
from prototype_dac_frame_dsp import FramePrototype
MODEL=runpy.run_path(str(ROOT/'scripts/yuyuhakusho/pcm-command-prototype.py'));Device=MODEL['Device'];F=896040

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--sort-by-cycle',action='store_true');a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);coeff,source=kernel();rows=[]
 for path in sorted(capture.glob('*.json.gz')):
  body=json.loads(gzip.decompress(path.read_bytes()));device=Device(body);events=device.run();initial=body['initial_dsp']['initial_state'];assert not any(x for ch in initial['fm_operator_key']for x in ch);dsp=FramePrototype(initial,coeff);packets={};raw_reversals=[];last_cycle=0
  for event in events:
   kind=event['kind'];cycle=event['cycle']
   if kind not in ['dac','pan','enable','reset','ym']:continue
   if cycle<last_cycle:raw_reversals.append({'previous_cycle':last_cycle,'cycle':cycle,'kind':kind})
   last_cycle=cycle;frame=event['start']//F
   if kind=='ym':continue # In this declared muted-FM profile this only key-offs6.
   if kind=='reset':mapped=[{'cycle':cycle,'kind':'data','value':128},{'cycle':cycle,'kind':'enable','value':0},{'cycle':cycle,'kind':'pan','value':192}]
   else:mapped=[{'cycle':cycle,'kind':'data'if kind=='dac'else kind,'value':event['value']&192 if kind=='pan'else event['value']}]
   packets.setdefault(frame,[]).extend(mapped)
  difference=None;total=0;nonzero=0
  for index,frame in enumerate(body['frames']):
   mapped=packets.get(index,[])
   if a.sort_by_cycle:mapped=sorted(mapped,key=lambda e:e['cycle'])
   predicted=dsp.render(mapped,(index+1)*F);native=bytes.fromhex(frame['pcm_hex']);total+=len(native)//4;nonzero+=sum(native[j:j+2]!=b'\0\0'for j in range(0,len(native),2))
   if predicted!=native and difference is None:
    first=next((i for i,(x,y)in enumerate(zip(predicted,native))if x!=y),None);difference={'kind':'pcm','frame':index,'first_byte':first,'actual_bytes':len(predicted),'original_bytes':len(native)}
   actual={'dac_value':dsp.value,'dac_enabled':dsp.enabled,'pan':dsp.pan,'previous_chip_output':dsp.previous,'integrator':dsp.integrator,'tail':dsp.tail}
   n=frame['native_dsp'];expected={'dac_value':n['dac_value'],'dac_enabled':n['dac_enabled'],'pan':(128 if n['dac_pan_masks'][0]else 0)+(64 if n['dac_pan_masks'][1]else 0),'previous_chip_output':n['previous_chip_output'],'integrator':n['resampler']['integrator'],'tail':n['resampler']['tail']}
   if actual!=expected and difference is None:difference={'kind':'typed-dsp-state','frame':index,'fields':{key:{'actual':actual[key],'original':expected[key]}for key in actual if actual[key]!=expected[key]}}
  row={'name':body['case']['name'],'pass':difference is None,'reference_frames':len(body['frames']),'stereo_frames':total,'nonzero_native_samples':nonzero,'first_difference':difference,'raw_timestamp_reversals':raw_reversals,'native_sha256':sha(path.read_bytes())};rows.append(row);print(json.dumps(row),flush=True)
 result={'pass':all(r['pass']for r in rows),'scope':'Autonomous typed command/payload/window model into DAC-only reference frame DSP; no FM synthesis/world/physical speaker claim','sort_by_cycle':a.sort_by_cycle,'raw_cycles_clamped':False,'model_sha256':sha((ROOT/'scripts/yuyuhakusho/pcm-command-prototype.py').read_bytes()),'checker_sha256':sha(Path(__file__).read_bytes()),'dsp_prototype_sha256':sha((ROOT/'scripts/yuyuhakusho/prototype_dac_frame_dsp.py').read_bytes()),'kernel_source_sha256':source,'cases':rows};out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n')
 if not result['pass']:raise SystemExit(1)
if __name__=='__main__':main()
