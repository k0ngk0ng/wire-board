#!/usr/bin/env python3
"""Pinned arena2/6/8 callback evidence from a supplied legal original control."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
import sys
import traceback

sys.dont_write_bytecode = True
from reference import ROOT, ROM_SHA256, ReferenceSession, buttons_at, sha

ENTRIES = {2: 0x20ae0, 6: 0x20e28, 8: 0x20fb4}
RESOURCES = {2: 0x2175c, 6: 0x217cc, 8: 0x21800}
ORIGINAL_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
HOOK_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
IRQ_ENTRY = 0xfff000
IRQ_EXITS = {0xfff09a, 0xfff0a8, 0xfff13a}
AUDIO_CLEARS = {0x37c56: 0x37c58, 0x37cac: 0x37cae}


def number(raw, offset, count=2, signed=False):
    return int.from_bytes(raw[offset:offset+count], 'big', signed=signed)


def inside(path):
    result = Path(path).resolve()
    if not result.is_relative_to(ROOT):
        raise ValueError('all artifact paths must remain inside the worktree')
    return result


def changes(before, after):
    assert len(before) == len(after) == 65536
    return [{'offset': i, 'before': a, 'after': b}
            for i, (a, b) in enumerate(zip(before, after)) if a != b]


def snapshot(ram):
    return {'arena': ram[0x8180:0x8200].hex(),
            'camera': ram[0xa00e:0xa01c].hex(), 'samples': ram[0xa800:0xa880].hex(),
            'prefix': ram[0xe600:0xe800].hex(),
            'scroll_a': ram[0xe800:0xea00].hex(), 'scroll_b': ram[0xea00:0xec00].hex(),
            'pan_a': ram[0x6000:0x61c0].hex(), 'pan_b': ram[0x6200:0x63c0].hex(),
            'source_a': number(ram, 0xf598, 4, True),
            'source_b': number(ram, 0xf59c, 4, True),
            'vertical': ram[0xe500:0xe540].hex(),
            'scroll_mode': ram[0xf6e7], 'backdrop_color': ram[0xf6df],
            'pause': ram[0xf550], 'slow': ram[0xf55a],
            'shake': number(ram, 0xf566, 2, True), 'pan': number(ram, 0xf572, 2, True)}


def extract_library(rom):
    # The callbacks address bytes/words in these original tables. The output
    # contains typed values only, never executable bytes or a runtime ROM view.
    return {'wave_x': [number(rom, 0x34000+i*2, 1, True) for i in range(1024)],
            'linear_rows': {str(x): [number(rom, 0x24000+(x-0x2e0)*256+i*2, 2, True)
                                    for i in range(128)] for x in range(744, 985)}}


def rule_addresses(arena):
    fields = {2: [(6, 2), (8, 6), (0x14, 6), (0x1c, 10)],
              6: [(6, 2), (8, 2), (0xc, 2), (0x10, 2), (0x14, 2), (0x18, 2)],
              8: [(6, 2), (8, 2), (0xc, 6), (0x14, 2), (0x18, 2), (0x1c, 2)]}
    result = {0xf6e7, 0xf6df}
    for offset, length in fields[arena]: result.update(range(0x8180+offset, 0x8180+offset+length))
    for start, length in [(0xa00e, 14), (0xe500, 2), (0xe528, 2), (0xf598, 8),
                          (0x6000, 448), (0x6200, 448)]:
        result.update(range(start, start+length))
    rows = {2: [(0xe8a0, 336), (0xea00, 480)],
            6: [(0xe820, 464), (0xea20, 384)],
            8: [(0xe8a0, 336), (0xea20, 448)]}
    for start, length in rows[arena]: result.update(range(start, start+length))
    return result


def export_linear(out, rom):
    if sha(rom) != ROM_SHA256:
        raise ValueError('ROM differs from the pinned static source')
    library = extract_library(rom)
    old = json.loads((ROOT/'games/yuyuhakusho/data/arena-runtime-rules.json').read_bytes())['library']
    assert library['wave_x'] == old['wave_x']
    assert all(library['linear_rows'][str(x)][40:96] == old['tails'][str(x)] for x in range(744, 985))
    payload = {'schema': 1, 'rom_sha256': ROM_SHA256, 'linear_rows': library['linear_rows'],
               'source': {'base': 0x24000, 'x_origin': 736, 'x_min': 744, 'x_max': 984,
                          'x_byte_stride': 256, 'word_count': 128, 'encoding': 'signed 68000 big-endian word',
                          'source_start': 0x24800, 'source_end': 0x33900,
                          'source_sha256': sha(rom[0x24800:0x33900])}}
    raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode()
    compressed = gzip.compress(raw, mtime=0)
    output = out/'arena-scene-linear-rows.json.gz'
    output.write_bytes(compressed)
    report = {'pass': True, 'kind': 'static typed table extraction; not CPU or stage acceptance',
              'rom_sha256': ROM_SHA256, 'output': str(output.relative_to(ROOT)),
              'rows': 241, 'words_per_row': 128, 'source_bytes': 61696,
              'source_sha256': payload['source']['source_sha256'], 'raw_sha256': sha(raw),
              'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed),
              'all_default_tail_slices_match': True, 'all_wave_bytes_match_existing_library': True,
              'extractor': str(Path(__file__).resolve().relative_to(ROOT)),
              'extractor_sha256': sha(Path(__file__).read_bytes())}
    (out/'arena-scene-linear-validation.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--arena', type=lambda value: int(value, 16), choices=ENTRIES)
    parser.add_argument('--baseline', nargs=3, metavar=('INPUTS', 'REFERENCE', 'MANIFEST'))
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--export-linear', action='store_true', help='export the shared typed table without executing a core')
    parser.add_argument('--checkpoint', type=int, default=2400)
    parser.add_argument('--natural-only', action='store_true')
    args = parser.parse_args()
    out = inside(args.out); out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    if args.export_linear:
        export_linear(out, rom)
        return
    if args.arena is None or not args.baseline:
        parser.error('--arena and --baseline are required for CPU evidence capture')
    ip, rp, mp = [inside(path) for path in args.baseline]
    events, expected, baseline = json.loads(ip.read_bytes()), json.loads(rp.read_bytes()), json.loads(mp.read_bytes())
    if baseline['identity']['core_sha256'] != ORIGINAL_SHA:
        raise ValueError('a separately supplied pinned original-core control is required')
    if baseline['input']['sha256'] != sha(ip.read_bytes()) or baseline['output']['sha256'] != sha(rp.read_bytes()):
        raise ValueError('the supplied input/reference does not match its original manifest')
    if len(expected) != 3602:
        raise ValueError('the arena control must contain exactly 3602 original video steps')
    arena = args.arena
    library = extract_library(rom)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = {ENTRIES[arena], 0x21690, 0x3d8, IRQ_ENTRY, *IRQ_EXITS, *AUDIO_CLEARS, *AUDIO_CLEARS.values()}
    points |= {point | 0xff000000 for point in [IRQ_ENTRY, *IRQ_EXITS]}
    natural, isolated, streams, errors, other_contexts = [], [], [], [], []
    pending = None; private = False; label = ''; sequence = 0
    allowed = rule_addresses(arena)

    with ReferenceSession(ROOT, out/'observed-core', core=core) as ref:
        assert ref.identity['core_sha256'] == HOOK_SHA

        def configure():
            assert len(points) <= 64
            pcs = (C.c_uint*len(points))(*sorted(points))
            assert bridge.yyh_set_observer(observe, pcs, len(points))

        def segment(ram, owner=None, irq_entry_stack=None):
            row = pending
            delta = changes(row['_segment'], ram)
            owner = owner or row['_owner']
            classified = []
            for item in delta:
                item = dict(item)
                if irq_entry_stack and item['offset'] in irq_entry_stack:
                    item['owner'] = 'irq-exception-stack'
                elif owner == 'arena':
                    if item['offset'] in allowed: item['owner'] = 'arena'
                    elif item['offset'] in row['_stack']: item['owner'] = 'arena-call-stack'
                    else: raise AssertionError(('unexplained arena RAM write', item, row['name']))
                else:
                    item['owner'] = owner
                classified.append(item)
            row['timeline'].append({'order': len(row['timeline']), 'owner': owner,
                                    'frame': ref.frame, 'changes': classified})
            row['_segment'] = ram

        @cbtype
        def observe(kind, width, raw_pc, value):
            nonlocal pending, sequence
            try:
                if kind != 1: return
                pc = raw_pc & 0xffffff
                ram = ref.ram_cpu(); sp = ref.get_register(15)
                if pending is not None:
                    if pc == IRQ_ENTRY:
                        assert pending['_owner'] == 'arena', 'nested or unmatched IRQ'
                        exception_stack = {(sp+i)&65535 for i in range(6)}
                        segment(ram, irq_entry_stack=exception_stack)
                        pending['timeline'].append({'order': len(pending['timeline']), 'event': 'irq-entry',
                                                    'pc': hex(raw_pc), 'frame': ref.frame, 'sp': sp})
                        pending['_owner'] = 'irq'
                        return
                    if pc in IRQ_EXITS:
                        assert pending['_owner'] == 'irq'
                        segment(ram)
                        pending['timeline'].append({'order': len(pending['timeline']), 'event': 'irq-exit',
                                                    'pc': hex(raw_pc), 'frame': ref.frame, 'sp': sp})
                        pending['_owner'] = 'arena'
                        return
                    if pc in AUDIO_CLEARS:
                        address = ref.get_register(9)&65535
                        pending['timeline'].append({'order': len(pending['timeline']), 'event': 'audio-clear-entry',
                                                    'pc': hex(pc), 'frame': ref.frame, 'address': address,
                                                    'before': ram[address], 'after_pc': AUDIO_CLEARS[pc]})
                        pending['_audio'].append((AUDIO_CLEARS[pc], address, ram[address]))
                    elif pending['_audio'] and pc == pending['_audio'][-1][0]:
                        _, address, before = pending['_audio'].pop()
                        assert ram[address] == 0
                        pending['timeline'].append({'order': len(pending['timeline']), 'event': 'audio-clear-exit',
                                                    'pc': hex(pc), 'frame': ref.frame, 'address': address,
                                                    'before': before, 'after': 0})
                    if pc == 0x21690:
                        pending['feedback_sources'] = [number(ram, 0xf598, 4, True), number(ram, 0xf59c, 4, True)]
                        pending['pre_feedback'] = snapshot(ram)
                    if pc == pending['return_pc'] and sp == pending['entry_sp']+4:
                        assert pending['_owner'] == 'arena' and not pending['_audio']
                        segment(ram)
                        row = pending; pending = None
                        row.update(after=snapshot(ram), exit_frame=ref.frame, exit_pc=pc,
                                   ram_before_sha256=sha(row['_ram']), ram_after_sha256=sha(ram),
                                   ram_changes=changes(row['_ram'], ram))
                        # Replaying every ordered segment must reproduce the
                        # entire 64KiB image, including writes later overwritten.
                        replay = bytearray(row['_ram'])
                        for part in row['timeline']:
                            for item in part.get('changes', []):
                                assert replay[item['offset']] == item['before']
                                replay[item['offset']] = item['after']
                        assert bytes(replay) == ram
                        for key in ['_ram', '_segment', '_owner', '_stack', '_audio']: row.pop(key)
                        if row['before']['pan'] and row.get('feedback_sources'):
                            if any(not (0xe600 <= (source&65535) and (source&65535)+448 <= 0xec00)
                                   for source in row['feedback_sources']):
                                row['reference_only'] = 'feedback reads outside the supplied HUD/row view'
                        if private:
                            isolated.append(row); ref.cpu.cycles = 4000000
                        else: natural.append(row)
                        return
                if pc == ENTRIES[arena]:
                    address = ref.get_register(13)&65535
                    assert pending is None, ('nested arena entry', address)
                    if address != 0x8180:
                        assert not private
                        other_contexts.append({'frame': ref.frame, 'pc': pc, 'object_id': address,
                                               'source_register': ref.get_register(13), 'entry_sp': sp,
                                               'header': ram[address:address+8].hex(),
                                               'return_pc': number(ram, sp&65535, 4)&0xffffff,
                                               'scope': 'non-8180 original menu context; not a ready-match callback claim'})
                        return
                    assert number(ram, address, 4) == ENTRIES[arena], ('canonical arena header mismatch', address)
                    sequence += 1
                    ret = number(ram, sp&65535, 4)&0xffffff
                    pending = {'arena_id': arena, 'kind': 'arena'+format(arena, 'x'), 'name': label,
                               'sequence': sequence, 'entry': pc, 'frame': ref.frame,
                               'object_id': address, 'entry_sp': sp, 'return_pc': ret,
                               'before': snapshot(ram), 'timeline': [],
                               '_ram': ram, '_segment': ram, '_owner': 'arena',
                               '_stack': {(sp-i)&65535 for i in range(1, 9)}, '_audio': []}
                    if ret not in points:
                        points.add(ret); configure()
            except Exception as exc:
                errors.append({'pc': hex(raw_pc), 'frame': ref.frame, 'error': repr(exc),
                               'traceback': traceback.format_exc()})
                if private: ref.cpu.cycles = 4000000

        configure()
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for frame in range(1, len(expected)+1):
            observed = ref.step(frame, buttons_at(events, frame))['reference']
            if observed != expected[frame-1]:
                raise AssertionError(('observer differs from original control', frame, observed, expected[frame-1]))
            if errors:
                raise AssertionError(('CPU ownership observation failed', errors[0]))
            frames.append(observed)
            if frame == args.checkpoint: checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None)
        assert not errors and pending is None and natural, (errors[:3], pending is not None)
        assert number(base_ram, 0xa060) == arena and number(base_ram, 0x8180, 4) == ENTRIES[arena]
        identity = ref.identity
        if not args.natural_only:
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, count=1):
                write_bytes(address, (value&((1 << (count*8))-1)).to_bytes(count, 'big'))

            def setup(seed=0, stage=2, pause=0, slow=0, pan=0, x=864, y=16, shake=0, used=0):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for reg in range(15): setreg(reg, 0)
                for start, length in [(0x8180, 128), (0xe500, 64), (0xe600, 1536), (0x6000, 1024)]:
                    write_bytes(start, bytes((i*73+seed*29+17)&255 for i in range(length)))
                write(0x8180, ENTRIES[arena], 4); write(0x8184, 128, 2); write(0x8186, stage, 2)
                write(0xf550, pause); write(0xf55a, slow); write(0xf566, shake, 2); write(0xf572, pan, 2)
                write(0xf6e7, (seed*19+0x81)&255); write(0xf6df, (seed*7+0xa3)&255)
                for address, value in [(0xa00e, used), (0xa010, x+16), (0xa012, x+304),
                                       (0xa014, x), (0xa016, y), (0xa018, 0x1234), (0xa01a, 0x5678)]:
                    write(address, value, 2)
                for i in range(16):
                    write(0xa800+i*4, ((900+i*3)<<16)|0x1234, 4)
                    write(0xa840+i*4, ((256-i*2)<<16)|0xabcd, 4)
                write(0xf598, 0x89abcdef, 4); write(0xf59c, 0x76543210, 4)

            def invoke(name):
                nonlocal label
                label = name
                setreg(17, 0x2700); setreg(15, 0xffff00); write(0xff00, 0x3d8, 4)
                setreg(13, 0xffff8180); setreg(16, ENTRIES[arena])
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                count = len(isolated)
                hook(observer); execute(2000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors[:3])
                assert not any(part.get('event') == 'irq-entry' for part in isolated[-1]['timeline'])
                return isolated[-1]

            # Active rule matrix; dirty bytes make every untouched half-word,
            # initial row gap and object suffix observable.
            for seed, values in enumerate(itertools.product([0, 2, 0xfffe], [0, 1, -32768],
                                                             [744, 864, 984], [0, 16, 32], [0, 64])):
                stage, pan, x, y, used = values
                setup(seed, stage=stage, pan=pan, x=x, y=y, used=used)
                invoke('active-'+str(values))
            for seed, values in enumerate(itertools.product([0, 2, 0xfffe], [0, 127, 128, 255], [0, 1, 255], [0, 1])):
                stage, pause, slow, pan = values
                setup(seed, stage=stage, pause=pause, slow=slow, pan=pan)
                invoke('gates-'+str(values))
            for seed, values in enumerate(itertools.product([-257, -256, -255, -33, -1, 0, 1, 31, 32, 33, 16383, 16384, 32767, -32768],
                                                             [0, 1, -32768, 32767], [0, 2])):
                dy, pan, stage = values
                setup(seed, stage=stage, pan=pan, y=0, shake=-dy)
                invoke('signed-feedback-'+str(values))
            for seed, values in enumerate(itertools.product([0x7ffffff0, 0xfffffff0, 0x80000001],
                                                             [0x7ff8, 0xfff8, 0xffff], [0, 1])):
                drift, phase, pan = values
                setup(seed, pan=pan)
                for off in [8, 0x14, 0x1c]: write(0x8180+off, drift, 4)
                for off in [0x20, 0x22, 0x24]: write(0x8180+off, phase, 2)
                invoke('wrap-'+str(values))
            boundary_count = len(isolated)
            for scenario in ['initialized', 'dirty-wrap', 'negative-feedback']:
                setup(91, stage=0 if scenario == 'initialized' else 2)
                if scenario == 'dirty-wrap':
                    for off in [8, 0x14, 0x1c]: write(0x8180+off, 0xffffc000, 4)
                    for off in [0x20, 0x22, 0x24]: write(0x8180+off, 0xfff0, 2)
                calls = []
                for step in range(320):
                    pause = 128 if 40 <= step < 44 else 127 if step == 44 else 0
                    slow = 255 if 80 <= step < 84 else 0
                    pan = -32768 if 120 <= step < 136 else 7 if 200 <= step < 208 else 0
                    shake = 64 if scenario == 'negative-feedback' and pan else step%5
                    samples_x = [((904+(step%160))<<16)|0x1234]*4
                    samples_y = [((192+(step%129))<<16)|0x5678]*4
                    for i, value in enumerate(samples_x): write(0xa800+i*4, value, 4)
                    for i, value in enumerate(samples_y): write(0xa840+i*4, value, 4)
                    for address, value, count in [(0xa00e, 16, 2), (0xf550, pause, 1), (0xf55a, slow, 1),
                                                   (0xf572, pan, 2), (0xf566, shake, 2)]: write(address, value, count)
                    row = invoke('stream-'+scenario+'/'+str(step))
                    row['external_input'] = {'samples_x': samples_x, 'samples_y': samples_y, 'used': 16,
                                             'pause': pause, 'slow': slow, 'pan': pan, 'shake': shake}
                    calls.append(row)
                streams.append({'name': scenario, 'initial': calls[0]['before'], 'calls': calls})
            boundaries = isolated[:boundary_count]
        else:
            boundaries = []

    binding = {'arena_id': arena, 'callback': ENTRIES[arena], 'resource_list': RESOURCES[arena],
               'checkpoint_frame': args.checkpoint, 'checkpoint_arena_sha256': sha(base_ram[0x8180:0x8200])}
    baseline_binding = {'input': str(ip.relative_to(ROOT)), 'input_sha256': sha(ip.read_bytes()),
                        'reference': str(rp.relative_to(ROOT)), 'reference_sha256': sha(rp.read_bytes()),
                        'manifest': str(mp.relative_to(ROOT)), 'manifest_sha256': sha(mp.read_bytes()),
                        'video_steps': len(frames), 'all_ram_video_match': frames == expected}
    runtime = {'schema': 1, 'rom_sha256': identity['rom_sha256'], 'library': library,
               'source': {'wave_byte_base': 0x34000, 'wave_byte_stride': 2,
                          'parallax_word_base': 0x24000, 'parallax_x_origin': 736,
                          'parallax_x_byte_stride': 256, 'parallax_words_per_row': 128}}
    (out/'arenas-2-6-8-runtime.json').write_text(json.dumps(runtime, separators=(',', ':'))+'\n')
    for suffix, records in [('oracle', natural), ('boundaries', boundaries), ('sequences', streams)]:
        name = 'arena'+format(arena, 'x')+'-'+suffix
        payload = {'schema': 1, 'binding': binding, 'library': library, 'calls': records}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode(); compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        calls = [row for stream in records for row in stream['calls']] if suffix == 'sequences' else records
        report = {'pass': bool(records) and not errors, 'scope': 'complete arena callback including camera21556 and feedback21690',
                  'identity': identity, 'binding': binding, 'baseline': baseline_binding,
                  'records': len(records), 'calls': len(calls),
                  'other_entry_contexts': other_contexts if suffix == 'oracle' else [],
                  'reference_only': sum(bool(row.get('reference_only')) for row in calls),
                  'timeline_owners': dict(Counter(part.get('owner', part.get('event')) for row in calls for part in row['timeline'])),
                  'complete_64k_delta': True, 'ordered_timeline_replay_matches_64k': True,
                  'unexplained_changes': 0, 'incomplete_calls': 0, 'errors': errors,
                  'capture_sha256': sha(Path(__file__).read_bytes()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed)}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
