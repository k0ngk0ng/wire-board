#!/usr/bin/env python3
"""Validate bounded native BGM recurrence separately from PCM loop claims."""
import argparse
from array import array
from collections import Counter, defaultdict
from fractions import Fraction
import gzip
import json
import math
from pathlib import Path
import struct
import sys
import wave

from reference import ROOT, sha


def json_bytes(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':'))+'\n').encode()


def digest(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path.read_bytes())}


def pcm_values(pcm):
    values = array('h')
    values.frombytes(pcm)
    if sys.byteorder != 'little':
        values.byteswap()
    return values


def waveform_difference(first, second):
    a, b = pcm_values(first), pcm_values(second)
    count = min(len(a), len(b))
    different = peak = sum_square = 0
    for x, y in zip(a, b):
        error = x-y
        different += error != 0
        peak = max(peak, abs(error))
        sum_square += error*error
    return {'same_length': len(a) == len(b), 'pcm_bytes_equal': first == second,
            'compared_sample_values': count, 'different_sample_values': different,
            'max_absolute_difference': peak, 'rms_difference': math.sqrt(sum_square/count)}


def context_difference(a, b):
    changed = [i for i, (x, y) in enumerate(zip(a, b)) if x != y]
    return {'equal': a == b, 'bytes': len(a), 'changed_byte_count': len(changed),
            'changed_byte_offsets': changed, 'first_sha256': sha(a), 'second_sha256': sha(b)}


def normalized_events(events, start, period):
    excluded = {'cpu_cycles', 'row_hex', 'globals_hex', 'driver_hex'}
    return [{k: v-start if k == 'step' else v for k, v in event.items() if k not in excluded}
            for event in events if start <= event['step'] < start+period and event['kind'] != 'tick']


