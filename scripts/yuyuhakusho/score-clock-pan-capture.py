"""Original voice/instrument pan routing and actual DAC marker IO, research only."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
import os
import subprocess
from pathlib import Path

from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
from inspect_dac_dsp_state import symbols

spec=importlib.util.spec_from_file_location('pan_shared',ROOT/'scripts/yuyuhakusho/score-clock-capture.py');shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)
VOICE=0xfff840


def cases():
    result=[]
    for entry in ['voice','instrument']:
        for channel in [0,1,2,4,5,6]:
            for marker in [0,1,128,255]:
                for pan in [192,181]:result.append({'entry':entry,'channel':channel,'marker':marker,'pan':pan,'claimed':False})
    for channel in [0,1,2,4,5,6]:
        for pan in [192,181]:result.append({'entry':'voice','channel':channel,'marker':1,'pan':pan,'claimed':True})
    phases=[(0,0),(1,896),(1008,1008),(2048,2050)]
    return[{**row,'id':f"{i}-{row['entry']}-c{row['channel']}-m{row['marker']}-p{row['pan']}",'origin':phases[i%4][0],'refresh':phases[i%4][1]}for i,row in enumerate(result)]


def build(out):
    tmp=out/'tmp';tmp.mkdir();library=out/'pan-observer.dylib';source=ROOT/'scripts/yuyuhakusho/score-clock-pan-observer.c'
    subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(library)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
    b=C.CDLL(str(library));b.score_clock_attach.argtypes=[C.c_void_p]*9;b.score_clock_attach.restype=C.c_int;b.score_clock_pc.argtypes=[C.c_uint];b.score_clock_count.restype=C.c_size_t;b.score_clock_events.restype=C.POINTER(shared.Event);b.score_clock_overflow.restype=C.c_uint;b.score_clock_detach.restype=C.c_int;b.score_clock_pan_enable.restype=C.c_int;b.score_clock_pan_disable.restype=C.c_int
    return b,library


def setup(ref,checkpoint,case,lookup):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0]
    C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0;C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0]
    ref.cpu.cycles=case['origin'];ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,case['origin']);ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['refresh']
    write_cpu(ref,VOICE,4 if case['claimed'] else 0);write_cpu(ref,VOICE+1,case['channel']);write_cpu(ref,VOICE+0x27,case['pan'])
    zram=(C.c_ubyte*8192).in_dll(ref.lib,'zram');zram[0x1ffd]=case['marker'];zram[0x1ff8]=64;zram[0x1f2a]=0
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for index in range(15):setter(index,0)
    for index,value in[(17,0x2700),(15,0xffff00),(13,VOICE),(1,case['pan']),(16,0x37b34 if case['entry']=='voice'else 0x38fba)]:setter(index,value)
    write_cpu(ref,0xffff00,0x3d8,4);return ref.save()


def execute(ref,case,b=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[]
    for _ in range(1000):
        pc=ref.get_register(16)&0xffffff
        if pc==0x3d8:return steps
        assert 0x37b34<=pc<=0x37b8e or 0x38fba<=pc<=0x39012 or 0x38b22<=pc<=0x38c2c,('escaped pan phase',case['id'],hex(pc))
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles
        if b:b.score_clock_pc(pc)
        run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('pan step cap',case['id']))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);b,library=build(out);inputs=cases();assert len(inputs)==108;control={};rows=[];identities={}
    for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint=quiet_checkpoint(ref);lookup=symbols(core);identities[name]=ref.identity
            for i,case in enumerate(inputs):
                initial=setup(ref,checkpoint,case,lookup);plain=execute(ref,case);plain_state=ref.save();plain_ram=ref.ram_cpu()
                again=setup(ref,checkpoint,case,lookup);assert initial['data']==again['data'];shared.attach(b,ref);assert b.score_clock_pan_enable()==1
                if name=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.score_clock_hook,C.c_void_p))
                try:steps=execute(ref,case,b);events=shared.events(b)
                finally:
                    if name=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(None)
                    assert b.score_clock_pan_disable()==1 and b.score_clock_detach()==1
                saved=ref.save();assert plain==steps and plain_state['data']==saved['data']and plain_state['refresh_cycles']==saved['refresh_cycles']and plain_ram==ref.ram_cpu(),('observer changed pan',case['id'])
                zram=(C.c_ubyte*8192).in_dll(ref.lib,'zram');result={'steps':steps,'events':[e for e in events if e['kind']!=7],'end':{'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles},'restore_pan':int(zram[0x1ff8]),'marker':int(zram[0x1ffd])}
                if name=='original':control[i]=result
                else:
                    assert control[i]==result,('pinned cores disagree',case['id']);rows.append({'input':case,**result,'native_observer_state_equal':True})
        print(json.dumps({'core':name,'cases':len(inputs),'pass':True}),flush=True)
    sources=[Path(__file__),ROOT/'scripts/yuyuhakusho/score-clock-pan-observer.c',ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    payload={'version':1,'scope':'Legal FM voice rows in private held-bus/mailbox0 no-extraIRQ profiles; live currentmarker0/1/128/255','identities':identities,'cases':rows,'sources':{str(p.relative_to(ROOT)):shared.sha(p.read_bytes())for p in sources}}
    target=out/'native-pan-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'cases':len(rows),'executions':len(rows)*4,'fixture_sha256':shared.sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
