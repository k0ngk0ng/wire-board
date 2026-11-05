#!/usr/bin/env python3
"""Source4 MAIN pre-tail motion, original D0 residue and exact narrow writes."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_ground_idle import actor as body_state
from capture_effect_pool import slot_index
from capture_animation_records import selected_data

ENTRIES = {0x1ca46: 0x0e, 0x1cb68: 0x12, 0x1b1e8: 0x25, 0x1b21e: 0x26,
           0x1b246: 0x27, 0x1b2bc: 0x28, 0x1b2d4: 0x29, 0x1b3f0: 0x2a,
           0x1b48a: 0x2b, 0x1b51a: 0x2c}
POSES = {0x0e: 11, 0x12: 12, 0x25: 0xa0, 0x26: 0xa4, 0x27: 0xa5,
         0x28: 0xac, 0x29: 0xa6, 0x2a: 0xa7, 0x2b: 0xab, 0x2c: 0xa8}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x15afc, 0x1fae6, 0x1faee, 0x19cc2, 0x3982, 0x37c56, 0x37cac, 0x1b282, 0x1b296]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    sources = [('docs/research/md-yuyuhakusho/source4-next-inputs.json',
                'games/yuyuhakusho/engine/testdata/fighter4-attacks-reference-frames.json',
                'games/yuyuhakusho/engine/testdata/fighter4-attacks-reference-manifest.json')]
    special = ('games/yuyuhakusho/engine/testdata/fighter4-special-inputs.json',
               'games/yuyuhakusho/engine/testdata/fighter4-attacks-special-reference-frames.json',
               'games/yuyuhakusho/engine/testdata/fighter4-attacks-special-reference-manifest.json')
    if all((ROOT/name).is_file() for name in special): sources.append(special)
    natural, isolated, sequences, errors, baselines = [], [], [], [], []
    pending = None; private = False; clock_only = False; clock_done = False; label = ''; scenario = ''; library = {}

    def snapshot(ram, address):
        off = address&65535; obj = ram[off:off+256]; pointer = n(obj, 0x90, 4)&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': off, 'character_id': obj[0x58], 'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71]|obj[0xc0]<<8, 'pressed': obj[0x72]|obj[0xc1]<<8,
                             'changed': obj[0x73]|obj[0xc2]<<8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'energy': n(obj, 0xb8, 4, True), 'power': n(obj, 0x7c, 2, True),
                'alternate_velocity': n(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[off+256:off+1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'movement_source': n(obj, 0x90, 4), 'dragon_bank_cursor': n(ram, 0xa038),
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'animation_set': n(obj, 0x50, 4), 'shape_set': n(obj, 0x8c, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}}

    for source_index, (input_name, reference_name, manifest_name) in enumerate(sources):
        scenario = Path(input_name).name; input_path = ROOT/input_name; reference_path = ROOT/reference_name
        reference_raw = reference_path.read_bytes(); reference_manifest = json.loads((ROOT/manifest_name).read_text())
        assert reference_manifest['observer_attached'] is False
        assert reference_manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert reference_manifest['input']['sha256'] == sha(input_path.read_bytes())
        assert reference_manifest['output']['sha256'] == sha(reference_raw)
        expected = json.loads(reference_raw); events = json.loads(input_path.read_text())
        with ReferenceSession(ROOT, out/('observed-'+str(source_index)), core=core) as ref:
            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending, clock_done
                try:
                    if clock_only:
                        if pc == 0x19cc2: clock_done = True; ref.cpu.cycles = 8000000
                        return
                    if pc == 0x3982:
                        if pending is not None: pending['rng_calls'] += 1
                        return
                    if pc in [0x37c56, 0x37cac]:
                        if pending is not None:
                            address = ref.get_register(9)&65535
                            pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(address), 'before': ref.ram_cpu()[address]})
                        return
                    if pc in [0x1b282, 0x1b296]:
                        if pending is not None:
                            pending['register_observations'].append({'pc': hex(pc), 'd0': ref.get_register(0)})
                        return
                    address = ref.get_register(13)&0xffffff; ram = ref.ram_cpu()
                    if pc in ENTRIES and ram[(address&65535)+0x58] == 4:
                        assert pending is None, 'overlapping source4 MAIN'
                        pending = {'kind': ENTRIES[pc], 'frame': ref.frame, 'scenario': scenario,
                                   'source_register': hex(ref.get_register(13)), 'entry_sp': ref.get_register(15),
                                   'before': snapshot(ram, address), 'allocations': [], 'register_observations': [],
                                   'rng_calls': 0, 'external_queue_clears': []}
                        assert ref.get_register(13) == 0xffff0000|(address&65535)
                    elif pending is not None:
                        if pc in [0x1fae6, 0x1faee]: pending['allocations'].append(slot_index(ref.get_register(8)))
                        elif pc == 0x15afc:
                            assert ref.get_register(15) == pending['entry_sp']
                            row = pending; pending = None
                            row.update(after=snapshot(ram, address), exit='0x15afc', exit_frame=ref.frame)
                            assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                            if row['before']['sound_queue'] != row['after']['sound_queue']: assert row['external_queue_clears']
                            if private: row['name'] = label; isolated.append(row); ref.cpu.cycles = 8000000
                            else: natural.append(row)
                except Exception as error:
                    errors.append(repr(error))
                    if private: ref.cpu.cycles = 8000000
            assert bridge.yyh_set_observer(observe, pcs, len(points))
            hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
            hook(observer); frames = []
            for frame in range(1, len(expected)+1):
                frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
                if frame == 2400: checkpoint, base_ram = ref.save(), ref.ram_cpu()
            hook(None); assert frames == expected and pending is None and not errors, (scenario, errors)
            baselines.append({'input': input_name, 'input_sha256': sha(input_path.read_bytes()), 'reference': reference_name,
                              'reference_raw_sha256': sha(reference_raw), 'reference_file_sha256': sha(reference_path.read_bytes()),
                              'reference_manifest': manifest_name, 'reference_manifest_sha256': sha((ROOT/manifest_name).read_bytes()),
                              'video_steps': len(expected), 'all_ram_video_match': True})
            identity = ref.identity
            if source_index < len(sources)-1: continue
            assert base_ram[0x8458] == 4
            movement = n(base_ram, 0x8490, 4)
            binding = {'source_tag': 4, 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4),
                       'parameter_pointer': movement, 'boost_pointer': 0x15946, 'source_actor_sha256': sha(base_ram[0x8400:0x8500])}
            private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1):
                write_bytes(address, (value&((1<<(size*8))-1)).to_bytes(size, 'big'))

            def add_record():
                ram = ref.ram_cpu(); key, data, source = selected_data(rom, ram, ram[0x8400:0x8500]); identity = json.dumps(key, sort_keys=True)
                assert identity not in library or library[identity]['data'] == data
                library[identity] = {'key': key, 'data': data, 'source': source}
                write(0xff845a, data['duration']); write(0xff845c, data['flags'])
                return key

            def setup(kind, pool='empty', children=0, cursor=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, bytes((i*73+kind*29+17)&255 for i in range(1024)))
                write_bytes(0xff9400, bytes((i*37+kind*23+13)&255 for i in range(2048)))
                free = {'empty': set(range(16)), 'full': set(), 'single': {1}, 'two': {1, 3}, 'three': {0, 5, 14}}
                for i in range(16): write(0xff9404+i*128, 0 if i in free[pool] else 128, 2)
                for i in range(3): write(0xff8504+i*256, 256 if children&(1<<i) else 0, 2)
                write_bytes(0xff8450, base_ram[0x8450:0x8454]); write_bytes(0xff848c, base_ram[0x848c:0x8494])
                pose = changes.pop('pose', POSES[kind]); index = changes.pop('index', 0); bank_cursor = changes.pop('bank_cursor', 0x600)
                fields = {0x24: (0x4321, 2), 0x6e: (0xf8, 1), 0x70: (255, 1), 0x58: (4, 1), 0x59: (index, 1),
                          0x5a: (8, 1), 0x5b: (0, 1), 0x5c: (0, 1), 0x5d: (kind, 1), 0x5f: (pose, 1), 0x60: (0, 1),
                          0x61: (0, 1), 0x6b: (0x57, 1), 0x71: (0, 1), 0x72: (0, 1), 0x73: (0, 1),
                          0xc0: (0xa1, 1), 0xc1: (0xb2, 1), 0xc2: (0xc3, 1), 0x87: (7, 1),
                          0x40: (0x0064abcd, 4), 0x44: (0x01305678, 4), 0x9c: (0x01401234, 4),
                          0x10: (0x28001, 4), 0x14: (-0x40001, 4), 0x48: (0x601234, 4), 0x4c: (0x5000, 4),
                          0xb8: (223<<16|0xabcd, 4), 0x7c: (127, 2), 0x89: (0, 1), 0x90: (movement, 4)}
                aliases = {'attrs': 0x24, 'team': 0x6e, 'target_bit': 0x70, 'ticks': 0x5b, 'duration': 0x5a, 'flags': 0x5c,
                           'completed': 0x60, 'facing': 0x61, 'request': 0x6b, 'held': 0x71, 'pressed': 0x72, 'changed': 0x73,
                           'x': 0x40, 'y': 0x44, 'ground': 0x9c, 'vx': 0x10, 'vy': 0x14, 'alt': 0x48, 'gravity': 0x4c,
                           'energy': 0xb8, 'power': 0x7c, 'contact': 0x89, 'extra_jump': 0x87, 'movement_source': 0x90}
                child_fields = {'cx': (0x8540, 4), 'cy': (0x8544, 4), 'child_size': (0x8504, 2),
                                'child_contact': (0x8589, 1), 'child_direction': (0x8548, 4),
                                'child_held': (0x8571, 1), 'child_pressed': (0x8572, 1), 'child_bank': (0x85fe, 2)}
                for key, value in changes.items():
                    if key in child_fields:
                        off, size = child_fields[key]; write(0xff0000|off, value, size)
                    else:
                        off = aliases[key]; fields[off] = value, fields[off][1]
                for off, (value, size) in fields.items(): write(0xff8400+off, value, size)
                write(0xffa036, cursor, 2); write(0xffa038, bank_cursor, 2); write(0xffa030, 0x97); write(0xffff18, 0x12345678, 4)

            def prepare(entry):
                for reg in range(8): setreg(reg, 0)
                setreg(17, 0x2700); setreg(15, 0xffffff00); write(0xffff00, 0x3d8, 4); setreg(13, 0xffff8400); setreg(16, entry)
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896

            def call(kind, name):
                nonlocal label
                label = name; prepare(next(pc for pc, k in ENTRIES.items() if k == kind)); before = ref.ram_cpu(); count = len(isolated)
                hook(observer); execute(6000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors)
                after = ref.ram_cpu()
                allowed = set(range(0x8400, 0x8800))|set(range(0x9400, 0x9c00))|{0xa030, 0xa036, 0xa037, 0xa038, 0xa039}|set(range(0xfec0, 0xff04))
                unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                assert not unknown, (name, [hex(i) for i in unknown])
                assert before[0x7000:0x7800] == after[0x7000:0x7800], 'MAIN changed child history'
                return isolated[-1]

            def case(kind, name, pool='empty', children=0, cursor=0, **changes):
                setup(kind, pool, children, cursor, **changes); return call(kind, name)

            def clock():
                nonlocal clock_only, clock_done
                clock_only, clock_done = True, False; prepare(0x19c96); hook(observer); execute(100000); hook(None); clock_only = False
                assert clock_done and not errors

            for completed, facing, ticks, y in itertools.product([0, 1, 2], [0, 1, 2, 255], [0, 1, 255], [0x140ffff, 0x1301234]):
                case(0x0e, f'forward-{completed}-{facing}-{ticks}-{y}', completed=completed, facing=facing, ticks=ticks, y=y)
            for index, ticks, facing, completed, y in itertools.product(range(8), [0, 1], [0, 1, 2, 255], [0, 1, 2], [0x140ffff, 0x1301234]):
                case(0x12, f'reverse8-{index}-{ticks}-{facing}-{completed}-{y}', index=index, ticks=ticks, facing=facing, completed=completed, y=y)
            for completed, facing, vx, x in itertools.product([0, 1, 2], [0, 1, 2, 255], [-0x80000000, -0x28000, 0, 0x28000, 0x7fffffff], [0x64abcd, 0x7fffffff, -0x80000000]):
                case(0x25, f'rush-{completed}-{facing}-{vx}-{x}', completed=completed, facing=facing, vx=vx, x=x)
            for kind, count in [(0x26, 7), (0x27, 11)]:
                for index, ticks, facing, pool in itertools.product(range(count), [0, 1, 2, 3, 4, 255], [0, 1, 2, 255], ['empty', 'full']):
                    case(kind, f'd0-{kind:x}-{index}-{ticks}-{facing}-{pool}', pool, index=index, ticks=ticks, facing=facing, vx=-0x7654321)
                for index, completed, facing in itertools.product([0, count-1], [1, 2, 255], [0, 1, 2, 255]):
                    case(kind, f'finish-{kind:x}-{index}-{completed}-{facing}', index=index, completed=completed, facing=facing)
                case(kind, f'completed-before-invalid-{kind:x}', index=255, completed=1)
            for completed, extra, source, y in itertools.product([0, 1, 2, 255], [0, 1, 254, 255], [movement, 0x15946], [0x140ffff, 0x1301234]):
                case(0x28, f'air-{completed}-{extra}-{source}-{y}', completed=completed, extra_jump=extra, movement_source=source, y=y)
            for ticks, facing, pool in itertools.product([0, 1, 15, 16, 17, 240, 255], [0, 1, 2, 255], ['empty', 'full', 'single', 'two']):
                case(0x29, f'dragon-fx-{ticks}-{facing}-{pool}', pool, index=0, ticks=ticks, facing=facing)
            for ticks, children, facing, cursor in itertools.product([0, 1], [0, 1, 2, 4, 7], [0, 1, 2, 255], [0, 0x600, 0x7ff, 0xffff]):
                case(0x29, f'dragon-create-{ticks}-{children}-{facing}-{cursor}', children=children, index=2, ticks=ticks, facing=facing, bank_cursor=cursor)
            for index, completed, facing in itertools.product([0, 2, 255], [1, 2], [0, 1, 2, 255]):
                case(0x29, f'dragon-follow-{index}-{completed}-{facing}', index=index, completed=completed, facing=facing)
            distances = [(0, 0), (24, 32), (25, 0), (-24, -32), (-25, 0), (0, 33), (0, -33), (-32768, 0), (0, -32768)]
            for (dx, dy), pressed, child_size, pool, facing, completed in itertools.product(distances, [0, 0x20, 0x90], [0, 256], ['empty', 'full'], [0, 2], [0, 1]):
                case(0x2a, f'swallow-{dx}-{dy}-{pressed}-{child_size}-{pool}-{facing}-{completed}', pool,
                     cx=((100+dx)&65535)<<16|0xbeef, cy=((320-32+dy)&65535)<<16|0x1234,
                     child_size=child_size, child_contact=0xa5, held=0xef, pressed=pressed, facing=facing, completed=completed, y=0x1405678)
            for index, ticks, children, completed, facing in itertools.product([0, 1, 2, 255], [0, 1], [0, 1, 2, 4, 7], [0, 1, 2], [0, 2]):
                case(0x2b, f'boost-child-{index}-{ticks}-{children}-{completed}-{facing}', children=children,
                     index=index, ticks=ticks, completed=completed, facing=facing, movement_source=0x15946)
            for pose, held, vx, facing in itertools.product([0xa8, 0xae, 255], [0, 4, 8, 12],
                    [-0x80000000, -0x40001, -0x40000, -1, 0, 1, 0x40000, 0x40001, 0x7fffffff], [0, 1, 2, 255]):
                case(0x2c, f'air-direction-{pose}-{held}-{vx}-{facing}', pose=pose, held=held, vx=vx, facing=facing)
            for pose, distance, pressed, contact, index in itertools.product([0xa8, 0xae], [-32768, -33, -32, -31, 0, 31, 32, 33], [0, 0x10, 0x40, 0x50], [0, 4, 8, 12], [0, 255]):
                case(0x2c, f'air-height-{pose}-{distance}-{pressed}-{contact}-{index}', pose=pose, y=((320-distance)&65535)<<16|0xabcd,
                     vy=0, gravity=0, vx=0, pressed=pressed, contact=contact, index=index)
            for facing, extra, source, completed, y in itertools.product([0, 1, 2, 255], [0, 1, 254, 255], [movement, 0x15946], [0, 1, 2], [0x140ffff, 0x1301234]):
                case(0x2c, f'air-complete-{facing}-{extra}-{source}-{completed}-{y}', facing=facing, extra_jump=extra, movement_source=source, completed=completed, y=y)
            for vy, gravity, y in itertools.product([-0x80000000, -1, 0, 1, 0x7fffffff], [-0x80000000, 0, 0x5000, 0x7fffffff],
                                                    [0x13fffff, 0x1401234, 0x1410000, -0x80000000, 0x7fffffff]):
                case(0x2c, f'air-wrap-{vy}-{gravity}-{y}', vy=vy, gravity=gravity, y=y)
            boundary_count = len(isolated)
            configs = [(0x0e, 'forward-ground', {'y': 0x1405678}, 0), (0x0e, 'forward-air', {}, 2),
                       (0x12, 'retreat-ground', {'y': 0x1405678}, 0), (0x12, 'retreat-air', {}, 2),
                       (0x25, 'rush', {}, 3), (0x26, 'dash-short', {}, 0), (0x27, 'dash-long', {}, 0),
                       (0x28, 'air-weak', {}, 2), (0x28, 'air-strong', {'pose': 0xad}, 2),
                       (0x29, 'dragon-create-cancel', {}, 3),
                       (0x29, 'dragon-existing-swallow', {'child_size': 256, 'cx': 0x64beef, 'cy': 0x1101234}, 0),
                       (0x2a, 'swallow-empty', {'child_size': 0, 'cx': 0x64beef, 'cy': 0x1101234}, 0),
                       (0x2a, 'swallow-existing', {'child_size': 256, 'cx': 0x64beef, 'cy': 0x1101234}, 0),
                       (0x2a, 'follow-cancel', {'child_size': 256}, 3),
                       (0x2b, 'boost-create', {'movement_source': 0x15946}, 0),
                       (0x2c, 'air-landing', {'vy': -0x68000}, 3),
                       (0x2c, 'air-cancel', {'y': 0x1005678, 'vy': 0}, 3),
                       (0x2c, 'air-contact', {'pose': 0xae, 'contact': 4, 'vy': 0x28000}, 3),
                       (0x2c, 'air-animation-default', {'pose': 0xae, 'y': -0x1000000, 'gravity': 0, 'vy': 0}, 2),
                       (0x2c, 'air-animation-boost', {'pose': 0xae, 'y': -0x1000000, 'gravity': 0, 'vy': 0, 'movement_source': 0x15946}, 2)]
            for (kind, name, changes, terminal), pool in itertools.product(configs, ['empty', 'full']):
                setup(kind, pool, **changes); write(0xff845b, 255); add_record(); initial = snapshot(ref.ram_cpu(), 0xff8400); steps = []
                for index in range(1200):
                    pressed = 0x20 if name in ['dragon-create-cancel', 'follow-cancel'] and index == 200 else 0
                    if name == 'air-cancel' and index == 0: pressed = 0x10
                    write(0xff8471, 0); write(0xff8472, pressed); clock()
                    active = ref.ram_cpu()[0x845d]; assert active in ENTRIES.values(), (name, active)
                    row = call(active, f'stream-{name}-{pool}-{index}'); row['advance_clock'] = True
                    row['external_input'] = {'held': 0, 'pressed': pressed}; row['record_after'] = add_record()
                    row['after_record'] = snapshot(ref.ram_cpu(), 0xff8400); steps.append(row)
                    if row['after_record']['body']['action_kind'] not in ENTRIES.values(): break
                else: raise AssertionError(('source4 MAIN did not reach endpoint', name, pool))
                assert row['after_record']['body']['action_kind'] == terminal, (name, row['after_record']['body']['action_kind'], terminal)
                sequences.append({'kind': kind, 'name': name, 'pool': pool, 'initial': initial, 'steps': steps, 'terminal': terminal})
    parameter_records = []
    for source in [binding['parameter_pointer'], 0x15946]:
        record = rom[source:source+16]; assert len(record) == 16
        parameter_records.append({'source': source, 'raw_bytes': record.hex(),
                                  'data': {key: n(record, offset, 4, True) for key, offset in
                                           [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}})
    parameter_raw = (json.dumps({'rom_sha256': sha(rom), 'records': parameter_records}, indent=2)+'\n').encode()
    (out/'fighter4-motion-parameters.json').write_bytes(parameter_raw)
    for name, rows, scope in [('fighter4-motion-oracle', natural, 'Actual source4 MAIN pre-tail entries from declared real-menu inputs'),
                              ('fighter4-motion-boundaries', isolated[:boundary_count], 'Explicit complete source4 MAIN phase contexts'),
                              ('fighter4-motion-sequences', sequences, 'Single initial state, original clock and actual readonly animation timing records')]:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':'))+'\n').encode(); compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        calls = [row for stream in rows for row in stream['steps']] if name.endswith('sequences') else rows
        report = {'pass': True, 'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(rows), 'calls': len(calls),
                  'kind_counts': dict(Counter(str(row['kind']) for row in calls)),
                  'character_counts': dict(Counter(str(row['before']['character_id']) for row in calls)),
                  'allocation_attempts': sum(len(row['allocations']) for row in calls),
                  'external_queue_calls': sum(bool(row['external_queue_clears']) for row in calls),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'baselines': baselines,
                  'frames_match_original_core': all(b['all_ram_video_match'] for b in baselines), 'binding': binding,
                  'library_frames': len(library), 'movement_parameters_sha256': sha(parameter_raw),
                  'rng_calls': sum(row['rng_calls'] for row in calls),
                  'private_unknown_ram_writes': 0, 'main_history_buffer_writes': 0, 'errors': errors, 'incomplete_calls': 0,
                  'limits': ['Stops before15afc; no shared tail, child/FX callbacks or world scheduling.',
                             'Only Duration/Flags are refreshed from actual immutable animation records; no intermediate state injection.']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': True}))


if __name__ == '__main__': main()
