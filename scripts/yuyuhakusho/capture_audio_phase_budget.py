#!/usr/bin/env python3
"""Research-only sound-write audit and semantic arena-block timing oracle."""
import argparse
import ctypes as C
import gzip
import itertools
import json
import os
from pathlib import Path
import subprocess

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT, CPUHead, buttons_at, sha

CORE=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
SOURCE=ROOT/'scripts/yuyuhakusho/audio_phase_budget_bridge.c'
FIELDS=[0xf800,0xf807,0xf809,0xf80a,0xf80b,0xf80c,0xf80d]
class Event(C.Structure):
 _fields_=[(name,C.c_int32 if name in ['cycles','refresh']else C.c_uint32)for name in ['kind','pc','address','width','value','frame','cycles','refresh','callback_id','callback_behavior','irq_depth','arena','b0','b1','b2','b3','b4','b5','b6','a0','a1','a2','a3']]

def inside(path):
 path=path.resolve();assert path.is_relative_to(ROOT),path;return path

def owned(ram):return[ram[p]for p in FIELDS]
def pack(e):return[getattr(e,name)for name,_ in Event._fields_]
def write_json(path,value):path.write_text(json.dumps(value,indent=2)+'\n')
def save(path,value):
 raw=(json.dumps(value,separators=(',',':'))+'\n').encode();path.write_bytes(gzip.compress(raw,mtime=0));return{'path':str(path.relative_to(ROOT)),'json_sha256':sha(raw),'gzip_sha256':sha(path.read_bytes())}

def bridge(out):
 tmp=out/'tmp';tmp.mkdir(exist_ok=True)
 env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache'))
 lib=out/'audio-phase-budget.dylib'
 subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(SOURCE),'-o',str(lib)],cwd=ROOT,env=env,check=True)
 b=C.CDLL(str(lib));b.phase_memory.argtypes=[C.c_void_p,C.c_void_p,C.c_void_p,C.c_void_p]
 b.phase_begin.argtypes=[C.c_uint];b.phase_stop_at.argtypes=[C.c_uint]
 b.phase_count.restype=C.c_size_t;b.phase_overflow.restype=C.c_uint;b.phase_events.restype=C.POINTER(Event)
 return b,lib

def bind(b,ref):
 addr=C.addressof(ref.cpu)
 b.phase_memory(ref.ram_pointer,addr+CPUHead.dar.offset,addr+CPUHead.cycles.offset,addr+CPUHead.refresh_cycles.offset)
 b.phase_reset()

def rows(b):
 assert b.phase_overflow()==0,'C recorder overflow'
 p=b.phase_events();return[pack(p[i])for i in range(b.phase_count())]

def replay(before,events,after):
 current=list(before)
 for event in events:
  expected=event[12:19]
  assert current==expected,('unexplained owned state transition before event',event,current,expected)
  if event[0]==2:
   _,pc,address,width,value=event[:5]
   assert width in[1,2,4]
   raw=(value&((1<<(width*8))-1)).to_bytes(width,'big')
   for index,byte in enumerate(raw):
    address_byte=(address+index)&65535
    if address_byte in FIELDS:current[FIELDS.index(address_byte)]=byte
 assert current==after,('unobserved final owned write',current,after)
 return current

def natural(out,b,kind,frames):
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
 if kind=='pause':inputs +=[{'start':3000,'end':3004,'buttons':[3]},{'start':3040,'end':3044,'buttons':[3]}]
 expected_path=ROOT/f'games/yuyuhakusho/engine/testdata/audio-timing-{kind}-original.json.gz'
 expected=json.loads(gzip.decompress(expected_path.read_bytes()))
 assert frames<=len(expected)
 all_events=[];frame_states=[]
 with AudioSession(ROOT,out/('core-'+kind),core=CORE)as ref:
  bind(b,ref);b.phase_stop_at(0)
  ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.phase_cpu_hook,C.c_void_p))
  initial=owned(ref.ram_cpu());previous=initial
  for frame in range(1,frames+1):
   b.phase_begin(frame)
   result,pcm=ref.step_audio(frame,buttons_at(inputs,frame));control=expected[frame-1]
   assert result['reference']==control['reference'],('RAM/video changed',kind,frame)
   assert len(pcm)//4==control['frames']and sha(pcm)==control['pcm_sha256'],('PCM changed',kind,frame)
   events=rows(b);after=owned(ref.ram_cpu());replay(previous,events,after)
   all_events+=events;frame_states.append(after);previous=after
   if frame%1000==0:print(json.dumps({'kind':kind,'frame':frame,'events':len(all_events)}),flush=True)
  ref.api('set_cpu_hook',None,C.c_void_p)(None);identity=ref.identity
 payload={'schema':1,'event_fields':[name for name,_ in Event._fields_],'owned_fields':FIELDS,'initial':initial,'events':all_events,'frame_states':frame_states}
 artifact=save(out/f'audio-phase-{kind}.json.gz',payload)
 report={'pass':True,'scenario':kind,'frames':frames,'identity':identity,'original':{'path':str(expected_path.relative_to(ROOT)),'sha256':sha(expected_path.read_bytes())},'artifact':artifact,'events':len(all_events),'sound_writes':sum(e[0]==2 for e in all_events),'unexplained_owned_writes':0,'ram_video_pcm_equal':True,'producer_sha256':sha(Path(__file__).read_bytes()),'bridge_source_sha256':sha(SOURCE.read_bytes()),'limits':['All writes to declared sound fields are recorded including unchanged values; this is a passive original oracle, not an independent timing implementation','Private model and semantic equivalence analysis are separate obligations']}
 write_json(out/f'audio-phase-{kind}-manifest.json',report)
 return report

