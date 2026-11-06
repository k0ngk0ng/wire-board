"""Original private fixed fade-start and header-prefix clock/effect oracle.

No later level-scan or full-clear instruction is executed. Actual observed-core
RAM write hooks corroborate effect phases derived from original executed steps
and the fixed architecture store costs; raw PCs remain research provenance.
"""
import argparse
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT, CPUHead, sha
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint

SOURCE=ROOT/'scripts/yuyuhakusho/score-clock-fade-observer.c'
STEP_FIELDS=['pc','before','deadline','after','next_deadline']
EFFECT_FIELDS=['pc','at','address','value']
class Step(C.Structure): _fields_=[(name,C.c_uint32) for name in STEP_FIELDS]
class Effect(C.Structure): _fields_=[(name,C.c_uint32) for name in EFFECT_FIELDS]
WRITES={0x38916:0xfff806,0x3891c:0xfff804,0x38922:0xfff840,0x38928:0xfff806,0x3892e:0xfff804,0x38934:0xfff840,0x38948:0xfff806,0x3894e:0xfff804,0x38956:0xfff806}
FIELDS={0xfff806:'remaining',0xfff804:'steps',0xfff840:'DAC-flags'}

def saved(path,value):
    raw=(json.dumps(value,separators=(',',':'))+'\n').encode()
    path.write_bytes(gzip.compress(raw,mtime=0) if path.suffix=='.gz' else raw)

def build(out):
    tmp=out/'tmp';tmp.mkdir(parents=True,exist_ok=True);library=out/'fade-observer.dylib'
    subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(SOURCE),'-o',str(library)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
    b=C.CDLL(str(library));b.score_clock_fade_attach.argtypes=[C.c_void_p]*3;b.score_clock_fade_attach.restype=C.c_int
    b.score_clock_header_detach.restype=C.c_int;b.score_clock_header_steps.restype=C.POINTER(Step);b.score_clock_header_count.restype=C.c_uint;b.score_clock_header_overflow.restype=C.c_uint
    b.score_clock_fade_effects.restype=C.POINTER(Effect);b.score_clock_fade_count.restype=C.c_uint;b.score_clock_fade_overflow.restype=C.c_uint;b.score_clock_fade_size.restype=C.c_uint
    assert b.score_clock_fade_size()==C.sizeof(Effect)
    return b,library

def profiles():
    variants=[{'kind':'start','command':1,'steps':0,'remaining':0,'DAC_flags':255}, {'kind':'start','command':5,'steps':0,'remaining':0,'DAC_flags':255},
              {'kind':'start','command':1,'steps':40,'remaining':3,'DAC_flags':0}, {'kind':'start','command':5,'steps':50,'remaining':32,'DAC_flags':0}]
    variants +=[{'kind':'header','command':0,'steps':steps,'remaining':remaining,'DAC_flags':165} for steps,remaining in [(0,0),(0,255),(1,1),(40,3),(255,255),(1,0),(2,0),(40,0),(255,0)]]
    cases=[]
    for variant in variants:
        phases=[('entry',0,0),('future',1,4096),('aligned',1008,1008),('near-entry',2048,2050)]
        if variant['kind']=='start':
            phases +=[('second-store',1,113),('flag-store',1,225),('return',1,337)]
        else:
            steps,remaining=variant['steps'],variant['remaining']
            effect=238 if steps==0 else 434 if remaining else 448
            last=308 if steps==0 else 546 if remaining else 560 if steps==1 else 812
            phases +=[('after-caller',1,127),('effect-boundary',1,effect+1),('last-boundary',1,last+1)]
        for phase,origin,refresh in phases:
            cases.append({**variant,'id':f"{variant['kind']}-c{variant['command']:02x}-s{variant['steps']:02x}-r{variant['remaining']:02x}-f{variant['DAC_flags']:02x}-{phase}",'origin':origin,'refresh':refresh})
    assert len(cases)==91
    return cases

def setup(ref,checkpoint,case):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['refresh']
    start=(0x38916 if case['command']==1 else 0x38928) if case['kind']=='start' else 0x375c2
    setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for index,value in [(17,0x2700),(15,0xffff00),(16,start)]:setreg(index,value)
    write_cpu(ref,0xffff00,0x3d8,4)
    for address,value in [(0xfff804,case['steps']),(0xfff806,case['remaining']),(0xfff840,case['DAC_flags']),(0xfff800,0x55),(0xfff809,0xa6),(0xfff82c,0xfe)]:write_cpu(ref,address,value)
    return ref.save()

def root_values(ref):
    ram=ref.ram_cpu()
    return {'steps':ram[0xf804],'remaining':ram[0xf806],'DAC_flags':ram[0xf840],'priority':ram[0xf800],'selected':ram[0xf809],'choice_bits':ram[0xf82c]}

