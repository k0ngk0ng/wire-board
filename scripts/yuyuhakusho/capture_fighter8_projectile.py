#!/usr/bin/env python3
"""Original147ae narrow callback and source8 creator/clock/child contexts."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', type=Path, required=True)
    p.add_argument('--inputs', type=Path, default=ROOT/'docs/research/md-yuyuhakusho/source8-next-inputs.json')
    p.add_argument('--reference', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter8-attacks-reference-frames.json')
    p.add_argument('--reference-manifest', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter8-attacks-reference-manifest.json')
    p.add_argument('--natural-only', action='store_true'); p.add_argument('--prefix', default='fighter8-projectile'); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT) and args.prefix.replace('-', '').isalnum(); out.mkdir(parents=True, exist_ok=True)
    input_path, ref_path, manifest_path = args.inputs.resolve(), args.reference.resolve(), args.reference_manifest.resolve()
    events = json.loads(input_path.read_text()); reference_raw = ref_path.read_bytes(); expected = json.loads(reference_raw)
    original = json.loads(manifest_path.read_text()); assert original['output']['sha256'] == sha(reference_raw) and original['input']['sha256'] == sha(input_path.read_bytes())
    assert original['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [0x147ae, 0x1c196, 0x19c96, 0x19cc2, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac, 0x3982,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint*len(points))(*points)
    natural, isolated, streams, errors = [], [], [], []
    library = {}; pending = None; private = False; requested = ''; label = ''
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'

    def snapshot(ram, parent=0x8400):
        obj = ram[parent:parent+256]; pointer = n(obj, 0x90, 4)&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(), 'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)], 'pending_sound': ram[0xa030],
                'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        def remember_frame(ram, address):
            off = address&65535; key, data, origin = selected_data(rom, ram, ram[off:off+256]); identity = json.dumps(key, sort_keys=True)
            assert identity not in library or library[identity]['data'] == data
            library[identity] = {'key': key, 'data': data, 'source': origin}
            return key, data

        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                raw_pc = pc; pc &= 0xffffff
                if pc == 0xfff000:
                    if pending is not None: pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                    return
                if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                    if pending is not None and pending['_irq']:
                        irq = pending['_irq'].pop(); ram = ref.ram_cpu(); irq.update(exit=hex(pc), changes=[{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                        pending['interrupts'].append(irq)
                    return
                if pc in [0x37c56, 0x37cac]:
                    if pending is not None:
                        a = ref.get_register(9)&65535; pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                    return
                if pc == 0x3982:
                    if pending is not None: pending['rng_calls'] += 1
                    return
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15); parent = (address&65535)&0xfc00; ram = ref.ram_cpu()
                role = 'child' if pc == 0x147ae else None
                if private and pc == 0x1c196: role = 'creator'
                if private and requested == 'clock' and pc == 0x19c96: role = 'clock'
                if role and ram[parent+0x58] == 8 and (private or ref.frame <= 3600):
                    assert pending is None
                    pending = {'kind': role, 'frame': ref.frame, 'object_id': address&65535, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp,
                               'before': snapshot(ram, parent), 'draw': False, 'cleared_bytes': 0, 'external_queue_clears': [], 'rng_calls': 0, 'interrupts': [], '_irq': [], '_ram': ram}
                elif pending is not None:
                    if pc == 0x1eb40: remember_frame(ram, address)
                    elif pc == 0x3556:
                        pending['cleared_bytes'] = (n(ram, (address&65535)+4) or 64)//16*16; assert 0 < pending['cleared_bytes'] <= 256
                    finish = (pc == 0x1580 and pending['kind'] == 'child' or pc == 0x3574 and pending['cleared_bytes'] or
                              pc == 0x15afc and pending['kind'] == 'creator' or pc == 0x19cc2 and pending['kind'] == 'clock')
                    if finish:
                        row = pending; pending = None; assert sp == row['entry_sp'] and not row.pop('_irq')
                        row.update(after=snapshot(ram, parent), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                        allowed = (set(range(parent, parent+1024)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa600)) | set(range(0xa004, 0xa00c)) |
                                   {0xa030, 0xa036, 0xa037} | set(range(0xf80a, 0xf80e)) | {(row['entry_sp']-i)&65535 for i in range(0x180)})
                        row['external_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(row.pop('_ram'), ram)) if a != b and i not in allowed]
                        row['unexplained_changes'] = []
                        for change in row['external_changes']:
                            current, steps = change['before'], 0
                            for irq in row['interrupts']:
                                for delta in irq['changes']:
                                    if delta['offset'] != change['offset']: continue
                                    assert delta['before'] == current; current, steps = delta['after'], steps+1
                            if steps == 0 or current != change['after']: row['unexplained_changes'].append(change)
                        assert not row['unexplained_changes'] and row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                        if private:
                            assert not row['external_changes'] and not row['external_queue_clears']; row['name'] = label; isolated.append(row); ref.cpu.cycles = 8000000
                        else: natural.append(row)
            except Exception as exc:
                errors.append(repr(exc))
                if private: ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p); hook(observer); frames = []
        for frame in range(1, len(expected)+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400: checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None); assert frames == expected and pending is None and not errors, (errors[:3], pending is not None)
        identity = ref.identity
        baselines = [{'input': str(input_path.relative_to(ROOT)), 'input_sha256': sha(input_path.read_bytes()), 'reference': str(ref_path.relative_to(ROOT)),
                      'reference_raw_sha256': sha(reference_raw), 'reference_manifest': str(manifest_path.relative_to(ROOT)), 'reference_manifest_sha256': sha(manifest_path.read_bytes()),
                      'video_steps': len(expected), 'all_ram_video_match': True}]
        assert base_ram[0x8458] == 8
        binding = {'source_tag': 8, 'source_actor_sha256': sha(base_ram[0x8400:0x8500]), 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4), 'movement_source': n(base_ram, 0x8490, 4)}
        if not args.natural_only:
            private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))

            def setup(seed=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True); write_bytes(0xff8400, base_ram[0x8400:0x8500])
                write_bytes(0xff8500, bytes((i*73+seed*29+17)&255 for i in range(768))); write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048)))
                write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                values = {'stage': 2, 'size': 256, 'index': 0, 'duration': 8, 'ticks': 0, 'flags': 0, 'completed': 0, 'pause': 0, 'slow': 0,
                          'facing': 0, 'x': 100<<16|0xabcd, 'y': 260<<16|0x5678, 'offset_x': -3, 'offset_y': 4, 'camera_x': 31, 'camera_y': 23, 'shake': -3}
                values.update(changes)
                fields = {0: (0x147ae, 4), 4: (values['size'], 2), 6: (values['stage'], 2), 0x1c: (0xd104a, 4), 0x20: (0x1e0000, 4), 0x50: (0xd1e5a, 4), 0x8c: (0xd2c6a, 4),
                          0x59: (values['index'], 1), 0x5a: (values['duration'], 1), 0x5b: (values['ticks'], 1), 0x5c: (values['flags'], 1), 0x60: (values['completed'], 1),
                          0x5f: (0xa6, 1), 0x61: (values['facing'], 1), 0x40: (values['x'], 4), 0x44: (values['y'], 4), 0x54: (values['offset_x'], 2), 0x56: (values['offset_y'], 2)}
                for off, (value, count) in fields.items(): write(0xff8500+off, value, count)
                for i in range(4): write(0xffa004+i*2, [0, 240, 256][(i+seed)%3], 2)
                write(0xffa036, 1 if seed&1 else 0, 2); write(0xffa030, 0x97); write(0xffff18, 0x12345678, 4)
                write_bytes(0xfff80a, bytes([0, 0x80, 0, 0x81])); write(0xffa03e, [0, 0x2000, 0xffff][seed%3], 2)
                for off, value in [(0xa014, values['camera_x']), (0xa016, values['camera_y']), (0xf566, values['shake'])]: write(0xff0000+off, value, 2)
                write(0xfff550, values['pause']); write(0xfff55a, values['slow'])

            def run(role, name, address=0x8500):
                nonlocal label, requested
                label, requested = name, role
                for reg in range(8): setreg(reg, 0)
                setreg(17, 0x2700); setreg(15, 0xffffff00); write(0xffff00, 0x3d8, 4); setreg(13, 0xffff0000|address)
                setreg(16, {'child': 0x147ae, 'creator': 0x1c196, 'clock': 0x19c96}[role]); ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                count = len(isolated); hook(observer); execute(8000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors[:3])
                return isolated[-1]

            def record(name):
                before = snapshot(ref.ram_cpu()); key, data = remember_frame(ref.ram_cpu(), 0xff8400)
                write(0xff845a, data['duration']); write(0xff845c, data['flags'])
                row = {'kind': 'record', 'name': name, 'object_id': 0x8400, 'before': before, 'after': snapshot(ref.ram_cpu()), 'record_key': key,
                       'draw': False, 'cleared_bytes': 0, 'rng_calls': 0, 'external_queue_clears': [], 'external_changes': [], 'interrupts': [], 'unexplained_changes': []}
                isolated.append(row); return row

            def creator_setup(seed, facing=0, speed=0x300000, busy=False, attributes=0xffff, team=255, target=255):
                setup(seed); write_bytes(0xff8500, bytes(768))
                write(0xff8400, 0x12668, 4); write(0xff8404, 256, 2); write(0xff8406, 2, 2)
                for off, value in [(0x5d, 0x3f), (0x5f, 0xa7), (0x59, 0), (0x5b, 0), (0x60, 0), (0x61, facing), (0x6e, team), (0x70, target)]: write(0xff8400+off, value)
                write(0xff8424, attributes, 2); write(0xff8410, speed, 4); write(0xff8440, 100<<16|0xabcd, 4); write(0xff8444, 320<<16|0x5678, 4)
                write(0xff849c, 320<<16|0x1234, 4); write(0xff8504, 256 if busy else 0, 2)

            for index, facing in itertools.product(range(4), [0, 1]):
                setup(index+facing, index=index, facing=facing, duration=255); run('child', f'animation-{index}-{facing}')
            for seed, (stage, index, duration, ticks, flags, completed, paused, facing) in enumerate(itertools.product([0, 2], [0, 1, 2], [0, 8, 255], [0, 1, 255], [0, 128], [0, 1, 2], range(3), [0, 1, 3])):
                setup(seed, stage=stage, index=index, duration=duration, ticks=ticks, flags=flags, completed=completed, pause=128 if paused == 1 else 0, slow=255 if paused == 2 else 0, facing=facing)
                run('child', f'phase-{stage}-{index}-{duration}-{ticks}-{flags}-{completed}-{paused}-{facing}')
            for seed, (x, y, offsets, facing) in enumerate(itertools.product([-0x80000000, -1, 0, 0x7fffffff], [-0x80000000, -1, 0, 0x7fffffff], [(-32768, 32767), (-1, 1), (0, 0)], [0, 1, 2, 255])):
                setup(seed, x=x, y=y, offset_x=offsets[0], offset_y=offsets[1], camera_x=32767, camera_y=-32768, shake=-32768, facing=facing); run('child', f'geometry-{seed}')
            for seed, size in enumerate([0, 31, 271, *range(16, 257, 16)]): setup(seed, size=size, completed=1); run('child', f'clear-{size}')
            for seed, (busy, facing, attributes, team, target) in enumerate(itertools.product([False, True], [0, 1, 2, 255], [0, 0xffff, 0x9fa0], [0, 0xf0, 255], [0, 255])):
                creator_setup(seed, facing=facing, busy=busy, attributes=attributes, team=team, target=target); run('creator', f'creator-boundary-{seed}', 0x8400)
            boundary_count = len(isolated)
            for stream, (facing, speed, paused) in enumerate(itertools.product([0, 1, 2, 255], [0x300000, 0x700000, 0xb00000], [False, True])):
                creator_setup(stream, facing=facing, speed=speed); write(0xff845b, 255); record('initial-record'); initial = snapshot(ref.ram_cpu()); calls = []
                for step in range(512):
                    external = {}
                    if paused and step == 1: write(0xfff550, 128); external['pause_flags'] = 128
                    if paused and step == 2: write(0xfff550, 0); external['pause_flags'] = 0
                    rows = []
                    if ref.ram_cpu()[0x845d] == 0x3f:
                        rows.append(run('clock', f'{stream}/{step}-clock', 0x8400)); rows.append(run('creator', f'{stream}/{step}-parent', 0x8400)); rows.append(record(f'{stream}/{step}-record'))
                    if n(ref.ram_cpu(), 0x8504): rows.append(run('child', f'{stream}/{step}-child'))
                    assert rows; rows[0]['external_input'] = external; calls.extend(rows)
                    ram = ref.ram_cpu()
                    if ram[0x845d] != 0x3f and n(ram, 0x8504) == 0: break
                else: raise AssertionError('source8 parent/child flow did not finish')
                streams.append({'name': f'{facing}-{speed}-{paused}', 'initial': initial, 'calls': calls})

    collections = [(args.prefix+'-oracle', natural, 'Actual source8 child callbacks from the declared real-menu input flow')]
    if not args.natural_only: collections += [(args.prefix+'-boundaries', isolated[:boundary_count], 'Explicit original child/creator contexts from a real source8 checkpoint'), (args.prefix+'-sequences', streams, 'Single-initial source8 parent clock/motion and child lifecycle')]
    for name, rows, scope in collections:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':'))+'\n').encode(); packed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed); calls = [r for stream in rows for r in stream['calls']] if name.endswith('sequences') else rows
        report = {'pass': bool(rows) and not errors, 'baseline_verified': frames == expected, 'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(rows), 'calls': len(calls), 'kind_counts': dict(Counter(r['kind'] for r in calls)), 'library_frames': len(library),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'baselines': baselines, 'binding': binding, 'frames_match_original_core': frames == expected,
                  'rng_calls': sum(r['rng_calls'] for r in calls), 'external_queue_calls': sum(bool(r['external_queue_clears']) for r in calls),
                  'external_change_calls': sum(bool(r['external_changes']) for r in calls), 'unexplained_change_calls': sum(bool(r['unexplained_changes']) for r in calls),
                  'interrupt_calls': sum(len(r['interrupts']) for r in calls), 'clears': sum(r['cleared_bytes'] != 0 for r in calls), 'errors': errors, 'incomplete_calls': 0,
                  'limits': ['Zero natural records remain pass:false; no private context is labeled natural', 'Pre-tail/draw/completed-clear boundaries only; no full actor/world/collision/active FX simulation',
                             'Parent timing refreshes modify only Duration/Flags from own current immutable keys; no expected-state injection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': report['pass']}))
    child = [r for r in library.values() if r['key']['animation_set'] == 0xd1e5a and r['key']['shape_set'] == 0xd2c6a and r['key']['pose'] == 0xa6]
    catalog_path = ROOT/'games/yuyuhakusho/data/animation-catalog.json.gz'; existing = json.loads(gzip.decompress(catalog_path.read_bytes()))['frames']
    for row in child: assert any(r['key'] == row['key'] and r['data'] == row['data'] for r in existing)
    evidence = {'pass': len(child) == 4, 'scope': 'Existing source8 posea6 records independently exercised by147ae, no runtime-data/bundle change', 'frames': len(child),
                'keys': [r['key'] for r in child], 'catalog': str(catalog_path.relative_to(ROOT)), 'catalog_sha256': sha(catalog_path.read_bytes()),
                'all_existing_records_equal': True, 'capture_sha256': sha(Path(__file__).read_bytes()), 'fixture': args.prefix+'-boundaries.json.gz' if not args.natural_only else args.prefix+'-oracle.json.gz'}
    (out/(args.prefix+'-animation-validation.json')).write_text(json.dumps(evidence, indent=2)+'\n')


if __name__ == '__main__': main()
