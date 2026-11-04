#!/usr/bin/env python3
"""Research-only PCM acquisition from the pinned original game/core.

Media stays in caller-selected in-worktree .local output. This frontend never
changes shared reference.py and is not part of the independent game runtime.
"""
import argparse
from array import array
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
import sys
import wave

from reference import ROOT, ReferenceSession, buttons_at, probe, sha


class AudioSession(ReferenceSession):
    def __init__(self, *args, **kwargs):
        self.pcm_chunks = []
        self.audio_frames = []
        self.batch_calls = 0
        self.single_calls = 0
        super().__init__(*args, **kwargs)
        batch_type = C.CFUNCTYPE(C.c_size_t, C.c_void_p, C.c_size_t)
        single_type = C.CFUNCTYPE(None, C.c_int16, C.c_int16)
        @batch_type
        def batch(pointer, frames):
            if frames:
                self.pcm_chunks.append(C.string_at(pointer, frames*4))
            self.batch_calls += 1
            return frames
        @single_type
        def single(left, right):
            self.pcm_chunks.append(array('h', [left, right]).tobytes())
            self.single_calls += 1
        self.callbacks.extend([batch, single])
        self.api('retro_set_audio_sample_batch', None, batch_type)(batch)
        self.api('retro_set_audio_sample', None, single_type)(single)
        av = probe.AV()
        self.api('retro_get_system_av_info', None, C.POINTER(probe.AV))(C.byref(av))
        self.sample_rate = int(av.timing.sample_rate)
        assert self.sample_rate == av.timing.sample_rate

    def step_audio(self, frame, buttons):
        self.pcm_chunks = []
        result = self.step(frame, buttons)
        pcm = b''.join(self.pcm_chunks)
        if sys.byteorder != 'little':
            samples = array('h'); samples.frombytes(pcm); samples.byteswap(); pcm = samples.tobytes()
        assert len(pcm)%4 == 0
        return result, pcm


def write_wave(path, pcm, sample_rate):
    path = Path(path).resolve()
    assert path.is_relative_to(ROOT)
    with wave.open(str(path), 'wb') as f:
        f.setnchannels(2); f.setsampwidth(2); f.setframerate(sample_rate); f.writeframes(pcm)


def audio_summary(pcm):
    samples = array('h'); samples.frombytes(pcm)
    if sys.byteorder != 'little': samples.byteswap()
    return {'stereo_frames': len(pcm)//4, 'pcm_bytes': len(pcm), 'pcm_sha256': sha(pcm),
            'nonzero_samples': sum(s != 0 for s in samples), 'peak': max((abs(s) for s in samples), default=0)}


def driver_state(ref):
    ram = ref.ram_cpu()
    zram = bytes((C.c_ubyte*0x2000).in_dll(ref.lib, 'zram'))
    offsets = list(range(0xf840, 0xfba0, 0x30))
    active = [off for off in offsets if ram[off]&128]
    flags = {hex(off): zram[off] for off in [0x1ffc, 0x1ffd, 0x1ffe, 0x1f86, 0x1fa6]}
    return {'active_rows': active, 'z80_flags': flags, 'selected': ram[0xf809],
            'queue': list(ram[0xf80a:0xf80e]), 'priority': ram[0xf800],
            'driver_ram_sha256': sha(ram[0xf800:0xfba0]),
            'music_signature': sha(ram[0xf800:0xf808]+ram[0xf80f:0xfa20]),
            'world_clear': not any(ram[0x8000:0xa000]),
            'busy': bool(active) or any(zram[off] for off in [0x1ffc, 0x1ffd, 0x1ffe, 0x1f86, 0x1fa6])}


def write_cpu(ref, address, value, size=1):
    for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
        C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte


def queue_command(ref, code):
    for slot in range(4): write_cpu(ref, 0xfff80a+slot, code if slot == 0 else 0)
    write_cpu(ref, 0xfff809, 255)


