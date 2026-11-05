"""Private mixed-score native controls and source-free semantic observations."""
from pathlib import Path
import argparse
import ctypes as C
import gzip
import hashlib
import json

from capture_audio_score_reads import POINTS, cpu_range
from extract_audio_assets import AudioSession, queue_command
from reference import ROOT, sha
from research_audio_loops import BRIDGE, OBSERVED, ORIGINAL, quiet_checkpoint
from score_claim_check import cases
from score_parameter_tables import read_fm_patch

ALL_CODES = [130, 131, 132, 133, 134, 135, 136, 138, 141, 149, 143] + list(range(160, 172))
SLOTS = ([('music.' + name, 'music') for name in
          ('dac', 'fm1', 'fm2', 'fm3', 'fm4', 'fm5', 'fm6', 'psg1', 'psg2', 'psg3')] +
         [('primary.' + name, 'effect-primary') for name in
          ('fm3', 'fm4', 'fm5', 'psg1', 'psg2', 'psg3')] +
         [('secondary.fm4', 'effect-secondary'), ('secondary.psg3', 'effect-secondary')])
CHANNELS = {0: 'fm1', 1: 'fm2', 2: 'fm3', 4: 'fm4', 5: 'fm5', 6: 'fm6',
            128: 'psg1', 160: 'psg2', 192: 'psg3', 224: 'psg-noise'}
EXTRA_POINTS = [0x37cb4, 0x376c4, 0x37df0]
OBSERVATION_POINTS = list(dict.fromkeys(POINTS + EXTRA_POINTS))
assert len(OBSERVATION_POINTS) <= 64


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def saved(path, value):
    raw = json.dumps(value, separators=(',', ':')).encode() + b'\n'
    path.write_bytes(gzip.compress(raw, mtime=0) if path.suffix == '.gz' else raw)


def binding(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes()), 'bytes': path.stat().st_size}


def s8(value):
    return value - 256 if value >= 128 else value


def s16(value):
    return value - 65536 if value >= 32768 else value


def word(raw, at):
    return int.from_bytes(raw[at:at + 2], 'big')


def ym_effective(register, value):
    if 0x30 <= register <= 0x9f:
        value &= {0x30: 0x7f, 0x40: 0x7f, 0x50: 0xdf, 0x60: 0x9f,
                  0x70: 0x1f, 0x80: 0xff, 0x90: 0x0f}[register & 0xf0]
    elif 0xa4 <= register <= 0xa6 or 0xac <= register <= 0xae:
        value &= 0x3f
    elif 0xb0 <= register <= 0xb2:
        value &= 0x3f
    elif 0xb4 <= register <= 0xb6:
        value &= 0xf7
    elif register == 0x22:
        value &= 15
    elif register == 0x25:
        value &= 3
    elif register == 0x28:
        value &= 0xf7
    elif register == 0x2b:
        value &= 128
    return value


