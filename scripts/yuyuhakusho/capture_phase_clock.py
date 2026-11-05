"""Capture bank steals and original semantic/IRQ/sound events in one ledger."""
import argparse,ctypes as C,gzip,json,os,subprocess
from collections import Counter
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,CPUHead,buttons_at,sha
from capture_audio_phase_budget import Event,owned,pack,replay,bind

SOURCE=ROOT/'scripts/yuyuhakusho/phase_clock_bridge.c'
CORE=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
PINNED_CORE_SHA='e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
BUS_FIELDS=['ordinal','frame','kind','guest_address','mapped_address','value','z80_pc','bank','m68k_before','m68k_after','z80_before','z80_after','dma_end','dma_type','dma_length','zstate_before','zstate_after']
class Bus(C.Structure):
 _fields_=[(name,C.c_int32 if name in['m68k_before','m68k_after']else C.c_uint32)for name in BUS_FIELDS]

def inside(path):
 p=path.resolve();assert p.is_relative_to(ROOT),p;return p
def store(path,value):
 raw=(json.dumps(value,separators=(',',':'))+'\n').encode();data=gzip.compress(raw,mtime=0);path.write_bytes(data)
 return {'path':str(path.relative_to(ROOT)),'json_sha256':sha(raw),'gzip_sha256':sha(data),'json_bytes':len(raw),'gzip_bytes':len(data)}
def build(out):
 tmp=out/'tmp';tmp.mkdir(parents=True,exist_ok=True)
 env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache'))
 lib=out/'phase-clock.dylib';headers=CORE.parent/'core/z80'
 subprocess.run(['clang','-dynamiclib','-O2','-fno-modules','-I'+str(headers),str(SOURCE),'-o',str(lib)],cwd=ROOT,env=env,check=True)
 b=C.CDLL(str(lib));b.phase_memory.argtypes=[C.c_void_p]*4;b.phase_clock_attach.argtypes=[C.c_void_p]*8;b.phase_clock_attach.restype=C.c_int
 b.phase_clock_detach.restype=C.c_int;b.phase_clock_begin.argtypes=[C.c_uint];b.phase_stop_at.argtypes=[C.c_uint]
 b.phase_count.restype=C.c_size_t;b.phase_overflow.restype=C.c_uint;b.phase_events.restype=C.POINTER(Event)
 b.phase_clock_bus_count.restype=C.c_size_t;b.phase_clock_bus_events.restype=C.POINTER(Bus);b.phase_clock_bus_overflow.restype=C.c_uint
 b.phase_clock_bus_size.restype=C.c_uint;b.phase_clock_z80_size.restype=C.c_uint
 assert b.phase_clock_bus_size()==C.sizeof(Bus);assert b.phase_clock_z80_size()==88
 return b,lib

