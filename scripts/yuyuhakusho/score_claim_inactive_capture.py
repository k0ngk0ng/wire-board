"""Extra native inactive-music restore case; shared observer/view remain frozen."""
from pathlib import Path
import argparse
import ctypes as C
import gzip
import json
from score_claim_capture import (AudioSession, BRIDGE, OBSERVED, ORIGINAL, ROOT,
    OBSERVATION_POINTS, SLOTS, SemanticView, binding, canonical, cpu_range,
    queue_command, quiet_checkpoint, read_fm_patch, saved, sha, ym_effective)


def cases():
    return [{'id': 'primary-before-first-music', 'steps': 160,
             'commands': [{'step': 0, 'code': 164}, {'step': 10, 'code': 130}],
             'headers': [164, 130]}]


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
                    elif pc == 0x38f78:
                        state = cpu_range(ref, row, 48)
                        address = register(9) & 0xffffff
                        event.update(kind='fm-patch-read', row_hex=state.hex(),
                                     read_address=address, bank_address=address - 25 * state[11],
                                     instrument_index=state[11], read_hex=rom[address:address + 25].hex())
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
    source_paths = [Path(__file__).resolve(), ROOT / 'scripts/yuyuhakusho/score_claim_capture.py', ROOT / 'scripts/yuyuhakusho/score_claim_inactive_check.py',
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
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    patch = read_fm_patch(rom, 0, 0).to_score_data()
    identity = 'patch-' + sha(canonical(patch))[:20]
    saved(output / 'fallback-patches.json', {'schema': 1,
          'use': 'stop-primary-before-first-music',
          'fm_patches': [{'id': identity, **patch}]})
    saved(output / 'fallback-provenance.json', {'read_address': 0, 'bank_address': 0,
          'instrument_index': 0, 'read_hex': rom[:25].hex(),
          'rom_sha256': sha(rom), 'semantic_patch_id': identity})