class SemanticView:
    def __init__(self, rom):
        self.rom = rom
        self.owners = {}
        self.modulations = {}
        self.patch_ids = {}
        self.headers = {}
        for code in ALL_CODES:
            entry = 0x3a492 + (code - 129) * 4 if code < 160 else 0x3a60c + (code - 160) * 4
            address = int.from_bytes(rom[entry:entry + 4], 'big')
            self.headers[address] = code
        self.descriptors = {}
        for note in range(129, 154):
            at = 0x37724 + (note - 129) * 8
            raw = rom[at:at + 8]
            bank = raw[0] << 16 | (raw[1] & 128) << 8
            header = bank + (int.from_bytes(raw[2:4], 'little') & 32767)
            begin = bank + (int.from_bytes(rom[header:header + 2], 'little') & 32767)
            size = int.from_bytes(rom[header + 2:header + 4], 'little')
            if not size or begin + size > len(rom):
                continue
            descriptor = {'sample_id': sha(rom[begin:begin + size]), 'native_rate': raw[4],
                          'flags': raw[5], 'routing': 'track-pan'}
            self.descriptors[note] = 'descriptor-' + sha(canonical(descriptor))[:20]

    def patch(self, bank, index):
        key = (bank, index)
        if key not in self.patch_ids:
            data = read_fm_patch(self.rom, bank, index).to_score_data()
            data.pop('id', None)
            self.patch_ids[key] = 'patch-' + sha(canonical(data))[:20]
        return self.patch_ids[key]

    def modulation(self, pointer):
        if not pointer:
            return None
        if pointer not in self.modulations:
            raw = self.rom[pointer:pointer + 4]
            assert len(raw) == 4
            self.modulations[pointer] = {'delay': raw[0], 'rate': raw[1],
                                         'delta': s8(raw[2]), 'steps': raw[3]}
        return self.modulations[pointer]

    def state(self, driver):
        voices = []
        for index, (slot, domain) in enumerate(SLOTS):
            at = 64 + index * 48
            row = driver[at:at + 48]
            sequence_present = bool(int.from_bytes(row[4:8], 'big'))
            owner = self.owners.get(0xf840 + index * 48) if sequence_present else None
            dac = slot == 'music.dac'
            psg = '.psg' in slot
            bank = int.from_bytes(driver[32:36] if domain == 'music' else row[32:36], 'big')
            tone = row[11] if psg else self.patch(bank, row[11]) if owner and not dac else None
            pan_enabled = bool(row[31]) and not psg
            flags = row[0]
            assert flags & ~0x96 == 0, (slot, flags)
            voices.append({
                'slot': slot, 'domain': domain, 'owner': owner,
                'active': bool(flags & 128), 'note_off': bool(flags & 2),
                'claimed': bool(flags & 4), 'tie': bool(flags & 16),
                'hardware_voice': 'dac' if dac else CHANNELS[row[1]] if sequence_present else None,
                'duration_multiplier': row[2], 'pitch_envelope_scale': s8(row[3]),
                'transpose': s8(row[8]), 'base_level': row[9],
                'pitch_control': {'envelope_id': row[10] & 127, 'modulation_enabled': bool(row[10] & 128)},
                'tone': tone, 'volume_envelope_position': row[12],
                'note_wait': row[14], 'last_duration': row[15],
                'base_pitch': self.descriptors.get(row[16]) if dac else word(row, 16),
                'gate_remaining': row[18], 'gate_ticks': row[19],
                'modulation_definition': self.modulation(int.from_bytes(row[20:24], 'big')),
                'modulation_delay_remaining': row[24], 'modulation_rate_remaining': row[25],
                'modulation_delta': s8(row[26]), 'modulation_steps_remaining': row[27],
                'modulation_offset': s16(word(row, 28)), 'detune': s8(row[30]),
                'pan_automation': {'mode': row[31], 'pattern_id': row[32] if pan_enabled else 0,
                                   'step': row[33] if pan_enabled else 0,
                                   'step_limit': row[34] if pan_enabled else 0,
                                   'elapsed': row[35] if pan_enabled else 0,
                                   'period': row[36] if pan_enabled else 0},
                'loaded_algorithm': row[37] & 7 if not psg else None,
                'loaded_feedback': (row[37] >> 3) & 7 if not psg else None,
                'noise_control': row[37] if psg else None,
                'pitch_envelope_position': row[38],
                'pan': {'left': bool(row[39] & 128), 'right': bool(row[39] & 64),
                        'ams': row[39] >> 4 & 3, 'fms': row[39] & 7},
            })
        return {'priority': driver[0], 'tempo_remaining': driver[1], 'tempo_period': driver[2],
                'control_signal': driver[3], 'fade_steps': driver[4], 'fade_period': driver[5],
                'fade_remaining': driver[6], 'pause_state': s8(driver[7]),
                'selected_sound': driver[9], 'queue': list(driver[10:14]),
                'phase': {0: 'music', 128: 'effect-primary', 64: 'effect-secondary'}[driver[14]],
                'fm3_mode': driver[15],
                'music_fm3_offsets': [word(driver, 16 + 2 * i) for i in range(4)],
                'effect_fm3_offsets': [word(driver, 24 + 2 * i) for i in range(4)],
                'voices': voices}


