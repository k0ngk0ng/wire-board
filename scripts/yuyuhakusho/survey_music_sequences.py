"""Bounded eight-song native survey; old producers/media are read-only."""
from pathlib import Path
from collections import defaultdict, Counter
from array import array
import argparse, ctypes as C, gzip, hashlib, json, math, struct, wave

from reference import ROOT, sha
from extract_audio_assets import AudioSession, driver_state, queue_command
from research_audio_loops import ORIGINAL, OBSERVED, BRIDGE, quiet_checkpoint, chip_context
from analyze_audio_loops import waveform_difference

CODES = [0x83, 0x84, 0x85, 0x86, 0x88, 0x8a, 0x8d, 0x95]
LIMIT = 4096
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2)+'\n')


def binding(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes())}


def header_for(rom, code):
    entry = 0x3a492+(code-0x81)*4
    base = int.from_bytes(rom[entry:entry+4], 'big')
    fm, psg = rom[base+2:base+4]
    assert (fm, psg) == (6, 3)
    raw = rom[base:base+6+fm*4+psg*6]
    assert len(raw) == 48
    off = 6; voices = []
    for index in range(fm+psg):
        width = 4 if index < fm else 6
        record = raw[off:off+width]
        sequence = base+int.from_bytes(record[:2], 'big')
        assert 0 <= sequence < len(rom)
        voices.append({'index': index, 'kind': 'dac' if index == 0 else 'fm' if index < fm else 'psg',
                       'row': 0xf840+index*0x30 if index < fm else 0xf990+(index-fm)*0x30,
                       'record': record.hex(), 'sequence': sequence})
        off += width
    return {'table_entry': entry, 'address': base, 'bytes': raw.hex(), 'sha256': sha(raw),
            'fm_rows_including_dac': fm, 'psg_rows': psg, 'instrument_data': base+int.from_bytes(raw[:2], 'big'),
            'duration_multiplier': raw[4], 'tempo_byte': raw[5], 'voices': voices}


def compare_periods(states, events, wav_path, period):
    raw = {'verified': False, 'reason': 'All active rows do not yet have a repeated F6'}
    selected = {'verified': False}
    pcm = {'verified': False, 'reason': 'No complete raw-state boundary proved'}
    if not period or len(states) <= period:
        return raw, selected, pcm
    differences = [i for i in range(len(states)-period) if states[i]['driver_hex'] != states[i+period]['driver_hex']]
    first = differences[-1]+1 if differences else 0
    pairs = len(states)-period-first
    raw = {'verified': pairs >= period, 'first_equal_after_step': first, 'equal_pairs': pairs,
           'period_frames': period, 'complete_raw_driver_bytes': 928,
           'reason': 'Full raw driver recurrence' if pairs >= period else 'Insufficient complete recurrence window; no state bytes excluded'}
    start = first+1
    if not raw['verified'] or start+2*period > len(states):
        return raw, selected, pcm
    def normalize(begin):
        excluded = {'cpu_cycles', 'row_hex', 'driver_hex', 'globals_hex'}
        return [{key: value-begin if key == 'step' else value for key,value in event.items() if key not in excluded}
                for event in events if begin <= event['step'] < begin+period and event['kind'] != 'tick']
    left, right = normalize(start), normalize(start+period)
    selected = {'verified': left == right, 'events_first': len(left), 'events_second': len(right),
                'first_sha256': sha(json.dumps(left,separators=(',', ':')).encode()),
                'second_sha256': sha(json.dumps(right,separators=(',', ':')).encode()),
                'limits': ['Selected sequence/YM/DAC order and relative video steps only, not all chip writes at exact clocks']}
    offsets = [0]
    for state in states: offsets.append(offsets[-1]+state['stereo_frames'])
    boundaries = [start, start+period, start+2*period]
    samples = [offsets[step] for step in boundaries]
    with wave.open(str(wav_path), 'rb') as wav:
        whole = wav.readframes(wav.getnframes())
    a,b,c = samples
    pcm = {'verified': True, 'video_steps_completed': boundaries, 'stereo_sample_offsets': samples,
           'period_stereo_frames': [b-a,c-b], 'comparison': waveform_difference(whole[a*4:b*4],whole[b*4:c*4]),
           'first_100ms_comparison': waveform_difference(whole[a*4:(a+4410)*4],whole[b*4:(b+4410)*4]),
           'psg_noise_at_boundaries': [struct.unpack_from('<i',bytes.fromhex(states[step-1]['psg_hex']),8)[0] for step in boundaries],
           'ym_context_sha256_at_boundaries': [states[step-1]['ym_context_sha256'] for step in boundaries],
           'exact_media_loop_accepted': False,
           'limits': ['Video-step boundaries are not precise intra-frame sample cut points', 'This bounded comparison does not prove that no longer PCM period exists']}
    return raw, selected, pcm


