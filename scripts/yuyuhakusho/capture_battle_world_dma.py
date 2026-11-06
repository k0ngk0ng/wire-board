"""Original World wrapper and live graphic DMA before logical World work."""
import argparse,ctypes as C,gzip,json,os,subprocess
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,CPUHead,buttons_at,sha
from capture_battle_world_entry import CORE,FIELDS as ENTRY_FIELDS,CPUControl
from capture_pcm_transfer_gate import command
FIELDS=ENTRY_FIELDS+['dma_remaining','dma_end','dma_type','vdp_status','transfer_end','transfer_flag','frame_index']
class Event(C.Structure):_fields_=[(n,C.c_uint32)for n in FIELDS]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);tmp=out/'tmp';tmp.mkdir();source=ROOT/'scripts/yuyuhakusho/battle-world-dma-observer.c';binary=tmp/'world-dma.dylib'
 subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(binary));b.battle_prefix_attach.argtypes=[C.c_void_p]*14;b.battle_prefix_attach.restype=C.c_int;b.battle_prefix_count.restype=C.c_uint;b.battle_prefix_complete.restype=C.c_uint;b.battle_world_attach.argtypes=[C.c_void_p]*3;b.battle_world_attach.restype=C.c_int;b.battle_world_dma_attach.argtypes=[C.c_void_p]*4;b.battle_world_dma_attach.restype=C.c_int;b.battle_world_dma_events.restype=C.POINTER(Event);b.battle_world_dma_size.restype=C.c_uint;b.battle_world_dma_transfers.restype=C.POINTER(C.c_uint16);b.battle_world_dma_transfer_words.restype=C.c_uint;assert b.battle_world_dma_size()==C.sizeof(Event)
 controls_path=ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-baseline-original.json.gz';controls=json.loads(gzip.decompress(controls_path.read_bytes()));inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());selected=set(range(2068,2075))|set(range(2393,2401));rows=[]
 with AudioSession(ROOT,out/'reference',core=CORE)as ref:
  pointers=[t.in_dll(ref.lib,n)for n,t in [('reg',C.c_ubyte),('mcycles_vdp',C.c_uint),('v_counter',C.c_ushort),('h_counter',C.c_ubyte),('hint_pending',C.c_ubyte),('vint_pending',C.c_ubyte),('input',C.c_ubyte)]];r,line,vc,h,hp,vp,inp=map(C.addressof,pointers);get=ref.api('m68k_get_reg',C.c_uint,C.c_uint)
  assert b.battle_prefix_attach(ref.ram_pointer,r,C.addressof(ref.cpu)+CPUHead.cycles.offset,C.addressof(ref.cpu)+CPUHead.refresh_cycles.offset,line,vc,h,hp,vp,C.addressof(ref.cpu)+CPUHead.cycle_end.offset,inp,C.cast(get,C.c_void_p),C.addressof(ref.cpu)+CPUControl.pin.offset,C.addressof(ref.cpu)+CPUControl.mask.offset)
  native=CPUControl.in_dll(ref.lib,'m68k');assert native.cycles==ref.cpu.cycles and native.mask==get(17)&0x700
  z=command.Z80.in_dll(ref.lib,'Z80');zr=(C.c_uint8*8192).in_dll(ref.lib,'zram');bus=C.c_uint8.in_dll(ref.lib,'zstate');assert b.battle_world_attach(C.addressof(zr),C.addressof(bus),C.addressof(z)+command.Z80.cycles.offset)
  dma=[t.in_dll(ref.lib,n)for n,t in [('dma_length',C.c_uint),('dma_endCycles',C.c_uint),('dma_type',C.c_ubyte),('status',C.c_ushort)]];assert b.battle_world_dma_attach(*map(C.addressof,dma))
  hook=ref.api('set_cpu_hook',None,C.c_void_p)
  def captured(frame,private=None):
   assert b.battle_prefix_complete(),('World wrapper did not complete',frame,b.battle_prefix_count());events=b.battle_world_dma_events();words=b.battle_world_dma_transfers();row={'frame':frame,'events':[[getattr(events[i],n)for n in FIELDS]for i in range(b.battle_prefix_count())],'transfers':[words[i]for i in range(b.battle_world_dma_transfer_words())]}
   if private is not None:row['private']=private
   return row
  collecting=False;start_frame=0
  for frame in range(1,2401):
   b.battle_world_dma_frame(frame)
   if frame in selected and not collecting:b.battle_world_dma_begin(0,0,0,0,1);collecting=True;start_frame=frame
   if collecting:hook(C.cast(b.battle_world_dma_hook,C.c_void_p))
   try:result,pcm=ref.step_audio(frame,buttons_at(inputs,frame))
   finally:hook(None)
   expected=controls[frame-1];assert result['reference']==expected['reference']and len(pcm)//4==expected['frames']and sha(pcm)==expected['pcm_sha256'],('observer changed original frame',frame)
   if collecting and b.battle_prefix_complete():
    row=captured(start_frame);row['end_frame']=frame;rows.append(row);collecting=False
   if collecting:
    events=b.battle_world_dma_events();partial={'start_frame':start_frame,'end_frame':frame,'events':[[getattr(events[i],n)for n in FIELDS]for i in range(b.battle_prefix_count())]};(out/'in-progress.json').write_text(json.dumps(partial,separators=(',',':'))+'\n');assert b.battle_prefix_count()<8192 and frame-start_frame<8,('wrapper did not reach logic in bounded frames',start_frame,frame,b.battle_prefix_count())
  assert not collecting,('unfinished final natural wrapper',start_frame)
  checkpoint=ref.save()
  for busy in [False,True]:
   for pause in [0,255]:
    for empty in [False,True]:
     pair=[];private={'busy_at_wrapper_entry':busy,'pause_at_wrapper_entry':pause,'empty_queue_at_wrapper_entry':empty}
     for observed in [False,True]:
      ref.restore(checkpoint,restore_refresh_cycles=True);b.battle_world_dma_begin(1,int(busy),pause,int(empty),int(observed));hook(C.cast(b.battle_world_dma_hook,C.c_void_p))
      full=[]
      for frame in range(2401,2409):
       b.battle_world_dma_frame(frame)
       try:result,pcm=ref.step_audio(frame,buttons_at(inputs,frame))
       finally:hook(None)
       full.append((result['reference'],sha(pcm)))
       if observed and b.battle_prefix_complete():break
       # Both runs use the same finite frame window. Observation-off has no
       # event writes but still shares the same actual completion state.
       if not observed and b.battle_prefix_complete():break
       hook(C.cast(b.battle_world_dma_hook,C.c_void_p))
      pair.append(full)
      if observed:row=captured(2401,private);row['end_frame']=frame
     assert pair[0]==pair[1],('private observation changed frame',private);rows.append(row)
  identity=ref.identity
 raw=(json.dumps({'version':1,'fields':FIELDS,'cases':rows},separators=(',',':'))+'\n').encode();fixture=out/'native-world-dma.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0))
 report={'pass':True,'cases':len(rows),'events':sum(len(r['events'])for r in rows),'all_2400_video_ram_pcm_controls_equal':True,'private_observer_on_off_equal':True,'original_control_sha256':sha(controls_path.read_bytes()),'identity':identity,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),source,ROOT/'scripts/yuyuhakusho/battle-world-observer.c',ROOT/'scripts/yuyuhakusho/battle-prefix-observer.c']},'scope':'Actual wrapper and0654/DMA until3a2e entry, or busy return. No logical World execution or full frame timing reconstruction.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
