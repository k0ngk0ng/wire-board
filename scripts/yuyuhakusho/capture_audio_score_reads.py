"""Actual native score reads and tick/bus intents, separate from old producers."""
from pathlib import Path
import argparse,ctypes as C,gzip,json,struct
from reference import ROOT,sha
from extract_audio_assets import AudioSession,driver_state,queue_command
from research_audio_loops import ORIGINAL,OBSERVED,BRIDGE,quiet_checkpoint

MUSIC=[0x82,0x83,0x84,0x85,0x86,0x87,0x88,0x8a,0x8d,0x95,0x8f]
EFFECTS=list(range(0xa0,0xac))
POINTS=[0x375ae,0x37656,0x384fc,0x38656,0x38560,0x385a0,0x386e4,
        0x37820,0x3766c,0x3a02e,0x3783a,0x37682,0x3a044,
        0x3789e,0x37694,0x3787c,0x3a072,0x37894,
        0x38dae,0x37678,0x3782c,0x3a03a,0x38f78,
        0x379d0,0x3a0fe,0x37b06,0x38ee6,0x38efe,0x38f06,0x3923e,
        0x376e4,0x38b2e,0x38b9e,0x3a0c6,0x3a0cc,0x3a140,0x3a186,
        0x391a2,0x38610,0x3869a,0x386a4,0x387de,0x387e8,
        0x38afc,0x38ae0,0x3a178,0x3867e,0x3868a,0x3912e,0x39176,0x38858,0x3888a,
        0x38f28,0x37a26,0x37a30,0x37996,0x3918e]
assert len(POINTS)<=64


def write_json(path,value):path.write_text(json.dumps(value,indent=2)+'\n')
def binding(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())}

def cpu_range(ref,address,size):
    if address<0 or address+size>65536:raise ValueError('RAM read outside supplied native storage')
    if address%2==0 and size%2==0:
        raw=C.string_at(ref.ram_pointer+address,size);out=bytearray(size);out[::2]=raw[1::2];out[1::2]=raw[::2];return bytes(out)
    return bytes(C.c_ubyte.from_address(ref.ram_pointer+((address+i)^1)).value for i in range(size))


def old_control(code):
    if code in(0x82,0x87):return ROOT/f'.local/md-yuyuhakusho/audio-loop-verified/{code:02x}-native-states.jsonl.gz'
    if code==0x8d:return ROOT/'.local/md-yuyuhakusho/audio-music-survey/raw-proof-8d/8d-states.jsonl.gz'
    if code in MUSIC and code!=0x8f:return ROOT/f'.local/md-yuyuhakusho/audio-music-survey/extended-{code:02x}/{code:02x}-states.jsonl.gz'
    return None


def loaded_rows(path):
    with gzip.open(path,'rt')as stream:return [json.loads(line)for line in stream]


def header_info(rom,code):
    entry=(0x3a492+(code-0x81)*4)if code in MUSIC else(0x3a60c+(code-0xa0)*4)
    address=int.from_bytes(rom[entry:entry+4],'big')
    count=rom[address+2]+rom[address+3]if code in MUSIC else rom[address+3]
    size=6+rom[address+2]*4+rom[address+3]*6 if code in MUSIC else 4+count*6
    return address,count,size