def sample_descriptor(rom, code):
    assert 6 <= code <= 127
    address = 0x380a8+(code-6)*8
    data = rom[address:address+8]
    bank = (data[0]<<16)|((data[1]&128)<<8)
    z80_header = int.from_bytes(data[2:4], 'little')
    header_address = bank+(z80_header&0x7fff)
    header = rom[header_address:header_address+4]
    start = int.from_bytes(header[:2], 'little')
    size = int.from_bytes(header[2:], 'little')
    sample_address = bank+(start&0x7fff)
    assert z80_header >= 0x8000 and start >= 0x8000 and 0 < size <= 32768
    assert sample_address+size <= bank+32768 and sample_address+size <= len(rom)
    return {'code': code, 'descriptor_address': address, 'descriptor_bytes': data.hex(),
            'z80_header': z80_header, 'header_address': header_address, 'header_bytes': header.hex(),
            'sample_address': sample_address, 'encoded_bytes': size, 'encoded_sha256': sha(rom[sample_address:sample_address+size]),
            'rate_byte': data[4], 'flags': data[5], 'routing_byte': data[6]}


def sequence_descriptor(rom,code):
    assert 160 <= code <= 171
    entry=0x3a60c+(code-160)*4
    address=int.from_bytes(rom[entry:entry+4],'big')
    header=rom[address:address+4]
    count=header[3]
    assert 0 < address < len(rom)-4 and 1 <= count <= 6
    records=rom[address+4:address+4+count*6]
    return {'code':code,'classification':'native FM/PSG sequence command',
            'table_entry':entry,'sequence_address':address,'header_bytes':header.hex(),
            'channel_records':records.hex(),'channel_count':count,'header_sha256':sha(header+records)}


