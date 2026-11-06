"""Original prepared frequency writers and complete pan automation callers."""
import argparse
import ctypes as C
import gzip
import importlib.util
import itertools
import json
from pathlib import Path

from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT,CPUHead
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
from inspect_dac_dsp_state import symbols
from score_parameter_tables import decode_score_pan_automation,PanState,advance_pan_automation

def module(name,file):
    s=importlib.util.spec_from_file_location(name,ROOT/'scripts/yuyuhakusho'/file);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
pan=module('frequency_pan','score-clock-pan-capture.py');psg=module('frequency_psg','score-clock-psg-capture.py')
VOICE=0xfff840


def cases():
    result=[]
    for channel,word in itertools.product([0,1,2,4,5,6,128,160,192,224],[0,1,8805,65535]):
        result.append({'kind':'frequency','channel':channel,'word':word})
    shapes={}
    for mode,on,noteoff,claimed,step,period,elapsed,limit in itertools.product(range(5),[False,True],[False,True],[False,True],[0,1,3,4],[1,255],[0,1,255],[0,3]):
        definition=decode_score_pan_automation(mode,1,0,limit,period)
        try:tick=advance_pan_automation(definition,PanState(step,elapsed),on,noteoff)
        except ValueError:continue
        reset=mode==4 or on and mode in[2,3]
        eq=reset or elapsed==period;overflow=eq and not reset and limit<step
        key=(mode,on,noteoff,claimed,reset,eq,overflow,tick.pan is not None)
        shapes.setdefault(key,{'kind':'automation','channel':0,'mode':mode,'on_note':on,'note_off':noteoff,'claimed':claimed,'step':step,'period':period,'elapsed':elapsed,'limit':limit,'definition':definition.to_data(),'state':{'step':step,'elapsed':elapsed},'after':{'step':tick.state.step,'elapsed':tick.state.elapsed},'output':tick.pan})
    result+=list(shapes.values());phases=[(0,0),(1,896),(1008,1008),(2048,2050)]
    return[{**r,'id':f'{i}-{r["kind"]}','origin':phases[i%4][0],'refresh':phases[i%4][1]}for i,r in enumerate(result)]


def setup(ref,checkpoint,case,lookup):
    ref.restore(checkpoint,restore_refresh_cycles=True);ref.api('psg_reset',None)()
    image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0];C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0;C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0]
    ref.cpu.cycles=case['origin'];ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,case['origin']);ref.cpu.cycles,ref.cpu.refresh_cycles=case['origin'],case['refresh']
    z=(C.c_ubyte*8192).in_dll(ref.lib,'zram');z[0x1f2a]=0;z[0x1ffd]=z[0x1ffe]=0
    write_cpu(ref,VOICE,(2 if case.get('note_off')else 0)|(4 if case.get('claimed')else 0));write_cpu(ref,VOICE+1,case['channel']);write_cpu(ref,VOICE+0x27,181);write_cpu(ref,0xfff80f,0)
    if case['kind']=='automation':
        for offset,value in[(0x1f,case['mode']),(0x20,1),(0x21,case['step']),(0x22,case['limit']),(0x23,case['period']),(0x24,case['elapsed'])]:write_cpu(ref,VOICE+offset,value)
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for i in range(15):setter(i,0)
    start=(0x37986 if case['channel']<128 else 0x3a0aa)if case['kind']=='frequency'else(0x37a8a if case['on_note']else 0x37aa4)
    for i,value in[(17,0x2700),(15,0xffff00),(13,VOICE),(6,case.get('word',0)),(16,start)]:setter(i,value)
    write_cpu(ref,0xffff00,0x3d8,4);return ref.save()


def execute(ref,case,fm=None,port=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[]
    for _ in range(256):
        pc=ref.get_register(16)&0xffffff
        if pc==0x3d8:return steps
        assert(0x37986<=pc<=0x379ac or 0x37a8a<=pc<=0x37b8e or 0x38b22<=pc<=0x38c2c or 0x3a0aa<=pc<=0x3a0d2),('escaped frequency/pan',case['id'],hex(pc))
        if fm:fm.score_clock_pc(pc)
        if port:port.score_clock_psg_pc(pc)
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles;run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('frequency/pan step cap',case['id']))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    inputs=cases();fm,_=pan.build(out);port,_=psg.build(out);controls={};rows=[];identities={};print(json.dumps({'cases':len(inputs)}),flush=True)
    for name,core in[('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint,lookup=quiet_checkpoint(ref),symbols(core);identities[name]=ref.identity
            for i,case in enumerate(inputs):
                initial=setup(ref,checkpoint,case,lookup);plain_steps=execute(ref,case);plain,plain_ram=ref.save(),ref.ram_cpu();plain_psg=psg.native_psg(ref)[1]
                again=setup(ref,checkpoint,case,lookup);assert initial['data']==again['data'];pan.shared.attach(fm,ref);address=C.addressof(ref.cpu);assert port.score_clock_psg_attach(address,address+CPUHead.cycles.offset,address+CPUHead.refresh_cycles.offset)
                try:steps=execute(ref,case,fm,port);events=pan.shared.events(fm);writes=psg.trace(port)
                finally:assert port.score_clock_psg_detach()==1 and fm.score_clock_detach()==1
                saved=ref.save();assert(steps==plain_steps and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and plain_ram==ref.ram_cpu()and plain_psg==psg.native_psg(ref)[1])
                result={'steps':steps,'events':[e for e in events if e['kind']!=7],'psg':writes,'end':{'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles},'step':ref.ram_cpu()[0xf861],'elapsed':ref.ram_cpu()[0xf864]}
                if name=='original':controls[i]=result
                else:
                    assert controls[i]==result,('pinned cores disagree',case['id']);rows.append({'input':case,**result,'native_observer_state_equal':True})
        print(json.dumps({'core':name,'cases':len(inputs),'pass':True}),flush=True)
    sources=[Path(__file__),ROOT/'scripts/yuyuhakusho/score_parameter_tables.py',ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-psg-capture.py',ROOT/'.local/md-yuyuhakusho/audio-score-parameters/parameter-instructions.txt',ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    payload={'version':1,'scope':'Prepared normalFM3-mode0 frequency writes and original pan automation callers; initialheld/mailbox0/noextraIRQ','identities':identities,'cases':rows,'sources':{str(p.relative_to(ROOT)):psg.sha(p.read_bytes())for p in sources}}
    target=out/'native-frequency-pan-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'cases':len(rows),'executions':len(rows)*4,'fixture_sha256':psg.sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
