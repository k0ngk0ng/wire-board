"""Original fixed tick-gate/RTS/priority-store boundaries, no driver VM."""
import argparse,ctypes as C,gzip,json,os,subprocess,importlib.util
from pathlib import Path
from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT,CPUHead,sha
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
spec=importlib.util.spec_from_file_location('score_clock_capture',ROOT/'scripts/yuyuhakusho/score-clock-capture.py');shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)
STOPS={0x375ba:1,0x3d8:2,0x37ba8:3,0x37c0c:4,0x390fa:5}
def setup(ref,checkpoint,case):
 ref.restore(checkpoint,restore_refresh_cycles=True);ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['refresh']
 setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
 start={'gate':0x375ae,'return':0x37656,'priority':0x390f6}[case['kind']]
 for index,value in [(17,0x2700),(15,0xffff00),(16,start)]:setreg(index,value)
 write_cpu(ref,0xffff00,0x3d8,4);write_cpu(ref,0xfff807,case['pause'],1);write_cpu(ref,0xfff80e,0xc0,1);write_cpu(ref,0xfff800,0x5a,1)
 return ref.save()
def execute(ref,b=None):
 call=ref.api('m68k_run',None,C.c_uint);steps=[]
 for _ in range(24):
  pc=ref.get_register(16)&0xffffff
  if pc in STOPS:return steps,STOPS[pc]
  assert 0x375ae<=pc<0x375ba or 0x37b90<=pc<=0x37c08 or pc in[0x37656,0x390f6],('Escaped fixed gate',hex(pc))
  before=ref.cpu.cycles;deadline=ref.cpu.refresh_cycles
  if b:b.score_clock_pc(pc)
  call(before+1);steps.append([pc,before,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles])
 raise AssertionError('Fixed native boundary exceeded declared steps')
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True);tmp=out/'tmp';tmp.mkdir(exist_ok=True)
 src=ROOT/'scripts/yuyuhakusho/score-clock-tick-bridge.c';lib=out/'tick.dylib';subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(src),'-o',str(lib)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(lib));b.score_clock_attach.argtypes=[C.c_void_p]*9;b.score_clock_attach.restype=C.c_int;b.score_clock_pc.argtypes=[C.c_uint];b.score_clock_count.restype=C.c_size_t;b.score_clock_events.restype=C.POINTER(shared.Event);b.score_clock_overflow.restype=C.c_uint;b.score_clock_detach.restype=C.c_int
 phases=[(0,0),(1,896),(895,896),(1008,1008),(2048,2050)]
 cases=[{'kind':'gate','pause':v,'origin':0,'refresh':0}for v in range(256)]
 cases +=[{'kind':'gate','pause':v,'origin':origin,'refresh':deadline}for v in [0,1,127,128,254,255]for origin,deadline in phases[1:]]
 cases +=[{'kind':kind,'pause':0,'origin':origin,'refresh':deadline}for kind in['return','priority']for origin,deadline in phases]
 controls={};rows=[];identities={}
 for mode,core in [('original',ORIGINAL),('hook',OBSERVED)]:
  with AudioSession(ROOT,out/('core-'+mode),core=core)as ref:
   checkpoint=quiet_checkpoint(ref);identities[mode]=ref.identity
   for index,case in enumerate(cases):
    initial=setup(ref,checkpoint,case);plain,route=execute(ref);plain_save=ref.save();ram=ref.ram_cpu()
    rebuilt=setup(ref,checkpoint,case);assert initial['data']==rebuilt['data'],'Private start differs'
    shared.attach(b,ref)
    if mode=='hook':ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.score_clock_tick_hook,C.c_void_p))
    try:observed,again=execute(ref,b);events=shared.events(b)
    finally:
     if mode=='hook':ref.api('set_cpu_hook',None,C.c_void_p)(None)
     assert b.score_clock_detach()==1
    after=ref.save();assert plain==observed and again==route and plain_save['data']==after['data']and plain_save['refresh_cycles']==after['refresh_cycles']and ram==ref.ram_cpu(),('Recorder changed original',case)
    end={'cycle':ref.cpu.cycles,'refresh':ref.cpu.refresh_cycles}
    if mode=='original':controls[index]={'steps':plain,'route':route,'end':end}
    else:
     assert controls[index]=={'steps':observed,'route':route,'end':end},('Two core builds differ',case)
     rows.append({'input':case,'route':route,'steps':observed,'events':events,'end':end,'all_native_state_equal':True})
  print(json.dumps({'mode':mode,'cases':len(cases),'pass':True}),flush=True)
 target=out/'native-boundaries-full.json.gz';shared.saved(target,{'schema':1,'scope':'Private SR2700 fixed tick gate/store/return boundaries; not duck/fade/tempo/whole tick or IRQ schedule','identity':identities,'cases':rows})
 shared.saved(out/'capture.json',{'pass':True,'cases':len(rows),'gate_cases':sum(x['input']['kind']=='gate'for x in rows),'two_native_builds':True,'recorder_on_off_calls':len(rows)*4,'identity':identities,'artifact':{'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())},'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[src,ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',Path(__file__)]}})
 print(json.dumps({'pass':True,'cases':len(rows),'events':sum(len(x['events'])for x in rows)}),flush=True)
if __name__=='__main__':main()