def native_controls(out,codes,rom,limit):
    acquisitions=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/audio-acquisition.json.gz').read_bytes()))['isolated']
    existing={row['code']:row for key in('sample_assets','music_excerpts')for row in acquisitions[key]}
    result={};new=[code for code in codes if old_control(code)is None]
    if new:
        with AudioSession(ROOT,out/'control-core',core=ORIGINAL)as ref:
            checkpoint=quiet_checkpoint(ref)
            for code in new:
                ref.restore(checkpoint,restore_refresh_cycles=True);queue_command(ref,code)
                previous=existing[code];steps=min(limit or previous['video_steps'],previous['video_steps']);rows=[]
                for step in range(steps):
                    value,pcm=ref.step_audio(ref.frame+1,[0]*8);ram=ref.ram_cpu();old=previous['frame_audio'][step]
                    assert sha(pcm)==old['pcm_sha256']and len(pcm)//4==old['stereo_frames'],(code,step,'finite acquisition PCM changed')
                    assert not any(ram[0x8000:0xa000])
                    rows.append({'step':step,'reference':value['reference'],'driver_hex':ram[0xf800:0xfba0].hex(),'pcm_sha256':sha(pcm),'stereo_frames':len(pcm)//4})
                path=out/f'{code:02x}-new-control.jsonl.gz'
                with gzip.open(path,'wt',compresslevel=5)as stream:
                    for row in rows:stream.write(json.dumps(row,separators=(',',':'))+'\n')
                result[code]=(rows,path)
    for code in codes:
        path=old_control(code)
        if path:
            rows=loaded_rows(path);result[code]=(rows[:limit]if limit else rows,path)
    return result


def run(out,codes,limit):
    rom_path=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rom_path.read_bytes();controls=native_controls(out,codes,rom,limit)
    bridge=C.CDLL(str(BRIDGE));typ=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[typ,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=(C.c_uint*len(POINTS))(*POINTS);cases=[]
    with AudioSession(ROOT,out/'observed-core',core=OBSERVED)as ref:
        checkpoint=quiet_checkpoint(ref)
        assert cpu_range(ref,0xf800,928)==ref.ram_cpu()[0xf800:0xfba0]
        for code in codes:
            ref.restore(checkpoint,restore_refresh_cycles=True);queue_command(ref,code)
            expected,control_path=controls[code];first=ref.frame+1;tick=0;errors=[];stats={};voice_count=0
            events_path=out/f'{code:02x}-reads.json.gz';frames_path=out/f'{code:02x}-frames.jsonl.gz'
            header_address,declared_voices,header_size=header_info(rom,code)
            event_stream=gzip.open(events_path,'wt',compresslevel=5);event_stream.write('{"events":[');event_count=0
            def emit(event):
                nonlocal event_count
                event['step']=ref.frame-first;event['tick']=tick;event['code']=code
                stats[event['kind']]=stats.get(event['kind'],0)+1
                if event_count:event_stream.write(',')
                event_stream.write(json.dumps(event,separators=(',',':')));event_count+=1
            def source(event,address,size):
                if not 0<=address<=len(rom)-size:raise ValueError(('read beyond ROM',hex(address),size))
                event.update(read_address=address,read_hex=rom[address:address+size].hex())
                return event
            def reg(index):return ref.get_register(index)
            def row_state(row):
                if not 0xf840<=row<=0xfb70 or(row-0xf840)%0x30:raise ValueError(('unowned score row',hex(row)))
                return cpu_range(ref,row,48)
            @typ
            def observe(kind,width,pc,value):
                nonlocal tick,voice_count
                try:
                    row=reg(13)&0xffff
                    if pc==0x375ae:
                        tick+=1;emit({'kind':'tick-before','pc':pc,'driver_hex':cpu_range(ref,0xf800,928).hex()});return
                    if pc==0x37656:
                        emit({'kind':'tick-return','pc':pc,'driver_hex':cpu_range(ref,0xf800,928).hex()});return
                    if pc in(0x384fc,0x38656):
                        address=reg(12 if pc==0x384fc else 11)&0xffffff
                        assert address==header_address,(code,pc,address,header_address)
                        emit(source({'kind':'header','pc':pc,'declared_voices':declared_voices},address,header_size));return
                    if pc in(0x38560,0x385a0,0x386e4):
                        target=reg(9)&0xffff if pc!=0x386e4 else row
                        state=row_state(target);record_size=4 if pc==0x38560 else 6
                        address=(reg(12 if pc!=0x386e4 else 9)&0xffffff)-record_size
                        emit(source({'kind':'voice-init','pc':pc,'voice_ordinal':voice_count,'row':target,'row_hex':state.hex()},address,record_size));voice_count+=1;return
                    if pc in(0x37820,0x3766c,0x3a02e,0x3783a,0x37682,0x3a044):
                        address=reg(12)&0xffffff
                        emit(source({'kind':'token-read','pc':pc,'row':row,'row_hex':row_state(row).hex(),'role':'duration-lookahead'if pc in(0x3783a,0x37682,0x3a044)else'sequence'},address,1));return
                    if pc in(0x3789e,0x37694):
                        state=row_state(row);emit({'kind':'note-outcome','pc':pc,'row':row,'row_hex':state.hex(),'effective_duration':state[15]or 256,'base_pitch':int.from_bytes(state[16:18],'big'),'next_read_address':reg(12)&0xffffff});return
                    if pc in(0x3787c,0x3a072,0x37894,0x37a26,0x37a30,0x37996):
                        emit({'kind':'parameter-outcome','pc':pc,'row':row,'row_hex':row_state(row).hex(),'d0':reg(0)&65535,'d5':reg(5)&65535,'d6':reg(6)&65535});return
                    if pc==0x38dae:
                        address=reg(12)&0xffffff;command=reg(5)&255
                        assert rom[address-1]==command
                        emit(source({'kind':'control-before','pc':pc,'row':row,'row_hex':row_state(row).hex(),'command':command,'command_address':address-1},address,8));return
                    if pc in(0x37678,0x3782c,0x3a03a):
                        emit({'kind':'control-after','pc':pc,'row':row,'row_hex':row_state(row).hex(),'next_read_address':reg(12)&0xffffff});return
                    if pc==0x38f78:
                        state=row_state(row);emit(source({'kind':'fm-patch-read','pc':pc,'row':row,'bank_address':(reg(9)&0xffffff)-25*state[11],'instrument_index':state[11],'row_hex':state.hex()},reg(9)&0xffffff,25));return
                    if pc in(0x379d0,0x3a0fe,0x37b06):
                        index=reg(0)&65535;address=reg(8)&0xffffff;state=row_state(row)
                        label={0x379d0:'pitch-envelope-read',0x3a0fe:'volume-envelope-read',0x37b06:'pan-pattern-read'}[pc]
                        emit(source({'kind':label,'pc':pc,'row':row,'index':index,'base_address':address,'row_hex':state.hex()},address+index,1));return
                    if pc==0x38ee6:
                        emit(source({'kind':'e9-bank-read','pc':pc,'row':row,'a0':reg(8)&0xffffff,'row_hex':row_state(row).hex()},reg(9)&0xffffff,1));return
                    if pc in(0x38efe,0x38f06):
                        emit(source({'kind':'e9-argument-read','pc':pc,'row':row},reg(12)&0xffffff,1));return
                    if pc==0x3923e:
                        # The indexed PC-relative word source is39250 + D1.w.
                        address=0x39250+(reg(1)&65535);emit(source({'kind':'fm3-offset-read','pc':pc,'row':row,'destination':reg(8)&0xffff},address,2));return
                    if pc==0x376e4:
                        state=row_state(row);emit(source({'kind':'dac-descriptor','pc':pc,'row':row,'pan':state[39],'row_hex':state.hex()},reg(11)&0xffffff,8));return
                    if pc in(0x38b2e,0x38b9e):
                        emit({'kind':'ym-request','pc':pc,'row':row,'port':0 if pc==0x38b2e else 1,'register':reg(0)&255,'value':reg(1)&255});return
                    if pc in(0x3a0c6,0x3a0cc,0x3a140,0x3a186,0x38610,0x3869a,0x386a4,0x387de,0x387e8):
                        register=6 if pc in(0x3a0cc,0x3a140)else 4 if pc in(0x387de,0x387e8)else 0
                        data=255 if pc==0x38610 else reg(register)&255
                        emit({'kind':'psg-request','pc':pc,'row':row,'value':data});return
                    if pc==0x391a2:
                        address=(reg(12)&0xffffff)-1;emit(source({'kind':'psg-request','pc':pc,'row':row,'value':rom[address]},address,1));return
                    if pc in(0x38afc,0x38ae0,0x3a178):
                        emit({'kind':'key-intent','pc':pc,'row':row,'row_hex':row_state(row).hex(),'action':'on'if pc==0x38ae0 else'off'});return
                    if pc in(0x3867e,0x3868a,0x3912e,0x39176,0x38858,0x3888a):
                        emit({'kind':'voice-routing','pc':pc,'row':row,'row_hex':row_state(row).hex(),'action':'claim'if pc in(0x3867e,0x3868a)else'restore-forced-rest'});return
                    if pc==0x38f28:
                        address=reg(12)&0xffffff;emit(source({'kind':'replace-first-enqueue','pc':pc,'row':row,'queue_before':list(cpu_range(ref,0xf80a,4))},address,1));return
                    if pc==0x3918e:emit({'kind':'track-stop','pc':pc,'row':row,'row_hex':row_state(row).hex()});return
                except Exception as error:errors.append(repr(error))
            assert bridge.yyh_set_observer(observe,points,len(points))
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
            try:
                with gzip.open(frames_path,'wt',compresslevel=5)as frames:
                    for step,prior in enumerate(expected):
                        result,pcm=ref.step_audio(ref.frame+1,[0]*8);driver=cpu_range(ref,0xf800,928)
                        assert result['reference']==prior['reference'],(code,step,'reference differs')
                        assert sha(pcm)==prior['pcm_sha256']and len(pcm)//4==prior['stereo_frames'],(code,step,'PCM differs')
                        assert driver.hex()==prior['driver_hex'],(code,step,'driver differs')
                        assert not errors,errors[:4]
                        frames.write(json.dumps({'step':step,'reference':result['reference'],'driver_hex':driver.hex(),'pcm_sha256':sha(pcm),'stereo_frames':len(pcm)//4},separators=(',',':'))+'\n')
                        if step%4096==0:print(json.dumps({'code':hex(code),'step':step,'events':event_count}),flush=True)
            finally:
                ref.api('set_cpu_hook',None,C.c_void_p)(None)
                event_stream.write('],"errors":'+json.dumps(errors)+'}');event_stream.close()
            assert not errors and voice_count==declared_voices and tick==len(expected),(code,errors[:4],voice_count,declared_voices,tick,len(expected))
            cases.append({'code':code,'steps':len(expected),'driver_steps':tick,'declared_voices':declared_voices,'native_reference_equal':True,'pcm_samples_equal':True,'control':binding(control_path),'statistics':stats,'files':{'events':binding(events_path),'frames':binding(frames_path)}})
            print(json.dumps({'complete':hex(code),'steps':len(expected),'statistics':stats}),flush=True)
    paths=[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/validate_audio_score_reads.py',ROOT/'scripts/yuyuhakusho/reference.py',ROOT/'scripts/yuyuhakusho/extract_audio_assets.py',ROOT/'scripts/yuyuhakusho/research_audio_loops.py']
    report={'schema':1,'scope':'native-score-read-oracle','pass':True,'production_ready':False,'dsp_implemented':False,'rom_sha256':sha(rom),'codes':codes,'identity':ref.identity,'cases':cases,'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in paths},'limits':['Isolated original driver proof; mixed override/restore needs separate cases','Raw source pointers/rows belong only to research evidence, never the productioncatalog']}
    write_json(out/'index.json',report)

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--codes',default=','.join(f'{code:02x}'for code in MUSIC+EFFECTS));parser.add_argument('--steps',type=int)
    args=parser.parse_args();out=args.out.resolve();codes=[int(code,16)for code in args.codes.split(',')];assert out.is_relative_to(ROOT/'.local')and len(set(codes))==len(codes)and set(codes)<=set(MUSIC+EFFECTS)
    if args.steps:assert 0<args.steps<=256
    out.mkdir(parents=True,exist_ok=True);run(out,codes,args.steps)
