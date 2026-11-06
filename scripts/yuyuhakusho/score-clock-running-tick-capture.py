"""Complete original running driver ticks without initial queue/dispatch/fade."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path

from extract_audio_assets import AudioSession,write_cpu
from export_audio_score import header
from inspect_dac_dsp_state import symbols
from reference import ROOT,CPUHead
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint
from importlib.util import spec_from_file_location,module_from_spec

def module(name,file):
    s=spec_from_file_location(name,ROOT/'scripts/yuyuhakusho'/file);m=module_from_spec(s);s.loader.exec_module(m);return m
note=module('running_note','score-clock-note-voice-capture.py')
BASE,MOD=0xfff840,0xfffc00

def seed(ref,address,v,context,work,env,patterns,desc):
    if not v['Owned']:
        write_cpu(ref,address,(128 if v['Active']else 0)|(4 if v['Claimed']else 0)|(2 if v['NoteOff']else 0));return
    ch=note.CHANNELS[v['Channel']];p=v['PanDefinition'];mode=p.get('mode','')or'disabled';pan=v['Pan'];ps=v['PanState']
    fields={0:(128 if v['Active']else 0)|(2 if v['NoteOff']else 0)|(4 if v['Claimed']else 0)|(16 if v['Tie']else 0),1:ch,2:context['multiplier'],3:v['PitchScale']&255,8:v['Transpose']&255,9:v['BaseLevel'],0xa:v['PitchControl']['envelope_id']|(128 if v['PitchControl']['modulation_enabled']else 0),0xb:v['VolumeEnvelope']if ch>=128 else work[v['Tone']]['records_skipped']if v['Tone']else 0,0xc:note.envelope_cursor(env,'volume',v['VolumeEnvelope'],v['VolumeCursor']),0xd:context['stack'],0xe:v['NoteWait'],0xf:context['duration'],0x12:v['GateRemaining'],0x13:v['GateTicks'],0x18:v['ModDelay'],0x19:v['ModRate'],0x1a:v['ModDelta']&255,0x1b:v['ModSteps'],0x1e:v['Detune']&255,0x1f:note.MODES.index(mode),0x20:patterns.index(p['pattern'])+1 if mode!='disabled'else 0,0x21:ps['step'],0x22:p.get('step_limit',0),0x23:p.get('period',0),0x24:ps['elapsed'],0x25:v['NoiseControl']if ch>=128 else v['LoadedAlgorithm'],0x26:note.envelope_cursor(env,'pitch',v['PitchControl']['envelope_id'],v['PitchCursor']),0x27:(128 if pan['left']else 0)|(64 if pan['right']else 0)|pan['ams']<<4|pan['fms']}
    for offset,value in fields.items():write_cpu(ref,address+offset,value)
    write_cpu(ref,address+4,context['position'],4);write_cpu(ref,address+0x10,v['BasePitch'],2);write_cpu(ref,address+0x1c,v['ModOffset']&65535,2)
    if v['Channel']=='dac':write_cpu(ref,address+0x10,desc.get(v['DACDescriptor'],128))
    modulation=MOD+(address-BASE)//48*4;write_cpu(ref,address+0x14,modulation,4)
    for i,key in enumerate(['delay','rate','delta','steps']):write_cpu(ref,modulation+i,v['Modulation'][key]&255)
    for offset,value in zip([0x28,0x2c],context['words']):write_cpu(ref,address+offset,value,4)
    if address>=BASE+10*48:write_cpu(ref,address+0x20,context['bank'],4)

def setup(ref,checkpoint,row,lookup,contexts,work,env,patterns,desc):
    ref.restore(checkpoint,restore_refresh_cycles=True);ref.api('psg_reset',None)();image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0];C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0;C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0];note.command.initial(ref);ref.cpu.cycles=0;ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,0)
    for offset in range(928):write_cpu(ref,0xfff800+offset,0)
    state=row['before'];musicbank=0
    for slot,v in enumerate(state['Voices']):
        context=None
        if v['Owned']:
            key=(v['Owner']['score_id'],v['Owner']['voice_id'],v['Cycle'],v['EventIndex'])
            context=contexts.get(key)
            if context is None:
                # A finite inactive row may retain its cursor past track-end.
                # Its parser is never called by this tick; retain a valid
                # owner bank/definition while leaving that unused cursor out.
                assert not v['Active'],key
                context=next(value for k,value in contexts.items()if k[:2]==key[:2])
            if slot<10:musicbank=context['bank']
        seed(ref,BASE+slot*48,v,context,work,env,patterns,desc)
    write_cpu(ref,0xfff820,musicbank,4);write_cpu(ref,0xfff800,63);write_cpu(ref,0xfff809,255);write_cpu(ref,0xfff801,state['TempoRemaining']);write_cpu(ref,0xfff802,state['TempoPeriod'])
    z=(C.c_ubyte*8192).in_dll(ref.lib,'zram');z[0x1fe6:0x1fea]=bytes.fromhex('00800480')
    for address,value in[(0x1ffd,0),(0x1ffc,0),(0x1f2a,0),(0x1ff8,64),(0x1ff9,64),(0x1ffe,17),(0x1ffb,5)]:z[address]=value
    setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
    for i in range(15):setter(i,0)
    for i,value in[(17,0x2700),(15,0xffff00),(16,0x375ae)]:setter(i,value)
    write_cpu(ref,0xffff00,0x3d8,4);ref.cpu.cycles,ref.cpu.refresh_cycles=row['origin'],row['refresh'];return ref.save()

def execute(ref,row,observer=None,port=None):
    run=ref.api('m68k_run',None,C.c_uint);steps=[];stores=[];entries=[]
    for _ in range(50000):
        pc=ref.get_register(16)&0xffffff
        if pc==0x3d8:return steps,stores,entries
        assert 0x375ae<=pc<=0x37656 or 0x37658<=pc<=0x37722 or 0x377ec<=pc<=0x37b8e or 0x37cb4<=pc<=0x37d0c or 0x38ae0<=pc<=0x38d06 or 0x38dae<=pc<=0x39250 or 0x3893a<=pc<=0x389d0 or 0x39ffe<=pc<=0x3a18c,('escaped running tick',row['id'],hex(pc))
        if observer:observer.score_clock_pc(pc)
        if port:port.score_clock_psg_pc(pc)
        before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles
        if pc in[0x37658,0x377ec,0x39ffe]:entries.append(dict(slot=((ref.get_register(13)&0xffff)-0xf840)//48,cycle=before,refresh=refresh))
        run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
        if pc in[0x38f28,0x390f6]:
            address=0xf80a if pc==0x38f28 else 0xf800;stores.append(dict(kind='queue'if pc==0x38f28 else'priority',at=before+(14 if before>=refresh else 0),value=C.c_ubyte.from_address(ref.ram_pointer+(address^1)).value))
    raise AssertionError(('running tick step cap',row['id']))

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    candidates=ROOT/'.local/md-yuyuhakusho/running-tick-case-export/cases.json.gz';inputs=note.load(candidates)['cases'];rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();catalog=note.load(ROOT/'games/yuyuhakusho/data/audio-score.json.gz');contexts=note.contexts(rom,catalog);work={r['id']:r for r in note.load(ROOT/'games/yuyuhakusho/data/audio-score-instrument-work.json.gz')['profiles']};envpath=ROOT/'.local/md-yuyuhakusho/envelope-work-export-v2/provenance.json.gz';env={(r['key']['family'],r['key']['envelope_id'],r['key']['section'],r['key']['tick_index']):r for r in note.load(envpath)['records']};patterns=[p['id']for p in catalog['pan_patterns']]
    registry=note.Registry(rom);desc={};wanted={d['id']for d in catalog['dac_descriptors']}
    for code in range(0x81,0x9a):
        try:identity=registry.descriptor(code)
        except(ValueError,IndexError):continue
        if identity in wanted:desc.setdefault(identity,code)
    for i,row in enumerate(inputs):row['id']=f'{i}-{row["id"]}';row['origin'],row['refresh']=[(0,0),(1,896),(895,896),(1008,1008)][i%4]
    source=ROOT/'scripts/yuyuhakusho/score-clock-note-voice-observer.c';tmp=out/'tmp';tmp.mkdir();library=out/'running-observer.dylib';note.subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(library)],cwd=ROOT,env=dict(note.os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True);observer=C.CDLL(str(library));observer.score_clock_attach.argtypes=[C.c_void_p]*9;observer.score_clock_attach.restype=C.c_int;observer.score_clock_pc.argtypes=[C.c_uint];observer.score_clock_count.restype=C.c_size_t;observer.score_clock_events.restype=C.POINTER(note.music.shared.Event)
    for name in['score_clock_overflow','score_clock_detach','score_clock_note_enable','score_clock_note_disable']:getattr(observer,name).restype=C.c_int
    port,_=note.psg.build(out);controls={};rows=[];identities={};print(json.dumps({'profiles':len(inputs)}),flush=True)
    for name,core in[('original',ORIGINAL),('observed',OBSERVED)]:
        with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
            checkpoint,lookup=quiet_checkpoint(ref),symbols(core);identities[name]=ref.identity
            for i,row in enumerate(inputs):
                initial=setup(ref,checkpoint,row,lookup,contexts,work,env,patterns,desc);plain_steps,plain_stores,plain_entries=execute(ref,row);plain,ram,plain_psg=ref.save(),ref.ram_cpu(),note.psg.native_psg(ref)[1];again=setup(ref,checkpoint,row,lookup,contexts,work,env,patterns,desc);assert initial['data']==again['data'];note.music.shared.attach(observer,ref);assert observer.score_clock_note_enable()==1;address=C.addressof(ref.cpu);assert port.score_clock_psg_attach(address,address+CPUHead.cycles.offset,address+CPUHead.refresh_cycles.offset)
                try:steps,stores,entries=execute(ref,row,observer,port);events,writes=note.music.shared.events(observer),note.psg.trace(port)
                finally:assert port.score_clock_psg_detach()==1 and observer.score_clock_note_disable()==1 and observer.score_clock_detach()==1
                saved=ref.save();assert steps==plain_steps and stores==plain_stores and entries==plain_entries and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and ram==ref.ram_cpu()and plain_psg==note.psg.native_psg(ref)[1]
                result=dict(steps=steps,entries=entries,stores=stores,events=[e for e in events if e['kind']!=7],psg=writes,end=dict(Cycle=ref.cpu.cycles,Refresh=ref.cpu.refresh_cycles),priority=ref.ram_cpu()[0xf800],queue=list(ref.ram_cpu()[0xf80a:0xf80e]),tempo_remaining=ref.ram_cpu()[0xf801],tempo_period=ref.ram_cpu()[0xf802])
                if name=='original':controls[i]=result
                else:assert controls[i]==result,('running cores mismatch',row['id']);rows.append(dict(input=row,**result,native_observer_state_equal=True))
        print(json.dumps({'core':name,'profiles':len(inputs),'pass':True}),flush=True)
    paths=[Path(__file__),source,candidates,ROOT/'.local/md-yuyuhakusho/running-tick-case-export/main.go',ROOT/'scripts/yuyuhakusho/score-clock-note-voice-capture.py',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py',ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'];payload=dict(version=1,scope='Complete running driver from gate through voices/return; initialqueue0/selectedFF/fade0/pause0/control0/noextraIRQ; actual domain and canonical PCM held0.',identities=identities,cases=rows,sources={str(p.relative_to(ROOT)):note.psg.sha(p.read_bytes())for p in paths});target=out/'native-running-tick-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'profiles':len(rows),'executions':len(rows)*4,'sha256':note.psg.sha(target.read_bytes())}),flush=True)
if __name__=='__main__':main()