def private(out,b):
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
 expected=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-baseline-original.json.gz').read_bytes()))
 cases=[]
 with AudioSession(ROOT,out/'private-core',core=CORE)as ref:
  for frame in range(1,2401):
   r,pcm=ref.step_audio(frame,buttons_at(inputs,frame));assert r['reference']==expected[frame-1]['reference'];assert sha(pcm)==expected[frame-1]['pcm_sha256']
  checkpoint=ref.save();bind(b,ref);b.phase_stop_at(0x3d8)
  setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
  for seed,(pause,slow,stage,used,pan,x,deadline)in enumerate(itertools.product([0,128],[0,1],[0,2],[0,8,64],[0,1],[744,984],[0,896])):
   ref.restore(checkpoint,restore_refresh_cycles=True)
   for addr,value,size in [(0xf550,pause,1),(0xf55a,slow,1),(0x8186,stage,2),(0xa00e,used,2),(0xa014,x,2),(0xa016,0,2),(0xf572,pan,2),(0xf566,0,2)]:write_cpu(ref,0xff0000+addr,value,size)
   for index in range(16):
    write_cpu(ref,0xffa800+index*4,(900+seed%60+index)<<16,4)
    write_cpu(ref,0xffa840+index*4,(300+seed%40+index)<<16,4)
   setreg(17,0x2700);setreg(15,0xffff00);setreg(13,0xffff8180);setreg(16,0x20918);write_cpu(ref,0xffff00,0x3d8,4)
   ref.cpu.cycles=0;ref.cpu.refresh_cycles=deadline
   before=owned(ref.ram_cpu());b.phase_reset();b.phase_begin(seed+1)
   ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.phase_cpu_hook,C.c_void_p));execute(300000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
   events=rows(b);after=owned(ref.ram_cpu());replay(before,events,after)
   assert events[-1][0]==12 and events[-1][19]==0xffff04,('unclosed private arena',seed,events[-1])
   assert not any(e[0]in[2,9]for e in events),('arena accessed owned sound state',seed)
   cases.append({'seed':seed,'pause':pause,'slow':slow,'stage':stage,'used':used,'pan':pan,'x':x,'refresh_deadline':deadline,'events':events})
  identity=ref.identity
 artifact=save(out/'audio-phase-arena-private.json.gz',{'schema':1,'cases':cases})
 report={'pass':True,'identity':identity,'cases':len(cases),'artifact':artifact,'owned_sound_accesses':0,'producer_sha256':sha(Path(__file__).read_bytes()),'bridge_source_sha256':sha(SOURCE.read_bytes()),'scope':'Original20918 semantic block cost oracle; isolatedSR2700, actual branch/loop execution, no production cost model'}
 write_json(out/'audio-phase-arena-private-manifest.json',report)
 return report

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--mode',choices=['baseline','pause','private','all'],default='all');args=p.parse_args()
 out=inside(args.out);out.mkdir(parents=True,exist_ok=True);b,lib=bridge(out);reports=[]
 if args.mode in['baseline','all']:reports.append(natural(out,b,'baseline',5500))
 if args.mode in['pause','all']:reports.append(natural(out,b,'pause',3602))
 if args.mode in['private','all']:reports.append(private(out,b))
 write_json(out/'capture.json',{'pass':True,'reports':reports,'bridge_sha256':sha(lib.read_bytes()),'producer_sha256':sha(Path(__file__).read_bytes())})
 print(json.dumps({'pass':True,'reports':len(reports)}),flush=True)
if __name__=='__main__':main()