def analyze_song(out, code, capture, rom):
    control_path = out/f'{code:02x}-control-summary.json'
    control = json.loads(control_path.read_text())
    states_path = ROOT/control['native_states_file']
    wav_path = ROOT/control['pcm_wav_file']
    events_path = out/f'{code:02x}-events.json.gz'
    assert sha(states_path.read_bytes()) == control['native_states_sha256']
    assert sha(wav_path.read_bytes()) == control['wav_sha256']
    observed = json.loads(gzip.decompress(events_path.read_bytes()))
    assert observed['original_ram_video_driver_pcm_equal'] and not observed['errors']
    events = observed['events']
    branches = defaultdict(list)
    for event in events:
        if event['kind'] == 'sequence' and event['command'] == 0xf6:
            source = event['source']
            assert rom[source] == 0xf6
            target = source+2+int.from_bytes(rom[source+1:source+3], 'big', signed=True)
            assert event['target'] == target
            branches[(event['row'], source, target)].append(event['step'])
    repeats = []
    initial = []
    for (row, source, target), steps in branches.items():
        item = {'row': row, 'source': source, 'target': target, 'observed_steps': steps}
        if len(steps) > 1:
            periods = set(b-a for a, b in zip(steps, steps[1:]))
            assert len(periods) == 1
            item['period_frames'] = next(iter(periods))
            repeats.append(item)
        else:
            initial.append(item)
    period = math.lcm(*(x['period_frames'] for x in repeats))
    assert period == capture['candidate_periods'][str(code)]

    drivers = []
    offsets = [0]
    with gzip.open(states_path, 'rt') as f, wave.open(str(wav_path), 'rb') as wav:
        assert (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) == (2, 2, 44100)
        for step, line in enumerate(f):
            state = json.loads(line)
            assert state['step'] == step
            driver = bytes.fromhex(state['driver_hex'])
            assert len(driver) == 928 and not any(driver[0x220:])
            pcm = wav.readframes(state['stereo_frames'])
            assert len(pcm) == state['stereo_frames']*4 and sha(pcm) == state['pcm_sha256']
            drivers.append(driver)
            offsets.append(offsets[-1]+state['stereo_frames'])
        assert wav.getnframes() == offsets[-1] and not wav.readframes(1)
    assert len(drivers) == control['steps'] == observed['steps']
    differences = [i for i in range(len(drivers)-period) if drivers[i] != drivers[i+period]]
    first_state = differences[-1]+1 if differences else 0
    equality_pairs = len(drivers)-period-first_state
    assert equality_pairs >= period, (code, first_state, equality_pairs, period)
    semantic_start = first_state
    semantic = None
    if code == 0x87:
        # Original F8 stores a return pointer after decreasing row+D by four;
        # F9 reads exactly at row+D. At D=30, slots 2c..2f are inactive. The
        # complete observed F7 trace uses index 0, so it never addresses those
        # bytes as loop counters. Preserve every other byte, including live
        # return pointers when D=2c. Keep raw 928-byte equality above separate.
        loop_indices = {int(x['args'][:2], 16) for x in events
                        if x['kind'] == 'sequence' and x['command'] == 0xf7}
        assert loop_indices == {0}
        def live_state(driver):
            live = bytearray(driver)
            for row in [0xf8d0, 0xf900]:
                base = row-0xf800
                if live[base+0xd] == 0x30:
                    live[base+0x2c:base+0x30] = bytes(4)
            return live
        live_drivers = [live_state(x) for x in drivers]
        live_differences = [i for i in range(len(drivers)-period)
                            if live_drivers[i] != live_drivers[i+period]]
        semantic_start = live_differences[-1]+1 if live_differences else 0
        assert semantic_start == 14 and semantic_start < first_state
        assert normalized_events(events, semantic_start+1, period) == normalized_events(events, semantic_start+1+period, period)
        completed = [semantic_start+1+i*period for i in range(3)]
        samples = [offsets[x] for x in completed]
        semantic = {'first_equal_after_step': semantic_start, 'video_steps_completed': completed,
                    'stereo_sample_offsets': samples, 'seconds': [x/44100 for x in samples],
                    'stereo_sample_counts': [samples[1]-samples[0], samples[2]-samples[1]],
                    'all_live_state_pairs_equal_from_first_equal': len(drivers)-period-semantic_start,
                    'only_excluded_bytes': {'rows': [0xf8d0, 0xf900], 'relative_offsets': [0x2c, 0x2d, 0x2e, 0x2f],
                                            'condition': 'Original row+D == 0x30: no live call-stack entries'},
                    'f7_loop_indices_observed': sorted(loop_indices),
                    'complete_selected_event_periods_equal': True,
                    'note': 'Semantic candidate after one-time delayed-channel setup; raw recurrence remains separately verified and this is not an accepted media cut'}
    # State i was captured AFTER native video step i. The corresponding media
    # boundary is after i+1 complete video steps, not before step i.
    start = first_state+1
    boundaries = [start, start+period, start+2*period]
    sample_boundaries = [offsets[x] for x in boundaries]
    native_frames = [first_state+i*period for i in range(3)]
    snapshots = {}
    with gzip.open(states_path, 'rt') as f:
        for line in f:
            state = json.loads(line)
            if state['step'] in native_frames:
                snapshots[state['step']] = state
    assert len(snapshots) == 3
    first_events = normalized_events(events, start, period)
    second_events = normalized_events(events, start+period, period)
    assert first_events == second_events
    rows = {int.from_bytes(drivers[first_state][i+4:i+8], 'big')
            for i in range(0x40, 0x220, 0x30) if drivers[first_state][i]&128}
    assert rows

    contexts = {}
    for key in ['driver_hex', 'z80_hex', 'ym2612_hex', 'psg_hex']:
        data = [bytes.fromhex(snapshots[i][key]) for i in native_frames]
        contexts[key.removesuffix('_hex')] = {
            'first_to_second': context_difference(data[0], data[1]),
            'second_to_third': context_difference(data[1], data[2])}
    noise_values = [struct.unpack_from('<i', bytes.fromhex(snapshots[i]['psg_hex']), 8)[0]
                    for i in native_frames]
    with wave.open(str(wav_path), 'rb') as wav:
        pcm = wav.readframes(wav.getnframes())
    if semantic:
        sa, sb, sc = semantic['stereo_sample_offsets']
        semantic['first_100ms_comparison'] = waveform_difference(pcm[sa*4:(sa+4410)*4], pcm[sb*4:(sb+4410)*4])
    a, b, c = sample_boundaries
    first_pcm, second_pcm = pcm[a*4:b*4], pcm[b*4:c*4]
    differences_pcm = waveform_difference(first_pcm, second_pcm)
    short_pcm = waveform_difference(first_pcm[:4410*4], second_pcm[:4410*4])
    before = struct.unpack_from('<hh', pcm, (b-1)*4)
    repeat_start = struct.unpack_from('<hh', pcm, a*4)
    natural_start = struct.unpack_from('<hh', pcm, b*4)
    seam = {'loop_last_sample': before, 'replay_first_sample': repeat_start,
            'natural_next_sample': natural_start,
            'replay_jump': [x-y for x, y in zip(repeat_start, before)],
            'native_jump': [x-y for x, y in zip(natural_start, before)],
            'replay_error_at_first_sample': [x-y for x, y in zip(repeat_start, natural_start)],
            'first_100ms_comparison': short_pcm}
    exact_contexts = all(x['first_to_second']['equal'] and x['second_to_third']['equal'] for x in contexts.values())
    exact_pcm = differences_pcm['pcm_bytes_equal'] and exact_contexts
    fps = Fraction(53693175, 3420*262)
    assert float(fps) == capture['identity']['original']['fps']
    sample_count = Fraction(44100*period, 1)/fps
    header_address = int.from_bytes(rom[0x3a492+(code-0x81)*4:0x3a496+(code-0x81)*4], 'big')
    print(json.dumps({'code': hex(code), 'logical_period': period, 'equal_state_pairs': equality_pairs,
                      'sample_counts': [b-a, c-b], 'pcm_bytes_equal': differences_pcm['pcm_bytes_equal']}), flush=True)
    return {'code': code, 'music_header_address': header_address,
            'music_header_hex': rom[header_address:header_address+6].hex(),
            'capture_steps': len(drivers), 'header_fm_rows_including_dac': rom[header_address+2],
            'header_psg_rows': rom[header_address+3], 'recurrent_f6_branches': repeats,
            'one_time_f6_transfers': initial, 'logical_loop_verified': True,
            'common_period_video_frames': period, 'common_period_seconds': float(period/fps),
            'complete_driver_bytes_compared': 928, 'first_equal_after_step': first_state,
            'all_driver_equal_pairs_from_first_equal': equality_pairs,
            'whole_common_period_of_equal_driver_pairs': True,
            'earlier_semantic_candidate': semantic,
            'media_candidate_boundaries': {'video_steps_completed': boundaries,
                                           'after_native_step': native_frames,
                                           'source_reference_frames': [2521+i for i in native_frames],
                                           'stereo_sample_offsets': sample_boundaries,
                                           'seconds': [x/44100 for x in sample_boundaries],
                                           'note': 'Conservative raw-driver recurrence boundary, not a unique authored musical intro or accepted media cut'},
            'driver_event_replay': {'equal': True, 'events_per_common_period': len(first_events),
                                    'kind_counts': dict(Counter(x['kind'] for x in first_events)),
                                    'normalized_sha256': sha(json_bytes(first_events)),
                                    'excluded_fields': ['cpu_cycles', 'row_hex', 'globals_hex', 'driver_hex'],
                                    'limits': 'Selected native sequence/YM request/DAC events only; not an exhaustive PSG/DAC hardware write trace or intra-frame timestamp equivalence'},
            'native_contexts': contexts, 'psg_noise_shift_values': noise_values,
            'stereo_sample_counts_per_common_period': [b-a, c-b],
            'nominal_sample_clock_count_per_common_period': {'numerator': sample_count.numerator,
                                                             'denominator': sample_count.denominator,
                                                             'integer': sample_count.denominator == 1},
            'pcm_comparison': differences_pcm, 'seam': seam,
            'exact_pcm_loop_verified': exact_pcm, 'loop_media_exported': False,
            'intro_loop_assets_ready': False, 'published': False,
            'files': [digest(x) for x in [control_path, states_path, wav_path, events_path]]}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--capture', required=True, type=Path)
    p.add_argument('--report', required=True, type=Path)
    args = p.parse_args()
    out, report = args.capture.resolve(), args.report.resolve()
    assert out.is_relative_to(ROOT/'.local') and report.is_relative_to(ROOT)
    capture_path = out/'capture.json'
    capture = json.loads(capture_path.read_text())
    assert capture['pass']
    capture_script = ROOT/'scripts/yuyuhakusho/research_audio_loops.py'
    assert sha(capture_script.read_bytes()) == capture['script_sha256']
    rom_path = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rom_path.read_bytes()
    assert sha(rom) == capture['identity']['original']['rom_sha256']
    original_core = ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
    observed_core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    observer = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    assert sha(original_core.read_bytes()) == capture['identity']['original']['core_sha256']
    assert sha(observed_core.read_bytes()) == capture['identity']['observed']['core_sha256']
    assert sha(observer.read_bytes()) == capture['observer_sha256']
    songs = [analyze_song(out, code, capture, rom) for code in [0x82, 0x87]]
    sources = [capture_script, Path(__file__).resolve(), rom_path, original_core, observed_core, observer,
               ROOT/'scripts/yuyuhakusho/static/MdControlProbe.java', ROOT/'scripts/yuyuhakusho/extract_audio_assets.py',
               ROOT/'scripts/yuyuhakusho/reference.py', ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt',
               ROOT/'docs/research/md-yuyuhakusho/AUDIO_LOOPS.md',
               ROOT/'docs/research/md-yuyuhakusho/AUDIO_RESEARCH.md',
               ROOT/'games/yuyuhakusho/data/audio-assets.json', ROOT/'games/yuyuhakusho/data/audio-validation.json',
               ROOT/'docs/research/md-yuyuhakusho/inputs.json', ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json',
               ROOT/'web/src/audio.ts',
               ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/system.c',
               ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/sound/psg.c',
               ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/sound/ym2612.c']
    value = {'scope': 'Bounded native 82/87 sequence recurrence and exact PCM loop feasibility',
             'pass': True, 'logical_loops_verified': all(x['logical_loop_verified'] for x in songs),
             'exact_pcm_loops_verified': all(x['exact_pcm_loop_verified'] for x in songs),
             'goal_complete': False, 'release_ready': False, 'published': False,
             'identity': capture['identity'], 'capture': digest(capture_path), 'songs': songs,
             'sources': [digest(x) for x in sources],
             'platform_media_plan': {
                 'interface': 'BoardProps.audio.music(sources, options)',
                 'existing_options': ['loop', 'startOffset', 'playbackID', 'loopStartSeconds'],
                 'loop_plan_status': 'blocked-on-exact-media-loop-or-explicitly-different-fidelity-boundary',
                 'proposed_carrier': 'One intro-plus-loop file and a sample-derived loopStartSeconds, only after media validation',
                 'current_native_finite_cue': {'code': 0x8f, 'loop': False, 'source': 'Existing accepted audio catalog'},
                 'browser_timing': 'HTML media ended/currentTime/play path is not a sample-accurate original-chip timing guarantee; browser E2E remains required',
                 'runtime_emulator': False, 'separate_game_player': False},
             'remaining_boundaries': [
                 'Driver sequence recurrence does not reset FM phase/envelopes, PSG counters/noise LFSR or resampling state',
                 'Successive measured common periods have different integer PCM sample counts and different complete PCM',
                 'No unique shortest authored intro or intra-video-frame sample boundary was asserted',
                 'Crossfade, trimming, zero-crossing selection or time stretching would be a separate media transformation with its own fidelity decision and validation',
                 'Pause/fade/effect stealing and other songs remain outside this isolated looping investigation',
                 'No new intro/loop media was exported, published or wired into the game runtime']}
    cleanup = ROOT/'games/yuyuhakusho/data/audio-loop-cleanup.json'
    if cleanup.exists():
        value['cleanup_report'] = digest(cleanup)
    report.parent.mkdir(parents=True, exist_ok=True)
    report.write_text(json.dumps(value, indent=2)+'\n')
    print(json.dumps({'pass': True, 'logical_loops_verified': value['logical_loops_verified'],
                      'exact_pcm_loops_verified': value['exact_pcm_loops_verified'],
                      'report': str(report.relative_to(ROOT))}), flush=True)


if __name__ == '__main__':
    main()
