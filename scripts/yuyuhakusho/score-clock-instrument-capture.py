"""Fixed original FM instrument load/level caller+service oracle, research only."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
from pathlib import Path

from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT,sha
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
from inspect_dac_dsp_state import symbols

spec=importlib.util.spec_from_file_location('instrument_shared',ROOT/'scripts/yuyuhakusho/score-clock-capture.py')
shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)
VOICE=0xfff840
CHANNELS=[0,1,2,4,5,6]


def requests():
    work=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/audio-score-instrument-work.json.gz').read_bytes()))
    data=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/audio-score.json.gz').read_bytes()))
    proof=json.loads((ROOT/'games/yuyuhakusho/verification/audio-instrument-work/source-provenance.json').read_text())
    definitions={p['id']:p for p in work['profiles']};patches={p['id']:p for p in data['fm_patches']}
    locations={}
    for row in proof['selectors']:locations.setdefault(row['reference'],(row['source_bank'],row['source_index']))
    locations[work['fallback_reference']]=(0,0)
    choices=[]
    for algorithm in range(8):
        ref=next(p['id']for p in work['profiles']if patches[p['patch_id']]['algorithm']==algorithm and p['id']in locations)
        for base in [0,127,255]:
            for channel in CHANNELS:choices.append({'mode':'load','reference':ref,'base':base,'channel':channel,'loaded_algorithm':algorithm,'claimed':False})
    for algorithm in range(8):
        ref=next(p['id']for p in work['profiles']if patches[p['patch_id']]['algorithm']==algorithm and p['id']in locations)
        for base in [0,1,127,128,255]:choices.append({'mode':'level','reference':ref,'base':base,'channel':CHANNELS[(algorithm+base)%6],'loaded_algorithm':(algorithm+3)%8,'claimed':False})
    special=[]
    for patch,skip in [('patch-ff7d20a3582bd2f75b87',8),('patch-ff7d20a3582bd2f75b87',13),('patch-4c6a6d97d872d62c7799',8),('patch-4c6a6d97d872d62c7799',14)]:
        special.append(next(p['id']for p in work['profiles']if p['patch_id']==patch and p['records_skipped']==skip))
    special.append(next(p['id']for p in work['profiles']if p['records_skipped']==74))
    for ref in special:
        for domain in ['music','primary','secondary']:
            for mode in ['load','level']:choices.append({'mode':mode,'reference':ref,'base':1,'channel':6,'loaded_algorithm':7,'claimed':False,'domain':domain})
    for domain in ['music','primary','secondary']:
        for mode in ['load','level']:choices.append({'mode':mode,'reference':special[0],'base':0,'channel':0,'loaded_algorithm':0,'claimed':True,'domain':domain})
    phases=[(0,0),(1,896),(1008,1008),(2048,2050)]
    result=[]
    for i,row in enumerate(choices):
        row=dict(row);row.setdefault('domain',['music','primary','secondary'][i%3]);row['origin'],row['refresh']=phases[i%4];row['id']=f"{i}-{row['mode']}-{row['domain']}-c{row['channel']}-b{row['base']}";row['work']=definitions[row['reference']];row['patch']=patches[row['work']['patch_id']];row['source_bank'],row['source_index']=locations[row['reference']];result.append(row)
    assert len(result)==220
    return result


def setup(ref,checkpoint,case,lookup):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0]
    C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0
    C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0]
    ref.cpu.cycles=case['origin'];ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,case['origin'])
    ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['refresh']
    for offset in range(48):write_cpu(ref,VOICE+offset,0)
    mode={'music':0,'primary':1,'secondary':128}[case['domain']]
    for address in [0xfff820,0xfff824,VOICE+0x20]:write_cpu(ref,address,case['source_bank'],4)
    write_cpu(ref,0xfff80e,mode);write_cpu(ref,VOICE,4 if case['claimed'] else 0)
    write_cpu(ref,VOICE+1,case['channel']);write_cpu(ref,VOICE+9,case['base']);write_cpu(ref,VOICE+0xb,case['source_index']);write_cpu(ref,VOICE+0x25,case['loaded_algorithm']);write_cpu(ref,VOICE+0x27,192)
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for index in range(15):setter(index,0)
    for index,value in [(17,0x2700),(15,0xffff00),(13,VOICE),(0,case['source_index']),(16,0x38f4e if case['mode']=='load' else 0x3901c)]:setter(index,value)
    write_cpu(ref,0xffff00,0x3d8,4);(C.c_ubyte*8192).in_dll(ref.lib,'zram')[0x1f2a]=0
    return ref.save()


def execute(ref,case,b=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[];req=[]
    stop=0x38fba if case['mode']=='load'and not case['claimed'] else 0x3d8
    for _ in range(8000):
        pc=ref.get_register(16)&0xffffff
        if pc==stop:return steps,req
        assert 0x38f4e<=pc<=0x3907e or 0x38b22<=pc<=0x38c2c,('escaped instrument phase',case['id'],hex(pc))
        if pc in [0x38b2e,0x38b9e]:req.append({'at':ref.cpu.cycles,'refresh':ref.cpu.refresh_cycles,'bank':int(pc==0x38b9e),'register':ref.get_register(0)&255,'value':ref.get_register(1)&255})
        before,deadline=ref.cpu.cycles,ref.cpu.refresh_cycles
        if b:b.score_clock_pc(pc)
        run(before+1);steps.append([pc,before,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('instrument step limit',case['id']))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    b,bridge=shared.build(out);inputs=requests();control={};rows=[];identities={}
    for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint=quiet_checkpoint(ref);identities[name]=ref.identity;lookup=symbols(core)
            for i,case in enumerate(inputs):
                initial=setup(ref,checkpoint,case,lookup);plain,plain_req=execute(ref,case);plain_state=ref.save();plain_ram=ref.ram_cpu()
                again=setup(ref,checkpoint,case,lookup);assert initial['data']==again['data']
                shared.attach(b,ref)
                if name=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.score_clock_hook,C.c_void_p))
                try:steps,req=execute(ref,case,b);events=shared.events(b)
                finally:
                    if name=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(None)
                    assert b.score_clock_detach()==1
                after=ref.save();assert plain==steps and plain_req==req and plain_state['data']==after['data']and plain_state['refresh_cycles']==after['refresh_cycles']and plain_ram==ref.ram_cpu(),('recorder changed instrument',case['id'])
                comparable={'steps':steps,'requests':req,'events':[e for e in events if e['kind']!=7],'end':{'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles}}
                if name=='original':control[i]=comparable
                else:
                    assert control[i]==comparable,('cores disagree',case['id'])
                    public={k:v for k,v in case.items()if k not in['source_bank','source_index']};rows.append({'input':public,**comparable,'native_recorder_state_equal':True})
        print(json.dumps({'core':name,'cases':len(inputs),'pass':True}),flush=True)
    payload={'version':1,'scope':'Declared held-bus/mailbox0 instrument caller+YM services, no extra IRQ or whole driver/world scheduling','identities':identities,'cases':rows,'sources':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [Path(__file__),ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',ROOT/'games/yuyuhakusho/data/audio-score-instrument-work.json.gz',ROOT/'games/yuyuhakusho/data/audio-score.json.gz']}}
    target=out/'native-instrument-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'cases':len(rows),'executions':len(rows)*4,'fixture_sha256':sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
