"""Portable nativeDSP frame/PCM/state oracle; no native context pointers."""
from pathlib import Path
import argparse,base64,gzip,json,wave
from prototype_dac_dsp import kernel
from reference import ROOT,sha
F=896040

def state(raw,time,ahead):
 pan=0
 for mask,bit in zip(raw['dac_pan_masks'],[128,64]):
  if mask:pan|=bit
 return {'Time':time,'DACValue':raw['dac_value'],'Enabled':raw['dac_enabled'],'Pan':pan,'LastChipOutput':raw['previous_chip_output'],'Integrator':raw['resampler']['integrator'],'Tail':raw['resampler']['tail'],'InputAhead':ahead,'Pending':None}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 summary=json.loads((capture/'capture.json').read_text());assert summary['pass']and len(summary['cases'])==9;coeff,source=kernel();cases=[]
 for case in summary['cases']:
  native_path=capture/(case['name']+'-observed.json.gz');native=json.loads(gzip.decompress(native_path.read_bytes()));origin=native['initial']['initial_state']['master_origin'];initial=native['initial']['initial_state'];factor=initial['resampler']['factor'];cfg=native['initial']['configuration']
  assert cfg['hq_fm']==1 and cfg['filter']==0 and cfg['ym2612']==0 and cfg['ym3438']==0 and cfg['fm_preamp']==100 and cfg['sample_rate']==44100 and cfg['mono']==0
  with wave.open(str(capture/(case['name']+'-observed.wav')),'rb')as wav:pcm=wav.readframes(wav.getnframes())
  cursor=0;frames=[];previous_input=0
  for frame in native['frames']:
   end=(frame['frame']+1)*F;events=[]
   for event in frame['events']:
    assert event['cycle']>=previous_input;previous_input=event['cycle'];events.append({'Cycle':event['cycle'],'Kind':{'data':1,'enable':2,'pan':3}[event['kind']],'Value':event['value']})
   ahead=max(0,previous_input-end);raw=frame['native_state'];typed=state(raw,end,ahead);assert raw['master_origin']==origin+end
   expected_offset=(factor//2+(origin+end)*factor)&((1<<52)-1);assert raw['resampler']['offset']==expected_offset and raw['resampler']['available_samples']==0
   assert raw['fm_timing']['fm_cycles_start']==(-(origin+end))%1008 and raw['previous_lowpass_output']==[0,0]
   block=pcm[cursor:cursor+frame['stereo_frames']*4];cursor+=len(block);assert sha(block)==frame['pcm_sha256']
   frames.append({'Events':events,'PCM':base64.b64encode(block).decode(),'State':typed,'NativeOffset':raw['resampler']['offset'],'NativeNextChipDelta':raw['fm_timing']['fm_cycles_start']})
  assert cursor==len(pcm)
  cases.append({'Name':case['name'],'Config':{'Profile':1,'MasterOrigin':origin},'Initial':state(initial,0,0),'Frames':frames,'NativeConfig':cfg,'PCM_SHA256':sha(pcm),'SourceBodySHA256':sha(native_path.read_bytes())})
 body={'Schema':1,'Kernel':coeff,'CoefficientSourceSHA256':source,'Cases':cases,'NativeSourceCaptureSHA256':sha((capture/'capture.json').read_bytes()),'Scope':'Given ordered referenceframe inputs and native PCM/typedstate oracle only; not an independent event timing engine'}
 raw=(json.dumps(body,separators=(',',':'))+'\n').encode();path=out/'native-frames.json.gz';path.write_bytes(gzip.compress(raw,mtime=0))
 manifest={'pass':True,'fixture_path':str(path.relative_to(ROOT)),'fixture_json_sha256':sha(raw),'fixture_gzip_sha256':sha(path.read_bytes()),'cases':len(cases),'frames':sum(len(c['Frames'])for c in cases),'stereo_frames':sum(len(base64.b64decode(f['PCM']))//4 for c in cases for f in c['Frames']),'producer_sha256':sha(Path(__file__).read_bytes()),'capture_path':str((capture/'capture.json').relative_to(ROOT)),'capture_sha256':sha((capture/'capture.json').read_bytes()),'source_coefficient_sha256':source,'limits':['Native Kernel numeric coefficients are the fixed mathematical resampling profile; no source/core binary is a runtime dependency','Input timestamps remain raw; explicitframe grouping carries the reference flushing contract','No native pointer, wholeYM context, CPU/RAM state or instruction stream is present in the portable oracle']}
 (out/'native-frames-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))
if __name__=='__main__':main()
