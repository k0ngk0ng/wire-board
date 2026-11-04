#!/usr/bin/env python3
"""Bounded, research-only native BGM loop observations; no runtime emulator.

Failure scenarios and evidence boundary precede this implementation in
docs/research/md-yuyuhakusho/AUDIO_LOOPS.md. Existing accepted WAVs and manifests
are read-only inputs. All new output must remain in this worktree's .local/.
"""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
import wave

from extract_audio_assets import AudioSession, driver_state, queue_command, write_cpu
from reference import ROOT, buttons_at, sha


ORIGINAL = ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
OBSERVED = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
BRIDGE = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
CODES = [0x82, 0x87]
# Located from native F6 returns and the independent noise sequence, rather
# than chosen by extending an arbitrary recording deadline. Two complete
# candidate common periods plus bounded initialization cover the final pass.
PERIODS = {0x82: 3840, 0x87: 11502}
CAPTURE_LIMITS = {0x82: 8000, 0x87: 24576}


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2)+'\n')


def quiet_checkpoint(ref):
    inputs = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    for frame in range(1, 2401):
        result, _ = ref.step_audio(frame, buttons_at(inputs, frame))
        assert result['reference'] == expected[frame-1]
    for address in range(0x8000, 0xa000):
        write_cpu(ref, 0xff0000+address, 0)
    for address, size in [(0xffa030, 1), (0xfff560, 2), (0xfff562, 1), (0xfff807, 1)]:
        write_cpu(ref, address, 0, size)
    queue_command(ref, 4)
    tail = []
    for _ in range(120):
        _, pcm = ref.step_audio(ref.frame+1, [0]*8)
        tail.append(pcm)
    state = driver_state(ref)
    assert not state['busy'] and state['world_clear'] and not any(b''.join(tail[-16:]))
    return ref.save()


def chip_context(ref, symbol):
    buffer = C.create_string_buffer(65536)
    size = ref.api(symbol, C.c_int, C.c_void_p)(buffer)
    assert 0 < size < len(buffer)
    return buffer.raw[:size]


def original_pcm(code):
    catalog = json.loads((ROOT/'games/yuyuhakusho/data/audio-assets.json').read_text())
    row = next(x for x in catalog['diagnostics'] if x['code'] == code)
    path = ROOT/row['media_file']
    assert sha(path.read_bytes()) == row['wav_sha256']
    with wave.open(str(path), 'rb') as f:
        assert (f.getnchannels(), f.getsampwidth(), f.getframerate()) == (2, 2, 44100)
        pcm = f.readframes(f.getnframes())
    assert sha(pcm) == row['pcm_sha256']
    return pcm, row


