"""Bounded existing-core pitch conversion and lookahead oracle, research only."""
import argparse
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path

from extract_audio_assets import AudioSession,write_cpu
from reference import ROOT
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint

VOICE=0xfff840
PARAM=0xfffd00


def sha(raw):return hashlib.sha256(raw).hexdigest()


def cases():
    values=[]
    for family,start,stop in [('fm',0x3784c,0x3787c),('psg',0x3a058,0x3a072)]:
        for index in range(128):
            values.append({'kind':'pitch','family':family,'form':'chromatic','index':index,'start':start,'stop':stop})
    for family,start,stop in [('fm',0x3784c,0x3789e),('psg',0x3a058,0x3a080),('dac',0x3767e,0x37682)]:
        values.append({'kind':'pitch','family':family,'form':'rest','index':0,'start':start,'stop':stop})
    values.append({'kind':'pitch','family':'dac','form':'dac','index':0,'start':0x3767e,'stop':0x37682})
    for family,start,take,reuse in [('fm',0x3783a,0x37844,0x3789e),('psg',0x3a044,0x3a050,0x3789e),('dac',0x37682,0x37690,0x37694)]:
        for outcome,stop in [('take',take),('reuse',reuse)]:
            values.append({'kind':'probe','family':family,'outcome':outcome,'start':start,'stop':stop})
    phases=[(0,0),(1,896),(4096,5296),(4096,5336)]
    return [{**value,'id':f"{value['kind']}-{value['family']}-{value.get('form',value.get('outcome'))}-{value.get('index',0)}-phase{phase}",
             'origin':origin,'refresh':refresh}for value in values for phase,(origin,refresh)in enumerate(phases)]


def setup(ref,checkpoint,case):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    for offset in range(48):write_cpu(ref,VOICE+offset,0)
    write_cpu(ref,VOICE+0xf,0x23)
    write_cpu(ref,PARAM,0x20 if case.get('outcome')=='take' else 0x90)
    note=0x80 if case.get('form')=='rest' else 0x81
    if case.get('form')=='chromatic':
        transpose=(case['index']-1)&127 if case['family']=='fm'else case['index']
        write_cpu(ref,VOICE+8,transpose)
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for index in range(15):setter(index,0)
    for index,value in [(17,0x2700),(15,0xffff00),(12,PARAM),(13,VOICE),(5,note),(16,case['start'])]:setter(index,value)
    write_cpu(ref,0xffff00,0x3d8,4)
    ref.cpu.cycles=case['origin'];ref.cpu.refresh_cycles=case['refresh']
    return ref.save()


def execute(ref,case,record):
    run=ref.api('m68k_run',None,C.c_uint);steps=[]
    for _ in range(64):
        pc=ref.get_register(16)&0xffffff
        if pc==case['stop']:return steps,{'Cycle':ref.cpu.cycles,'Refresh':ref.cpu.refresh_cycles}
        assert(0x3783a<=pc<=0x3789a or 0x3a044<=pc<=0x3a07a or 0x3767e<=pc<=0x3768e),('escaped note phase',case['id'],hex(pc))
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles;run(before+1)
        if record:steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
    raise AssertionError(('note phase exceeded step cap',case['id']))


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);a=parser.parse_args();out=a.out.resolve()
    assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    inputs=cases();assert len(inputs)==1064
    controls={};rows=[];identities={}
    for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint=quiet_checkpoint(ref);identities[name]=ref.identity
            for index,case in enumerate(inputs):
                initial=setup(ref,checkpoint,case);_,plain_end=execute(ref,case,False);plain=ref.save();plain_ram=sha(ref.ram_cpu());plain_regs=[ref.get_register(i)for i in range(18)]
                again=setup(ref,checkpoint,case);assert initial['data']==again['data']
                steps,end=execute(ref,case,True);saved=ref.save();ram=ref.ram_cpu();registers=[ref.get_register(i)for i in range(18)]
                assert plain_end==end and plain['data']==saved['data']and plain['refresh_cycles']==saved['refresh_cycles']and plain_ram==sha(ram)and plain_regs==registers,('passive trace changed native',case['id'])
                result={'steps':steps,'end':end,'registers':registers,'ram_sha256':sha(ram),'pitch_word':int.from_bytes(ram[0xf850:0xf852],'big'),'note_off':bool(ram[0xf840]&2),'wait':ram[0xf84e]}
                if name=='original':controls[index]=result
                else:
                    assert controls[index]==result,('cores disagree',case['id'])
                    public={k:case[k]for k in ['id','kind','family','form','index','outcome','origin','refresh']if k in case}
                    rows.append({'input':public,**result,'native_passive_state_equal':True})
        print(json.dumps({'core':name,'cases':len(inputs),'pass':True}),flush=True)
    rom=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';ops=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h'
    data={'version':1,'scope':'Private fixed note field-write/lookahead boundaries; no cleanup/caller/IRQ/whole-driver price','identities':identities,'cases':rows,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),rom,ops,ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']}}
    target=out/'native-note-full.json.gz';target.write_bytes(gzip.compress((json.dumps(data,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0))
    print(json.dumps({'pass':True,'cases':len(rows),'executions':len(rows)*4,'fixture_sha256':sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
