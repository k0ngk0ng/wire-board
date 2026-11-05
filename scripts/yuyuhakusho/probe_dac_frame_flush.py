"""Read-only reference FM lookahead/frame-flush linearization probe."""
from pathlib import Path
import argparse,ctypes as C,gzip,json
from extract_audio_assets import AudioSession,queue_command
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols
from reference import ROOT,sha

p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib';lookup=symbols(core)
expected=json.loads(gzip.decompress((ROOT/'.local/md-yuyuhakusho/audio-pcm-model-first/pair-long-control.json.gz').read_bytes()))['rows']
rows=[];writes=[];errors=[];frame=-1;latches=[255,255]
with AudioSession(ROOT,out/'reference',core=core)as ref:
 checkpoint=quiet_checkpoint(ref);ref.restore(checkpoint,restore_refresh_cycles=True);image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0]
 slot=C.c_void_p.in_dll(ref.lib,'fm_write');original_value=slot.value;typ=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint);original=typ(original_value)
 count=C.c_int.from_address(image+lookup['_fm_cycles_count'][0]);start=C.c_int.from_address(image+lookup['_fm_cycles_start'][0]);ptr=C.c_void_p.from_address(image+lookup['_fm_ptr'][0]);buffer=image+lookup['_fm_buffer'][0]
 last=[C.c_int.from_address(image+lookup['_fm_last.'+str(i)][0])for i in range(2)]
 def state():return {'count':count.value,'start':start.value,'buffer_stereo_position':(ptr.value-buffer)//8,'last_flushed_output':[x.value for x in last]}
 @typ
 def observe(cycles,address,data):
  try:
   port=address>>1&1
   if address&1==0:latches[port]=data
   selected=address&1 and port==0 and latches[0]==0x2a and cycles>=895032
   before=state()if selected else None
   original(cycles,address,data)
   if selected:writes.append({'frame':frame,'cycles':cycles,'data':data,'before':before,'after':state()})
  except Exception as error:errors.append(repr(error))
 slot.value=C.cast(observe,C.c_void_p).value
 for frame in range(160):
  if frame==0:queue_command(ref,0x1f)
  if frame==8:queue_command(ref,0x3f)
  before=state();r,pcm=ref.step_audio(ref.frame+1,[0]*8);current={'step':frame,'reference':r['reference'],'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm)};assert current==expected[frame],('probe changed original stream',frame)
  rows.append({'frame':frame,'before':before,'after_frame_flush':state(),'pcm_sha256':sha(pcm),'stereo_frames':len(pcm)//4});assert not errors,errors
 assert slot.value==C.cast(observe,C.c_void_p).value;slot.value=original_value;identity=ref.identity
r={'pass':True,'scope':'Reference-only scalar state around rawDAC writes and nativeframe flush; no production timing model','identity':identity,'frames':160,'ram_video_pcm_match_canonical':True,'writes_near_end':writes,'frame_states':rows,'errors':errors,'probe_sha256':sha(Path(__file__).read_bytes()),'boundary_frame':83}
(out/'result.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'pass':True,'near_end_writes':len(writes),'frame83_writes':[x for x in writes if x['frame']==83],'frame83_end':rows[83]['after_frame_flush'],'frame84_begin':rows[84]['before']}))
