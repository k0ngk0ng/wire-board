"""Bounded existing-core oracle for normal parser grammar, research only."""
import argparse
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint

PARAM = 0xfffd00
VOICE = 0xfff840
RETURN = 0x3d8
CONTROL = {'pan':0xe0,'detune':0xe1,'pan-automation':0xe4,'level-add':0xec,'tie':0xe7,
           'gate':0xe8,'tempo':0xea,'enqueue-replace-first':0xeb,'fm-patch':0xef,
           'fm-patch-select':0xf5,'modulation':0xf0,'track-end':0xf2,'psg-noise':0xf3,
           'volume-envelope':0xf5,'transpose-add':0xfb,'modulation-enable':0xfc,
           'jump':0xf6,'loop':0xf7,'call':0xf8,'return':0xf9}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def specifications(rom):
    cases = []
    for family, start in [('fm',0x3781e),('dac',0x3766a)]:
        for form, token in [('control',0xe0),('pitch',0x90),('duration',0x20)]:
            stop = (0x37828 if family=='fm' else 0x37674) if form=='control' else (0x3782e if family=='fm' else 0x3767a)
            cases.append({'id':f'token-{family}-{form}','kind':'work','work':{'kind':'token-read','form':form},'start':start,'stop':stop,'token':token})
    for action, token in CONTROL.items():
        branch = 0x38db8+(token-0xe0)*4
        assert rom[branch:branch+2] == b'\x60\x00'
        stop = branch+2+int.from_bytes(rom[branch+2:branch+4],'big',signed=True)
        cases.append({'id':'dispatch-'+action,'kind':'work','work':{'kind':'control-dispatch','action':action},'start':0x37828,'stop':stop,'d5':token})
    for kind, start in [('jump',0x391b8),('call',0x391de),('return',0x391f0)]:
        cases.append({'id':kind,'kind':'work','work':{'kind':kind},'start':start,'stop':RETURN})
    for first in [False,True]:
        for taken in [False,True]:
            cases.append({'id':f'loop-{first}-{taken}','kind':'work','work':{'kind':'loop','first':first,'taken':taken},'start':0x391c4,'stop':RETURN})
    for additions in [0,1]:
        cases.append({'id':f'duration-{additions}','kind':'work','work':{'kind':'duration-scale','additions':additions},'start':0x3787e,'stop':RETURN})
    for family,start,stop in [('fm',0x3782c,0x3781e),('dac',0x37678,0x3766a)]:
        cases.append({'id':'backedge-'+family,'kind':'backedge','start':start,'stop':stop})
    return [{**case,'id':case['id']+f'-phase{phase}','origin':origin,'refresh':refresh}
            for case in cases for phase,(origin,refresh)in enumerate([(0,0),(1,896),(895,896),(1008,1008)])]


def setup(ref, checkpoint, case):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    for offset in range(48):
        write_cpu(ref,VOICE+offset,0)
    write_cpu(ref,PARAM,0x00040004,4)
    write_cpu(ref,VOICE+0xd,0x28)
    write_cpu(ref,VOICE+0x28,PARAM,4)
    if 'token' in case:
        write_cpu(ref,PARAM,case['token'])
    work = case.get('work',{})
    if work.get('kind') == 'loop':
        write_cpu(ref,PARAM,0)
        write_cpu(ref,PARAM+1,2 if work['taken'] else 1)
        write_cpu(ref,VOICE+0x28,0 if work['first'] else (2 if work['taken'] else 1))
    if work.get('kind') == 'duration-scale':
        write_cpu(ref,VOICE+2,work['additions']+1)
    setter = ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for index in range(15):
        setter(index,0)
    for index,value in [(17,0x2700),(15,0xffff00),(12,PARAM),(13,VOICE),(5,case.get('d5',17)),(16,case['start'])]:
        setter(index,value)
    write_cpu(ref,0xffff00,RETURN,4)
    ref.cpu.cycles = case['origin']
    ref.cpu.refresh_cycles = case['refresh']
    return ref.save()


def execute(ref, case, record):
    run = ref.api('m68k_run',None,C.c_uint)
    trace = []
    for _ in range(96):
        pc = ref.get_register(16)&0xffffff
        if pc == case['stop']:
            return trace, {'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles}
        assert (0x3766a<=pc<=0x37678 or 0x3781e<=pc<=0x37828 or pc==0x3782c
                or 0x3787e<=pc<=0x37894 or 0x38dae<=pc<=0x38e30
                or 0x391b8<=pc<=0x39202), ('escaped declared parser phase',case['id'],hex(pc))
        before, refresh = ref.cpu.cycles, ref.cpu.refresh_cycles
        run(before+1)
        if record:
            trace.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('parser phase exceeded fixed instruction cap',case['id']))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    rompath = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    cases = specifications(rompath.read_bytes())
    assert len(cases) == 148
    controls, rows, identities = {}, [], {}
    for name, core in [('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint = quiet_checkpoint(ref)
            identities[name] = ref.identity
            for index,case in enumerate(cases):
                initial = setup(ref,checkpoint,case)
                _, plain_end = execute(ref,case,False)
                plain = ref.save()
                plain_regs = [ref.get_register(i)for i in range(18)]
                plain_ram = sha(ref.ram_cpu())
                rebuilt = setup(ref,checkpoint,case)
                assert initial['data'] == rebuilt['data'], ('private initialization changed',case['id'])
                trace, end = execute(ref,case,True)
                after = ref.save()
                registers = [ref.get_register(i)for i in range(18)]
                ram = sha(ref.ram_cpu())
                assert plain_end==end and plain['data']==after['data'] and plain['refresh_cycles']==after['refresh_cycles'] and plain_regs==registers and plain_ram==ram, ('passive trace changed native result',case['id'])
                comparable = {'steps':trace,'end':end,'registers':registers,'ram_sha256':ram}
                if name == 'original':
                    controls[index] = comparable
                else:
                    assert controls[index] == comparable, ('pinned cores disagree',case['id'])
                    public_input = {k:case[k]for k in ['id','kind','work','origin','refresh']if k in case}
                    rows.append({'input':public_input,**comparable,'native_passive_state_equal':True})
        print(json.dumps({'core':name,'cases':len(cases),'pass':True}),flush=True)
    document = {'version':1,'scope':'Private fixed parser grammar, no bus/IO/IRQ/whole-driver price; source PCs occur only in oracle steps',
                'identities':identities,'cases':rows,'sources':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [Path(__file__),rompath,ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']}}
    raw = (json.dumps(document,sort_keys=True,separators=(',',':'))+'\n').encode()
    target = out/'native-parser-full.json.gz'
    target.write_bytes(gzip.compress(raw,mtime=0))
    print(json.dumps({'pass':True,'cases':len(rows),'native_executions':len(rows)*4,'fixture_sha256':sha(target.read_bytes())}),flush=True)


if __name__ == '__main__':
    main()