def capture(out, requested_steps):
    controls = {}
    identities = {}
    bounds = {code: requested_steps or CAPTURE_LIMITS[code] for code in CODES}
    assert all(0 < bounds[c] <= CAPTURE_LIMITS[c] for c in CODES)
    with AudioSession(ROOT, out/'control-core', core=ORIGINAL) as ref:
        checkpoint = quiet_checkpoint(ref)
        identities['original'] = ref.identity
        for code in CODES:
            steps = bounds[code]
            ref.restore(checkpoint, restore_refresh_cycles=True)
            queue_command(ref, code)
            expected_pcm, binding = original_pcm(code)
            offset = 0
            frames = []
            path = out/f'{code:02x}-native-states.jsonl.gz'
            wav_path = out/f'{code:02x}-control.wav'
            with gzip.open(path, 'wt', compresslevel=3) as f, wave.open(str(wav_path), 'wb') as wav:
                wav.setnchannels(2)
                wav.setsampwidth(2)
                wav.setframerate(44100)
                for step in range(steps):
                    result, pcm = ref.step_audio(ref.frame+1, [0]*8)
                    ram = ref.ram_cpu()
                    assert not any(ram[0x8000:0xa000])
                    assert not any(ram[0xfa20:0xfba0]), ('Unexpected effect channel', code, step)
                    if offset < len(expected_pcm):
                        assert pcm == expected_pcm[offset:offset+len(pcm)], (code, step, 'existing PCM differs')
                    wav.writeframesraw(pcm)
                    offset += len(pcm)
                    summary = {'step': step, 'reference': result['reference'],
                               'stereo_frames': len(pcm)//4, 'pcm_sha256': sha(pcm),
                               'driver_hex': ram[0xf800:0xfba0].hex()}
                    frames.append(summary)
                    full = {**summary, 'z80_hex': bytes((C.c_ubyte*0x2000).in_dll(ref.lib, 'zram')).hex(),
                            'ym2612_hex': chip_context(ref, 'YM2612SaveContext').hex(),
                            'psg_hex': chip_context(ref, 'psg_context_save').hex()}
                    f.write(json.dumps(full, separators=(',', ':'))+'\n')
                    if step%1200 == 0:
                        print(json.dumps({'control': hex(code), 'step': step}), flush=True)
            controls[code] = frames
            write_json(out/f'{code:02x}-control-summary.json', {
                'steps': steps, 'existing_pcm_prefix_equal': True, 'existing_media': binding,
                'existing_pcm_prefix_bytes': min(offset, len(expected_pcm)),
                'pcm_bytes': offset, 'native_states_file': str(path.relative_to(ROOT)),
                'native_states_sha256': sha(path.read_bytes()),
                'pcm_wav_file': str(wav_path.relative_to(ROOT)), 'wav_sha256': sha(wav_path.read_bytes())})

    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(BRIDGE))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*5)(0x38dae, 0x375ae, 0x38b2e, 0x38b9e, 0x376e4)
    with AudioSession(ROOT, out/'observed-core', core=OBSERVED) as ref:
        checkpoint = quiet_checkpoint(ref)
        identities['observed'] = ref.identity
        for code in CODES:
            steps = bounds[code]
            ref.restore(checkpoint, restore_refresh_cycles=True)
            queue_command(ref, code)
            errors = []
            events = []
            first_frame = ref.frame+1

            @callback_type
            def observe(kind, width, pc, value):
                try:
                    ram = ref.ram_cpu()
                    a5 = ref.get_register(13)&0xffff
                    event = {'step': ref.frame-first_frame, 'pc': pc, 'row': a5,
                             'cpu_cycles': ref.cpu.cycles}
                    if pc == 0x38dae:
                        command = ref.get_register(5)&255
                        args = ref.get_register(12)&0xffffff
                        assert rom[args-1] == command
                        event.update(kind='sequence', command=command, source=args-1,
                                     args=rom[args:args+8].hex(), row_hex=ram[a5:a5+0x30].hex(),
                                     globals_hex=ram[0xf800:0xf840].hex())
                        if command in [0xf6, 0xf8]:
                            event['target'] = args+1+int.from_bytes(rom[args:args+2], 'big', signed=True)
                        if command == 0xf7:
                            event['target'] = args+3+int.from_bytes(rom[args+2:args+4], 'big', signed=True)
                    elif pc == 0x375ae:
                        event.update(kind='tick', driver_hex=ram[0xf800:0xfba0].hex())
                    elif pc == 0x376e4:
                        address = ref.get_register(11)&0xffffff
                        event.update(kind='dac-request', descriptor_address=address,
                                     descriptor_hex=rom[address:address+8].hex())
                    else:
                        event.update(kind='ym-register', register=ref.get_register(0)&255,
                                     value=ref.get_register(1)&255)
                    events.append(event)
                except Exception as e:
                    errors.append(str(e))

            assert bridge.yyh_set_observer(observe, pcs, len(pcs))
            ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
            try:
                for step in range(steps):
                    result, pcm = ref.step_audio(ref.frame+1, [0]*8)
                    expected = controls[code][step]
                    assert result['reference'] == expected['reference']
                    assert len(pcm)//4 == expected['stereo_frames'] and sha(pcm) == expected['pcm_sha256']
                    assert ref.ram_cpu()[0xf800:0xfba0].hex() == expected['driver_hex']
                    assert not errors, errors[:3]
                    if step%1200 == 0:
                        print(json.dumps({'observed': hex(code), 'step': step}), flush=True)
            finally:
                ref.api('set_cpu_hook', None, C.c_void_p)(None)
            with gzip.open(out/f'{code:02x}-events.json.gz', 'wt', compresslevel=3) as f:
                json.dump({'code': code, 'steps': steps, 'events': events, 'errors': errors,
                           'original_ram_video_driver_pcm_equal': True}, f, separators=(',', ':'))
            print(json.dumps({'code': hex(code), 'events': len(events), 'observer_equal': True}), flush=True)
    write_json(out/'capture.json', {'pass': True, 'steps': bounds, 'candidate_periods': PERIODS,
                                 'codes': CODES, 'identity': identities,
                                 'script_sha256': sha(Path(__file__).read_bytes()),
                                 'observer_sha256': sha(BRIDGE.read_bytes()),
                                 'limits': ['Video-frame snapshots do not themselves locate exact intra-frame sample boundaries',
                                            'Logical sequence recurrence and output PCM recurrence are separate obligations']})


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--steps', type=int, help='Optional shorter common capture bound; default uses located per-song periods')
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local')
    out.mkdir(parents=True, exist_ok=True)
    capture(out, args.steps)


if __name__ == '__main__':
    main()
