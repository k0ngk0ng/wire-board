"""Original FM/PSG key and gate leafs with actual hardware bus/port IO."""
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

def module(name,file):
    spec=importlib.util.spec_from_file_location(name,ROOT/'scripts/yuyuhakusho'/file);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
pan=module('key_pan','score-clock-pan-capture.py')
psg=module('key_psg','score-clock-psg-capture.py')
VOICE=0xfff840


def cases():
    rows=[]
    channels=[0,1,2,4,5,6,128,160,192,224]
    for channel in channels:
        for on,note,tie,claimed in itertools.product([False,True]if channel<128 else[False],[False,True],[False,True],[False,True]):
            rows.append({'kind':'key','channel':channel,'on':on,'note_off':note,'tied':tie,'claimed':claimed})
    for channel,remaining,tie,claimed in itertools.product(channels,[0,1,2,255],[False,True],[False,True]):
        rows.append({'kind':'gate','channel':channel,'remaining':remaining,'tied':tie,'claimed':claimed,'note_off':False,'on':False})
    phases=[(0,0),(1,896),(1008,1008),(2048,2050)]
    return[{**r,'id':f'{i}-{r["kind"]}-c{r["channel"]}','origin':phases[i%4][0],'refresh':phases[i%4][1]}for i,r in enumerate(rows)]


def setup(ref,checkpoint,case,lookup):
    ref.restore(checkpoint,restore_refresh_cycles=True);ref.api('psg_reset',None)()
    image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0];C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0;C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0]
    ref.cpu.cycles=case['origin'];ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,case['origin']);ref.cpu.cycles,ref.cpu.refresh_cycles=case['origin'],case['refresh']
    z=(C.c_ubyte*8192).in_dll(ref.lib,'zram');z[0x1f2a]=0;z[0x1ffd]=z[0x1ffe]=0
    write_cpu(ref,VOICE,(2 if case['note_off']else 0)|(4 if case['claimed']else 0)|(16 if case['tied']else 0));write_cpu(ref,VOICE+1,case['channel']);write_cpu(ref,VOICE+0x12,case.get('remaining',0))
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for i in range(15):setter(i,0)
    start=0x378e6 if case['kind']=='gate'else 0x38ae0 if case['on']else 0x3a178 if case['channel']>=128 else 0x38afc
    for i,value in[(17,0x2700),(15,0xffff00),(13,VOICE),(16,start)]:setter(i,value)
    write_cpu(ref,0xffff00,0x3d8,4);write_cpu(ref,0xffff04,0x3d8,4);return ref.save()


def execute(ref,case,b=None,p=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[]
    for _ in range(256):
        pc=ref.get_register(16)&0xffffff
        if pc==0x3d8:return steps
        assert(0x378e6<=pc<=0x3790c or 0x38ae0<=pc<=0x38c2c or 0x3a178<=pc<=0x3a18c),('escaped key/gate',case['id'],hex(pc))
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles
        if b:b.score_clock_pc(pc)
        if p:p.score_clock_psg_pc(pc)
        run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('key/gate step cap',case['id']))


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    fm,_=pan.build(out);port,_=psg.build(out);inputs=cases();assert len(inputs)==288;controls={};rows=[];identities={}
    for name,core in[('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint,lookup=quiet_checkpoint(ref),symbols(core);identities[name]=ref.identity
            for i,case in enumerate(inputs):
                initial=setup(ref,checkpoint,case,lookup);plain_steps=execute(ref,case);plain,plain_ram=ref.save(),ref.ram_cpu();plain_psg=psg.native_psg(ref)[1]
                again=setup(ref,checkpoint,case,lookup);assert initial['data']==again['data'];pan.shared.attach(fm,ref);address=C.addressof(ref.cpu);assert port.score_clock_psg_attach(address,address+CPUHead.cycles.offset,address+CPUHead.refresh_cycles.offset)
                try:steps=execute(ref,case,fm,port);events=pan.shared.events(fm);writes=psg.trace(port)
                finally:assert port.score_clock_psg_detach()==1 and fm.score_clock_detach()==1
                saved=ref.save();assert(steps==plain_steps and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and plain_ram==ref.ram_cpu()and plain_psg==psg.native_psg(ref)[1])
                result={'steps':steps,'events':[e for e in events if e['kind']!=7],'psg':writes,'end':{'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles},'note_off':bool(ref.ram_cpu()[0xf840]&2),'remaining':ref.ram_cpu()[0xf852],'stack_pointer':ref.get_register(15)}
                if name=='original':controls[i]=result
                else:
                    assert controls[i]==result,('pinned cores disagree',case['id']);rows.append({'input':case,**result,'native_observer_state_equal':True})
        print(json.dumps({'core':name,'cases':len(inputs),'pass':True}),flush=True)
    sources=[Path(__file__),ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-psg-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',ROOT/'scripts/yuyuhakusho/score-clock-psg-observer.c',ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    data={'version':1,'scope':'Fixed key/gate leafs, legitimate channels/flags and gate0,1,2,255; initialheld/mailbox0/noextraIRQ; sourceSPprovenanceonly','identities':identities,'cases':rows,'sources':{str(p.relative_to(ROOT)):psg.sha(p.read_bytes())for p in sources}}
    target=out/'native-key-gate-full.json.gz';target.write_bytes(gzip.compress((json.dumps(data,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'cases':len(rows),'executions':len(rows)*4,'fixture_sha256':psg.sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
