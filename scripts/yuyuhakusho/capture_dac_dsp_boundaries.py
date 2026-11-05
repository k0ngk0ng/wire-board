"""Declared private native DAC/enable/pan/frame DSP boundary inputs."""
from pathlib import Path
import argparse,ctypes as C,gzip,json,struct,wave
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols,snapshot
from prototype_dac_frame_dsp import FramePrototype
from prototype_dac_dsp import kernel
from reference import ROOT,sha
F=896040
CASES=[
 ('silence',2,{}),
 ('same-cycle',4,{0:[(1000,'data',0),(1000,'enable',128),(1000,'pan',192),(1000,'data',255)],1:[(0,'data',0),(0,'data',128),(1008,'data',127)],2:[(F,'data',57)]}),
 ('pan-sign',6,{0:[(0,'data',0),(0,'enable',128),(0,'pan',128)],1:[(0,'pan',64)],2:[(2300,'pan',0)],3:[(3000,'data',255),(3000,'pan',192)],4:[(1008,'data',128)],5:[(F-1,'enable',0)]}),
 ('ahead-boundary',4,{0:[(0,'data',0),(0,'enable',128),(0,'pan',192),(F+195,'data',255)],1:[(195,'data',127),(196,'data',57)],2:[(F+1008,'enable',0)],3:[(1008,'enable',128)]}),
 ('data-sweep',5,{0:[(i*3000,'data',i)for i in range(256)]+[(0,'enable',128),(0,'pan',192)],1:[(i*3000,'data',255-i)for i in range(256)],2:[(F,'pan',128)],3:[(F+1,'pan',64)],4:[(1,'enable',0)]}),
]
# Stable same-cycle order must remain the explicit input order. Sweep setup
# precedes its first data write; do not sort equal timestamps by event kind.
CASES[-1][2][0]=[(0,'enable',128),(0,'pan',192)]+[(i*3000,'data',i)for i in range(256)]
SOURCE_CAPTURE=ROOT/'.local/md-yuyuhakusho/audio-pcm-model-first'
SOURCE_PCM={}
source_manifest=json.loads((SOURCE_CAPTURE/'capture.json').read_text())
for item in source_manifest['cases']:
 body=json.loads(gzip.decompress((SOURCE_CAPTURE/(item['name']+'-events.json.gz')).read_bytes()))
 schedule={}
 for event in body['events']:
  if event['kind']==1:kind='data'
  elif event['kind']==2 and event['address']==0x2b:kind='enable'
  elif event['kind']==2 and event['address']==0x1b6:kind='pan'
  else:continue
  schedule.setdefault(event['frame'],[]).append((event['cycles_before'],kind,event['data']))
 name='source-'+item['name'];CASES.append((name,item['frames'],schedule))
 with wave.open(str(SOURCE_CAPTURE/(item['name']+'-original.wav')),'rb')as f:SOURCE_PCM[name]=f.readframes(f.getnframes())
ORIGINAL=ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib';OBSERVED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
def dump(path,body):
 raw=(json.dumps(body,separators=(',',':'))+'\n').encode();path.write_bytes(gzip.compress(raw,mtime=0));return sha(raw)
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 controls={};reports=[];identities={};coeff,_=kernel()
 for mode,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  lookup=symbols(core)
  with AudioSession(ROOT,out/mode,core=core)as ref:
   checkpoint=quiet_checkpoint(ref);identities[mode]=ref.identity
   for name,nframes,schedule in CASES:
    ref.restore(checkpoint,restore_refresh_cycles=True);initial=snapshot(ref,lookup);model=FramePrototype(initial['initial_state'],coeff);ram_before=sha(ref.ram_cpu());video_before=sha(ref.image)
    writer=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_write').value);end_time=C.c_uint.in_dll(ref.lib,'mcycles_vdp');update=ref.api('audio_update',C.c_int,C.c_void_p)
    frames=[];pcm_parts=[];previous_raw=0
    for frame in range(nframes):
     events=[]
     for cycle,kind,value in schedule.get(frame,[]):
      assert frame*F+cycle>=previous_raw,(name,'private input ordering');previous_raw=frame*F+cycle
      if kind=='data':port,reg=0,0x2a
      elif kind=='enable':port,reg=0,0x2b
      elif kind=='pan':port,reg=2,0xb6
      else:raise AssertionError(kind)
      writer(cycle,port,reg);writer(cycle,port+1,value);events.append({'cycle':frame*F+cycle,'kind':kind,'value':value})
     end_time.value=F;buffer=(C.c_int16*(4410*2))();count=update(buffer);assert 0<count<=4410;pcm=C.string_at(buffer,count*4);pcm_parts.append(pcm)
     predicted=model.render(events,(frame+1)*F);assert predicted==pcm,(name,frame,'prototype differs from private native boundary input')
     ref.frame=checkpoint['frame']+frame+1;state=snapshot(ref,lookup)
     row={'frame':frame,'events':events,'stereo_frames':count,'pcm_sha256':sha(pcm),'ram_sha256':sha(ref.ram_cpu()),'video_sha256':sha(ref.image),'native_state':state['initial_state']}
     assert row['ram_sha256']==ram_before and row['video_sha256']==video_before,(name,'non-audio guest state changed')
     frames.append(row)
    pcm=b''.join(pcm_parts)
    if name in SOURCE_PCM:assert pcm==SOURCE_PCM[name],('direct native DAC replay does not match prior original CPU stream',name)
    body={'name':name,'initial':initial,'frames':frames};path=out/(name+'-'+mode+'.json.gz');dump(path,body)
    wav_path=out/(name+'-'+mode+'.wav')
    with wave.open(str(wav_path),'wb')as wav:wav.setnchannels(2);wav.setsampwidth(2);wav.setframerate(44100);wav.writeframes(pcm)
    if mode=='original':controls[name]={'body':body,'pcm':pcm}
    else:
     assert body==controls[name]['body']and pcm==controls[name]['pcm'],('native original/observed DSP boundary differs',name)
     reports.append({'name':name,'frames':nframes,'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm),'all_original_observed_equal':True,'artifacts':[{'path':str(x.relative_to(ROOT)),'sha256':sha(x.read_bytes())}for x in[path,wav_path,out/(name+'-original.json.gz'),out/(name+'-original.wav')]]})
    print(json.dumps({'mode':mode,'case':name,'frames':nframes,'stereo_frames':len(pcm)//4}),flush=True)
 r={'pass':True,'scope':'Private direct nativefm_write and audio_update scalarDSP inputs; no CPU/game/cadence reachability claim','identity':identities,'cases':reports,'prior_cpu_dac_source_pcm_match_cases':len(SOURCE_PCM),'max_declared_ahead_master_cycles':1008,'same_cycle_order_preserved':True,'no_raw_event_times_modified':True,'producer_sha256':sha(Path(__file__).read_bytes()),'initial_inspector_sha256':sha((ROOT/'scripts/yuyuhakusho/inspect_dac_dsp_state.py').read_bytes()),'limits':['This explicitly drives audio-onlyframe boundaries and leaves guestRAM/video unchanged; it is not a new gameplay originalcontrol','1008 is a bounded tested DSP input contract, not a claim of maximum realhardware/nativeCPU instruction overrun','Raw writes at end and beyond end retain their original values; referenceframe flush discards lookahead as observed']}
 (out/'capture.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'pass':True,'cases':len(reports),'video_steps_per_core':sum(x['frames']for x in reports)}))
if __name__=='__main__':main()