def isolate_samples(out, codes, music_codes, music_frames=1800):
    """Research-only declared private input fixture, preserving native synthesis."""
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    pinned = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assets, music = [], []
    with AudioSession(ROOT, out/'isolated') as ref:
        for frame in range(1, 2401):
            result, _ = ref.step_audio(frame, buttons_at(events,frame))
            assert result['reference'] == pinned[frame-1]
        # These are deliberately private reference-process inputs. Clear all
        # non-audio objects and delayed world producers; do not touch native
        # sound rows, Z80 sample bytes, FM state or captured PCM.
        for offset in range(0x8000,0xa000): write_cpu(ref,0xff0000+offset,0)
        for address,size in [(0xffa030,1),(0xfff560,2),(0xfff562,1),(0xfff807,1)]: write_cpu(ref,address,0,size)
        queue_command(ref,4) # Actual37d20 control-table entry38a2c: clear/key-off.
        quiet_pcm = []
        for _ in range(120):
            _, pcm = ref.step_audio(ref.frame+1,[0]*8); quiet_pcm.append(pcm)
        quiet = driver_state(ref)
        assert quiet['world_clear'] and not quiet['busy'] and not any(b''.join(quiet_pcm[-16:])), quiet
        checkpoint = ref.save()

        def capture(code, maximum, finite):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            queue_command(ref,code)
            chunks, states, hashes = [], [], []
            seen_busy = False; idle_tail = 0; ended_at = None
            for step in range(maximum):
                result,pcm = ref.step_audio(ref.frame+1,[0]*8)
                state = driver_state(ref); assert state['world_clear']
                seen_busy |= state['busy']; chunks.append(pcm); states.append(state)
                hashes.append({'step':step,'stereo_frames':len(pcm)//4,'pcm_sha256':sha(pcm)})
                if seen_busy and not state['busy']:
                    if ended_at is None: ended_at = step
                    idle_tail += 1
                else: idle_tail = 0
                if finite and idle_tail >= 16 and not any(b''.join(chunks[-8:])):
                    break
            pcm = b''.join(chunks)
            summary = audio_summary(pcm)
            summary.update(code=code,video_steps=len(chunks),seen_busy=seen_busy,driver_ended_step=ended_at,
                           complete=bool(finite and ended_at is not None and idle_tail>=16 and not any(b''.join(chunks[-8:]))),
                           final_state=states[-1],frame_audio=hashes)
            return pcm,summary,states

        sample_dir=out/'samples';sample_dir.mkdir(exist_ok=True)
        for code in codes:
            pcm,first,states=capture(code,600,True)
            again,second,_=capture(code,600,True)
            assert pcm == again and first == second, ('PCM replay differs',code)
            path=sample_dir/f'{code:02x}.wav';write_wave(path,pcm,ref.sample_rate)
            first.update(sample_descriptor(rom,code)if code<128 else sequence_descriptor(rom,code));first.update(file=str(path.relative_to(ROOT)),wav_sha256=sha(path.read_bytes()),
                 first_nonzero_stereo_frame=next((i//4 for i in range(0,len(pcm),2) if pcm[i:i+2] != b'\0\0'),None),
                 pcm_replay_equal=True,native_terminal_flags=states[first['driver_ended_step']]['z80_flags'] if first['driver_ended_step'] is not None else None)
            assets.append(first)
            print(json.dumps({'sample':hex(code),'steps':first['video_steps'],'frames':first['stereo_frames'],'complete':first['complete']}),flush=True)
        music_dir=out/'music-excerpts';music_dir.mkdir(exist_ok=True)
        for code in music_codes:
            pcm,first,states=capture(code,music_frames,True)
            again,second,_=capture(code,music_frames,True)
            assert pcm==again and first==second,('BGM replay differs',code)
            path=music_dir/f'{code:02x}.wav';write_wave(path,pcm,ref.sample_rate)
            signatures={};repeat=None
            for i,state in enumerate(states):
                if first['complete'] or not state['busy'] or not state['active_rows']:
                    continue
                sig=state['music_signature']
                if sig in signatures:
                    j=signatures[sig];period=i-j
                    # A repeated driver state is a logical sequence clue, not
                    # proof of a seamless infinite PCM loop.
                    if i+period <= len(states) and all(states[j+k]['busy'] and states[i+k]['busy'] and states[j+k]['music_signature']==states[i+k]['music_signature']for k in range(period)):
                        repeat={'start_step':j,'period_steps':period,'second_start_step':i};break
                else:signatures[sig]=i
            first.update(file=str(path.relative_to(ROOT)),wav_sha256=sha(path.read_bytes()),pcm_replay_equal=True,
                         loop_verified=False,logical_driver_repeat=repeat,
                         classification='complete finite native music cue'if first['complete']else'bounded native music excerpt; not a complete loop asset')
            music.append(first);print(json.dumps({'music_excerpt':hex(code),'steps':first['video_steps'],'logical_repeat':repeat}),flush=True)
        identity=ref.identity;sample_rate=ref.sample_rate
    report={'scope':'Declared-private native sample-command extraction; music excerpts remain bounded', 'identity':identity,
            'script_sha256':sha(Path(__file__).read_bytes()),'sample_rate':sample_rate,'channels':2,'sample_width_bytes':2,
            'warmup_frames_match_pinned':2400,'quieting_command':4,'quieting_video_steps':120,'quiet_state':quiet,
            'quiet_tail_exact_zero':True,'sample_assets':assets,'music_excerpts':music,
            'finite_complete_assets':sum(a['complete']for a in assets),'bounded_incomplete_samples':[a['code']for a in assets if not a['complete']],
            'pass':bool(assets)and all(a['pcm_replay_equal']for a in assets),
            'limits':['World objects and delayed producers are deliberately disabled in private sampling','No native driver/Z80/FM code is shipped in game runtime',
                      'Termination is native driver-inactive plus16 video steps and8 exact-zero PCM batches, not an amplitude threshold',
                      'Music excerpt duration is bounded; driver-state repetition does not establish a seamless PCM loop or complete music lifecycle',
                      'Original mixed music/voice channel stealing and playback scheduling remain separate from isolated assets']}
    (out/'isolated-assets.json').write_text(json.dumps(report,indent=2)+'\n')
    return report


def build_catalog(out, frontend, natural, isolated, metadata_dir):
    """Verify acquired WAVs with the standard reader, then retain metadata."""
    assert frontend['frames_match_pinned_ram_video'] and frontend['replay_ram_video_equal'] and frontend['replay_pcm_equal']
    assert natural['ram_video_match_original'] and natural['pcm_frames_match_original'] and isolated['pass']
    current_sha=sha(Path(__file__).read_bytes())
    assert frontend['frontend_script_sha256']==natural['script_sha256']==isolated['script_sha256']==current_sha
    rows=[];diagnostics=[]
    for group in ['sample_assets','music_excerpts']:
        for item in isolated[group]:
            path=ROOT/item['file']
            with wave.open(str(path),'rb')as f:
                assert (f.getnchannels(),f.getsampwidth(),f.getframerate(),f.getcomptype())==(2,2,44100,'NONE')
                assert f.getnframes()==item['stereo_frames']
                pcm=f.readframes(f.getnframes())
            assert sha(path.read_bytes())==item['wav_sha256'] and sha(pcm)==item['pcm_sha256'] and len(pcm)==item['pcm_bytes']
            assert item['pcm_replay_equal']
            row={k:v for k,v in item.items()if k not in ['frame_audio','final_state']}
            row['native_final_state']=item['final_state']
            row['media_file']=row.pop('file')
            row['duration_seconds']=item['stereo_frames']/44100
            row['published']=False
            row['logical_asset']=f'yuyuhakusho/audio/{item["code"]:02x}.wav'
            if item['complete'] and item['peak']>0:
                assert not item['final_state']['busy'] and item['driver_ended_step']is not None
                assert not any(pcm[-8*730*4:]) # Exact-zero window shorter than eight actual callback batches.
                row['runtime_asset_ready']=True
                row['classification']='finite-music-cue'if group=='music_excerpts'else'pcm-sample-command'if item['code']<128 else'finite-fm-psg-sequence'
                rows.append(row)
            else:
                row['runtime_asset_ready']=False
                row['classification']='silent-one-byte-descriptor'if item['code']==127 and item['peak']==0 else'bounded-music-excerpt'
                row['reason']='No audible output/native busy phase observed; not a playable asset'if item['code']==127 else'Native music remains active; seamless loop and complete duration not proven'
                diagnostics.append(row)
    assert len({r['code']for r in rows})==len(rows)
    ready={r['code']for r in rows}
    animations=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/animation-catalog.json.gz').read_bytes()))
    bindings={}
    for frame in animations['frames']:
        code=frame['data']['sound']
        if code:
            bind=bindings.setdefault(code,{'code':code,'characters':set(),'references':0})
            bind['characters'].add(frame['character_id']);bind['references']+=1
    for item in bindings.values():
        item['characters']=sorted(item['characters']);item['asset_ready']=item['code']in ready
    assert all(b['asset_ready']for b in bindings.values())
    library={'schema':1,'rom_sha256':isolated['identity']['rom_sha256'],'format':{'sample_rate':44100,'channels':2,'sample_width_bytes':2,'pcm_encoding':'signed16-little-endian'},
             'assets':rows,'diagnostics':diagnostics,'animation_bindings':sorted(bindings.values(),key=lambda x:x['code']),
             'control_commands':[{'code':0,'entry':0x38a7a,'meaning':'upload original Z80 driver then reset; not media'},
                                 {'code':1,'entry':0x38916,'meaning':'set original fade control3/40; not media'},
                                 {'code':2,'entry':0x38808,'meaning':'stop primary effect channels; not media'},
                                 {'code':3,'entry':0x388ac,'meaning':'stop secondary effect channels; not media'},
                                 {'code':4,'entry':0x38a2c,'meaning':'clear/key-off original sound driver; not media'},
                                 {'code':5,'entry':0x38928,'meaning':'set original fade control32/50; not media'}],
             'event_bindings':{'default_arena_request':0x82,'default_chu_opponent_request':0x87,'results_music_request':0x8f,
                               'round_intro_requests':[0x58,0x59],'clash_guard_request':0x77,'physical_hit_requests':[0x7a,0x7b,0x7c],
                               'other_hit_request':0x78,'landing_request':0x7d,'chu_ko_request':0x3f,'chu_result_voice_request':0x6f},
             'limits':['Catalog contains verified finite acquisitions only; not an independent synthesizer or complete playback scheduler',
                       'Every media path is local/unpublished; platform publication must use immutableCDN assets andBoardProps.audio',
                       'Animation references are decoded-data bindings, not proof of all normal gameplay reachability',
                       'Music82/87 remain bounded excerpts; no loop inference from an idle or repeated state',
                       'Original mixed channel stealing/pause/fade timing and other music tracks remain unimplemented']}
    metadata_dir=Path(metadata_dir).resolve();assert metadata_dir.is_relative_to(ROOT);metadata_dir.mkdir(parents=True,exist_ok=True)
    catalog_path=metadata_dir/'audio-assets.json';catalog_path.write_text(json.dumps(library,indent=2)+'\n')
    # Full acquisition rows retain all per-video PCM hashes and native terminal
    # states without checking large media into the source repository.
    acquisition={'frontend':frontend,'natural':natural,'isolated':isolated}
    acquisition_bytes=(json.dumps(acquisition,separators=(',',':'))+'\n').encode()
    (metadata_dir/'audio-acquisition.json.gz').write_bytes(gzip.compress(acquisition_bytes,mtime=0))
    for src,name in [(out/'natural/audio-natural-events.json.gz','audio-natural-events.json.gz'),
                     (out/'frontend/frontend-replay.json','audio-frontend-replay.json')]:
        if src.suffix=='.gz':(metadata_dir/name).write_bytes(src.read_bytes())
        else:(metadata_dir/(name+'.gz')).write_bytes(gzip.compress(src.read_bytes(),mtime=0))
    (out/'audio-catalog-local.json').write_text(json.dumps(library,indent=2)+'\n')
    report={'scope':'Deterministic original PCM/sequence finite assets and bounded music diagnostics; no runtime MD synthesis',
            'pass':True,'goal_complete':False,'release_ready':False,'script_sha256':current_sha,'identity':isolated['identity'],
            'coverage':{'finite_pcm_sample_commands':sum(r['classification']=='pcm-sample-command'for r in rows),
                        'finite_fm_psg_sequences':sum(r['classification']=='finite-fm-psg-sequence'for r in rows),
                        'finite_music_cues':sum(r['classification']=='finite-music-cue'for r in rows),'ready_assets':len(rows),
                        'excluded_or_bounded_codes':[r['code']for r in diagnostics], 'animation_sound_codes':len(bindings),
                        'all_animation_sound_codes_have_finite_assets':True,'natural_ram_video_pcm_frames':natural['frames'],
                        'natural_requests':natural['request_count'],'natural_selected':natural['selected_count'],
                        'save_restore_pcm_frames':frontend['replay_frames'],'all_asset_pcm_replays_equal':True},
            'verification':[{'name':'Original3600-frame frontend RAM/video baseline','pass':frontend['frames_match_pinned_ram_video']},
                            {'name':'Original120-frame audio save/restore replay','pass':frontend['replay_pcm_equal']},
                            {'name':'Unhooked/hooked5500-frame fullRAM/video/PCM comparison','pass':natural['pcm_frames_match_original']},
                            {'name':'Twice-rendered each isolated command, native terminal + exact-zero tail','pass':isolated['pass']},
                            {'name':'StandardWAV reader headers, sample count, fullPCM/fileSHA for every acquisition','pass':True}],
            'files':[{'path':str(p.relative_to(ROOT)),'sha256':sha(p.read_bytes())}for p in metadata_dir.glob('audio-*')if p.name!='audio-validation.json'],
            'sources':[{'path':str(Path(__file__).relative_to(ROOT)),'sha256':current_sha},
                       {'path':'scripts/yuyuhakusho/reference.py','sha256':sha((ROOT/'scripts/yuyuhakusho/reference.py').read_bytes())}],
            'media_root':str(out.relative_to(ROOT)), 'published':False,'limits':library['limits']}
    (metadata_dir/'audio-validation.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'audio_assets_ready':len(rows),'diagnostics':[r['code']for r in diagnostics],'animation_bindings':len(bindings),'pass':True}),flush=True)
    return report


def trace_natural_audio(out, frames=5500):
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/engine/testdata/match-lifecycle-reference-frames.json.gz').read_bytes()))
    assert frames <= len(expected)
    control_audio=[];control_pcm=[]
    with AudioSession(ROOT,out/'trace-control')as ref:
        for frame in range(1,frames+1):
            result,pcm=ref.step_audio(frame,buttons_at(events,frame));assert result['reference']==expected[frame-1]
            control_audio.append({'frames':len(pcm)//4,'sha256':sha(pcm)});control_pcm.append(pcm)
        control_identity=ref.identity
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*2)(0x21960,0x37cb4);requests=[];selected=[];errors=[];observed_audio=[];observed_pcm=[]
    with AudioSession(ROOT,out/'trace-observed',core=core)as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                ram=ref.ram_cpu();sp=ref.get_register(15)&0xffff
                caller=int.from_bytes(bytes(ram[(sp+i)&0xffff]for i in range(4)),'big')
                common={'frame':ref.frame,'pc':hex(pc),'caller':hex(caller),'sr':ref.get_register(17),'stack':ref.get_register(15),'a5':ref.get_register(13)}
                if pc==0x21960:
                    requests.append({**common,'code':ref.get_register(0)&255,'queue_before':list(ram[0xf80a:0xf80e])})
                else:
                    code=ram[0xf809]
                    if code!=255:selected.append({**common,'code':code,'priority':ram[0xf800]})
            except Exception as e:errors.append(str(e))
        assert bridge.yyh_set_observer(observe,pcs,2)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        for frame in range(1,frames+1):
            result,pcm=ref.step_audio(frame,buttons_at(events,frame));assert result['reference']==expected[frame-1]
            observed_audio.append({'frames':len(pcm)//4,'sha256':sha(pcm)});observed_pcm.append(pcm)
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert not errors and observed_audio==control_audio
        pcm=b''.join(observed_pcm);write_wave(out/'natural-observed.wav',pcm,ref.sample_rate)
        report={'scope':'Passive command/PCM natural comparison, not isolated assets','identity':ref.identity,'control_identity':control_identity,
                'frames':frames,'ram_video_match_original':True,'pcm_frames_match_original':observed_audio==control_audio,
                'pcm':audio_summary(pcm),'sample_rate':ref.sample_rate,'channels':2,'script_sha256':sha(Path(__file__).read_bytes()),
                'observer_sha256':sha(bridge_path.read_bytes()),'request_count':len(requests),'selected_count':len(selected),
                'request_codes':dict(sorted(Counter(str(r['code'])for r in requests).items(),key=lambda x:int(x[0]))),
                'selected_codes':dict(sorted(Counter(str(r['code'])for r in selected).items(),key=lambda x:int(x[0]))),
                'errors':errors,'pass':not errors and observed_audio==control_audio}
    payload={'report':report,'requests':requests,'selected':selected,'frame_pcm':observed_audio}
    zipped=gzip.compress((json.dumps(payload,separators=(',',':'))+'\n').encode(),mtime=0)
    (out/'audio-natural-events.json.gz').write_bytes(zipped)
    (out/'audio-natural-report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
    return report


def probe_frontend(out, frames):
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    assert 2520 <= frames <= 3600
    hashes, pcm_frames, replay_first, replay_second = [], [], [], []
    with AudioSession(ROOT, out/'natural') as ref:
        for frame in range(1, frames+1):
            result, pcm = ref.step_audio(frame, buttons_at(events, frame))
            hashes.append(result['reference']); pcm_frames.append(pcm)
            if frame == 2400:
                checkpoint = ref.save()
            if 2401 <= frame <= 2520:
                replay_first.append({'reference': result['reference'], **audio_summary(pcm)})
        assert hashes == expected[:frames], 'PCM callback registration changed original RAM/video baseline'
        ref.restore(checkpoint, restore_refresh_cycles=True)
        for frame in range(2401, 2521):
            result, pcm = ref.step_audio(frame, buttons_at(events, frame))
            replay_second.append({'reference': result['reference'], **audio_summary(pcm)})
        combined = b''.join(pcm_frames)
        write_wave(out/'natural-mix.wav', combined, ref.sample_rate)
        report = {'scope': 'PCM frontend and save/replay evidence only; natural mix is not an isolated sound asset',
                  'identity': ref.identity, 'sample_rate': ref.sample_rate, 'channels': 2, 'sample_width_bytes': 2,
                  'frames': frames, 'frames_match_pinned_ram_video': hashes == expected[:frames],
                  'frontend_script_sha256': sha(Path(__file__).read_bytes()), 'natural_pcm': audio_summary(combined),
                  'batch_calls': ref.batch_calls, 'single_calls': ref.single_calls,
                  'replay_frames': 120, 'replay_ram_video_equal': all(a['reference'] == b['reference'] for a,b in zip(replay_first,replay_second)),
                  'replay_pcm_equal': all(a['pcm_sha256'] == b['pcm_sha256'] and a['stereo_frames'] == b['stereo_frames'] for a,b in zip(replay_first,replay_second)),
                  'replay_different_pcm_frames': [2401+i for i,(a,b) in enumerate(zip(replay_first,replay_second)) if a['pcm_sha256'] != b['pcm_sha256'] or a['stereo_frames'] != b['stereo_frames']],
                  'limits': ['Original core still performs synthesis during research acquisition', 'No isolation, command mapping, complete termination or loop metadata is proven by this probe'],
                  'pass': hashes == expected[:frames] and len(combined) > 0}
    (out/'frontend-probe.json').write_text(json.dumps(report,indent=2)+'\n')
    (out/'frontend-replay.json').write_text(json.dumps({'first':replay_first,'restored':replay_second},indent=2)+'\n')
    print(json.dumps(report,indent=2))
    return report


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--frames', type=int, default=3600)
    p.add_argument('--mode',choices=['probe','isolate','trace','all'],default='probe')
    p.add_argument('--codes',default='3f,58,59,5a,5b,5c,6f,77,78,7a,7b,7c,7d')
    p.add_argument('--music',default='82,87')
    p.add_argument('--music-frames',type=int,default=1800)
    p.add_argument('--metadata-dir',type=Path,default=ROOT/'games/yuyuhakusho/data')
    args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True,exist_ok=True)
    if args.mode=='probe':probe_frontend(out,args.frames)
    elif args.mode=='isolate':
        codes=list(range(6,128))if args.codes=='all-samples'else list(range(160,172))if args.codes=='all-sequences'else[int(x,16)for x in args.codes.split(',')if x]
        music=[int(x,16)for x in args.music.split(',')if x]
        assert all(6<=c<=127 or 160<=c<=171 for c in codes)and all(129<=c<=159 for c in music)
        isolate_samples(out,codes,music,args.music_frames)
    elif args.mode=='trace':trace_natural_audio(out,5500)
    else:
        frontend=probe_frontend(out/'frontend',3600)
        natural=trace_natural_audio(out/'natural',5500)
        isolated=isolate_samples(out/'isolated',list(range(6,128))+list(range(160,172)),[0x82,0x87,0x8f],7200)
        build_catalog(out,frontend,natural,isolated,args.metadata_dir)


if __name__ == '__main__': main()