def run(out, steps, codes):
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert sha(rom) == ROM_SHA
    controls = {}; songs = {}; identities = {}
    with AudioSession(ROOT,out/'control-core',core=ORIGINAL) as ref:
        checkpoint = quiet_checkpoint(ref)
        identities['original'] = ref.identity
        for code in codes:
            ref.restore(checkpoint,restore_refresh_cycles=True)
            queue_command(ref,code)
            path = out/f'{code:02x}-states.jsonl.gz'; wav_path=out/f'{code:02x}-control.wav'
            states=[]; tail=[]; busy_seen=False; end_step=None; idle=0; nonzero=0
            with gzip.open(path,'wt',compresslevel=5) as stream, wave.open(str(wav_path),'wb') as wav:
                wav.setnchannels(2);wav.setsampwidth(2);wav.setframerate(44100)
                for step in range(steps):
                    result,pcm=ref.step_audio(ref.frame+1,[0]*8)
                    ram=ref.ram_cpu(); native=driver_state(ref)
                    assert native['world_clear'] and not any(ram[0xfa20:0xfba0]),(code,step,'nonmusic producer/effect')
                    psg=chip_context(ref,'psg_context_save')
                    state={'step':step,'reference':result['reference'],'driver_hex':ram[0xf800:0xfba0].hex(),
                           'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm), 'world_clear':True,'effect_rows_clear':True,
                           'busy':native['busy'],'active_music_rows':native['active_rows'],
                           'psg_hex':psg.hex(),'ym_context_sha256':sha(chip_context(ref,'YM2612SaveContext'))}
                    states.append(state);stream.write(json.dumps(state,separators=(',',':'))+'\n');wav.writeframesraw(pcm)
                    samples=array('h');samples.frombytes(pcm);nonzero+=sum(value != 0 for value in samples)
                    tail.append(pcm);tail=tail[-8:];busy_seen|=native['busy']
                    if busy_seen and not native['busy']:
                        if end_step is None:end_step=step
                        idle+=1
                    else:idle=0
                    if idle>=16 and len(tail)==8 and not any(b''.join(tail)):break
                    if step%1024==0:print(json.dumps({'original':hex(code),'step':step}),flush=True)
            controls[code]=states
            complete=end_step is not None and idle>=16 and len(tail)==8 and not any(b''.join(tail))
            songs[code]={'code':code,'header':header_for(rom,code),
                         'sampling':{'steps':len(states),'limit':steps,'warmup_frames_checked':2400,
                                     'world_clear_every_step':True,'effect_rows_clear_every_step':True,
                                     'sample_rate':44100,'channels':2,'native_end_step':end_step,'idle_tail_steps':idle,
                                     'exact_zero_tail_batches':8 if complete else 0,'native_finite_complete':complete,
                                     'nonzero_sample_values':nonzero,'active_music_rows_at_end':states[-1]['active_music_rows']},
                         'files':{'states':binding(path),'wav':binding(wav_path)}}
            print(json.dumps({'original_complete':hex(code),'steps':len(states),'finite':complete}),flush=True)
    bridge=C.CDLL(str(BRIDGE));callback_type=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[callback_type,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=(C.c_uint*5)(0x38dae,0x375ae,0x38b2e,0x38b9e,0x376e4)
    with AudioSession(ROOT,out/'observed-core',core=OBSERVED) as ref:
        checkpoint=quiet_checkpoint(ref);identities['observed']=ref.identity
        for code in codes:
            ref.restore(checkpoint,restore_refresh_cycles=True);queue_command(ref,code)
            first=ref.frame+1;events=[];errors=[]
            @callback_type
            def observe(kind,width,pc,value):
                try:
                    row=ref.get_register(13)&0xffff
                    event={'step':ref.frame-first,'pc':pc,'row':row,'cpu_cycles':ref.cpu.cycles}
                    if pc==0x38dae:
                        command=ref.get_register(5)&255;args=ref.get_register(12)&0xffffff
                        assert rom[args-1]==command
                        event.update(kind='sequence',command=command,source=args-1,args=rom[args:args+8].hex())
                        if command in (0xf6,0xf8):event['target']=args+1+int.from_bytes(rom[args:args+2],'big',signed=True)
                        if command==0xf7:event['target']=args+3+int.from_bytes(rom[args+2:args+4],'big',signed=True)
                    elif pc==0x375ae:event.update(kind='tick')
                    elif pc==0x376e4:
                        address=ref.get_register(11)&0xffffff
                        event.update(kind='dac-request',descriptor_address=address,descriptor_hex=rom[address:address+8].hex())
                    else:event.update(kind='ym-register',register=ref.get_register(0)&255,value=ref.get_register(1)&255)
                    events.append(event)
                except Exception as error:errors.append(repr(error))
            assert bridge.yyh_set_observer(observe,points,len(points))
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
            try:
                for step,expected in enumerate(controls[code]):
                    result,pcm=ref.step_audio(ref.frame+1,[0]*8)
                    ram=ref.ram_cpu()
                    assert result['reference']==expected['reference'],(code,step,'RAM/video')
                    assert len(pcm)//4==expected['stereo_frames'] and sha(pcm)==expected['pcm_sha256'],(code,step,'PCM')
                    assert ram[0xf800:0xfba0].hex()==expected['driver_hex'],(code,step,'driver')
                    assert not errors,errors[:3]
            finally:ref.api('set_cpu_hook',None,C.c_void_p)(None)
            event_path=out/f'{code:02x}-events.json.gz'
            event_path.write_bytes(gzip.compress((json.dumps({'code':code,'events':events,'errors':errors,'observer_equal':True},separators=(',',':'))+'\n').encode(),mtime=0))
            grouped=defaultdict(list)
            for event in events:
                if event['kind']=='sequence' and event['command']==0xf6:
                    grouped[(event['row'],event['source'],event['target'])].append(event['step'])
            branches=[];periods=[];repeated_rows=set()
            for (row,source,target),observed in sorted(grouped.items()):
                branch={'row':row,'source':source,'target':target,'steps':observed}
                if len(observed)>1:
                    intervals={b-a for a,b in zip(observed,observed[1:])}
                    if len(intervals)==1:
                        period=next(iter(intervals));branch['period_frames']=period;periods.append(period);repeated_rows.add(row)
                    else:branch['nonuniform_intervals']=sorted(intervals)
                branches.append(branch)
            song=songs[code];sampling=song['sampling'];sampling['original_ram_video_driver_pcm_equal']=True
            active=set(sampling['active_music_rows_at_end']);all_rows=bool(active)and active<=repeated_rows
            candidate=math.lcm(*periods)if periods else None
            common=candidate if all_rows else None
            song.update(track_f6_observations=branches,track_periods=periods,candidate_period_lcm=candidate,
                        all_active_rows_have_repeated_f6=all_rows,common_period_frames=common,exact_pcm_loop_verified=False,
                        classification='finite-native-music'if sampling['native_finite_complete']else'bounded-loop-evidence'if periods else'bounded-unresolved',
                        tick_calls=sum(event['kind']=='tick'for event in events),sequence_command_counts=dict(sorted(Counter(str(event['command'])for event in events if event['kind']=='sequence').items())),
                        limits=['Probe only; common_period_frames is a structure-derived candidate until a complete recurrence window is validated',
                                'No semantic state bytes excluded','Selected events are not a complete exact-clock chip write stream',
                                'No accepted intro/loop media is produced'])
            song['raw_driver_recurrence'],song['selected_event_recurrence'],song['pcm_recurrence']=compare_periods(controls[code],events,ROOT/song['files']['wav']['path'],common)
            song['files']['events']=binding(event_path)
            print(json.dumps({'observed_complete':hex(code),'steps':sampling['steps'],'tracks_repeated':len(repeated_rows),'common_candidate':common}),flush=True)
    source_paths=[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/validate_music_survey.py',ROOT/'scripts/yuyuhakusho/reference.py',ROOT/'scripts/yuyuhakusho/extract_audio_assets.py',ROOT/'scripts/yuyuhakusho/research_audio_loops.py']
    report={'schema':1,'scope':'bounded-original-music-survey','rom_sha256':ROM_SHA,'codes':codes,'pass':True,
            'release_ready':False,'published':False,'identity':identities,'observer':binding(BRIDGE),
            'songs':[songs[code]for code in codes],
            'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in source_paths}}
    write_json(out/'survey.json',report)
    return report

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',required=True,type=Path)
    parser.add_argument('--steps',type=int,default=LIMIT);parser.add_argument('--codes',default=','.join(f'{code:02x}'for code in CODES))
    args=parser.parse_args();out=args.out.resolve();codes=[int(code,16)for code in args.codes.split(',')]
    assert out.is_relative_to(ROOT/'.local')and 0<args.steps<=LIMIT and codes and len(set(codes))==len(codes)and set(codes)<=set(CODES)
    out.mkdir(parents=True,exist_ok=True);run(out,args.steps,codes)