def run(out):
    rom_path = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    requested = cases()
    controls = {}
    with AudioSession(ROOT, out / 'original-core', core=ORIGINAL) as ref:
        checkpoint = quiet_checkpoint(ref)
        original_identity = ref.identity
        for case in requested:
            ref.restore(checkpoint, restore_refresh_cycles=True)
            initial = cpu_range(ref, 0xf800, 928)
            commands = {row['step']: row['code'] for row in case['commands']}
            rows = []
            for step in range(case['steps']):
                if step in commands:
                    queue_command(ref, commands[step])
                result, pcm = ref.step_audio(ref.frame + 1, [0] * 8)
                ram = ref.ram_cpu()
                assert not any(ram[0x8000:0xa000])
                rows.append({'step': step, 'reference': result['reference'],
                             'driver_hex': cpu_range(ref, 0xf800, 928).hex(),
                             'pcm_sha256': sha(pcm), 'stereo_frames': len(pcm) // 4})
            path = out / (case['id'] + '-original.json.gz')
            saved(path, {'initial_driver_hex': initial.hex(), 'frames': rows})
            controls[case['id']] = (initial, rows, path)
    bridge = C.CDLL(str(BRIDGE))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = (C.c_uint * len(OBSERVATION_POINTS))(*OBSERVATION_POINTS)
    results = []
    with AudioSession(ROOT, out / 'observed-core', core=OBSERVED) as ref:
        checkpoint = quiet_checkpoint(ref)
        observed_identity = ref.identity
        zram = (C.c_ubyte * 8192).in_dll(ref.lib, 'zram')
        for case in requested:
            ref.restore(checkpoint, restore_refresh_cycles=True)
            initial, control_rows, control_path = controls[case['id']]
            assert cpu_range(ref, 0xf800, 928) == initial
            view = SemanticView(rom)
            portable = {'id': case['id'], 'commands': case['commands'], 'headers': [],
                        'initial': view.state(initial), 'frames': []}
            first = ref.frame + 1
            commands = {row['step']: row['code'] for row in case['commands']}
            raw_events, errors = [], []
            frame_intents, frame_feedback, frame_routing = [], [], []
            current_header, ordinal = None, 0
            def emit(intent):
                frame_intents.append({'order': len(frame_intents), **intent})
            def register(index):
                return ref.get_register(index)
            @callback_type
            def observe(kind, width, pc, value):
                nonlocal current_header, ordinal
                try:
                    row = register(13) & 65535
                    event = {'step': ref.frame - first, 'pc': pc, 'row': row}
                    if pc in (0x375ae, 0x37656):
                        event.update(kind='tick-before' if pc == 0x375ae else 'tick-return',
                                     driver_hex=cpu_range(ref, 0xf800, 928).hex())
                    elif pc in (0x384fc, 0x38656):
                        address = register(12 if pc == 0x384fc else 11) & 0xffffff
                        current_header, ordinal = view.headers[address], 0
                        portable['headers'].append(current_header)
                        event.update(kind='header', sound_id=current_header, address=address)
                    elif pc in (0x38560, 0x385a0, 0x386e4):
                        target = register(9) & 65535 if pc != 0x386e4 else row
                        view.owners[target] = {'score_id': current_header, 'voice_id': ordinal}
                        event.update(kind='voice-init', target=target, owner=view.owners[target], row_hex=cpu_range(ref, target, 48).hex())
                        ordinal += 1
                    elif pc == 0x376c4:
                        note = cpu_range(ref, row + 16, 1)[0]
                        feedback = {'descriptor_id': view.descriptors[note],
                                    'blocks_score_dac': bool(zram[0x1ffd] & 128),
                                    'mode_priority': zram[0x1ffc] & 192}
                        frame_feedback.append(feedback)
                        event.update(kind='dac-feedback', pending_byte=zram[0x1ffd], mode_flags=zram[0x1ffc], **feedback)
                    elif pc == 0x376e4:
                        state = cpu_range(ref, row, 48)
                        intent = {'kind': 'dac', 'descriptor_id': view.descriptors[state[16]],
                                  'pan': {'left': bool(state[39] & 128), 'right': bool(state[39] & 64)}}
                        emit(intent)
                        event.update(intent, descriptor_address=register(11) & 0xffffff)
                    elif pc in (0x38b2e, 0x38b9e):
                        reg, data = register(0) & 255, register(1) & 255
                        emit({'kind': 'ym', 'bank': 0 if pc == 0x38b2e else 1,
                              'register': reg, 'value': ym_effective(reg, data)})
                        event.update(kind='ym-request', bank=0 if pc == 0x38b2e else 1, register=reg, value=data)
                    elif pc in (0x3a0c6, 0x3a0cc, 0x3a140, 0x3a186, 0x38610, 0x3869a, 0x386a4, 0x387de, 0x387e8, 0x391a2):
                        reg = 6 if pc in (0x3a0cc, 0x3a140) else 4 if pc in (0x387de, 0x387e8) else 0
                        data = 255 if pc == 0x38610 else rom[(register(12) & 0xffffff) - 1] if pc == 0x391a2 else register(reg) & 255
                        emit({'kind': 'psg', 'value': data})
                        event.update(kind='psg-request', value=data)
                    elif pc == 0x38f28:
                        sound = rom[register(12) & 0xffffff]
                        emit({'kind': 'queue', 'action': 'replace-first', 'sound_id': sound})
                        event.update(kind='replace-first-enqueue', sound_id=sound)
                    elif pc == 0x37cb4:
                        sound = cpu_range(ref, 0xf809, 1)[0]
                        event.update(kind='dispatch', sound_id=sound)
                        if 6 <= sound <= 127:
                            emit({'kind': 'pcm-command', 'sound_id': sound})
                    elif pc == 0x37df0:
                        event.update(kind='pcm-command-store', descriptor_address=register(8) & 0xffffff)
                    elif pc in (0x3867e, 0x3868a, 0x3912e, 0x39176, 0x38858, 0x3888a):
                        target = register(8) & 65535 if pc in (0x39176, 0x3888a) else row
                        slot = SLOTS[(target - 0xf840) // 48][0]
                        routing = {'action': 'claim' if pc in (0x3867e, 0x3868a) else 'restore-forced-rest',
                                   'slot': slot}
                        frame_routing.append(routing)
                        event.update(kind='routing', target=target, **routing, row_hex=cpu_range(ref, target, 48).hex())
                    else:
                        event.update(kind='parameter-observation')
                        if 0xf840 <= row <= 0xfb70 and (row - 0xf840) % 48 == 0:
                            event['row_hex'] = cpu_range(ref, row, 48).hex()
                        if pc == 0x38dae:
                            event.update(command=register(5) & 255, address=register(12) & 0xffffff)
                    raw_events.append(event)
                except Exception as error:
                    errors.append(repr(error))
            assert bridge.yyh_set_observer(observe, points, len(points))
            ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
            try:
                for step, expected in enumerate(control_rows):
                    frame_intents, frame_feedback, frame_routing = [], [], []
                    if step in commands:
                        queue_command(ref, commands[step])
                    result, pcm = ref.step_audio(ref.frame + 1, [0] * 8)
                    driver = cpu_range(ref, 0xf800, 928)
                    assert not errors, errors[:3]
                    assert result['reference'] == expected['reference'], (case['id'], step, 'RAM/video differs')
                    assert driver.hex() == expected['driver_hex'], (case['id'], step, 'driver differs')
                    assert sha(pcm) == expected['pcm_sha256'] and len(pcm) // 4 == expected['stereo_frames'], (case['id'], step, 'PCM differs')
                    portable['frames'].append({'step': step, 'state': view.state(driver),
                                               'intents': frame_intents, 'dac_feedback': frame_feedback,
                                               'routing': frame_routing, 'pcm_sha256': sha(pcm),
                                               'stereo_frames': len(pcm) // 4})
            finally:
                ref.api('set_cpu_hook', None, C.c_void_p)(None)
            assert not errors and portable['headers'] == case['headers'], (case['id'], errors, portable['headers'])
            portable_path = out / (case['id'] + '-portable.json.gz')
            provenance_path = out / (case['id'] + '-provenance.json.gz')
            saved(portable_path, portable)
            saved(provenance_path, {'events': raw_events, 'errors': errors})
            results.append({'id': case['id'], 'steps': case['steps'], 'original_equal': True,
                            'pcm_samples_equal': True, 'files': {'original': binding(control_path),
                            'portable': binding(portable_path), 'provenance': binding(provenance_path)}})
            print(json.dumps({'complete': case['id'], 'steps': case['steps'], 'events': len(raw_events)}), flush=True)
    source_paths = [Path(__file__).resolve(), ROOT / 'scripts/yuyuhakusho/score_claim_check.py',
                    ROOT / 'scripts/yuyuhakusho/capture_audio_score_reads.py', ROOT / 'scripts/yuyuhakusho/research_audio_loops.py',
                    ROOT / 'scripts/yuyuhakusho/extract_audio_assets.py', ROOT / 'scripts/yuyuhakusho/reference.py',
                    ROOT / 'scripts/yuyuhakusho/score_parameter_tables.py']
    report = {'schema': 1, 'format': 'native-score-claim-oracle-v1', 'pass': True,
              'go_driver_verified': False, 'dsp_verified': False, 'rom_sha256': sha(rom),
              'cases_requested': requested, 'cases': results,
              'identities': {'original': original_identity, 'observed': observed_identity},
              'sources': [binding(path) for path in source_paths],
              'limits': ['Original/observer equality and typed observations only; independent Go state and intent equality is not asserted.',
                         'Primary a0..ab coverage; group2 initialization and layered group2 claim/restore remain separate.',
                         'Intents retain driver order, not independently reproduced master-cycle cadence.',
                         'YM hardware-ignored bits are omitted from portable chip values; full requested bytes remain in provenance.',
                         'External PCM feedback is read at the actual DAC guard, never copied into score-owned state.',
                         'Native flow cursor/shared loop-return words are research provenance, not portable production state.']}
    saved(out / 'index.json', report)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    assert output.is_relative_to(ROOT / '.local/md-yuyuhakusho/audio-score-claim')
    output.mkdir(parents=True, exist_ok=False)
    run(output)
