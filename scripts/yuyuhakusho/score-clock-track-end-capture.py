"""Original complete finite-track calls, primary restoration and Root stores."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
from pathlib import Path

from extract_audio_assets import AudioSession, write_cpu
from export_audio_score import header
from inspect_dac_dsp_state import symbols
from reference import ROOT, CPUHead
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,ROOT/'scripts/yuyuhakusho'/file)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


note=module('track_note','score-clock-note-voice-capture.py')
pan=module('track_pan','score-clock-pan-capture.py')
psg=module('track_psg','score-clock-psg-capture.py')
BASE=0xfff840


def seed_target(ref,address,v,work):
    for i in range(48):write_cpu(ref,address+i,0)
    flags=(128 if v['Active']else 0)|(2 if v['NoteOff']else 0)|(4 if v['Claimed']else 0)|(16 if v['Tie']else 0)
    p=v['Pan'];channel=note.CHANNELS.get(v['Channel'],0)
    fields={0:flags,1:channel,8:v['Transpose']&255,9:v['BaseLevel'],0xe:v['NoteWait'],0x12:v['GateRemaining'],0x13:v['GateTicks'],0x25:v['LoadedAlgorithm']if channel<128 else v['NoiseControl'],0x27:(128 if p['left']else 0)|(64 if p['right']else 0)|p['ams']<<4|p['fms']}
    fields[0xb]=v['VolumeEnvelope']if channel>=128 else work[v['Tone']]['records_skipped']if v['Tone']else 0
    for offset,value in fields.items():write_cpu(ref,address+offset,value)
    write_cpu(ref,address+0x10,v['BasePitch'],2)


def setup(ref,checkpoint,row,lookup,contexts,work,envelopes,patterns,rom):
    slot=row['Slot'];state=row['Entry'];v=state['Voices'][slot]
    context=contexts[(v['Owner']['score_id'],v['Owner']['voice_id'],v['Cycle'],v['EventIndex'])]
    note.VOICE=BASE+48*slot
    ordinary={'before':state,'slot':slot,'origin':row['origin'],'refresh':row['refresh'],'marker':0,'flags':0,'previous_dac_note':128}
    note.setup(ref,checkpoint,ordinary,lookup,context,work,envelopes,patterns)
    current=bytes(C.c_ubyte.from_address(ref.ram_pointer+(((note.VOICE+i)&65535)^1)).value for i in range(48))
    for offset in range(18*48):write_cpu(ref,BASE+offset,0)
    for i,value in enumerate(current):write_cpu(ref,note.VOICE+i,value)
    target=row['Target']
    if target>=0:
        seed_target(ref,BASE+48*target,state['Voices'][target],work)
        owner=state['Voices'][target]['Owner']
        if state['Voices'][target]['Owned']:
            write_cpu(ref,0xfff820,header(rom,owner['score_id'])['bank'],4)
    # These are the actual domain markers of the driver's voice scans.
    write_cpu(ref,0xfff80e,0 if slot<10 else 128 if slot<16 else 64)
    if slot>=10:write_cpu(ref,note.VOICE+0x20,context['bank'],4)
    write_cpu(ref,0xfff800,63)
    write_cpu(ref,0xfff808,0)
    # DAC terminal handler also skips the enclosing driver return.
    write_cpu(ref,0xffff04,0x3d8,4)
    return ref.save()


def execute(ref,row,observer=None,port=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[];stores=[]
    for _ in range(12000):
        pc=ref.get_register(16)&0xffffff
        if pc==0x3d8:return steps,stores
        assert(0x37658<=pc<=0x37722 or 0x377ec<=pc<=0x37b8e or 0x38ae0<=pc<=0x38c2c or 0x38dae<=pc<=0x39250 or 0x39ffe<=pc<=0x3a18c),('escaped track-end',row['id'],hex(pc))
        if observer:observer.score_clock_pc(pc)
        if port:port.score_clock_psg_pc(pc)
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles;run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
        if pc in[0x38f28,0x390f6]:
            address=0xf80a if pc==0x38f28 else 0xf800
            stores.append(dict(kind='queue'if pc==0x38f28 else'priority',at=before+(14 if before>=refresh else 0),value=C.c_ubyte.from_address(ref.ram_pointer+(address^1)).value))
    raise AssertionError(('track-end step cap',row['id']))


def projection(ref,slot):
    ram=ref.ram_cpu();offset=0xf840+48*slot;b=lambda n:ram[offset+n];p=b(0x27)
    return dict(active=bool(b(0)&128),note_off=bool(b(0)&2),claimed=bool(b(0)&4),tie=bool(b(0)&16),channel=b(1),note_wait=b(0xe),gate_remaining=b(0x12),base_pitch=int.from_bytes(ram[offset+0x10:offset+0x12],'big'),loaded_algorithm=b(0x25)&7,noise=b(0x25),pan=dict(left=bool(p&128),right=bool(p&64),ams=p>>4&3,fms=p&7))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    cases_path=ROOT/'.local/md-yuyuhakusho/track-end-case-export/cases.json.gz';inputs=note.load(cases_path)['cases'];rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();catalog=note.load(ROOT/'games/yuyuhakusho/data/audio-score.json.gz');contexts=note.contexts(rom,catalog);work={r['id']:r for r in note.load(ROOT/'games/yuyuhakusho/data/audio-score-instrument-work.json.gz')['profiles']};envpath=ROOT/'.local/md-yuyuhakusho/envelope-work-export-v2/provenance.json.gz';envelopes={(r['key']['family'],r['key']['envelope_id'],r['key']['section'],r['key']['tick_index']):r for r in note.load(envpath)['records']};patterns=[p['id']for p in catalog['pan_patterns']]
    for i,row in enumerate(inputs):row['id']=f'{i}-stop-s{row["Slot"]}-{row["TargetMode"]}-c{int(row["Claimed"])}';row['origin'],row['refresh']=[(0,0),(1,896),(895,896),(1008,1008)][i%4]
    observer,_=pan.build(out);port,_=psg.build(out);rows=[];controls={};identities={};print(json.dumps({'profiles':len(inputs)}),flush=True)
    for name,core in[('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint,lookup=quiet_checkpoint(ref),symbols(core);identities[name]=ref.identity
            for i,row in enumerate(inputs):
                initial=setup(ref,checkpoint,row,lookup,contexts,work,envelopes,patterns,rom);plain_steps,plain_stores=execute(ref,row);plain,ram,plain_psg=ref.save(),ref.ram_cpu(),psg.native_psg(ref)[1]
                again=setup(ref,checkpoint,row,lookup,contexts,work,envelopes,patterns,rom);assert initial['data']==again['data'];pan.shared.attach(observer,ref);assert observer.score_clock_pan_enable()==1;address=C.addressof(ref.cpu);assert port.score_clock_psg_attach(address,address+CPUHead.cycles.offset,address+CPUHead.refresh_cycles.offset)
                try:steps,stores=execute(ref,row,observer,port);events,writes=pan.shared.events(observer),psg.trace(port)
                finally:assert port.score_clock_psg_detach()==1 and observer.score_clock_pan_disable()==1 and observer.score_clock_detach()==1
                saved=ref.save();assert steps==plain_steps and stores==plain_stores and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and ram==ref.ram_cpu()and plain_psg==psg.native_psg(ref)[1]
                result=dict(steps=steps,stores=stores,events=[e for e in events if e['kind']!=7],psg=writes,end=dict(Cycle=ref.cpu.cycles,Refresh=ref.cpu.refresh_cycles),stop=projection(ref,row['Slot']),target=projection(ref,row['Target'])if row['Target']>=0 else None,priority=ref.ram_cpu()[0xf800],queue=ref.ram_cpu()[0xf80a],stack_pointer=ref.get_register(15))
                if name=='original':controls[i]=result
                else:
                    assert controls[i]==result,('pinned core track-end mismatch',row['id']);rows.append(dict(input=row,**result,native_observer_state_equal=True))
        print(json.dumps({'core':name,'profiles':len(inputs),'pass':True}),flush=True)
    sources=[Path(__file__),cases_path,ROOT/'.local/md-yuyuhakusho/track-end-case-export/main.go',ROOT/'scripts/yuyuhakusho/score-clock-note-voice-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py',ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'];payload=dict(version=1,scope='Complete finite-track voice bodies; actual domain markers, sibling target at voice-entry working state, canonical auxiliary held@0, pending real PCM; no whole driver/world/IRQ claim.',identities=identities,cases=rows,sources={str(p.relative_to(ROOT)):psg.sha(p.read_bytes())for p in sources});target=out/'native-track-end-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'profiles':len(rows),'executions':len(rows)*4,'sha256':psg.sha(target.read_bytes())}),flush=True)


if __name__=='__main__':main()
