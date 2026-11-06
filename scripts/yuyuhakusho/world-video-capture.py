"""Cold passive VDP/IRQ snapshots against the pinned original 2400-step control."""
import argparse,ctypes as C,gzip,hashlib,json,os,subprocess
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,CPUHead,buttons_at,sha

CORE=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
SOURCE=ROOT/'scripts/yuyuhakusho/world-video-observer.c'
NAMES=['kind','frame','pc','address','width','value','cpu_cycle','instruction_cycles','line_cycle','vcounter','hcounter','reg0','reg1','reg10','reg12','hint_pending','vint_pending','status_irq','z80_irq','sr','phase_word','world_flags','palette_flags','dma_end','dma_length','dma_mode']
class Event(C.Structure):
 _fields_=[(name,C.c_int32 if name in ['cpu_cycle','instruction_cycles']else C.c_uint32)for name in NAMES]

def capture(out,ack=False):
 assert out.is_relative_to(ROOT) and not (out/'capture.json').exists()
 assert sha(CORE.read_bytes())=='e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
 assert sha((ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes())=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 out.mkdir(parents=True,exist_ok=True);tmp=out/'tmp';tmp.mkdir(exist_ok=True)
 compiled=ROOT/'scripts/yuyuhakusho/world-video-observer-ack.c'if ack else SOURCE
 sources=list(dict.fromkeys([SOURCE,compiled,Path(__file__),ROOT/'scripts/yuyuhakusho/reference.py',ROOT/'scripts/yuyuhakusho/extract_audio_assets.py']))
 source_hashes={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sources}
 (out/'executed-producer.py.txt').write_bytes(Path(__file__).read_bytes())
 (out/'executed-observer.c.txt').write_bytes(compiled.read_bytes())
 if ack:(out/'included-base-observer.c.txt').write_bytes(SOURCE.read_bytes())
 env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache'))
 binary=tmp/'world-video.dylib'
 arguments=['clang','-dynamiclib','-O2','-fno-modules','-I'+str(CORE.parent/'core/z80')]
 if ack:arguments+=['-DM68K_OVERCLOCK_SHIFT=20','-I'+str(CORE.parent/'core'),'-I'+str(CORE.parent/'core/m68k')]
 subprocess.run([*arguments,str(compiled),'-o',str(binary)],cwd=ROOT,env=env,check=True)
 b=C.CDLL(str(binary));b.world_video_attach.argtypes=[C.c_void_p]*15;b.world_video_attach.restype=C.c_int
 b.world_video_begin.argtypes=[C.c_uint];b.world_video_count.restype=C.c_size_t;b.world_video_events.restype=C.POINTER(Event)
 b.world_video_overflow.restype=C.c_uint;b.world_video_size.restype=C.c_uint
 if ack:
  b.world_video_attach_ack.argtypes=[C.c_void_p,C.c_void_p];b.world_video_attach_ack.restype=C.c_int;b.world_video_detach_ack.restype=C.c_int
 assert b.world_video_size()==C.sizeof(Event)
 expected_path=ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-baseline-original.json.gz'
 controls=json.loads(gzip.decompress(expected_path.read_bytes()));inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
 selected=set(range(2068,2075))|set(range(2393,2401));rows=[];all_events=[]
 with AudioSession(ROOT,out/'reference',core=CORE)as ref:
  pointers=[]
  for name,typ in [('reg',C.c_ubyte),('h_counter',C.c_ubyte),('v_counter',C.c_ushort),('hint_pending',C.c_ubyte),('vint_pending',C.c_ubyte),('status',C.c_ushort),('mcycles_vdp',C.c_uint),('Z80',C.c_ubyte),('dma_endCycles',C.c_uint),('dma_length',C.c_uint),('dma_type',C.c_ubyte)]:
   pointers.append(typ.in_dll(ref.lib,name))
  r,h,v,ih,iv,st,lc,z,de,dl,dt=map(C.addressof,pointers)
  getreg=ref.api('m68k_get_reg',C.c_uint,C.c_uint);ic=ref.api('m68k_cycles',C.c_int)
  delegate=ref.api('vdp_68k_irq_ack',C.c_int,C.c_int)if ack else None
  assert b.world_video_attach(r,h,v,ih,iv,st,lc,C.addressof(ref.cpu)+CPUHead.cycles.offset,ref.ram_pointer,z,C.cast(getreg,C.c_void_p),C.cast(ic,C.c_void_p),de,dl,dt)
  for frame in range(1,2401):
   observed=frame in selected
   if observed:
    b.world_video_begin(frame)
    if ack:assert b.world_video_attach_ack(C.addressof(ref.cpu),C.cast(delegate,C.c_void_p)),('Native ack ABI or original callback mismatch',frame)
    ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.world_video_hook_ack if ack else b.world_video_hook,C.c_void_p))
   try:result,pcm=ref.step_audio(frame,buttons_at(inputs,frame))
   finally:
    if observed:
     ref.api('set_cpu_hook',None,C.c_void_p)(None)
     if ack:assert b.world_video_detach_ack(),('Native delegated callback changed',frame)
     b.world_video_end()
   control=controls[frame-1]
   assert result['reference']==control['reference'],('reference changed',frame)
   assert len(pcm)//4==control['frames'] and sha(pcm)==control['pcm_sha256'],('PCM changed',frame)
   if observed:
    assert b.world_video_overflow()==0,('observer overflow',frame)
    p=b.world_video_events();events=[[getattr(p[i],name)for name in NAMES]for i in range(b.world_video_count())]
    all_events+=events
    rows.append({'frame':frame,'reference_equal':True,'pcm_equal':True,'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm),'events':len(events)})
   if frame%1000==0:print(json.dumps({'frame':frame,'events':len(all_events)}),flush=True)
  identity=ref.identity
 raw=(json.dumps({'fields':NAMES,'events':all_events},separators=(',',':'))+'\n').encode()
 artifact=out/'events.json.gz';artifact.write_bytes(gzip.compress(raw,mtime=0))
 assert source_hashes=={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in sources}
 report={'pass':True,'scope':'Passive selected natural VDP snapshots and CPU IO; does not infer CPU acceptance independently','delegated_original_ack':ack,'batch_target_recorded':ack,'checked_frames':2400,'selected_frames':sorted(selected),'all_reference_equal':True,'all_pcm_equal':True,'observer_overflow':0,'identity':identity,'events':len(all_events),'event_abi_bytes':C.sizeof(Event),'frame_checks':rows,'original_control':{'path':str(expected_path.relative_to(ROOT)),'sha256':sha(expected_path.read_bytes())},'source_sha256':source_hashes,'artifacts':[{'path':str(artifact.relative_to(ROOT)),'sha256':sha(artifact.read_bytes()),'json_sha256':sha(raw),'bytes':artifact.stat().st_size}],'production_modified':False,'global_irq_acceptance_proved':False}
 (out/'capture.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({'pass':True,'selected_frames':len(selected),'events':len(all_events)}))

if __name__=='__main__':
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--ack',action='store_true');a=p.parse_args();capture(a.out.resolve(),a.ack)