def execute(ref,case,b=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[];effects=[]
    stops={0x3d8:'Done'} if case['kind']=='start' else {0x375c6:'Done',0x38962:'LevelScan',0x38a2c:'FullClear'}
    allowed={0x38916,0x3891c,0x38922,0x38926,0x38928,0x3892e,0x38934,0x38938,0x375c2,0x3893a,0x3893c,0x38940,0x38942,0x38946,0x38948,0x3894c,0x3894e,0x38952,0x38956,0x3895c,0x38960}
    for _ in range(16):
        pc=ref.get_register(16)&0xffffff
        if pc in stops:return {'steps':steps,'effects':effects,'next':stops[pc],'end':{'cycle':ref.cpu.cycles,'refresh':ref.cpu.refresh_cycles},'after':root_values(ref)}
        assert pc in allowed,('Escaped fixed fade boundary',case['id'],hex(pc))
        before,deadline=ref.cpu.cycles,ref.cpu.refresh_cycles
        if b:b.score_clock_fade_before()
        run(before+1)
        if b:b.score_clock_header_after()
        steps.append([pc,before,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles])
        if pc in WRITES:
            # All nine fixed source stores have independently bound16CPU costs.
            # Native executes the store. No counter result is computed here.
            address=WRITES[pc];at=ref.cpu.cycles-16*7
            assert at in [before,before+14]
            effects.append({'pc':pc,'at':at,'address':address,'value':ref.ram_cpu()[address&65535]})
    raise AssertionError(('Fixed fade prefix exceeded cap',case['id']))

def recorded(b):
    assert not b.score_clock_header_overflow() and not b.score_clock_fade_overflow()
    p=b.score_clock_header_steps();steps=[[int(getattr(p[i],name)) for name in STEP_FIELDS] for i in range(b.score_clock_header_count())]
    p=b.score_clock_fade_effects();effects=[{name:int(getattr(p[i],name)) for name in EFFECT_FIELDS} for i in range(b.score_clock_fade_count())]
    return steps,effects

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--fixture',type=Path);parser.add_argument('--smoke',action='store_true');args=parser.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True);b,library=build(out)
    cases=profiles()
    if args.smoke:cases=[cases[i] for i in [0,7,28,42,63,70,84]]
    controls={};reports=[];identity={}
    for mode,core in [('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+mode),core=core) as ref:
            checkpoint=quiet_checkpoint(ref);identity[mode]=ref.identity
            for case in cases:
                initial=setup(ref,checkpoint,case);plain=execute(ref,case);plain_save=ref.save();plain_ram=ref.ram_cpu()
                rebuilt=setup(ref,checkpoint,case);assert initial['data']==rebuilt['data'] and initial['refresh_cycles']==rebuilt['refresh_cycles'],case['id']
                address=C.addressof(ref.cpu)
                assert b.score_clock_fade_attach(address+CPUHead.cycles.offset,address+CPUHead.refresh_cycles.offset,address+CPUHead.pc.offset)
                if mode=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(b.score_clock_fade_hook,C.c_void_p))
                try:
                    observed=execute(ref,case,b);sampled,write_hook=recorded(b)
                finally:
                    if mode=='observed':ref.api('set_cpu_hook',None,C.c_void_p)(None)
                    assert b.score_clock_header_detach()==1
                after=ref.save();assert plain==observed and sampled==observed['steps'] and plain_save['data']==after['data'] and plain_save['refresh_cycles']==after['refresh_cycles'] and plain_ram==ref.ram_cpu(),('Recorder changed original',mode,case['id'])
                assert observed['after']['priority']==0x55 and observed['after']['selected']==0xa6 and observed['after']['choice_bits']==0xfe
                if mode=='original':
                    assert write_hook==[];controls[case['id']]=observed
                else:
                    assert write_hook==observed['effects'],('Actual native write hook disagrees with original architecture phase',case['id'],write_hook,observed['effects'])
                    assert controls[case['id']]==observed,('Two pinned fade cores disagree',case['id'])
                    reports.append({'input':case,**observed,'actual_native_write_hook':write_hook,'native_recorder_on_off_state_equal':True,'both_native_builds_equal':True})
            print(json.dumps({'mode':mode,'cases':len(cases),'pass':True}),flush=True)
    full=out/'native-fade-clock-full.json.gz';scope='Private SR2700 no-external-stall fixed fade-start leaf and original caller/header prefix only; LevelScan/full04 remain explicit unfinished boundaries'
    saved(full,{'schema':1,'scope':scope,'identity':identity,'cases':reports})
    inputs=[SOURCE,Path(__file__),ROOT/'scripts/yuyuhakusho/score-clock-header-observer.c',ROOT/'scripts/yuyuhakusho/reference.py',ROOT/'scripts/yuyuhakusho/extract_audio_assets.py',ROOT/'scripts/yuyuhakusho/research_audio_loops.py']
    summary={'pass':True,'cases':len(reports),'effects':sum(len(row['effects']) for row in reports),'native_builds':2,'recorder_on_off_executions':len(reports)*4,'identity':identity,'scope':scope,
             'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes()) for path in inputs},'artifact':{'path':str(full.relative_to(ROOT)),'sha256':sha(full.read_bytes())},'observer_binary_sha256':sha(library.read_bytes()),'actual_observed_core_RAM_write_times_equal':True,'later_level_scan_or_full04_executed':False}
    if args.fixture:
        target=args.fixture.resolve();assert target.parent==ROOT/'games/yuyuhakusho/audio/scoreclock/testdata' and target.name.startswith('fade-clock-')
        saved(target,{'schema':1,'scope':scope,'cases':[{'input':row['input'],'next':row['next'],'end':row['end'],'after':row['after'],'effects':[{'at':e['at'],'field':FIELDS[e['address']],'value':e['value']} for e in row['effects']]} for row in reports]})
        manifest={'source':summary['artifact'],'fixture':{'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())},'cases':len(reports),'effects':summary['effects'],'producer_sha256':sha(Path(__file__).read_bytes()),'source_PC_or_RAM_in_fixture':False}
        saved(target.with_name('fade-clock-native-manifest.json'),manifest);summary['fixture']=manifest['fixture']
    saved(out/'capture.json',summary)
    print(json.dumps({'pass':True,'cases':len(reports),'effects':summary['effects']}),flush=True)

if __name__=='__main__':main()