def capture(out,b,scenario,frames):
 original=ROOT/f'games/yuyuhakusho/engine/testdata/audio-timing-{scenario}-original.json.gz'
 controls=json.loads(gzip.decompress(original.read_bytes()));assert frames<=len(controls)
 old_path=ROOT/f'docs/research/md-yuyuhakusho/audio-phase-budget/compact/audio-phase-{scenario}.json.gz'
 old=json.loads(gzip.decompress(old_path.read_bytes()));old_cursor=0
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
 if scenario=='pause':inputs +=[{'start':3000,'end':3004,'buttons':[3]},{'start':3040,'end':3044,'buttons':[3]}]
 events=[];bus_events=[];raw_count=0;frame_rows=[]
 with AudioSession(ROOT,out/('core-'+scenario),core=CORE)as ref:
  bind(b,ref);b.phase_stop_at(0);before=initial=owned(ref.ram_cpu())
  slots=[C.c_void_p.in_dll(ref.lib,name)for name in ['z80_readmem','z80_writemem']]
  refs=[C.c_byte.in_dll(ref.lib,'Z80'),C.c_uint.in_dll(ref.lib,'zbank'),C.c_uint.in_dll(ref.lib,'dma_endCycles'),C.c_ubyte.in_dll(ref.lib,'dma_type'),C.c_uint.in_dll(ref.lib,'dma_length'),C.c_ubyte.in_dll(ref.lib,'zstate')]
  assert b.phase_clock_attach(*[C.addressof(x)for x in slots+refs])
  ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.phase_cpu_hook,C.c_void_p))
  try:
   for frame in range(1,frames+1):
    b.phase_clock_begin(frame)
    result,pcm=ref.step_audio(frame,buttons_at(inputs,frame));control=controls[frame-1]
    assert result['reference']==control['reference'],('Observer changed RAM/video/registers',scenario,frame)
    assert len(pcm)//4==control['frames']and sha(pcm)==control['pcm_sha256'],('Observer changed PCM',scenario,frame)
    assert b.phase_overflow()==0 and b.phase_clock_bus_overflow()==0,'Observer overflow'
    p=b.phase_events();local=[pack(p[i])for i in range(b.phase_count())];after=owned(ref.ram_cpu());replay(before,local,after);before=after
    old_start=old_cursor
    while old_cursor<len(old['events'])and old['events'][old_cursor][5]==frame:old_cursor+=1
    projection=[e for e in local if e[0]not in[13,14]and not(e[0]==1 and e[1]&0xffffff==0x21a06)]
    assert projection==[e[:23]for e in old['events'][old_start:old_cursor]],('Existing frozen phase ledger changed',scenario,frame)
    bp=b.phase_clock_bus_events();first_bus=len(bus_events)
    for i in range(b.phase_clock_bus_count()):
     row=[getattr(bp[i],name)for name in BUS_FIELDS];row[0]+=raw_count;bus_events.append(row)
    for index,e in enumerate(local):
     if e[0]==1 and e[1]&0xffffff==0x21a06:continue
     if e[0]in[13,14]:e[19]+=first_bus
     events.append(e+[raw_count+index])
    raw_count+=len(local)
    frame_rows.append({'frame':frame,'ram_video_registers_equal':True,'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm),'bus_transactions':b.phase_clock_bus_count(),'sound_fields':after})
    if frame%1000==0:print(json.dumps({'scenario':scenario,'frame':frame,'retained_events':len(events),'bus_transactions':len(bus_events)}),flush=True)
  finally:
   ref.api('set_cpu_hook',None,C.c_void_p)(None);assert b.phase_clock_detach()==1,'Delegated handler changed during observation'
  identity=ref.identity
 payload={'schema':1,'event_fields':[name for name,_ in Event._fields_]+['source_ordinal'],'bus_fields':BUS_FIELDS,'owned_fields':[0xf800,0xf807,0xf809,0xf80a,0xf80b,0xf80c,0xf80d],'initial':initial,'events':events,'bus_events':bus_events}
 artifact=store(out/f'phase-clock-{scenario}.json.gz',payload)
 frame_artifact=store(out/f'phase-clock-{scenario}-frames.json.gz',{'scenario':scenario,'frames':frame_rows})
 report={'pass':True,'scenario':scenario,'frames':frames,'identity':identity,'control':{'path':str(original.relative_to(ROOT)),'sha256':sha(original.read_bytes())},'old_phase_ledger':{'path':str(old_path.relative_to(ROOT)),'sha256':sha(old_path.read_bytes()),'projected_events_equal':old_cursor},'all_frame_ram_video_pcm_equal':True,'full_old_sound_irq_semantic_ledger_equal':True,'observer_overflows':0,'raw_ledger_count':raw_count,'retained_ledger_count':len(events),'bank_transaction_count':len(bus_events),'kinds':dict(Counter(e[2]for e in bus_events)),'artifacts':[artifact,frame_artifact]}
 (out/f'phase-clock-{scenario}-manifest.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({'pass':True,'scenario':scenario,'frames':frames,'bus_transactions':len(bus_events)}),flush=True)
 return report

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--scenario',choices=['baseline','pause','all'],default='all');parser.add_argument('--frames',type=int);args=parser.parse_args()
 assert sha(CORE.read_bytes())==PINNED_CORE_SHA,'Pinned reference core changed'
 out=inside(args.out);out.mkdir(parents=True,exist_ok=True);b,lib=build(out)
 reports=[]
 for name,total in [('baseline',5500),('pause',3602)]:
  if args.scenario not in[name,'all']:continue
  reports.append(capture(out,b,name,args.frames or total))
 report={'pass':True,'scope':'Passive original bank transactions joined with the frozen sound/IRQ/semantic ledger; not an independent timing runtime','reports':reports,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[SOURCE,Path(__file__),ROOT/'scripts/yuyuhakusho/audio_phase_budget_bridge.c',ROOT/'scripts/yuyuhakusho/check_phase_clock.py']},'bridge_binary_sha256':sha(lib.read_bytes()),'whole_goal_complete':False,'global_independent_schedule_solved':False}
 (out/'capture.json').write_text(json.dumps(report,indent=2)+'\n')
if __name__=='__main__':main()
