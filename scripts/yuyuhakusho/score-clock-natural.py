"""Small natural pause/priority IO windows, reusing old complete controls."""
import argparse,ctypes as C,gzip,json,os,subprocess,importlib.util
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,buttons_at,sha
from research_audio_loops import OBSERVED
spec=importlib.util.spec_from_file_location('score_clock_capture',ROOT/'scripts/yuyuhakusho/score-clock-capture.py');capture=importlib.util.module_from_spec(spec);spec.loader.exec_module(capture)
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 source=ROOT/'scripts/yuyuhakusho/score-clock-natural-bridge.c';tmp=out/'tmp';tmp.mkdir(exist_ok=True);lib=out/'natural.dylib'
 subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(lib)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(lib));b.score_clock_attach.argtypes=[C.c_void_p]*9;b.score_clock_attach.restype=C.c_int;b.score_clock_count.restype=C.c_size_t;b.score_clock_events.restype=C.POINTER(capture.Event);b.score_clock_overflow.restype=C.c_uint;b.score_clock_detach.restype=C.c_int;b.score_clock_natural_memory.argtypes=[C.c_void_p]
 expected_path=ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-pause-original.json.gz';expected=json.loads(gzip.decompress(expected_path.read_bytes()))
 oldpath=ROOT/'docs/research/md-yuyuhakusho/audio-phase-budget/compact/audio-phase-pause.json.gz';old=json.loads(gzip.decompress(oldpath.read_bytes()))
 priority=sorted({e[5]for e in old['events']if e[0]==2 and e[2]&0xffff==0xf800 and e[4]==0 and e[5]>1500})[:8]
 selected=set(range(2998,3044))|set(priority)
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())+[{'start':3000,'end':3004,'buttons':[3]},{'start':3040,'end':3044,'buttons':[3]}]
 frames=[]
 with AudioSession(ROOT,out/'native-core',core=OBSERVED)as ref:
  capture.attach(b,ref);b.score_clock_natural_memory(ref.ram_pointer)
  ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.score_clock_natural_hook,C.c_void_p))
  try:
   for frame in range(1,3603):
    b.score_clock_reset();result,pcm=ref.step_audio(frame,buttons_at(inputs,frame));control=expected[frame-1]
    assert result['reference']==control['reference']and sha(pcm)==control['pcm_sha256']and len(pcm)//4==control['frames'],('Passive IO observer changed original',frame)
    assert not b.score_clock_overflow()
    if frame in selected:frames.append({'frame':frame,'events':capture.events(b),'reference':result['reference'],'pcm_sha256':sha(pcm),'stereo_frames':len(pcm)//4})
  finally:
   ref.api('set_cpu_hook',None,C.c_void_p)(None);assert b.score_clock_detach()==1
  identity=ref.identity
 target=out/'natural-windows.json.gz';capture.saved(target,{'scope':'Selected actual natural pause/priority port windows only; no bank recapture or autonomous schedule','frames':frames})
 report={'pass':True,'all_original_control_steps_equal':3602,'selected_priority_frames':priority,'selected_frames':sorted(selected),'window_frames':len(frames),'events':sum(len(f['events'])for f in frames),'identity':identity,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[source,ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',Path(__file__)]},'existing_control':{'path':str(expected_path.relative_to(ROOT)),'sha256':sha(expected_path.read_bytes())},'artifact':{'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())},'whole_driver_timing_verified':False,'independent_schedule_solved':False}
 capture.saved(out/'capture.json',report);print(json.dumps({'pass':True,'selected_frames':len(frames),'events':report['events']}))
if __name__=='__main__':main()
