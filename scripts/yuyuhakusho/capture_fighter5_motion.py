#!/usr/bin/env python3
"""Source5 MAIN CPU evidence against an independently retained original control."""
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

ENTRIES = {0x1cb08: 0x11, 0x1b67a: 0x2d, 0x1b6c4: 0x2e, 0x1b704: 0x2f,
           0x1b7fa: 0x30, 0x1b8f4: 0x31, 0x1b9bc: 0x32}
POSES = {0x11: 0x0c, 0x2d: 0xa8, 0x2e: 0xa4, 0x2f: 0xa0,
         0x30: 0xac, 0x31: 0xa3, 0x32: 0xa0}
CORE = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
ORIGINAL = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'


def n(raw, offset, size=2, signed=False):
    return int.from_bytes(raw[offset:offset + size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser()
    for name in ['input', 'reference', 'manifest', 'out']:
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--natural-only', action='store_true')
    args = parser.parse_args()
    input_path, reference_path, manifest_path, out = [p.resolve() for p in
        [args.input, args.reference, args.manifest, args.out]]
    assert all(p.is_relative_to(ROOT) for p in [input_path, reference_path, manifest_path, out])
    out.mkdir(parents=True, exist_ok=True)
    control_raw = reference_path.read_bytes()
    control = json.loads(manifest_path.read_text())
    assert control['observer_attached'] is False and control['identity']['core_sha256'] == ORIGINAL
    assert control['input']['sha256'] == sha(input_path.read_bytes())
    assert control['output']['sha256'] == sha(control_raw)
    expected, events = json.loads(control_raw), json.loads(input_path.read_text())
    assert control['frames'] == len(expected)
    entry_limit = control.get('entry_frame_limit', len(expected) - 2)
    assert len(expected) - entry_limit >= 2
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    assert sha(core.read_bytes()) == CORE
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x15afc, 0x19cc2, 0x3982, 0x1fae6, 0x1faee, 0x37c56, 0x37cac,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint * len(points))(*points)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    natural, isolated, sequences, errors, library = [], [], [], [], {}
    pending, private, clock_only, clock_done, label = None, False, False, False, ''
    checkpoint, base_ram = None, None

    def snapshot(ram, address):
        off = address & 65535
        obj = ram[off:off + 256]
        pointer = n(obj, 0x90, 4) & 0xffffff
        assert pointer + 16 <= len(rom), 'movement context must be immutable ROM'
        return {'object_id': off, 'character_id': obj[0x58], 'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71] | obj[0xc0] << 8, 'pressed': obj[0x72] | obj[0xc1] << 8,
                             'changed': obj[0x73] | obj[0xc2] << 8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'energy': n(obj, 0xb8, 4, True), 'power': n(obj, 0x7c, 2, True),
                'alternate_velocity': n(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[off + 256:off + 1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'movement_source': pointer, 'animation_set': n(obj, 0x50, 4), 'shape_set': n(obj, 0x8c, 4),
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(rom, pointer + delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}}

    with ReferenceSession(ROOT, out / 'observed', core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, clock_done
            try:
                raw_pc, pc = pc, pc & 0xffffff
                if clock_only:
                    if pc == 0x19cc2:
                        clock_done = True
                        ref.cpu.cycles = 8000000
                    return
                if pending is not None:
                    if pc == 0xfff000:
                        pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                        return
                    if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                        if pending['_irq']:
                            irq = pending['_irq'].pop()
                            ram = ref.ram_cpu()
                            irq.update(exit=hex(pc), changes=[{'offset': i, 'before': a, 'after': b} for i, (a, b) in
                                enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                            pending['interrupts'].append(irq)
                        return
                    if pc == 0x3982:
                        pending['rng_calls'] += 1
                        return
                    if pc in [0x37c56, 0x37cac]:
                        address = ref.get_register(9) & 65535
                        pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(address), 'before': ref.ram_cpu()[address]})
                        return
                    if pc in [0x1fae6, 0x1faee]:
                        pending['allocations'].append(slot_index(ref.get_register(8)))
                        return
                address = ref.get_register(13) & 65535
                ram = ref.ram_cpu()
                if pc in ENTRIES:
                    if not private and ref.frame > entry_limit:
                        return
                    if address not in range(0x8400, 0x9400, 1024) or ram[address + 0x58] != 5:
                        return
                    assert pending is None, 'overlapping source5 MAIN'
                    assert ref.get_register(13) == 0xffff0000 | address
                    pending = {'kind': ENTRIES[pc], 'frame': ref.frame, 'scenario': input_path.name,
                               'source_register': hex(ref.get_register(13)), 'entry_sp': ref.get_register(15),
                               'before': snapshot(ram, address), 'allocations': [], 'rng_calls': 0,
                               'external_queue_clears': [], 'interrupts': [], '_irq': [], '_ram': ram}
                elif pc == 0x15afc and pending is not None:
                    row, pending = pending, None
                    assert ref.get_register(15) == row['entry_sp'] and not row.pop('_irq')
                    row.update(after=snapshot(ram, address), exit='0x15afc', exit_frame=ref.frame,
                               exit_sp=ref.get_register(15), ram_bytes=len(ram))
                    allowed = set(range(address, address + 1024)) | {(row['entry_sp'] - i) & 65535 for i in range(0x180)}
                    allowed |= {int(clear['address'], 16) for clear in row['external_queue_clears']}
                    original_ram = row.pop('_ram')
                    row['ram_before_sha256'], row['ram_after_sha256'] = sha(original_ram), sha(ram)
                    row['ram_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in
                        enumerate(zip(original_ram, ram)) if a != b]
                    row['external_changes'] = [change for change in row['ram_changes'] if change['offset'] not in allowed]
                    row['unexplained_changes'] = []
                    for change in row['external_changes']:
                        current, count = change['before'], 0
                        for irq in row['interrupts']:
                            for delta in irq['changes']:
                                if delta['offset'] == change['offset']:
                                    assert delta['before'] == current
                                    current, count = delta['after'], count + 1
                        if count == 0 or current != change['after']:
                            row['unexplained_changes'].append(change)
                    assert not row['unexplained_changes'], row['unexplained_changes']
                    assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                    assert not row['allocations'] and row['before']['pool'] == row['after']['pool']
                    assert row['before']['pending_sound'] == row['after']['pending_sound']
                    if row['before']['sound_queue'] != row['after']['sound_queue']:
                        assert row['external_queue_clears'] or row['interrupts']
                    if private:
                        assert not row['interrupts'] and not row['external_queue_clears']
                        row['name'] = label
                        isolated.append(row)
                        ref.cpu.cycles = 8000000
                    else:
                        natural.append(row)
            except Exception as error:
                errors.append(repr(error))
                if private:
                    ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, len(expected) + 1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None)
        assert frames == expected and pending is None and not errors, (errors, pending is not None)
        assert natural, 'no actual source5 MAIN calls under declared input'
        if not args.natural_only:
            assert checkpoint is not None and base_ram[0x8458] == 5
            assert n(base_ram, 0x8404) == 256 and 0 < n(base_ram, 0x8400, 4) < len(rom)
            movement = n(base_ram, 0x8490, 4)
            binding = {'source_tag': 5, 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4),
                       'parameter_pointer': movement, 'checkpoint_frame': 2400,
                       'source_header': base_ram[0x8400:0x8408].hex(),
                       'source_actor_sha256': sha(base_ram[0x8400:0x8500])}
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte

            def write(address, value, size=1):
                write_bytes(address, (value & ((1 << (size * 8)) - 1)).to_bytes(size, 'big'))

            def add_record():
                ram = ref.ram_cpu()
                key, data, origin = selected_data(rom, ram, ram[0x8400:0x8500])
                identity = json.dumps(key, sort_keys=True)
                assert identity not in library or library[identity]['data'] == data
                library[identity] = {'key': key, 'data': data, 'source': origin}
                write(0xff845a, data['duration'])
                write(0xff845c, data['flags'])
                return key

            def setup(kind, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, bytes((i * 73 + kind * 29 + 17) & 255 for i in range(1024)))
                write_bytes(0xff9400, bytes((i * 37 + kind * 23 + 13) & 255 for i in range(2048)))
                for i in range(16):
                    write(0xff9404 + i * 128, 0 if i & 1 else 128, 2)
                for i in range(3):
                    write(0xff8504 + i * 256, 0, 2)
                write_bytes(0xff8450, base_ram[0x8450:0x8454])
                write_bytes(0xff848c, base_ram[0x848c:0x8494])
                fields = {0x24: (0x4321, 2), 0x26: (0, 1), 0x3e: (0x7abc, 2),
                          0x6e: (0xf8, 1), 0x70: (255, 1), 0x58: (5, 1), 0x59: (0, 1),
                          0x5a: (8, 1), 0x5b: (0, 1), 0x5c: (0, 1), 0x5d: (kind, 1), 0x5f: (POSES[kind], 1), 0x60: (0, 1),
                          0x61: (0, 1), 0x6b: (0x57, 1), 0x71: (0, 1), 0x72: (0, 1), 0x73: (0, 1),
                          0xc0: (0xa1, 1), 0xc1: (0xb2, 1), 0xc2: (0xc3, 1), 0x87: (7, 1),
                          0x40: (0x0064abcd, 4), 0x44: (0x01305678, 4), 0x9c: (0x01401234, 4),
                          0x10: (0x28001, 4), 0x14: (-0x40001, 4), 0x48: (0x001e1234, 4), 0x4c: (0x005500aa, 4),
                          0xb8: (223 << 16 | 0xabcd, 4), 0x7c: (127, 2), 0x89: (0, 1), 0x90: (movement, 4)}
                aliases = {'attrs': 0x24, 'layer': 0x26, 'depth': 0x3e,
                           'team': 0x6e, 'target_bit': 0x70, 'pose': 0x5f, 'index': 0x59,
                           'ticks': 0x5b, 'duration': 0x5a, 'flags': 0x5c, 'completed': 0x60, 'facing': 0x61,
                           'request': 0x6b, 'held': 0x71, 'pressed': 0x72, 'x': 0x40, 'y': 0x44, 'ground': 0x9c,
                           'vx': 0x10, 'vy': 0x14, 'alt': 0x48, 'gravity': 0x4c, 'contact': 0x89, 'extra_jump': 0x87}
                child_fields = {'child_size': (0x8504, 2), 'charge_size': (0x8604, 2),
                                'child_gravity': (0x854c, 4), 'child_owner': (0x85fe, 2),
                                'charge_vx': (0x8610, 4), 'charge_vy': (0x8614, 4)}
                for key, value in changes.items():
                    if key in child_fields:
                        offset, size = child_fields[key]
                        write(0xff0000 | offset, value, size)
                    else:
                        offset = aliases[key]
                        fields[offset] = value, fields[offset][1]
                for offset, (value, size) in fields.items():
                    write(0xff8400 + offset, value, size)
                write(0xffa036, 0xff80, 2)
                write(0xffa030, 0x97)
                write(0xffff18, 0x12345678, 4)

            def prepare(entry):
                for reg in range(8):
                    setreg(reg, 0)
                setreg(17, 0x2700)
                setreg(15, 0xffffff00)
                write(0xffff00, 0x3d8, 4)
                setreg(13, 0xffff8400)
                setreg(16, entry)
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896

            def call(kind, name):
                nonlocal label
                label = name
                prepare(next(pc for pc, slot in ENTRIES.items() if slot == kind))
                before, count = ref.ram_cpu(), len(isolated)
                hook(observer)
                execute(6000000)
                hook(None)
                assert len(isolated) == count + 1 and pending is None and not errors, (name, errors)
                after = ref.ram_cpu()
                allowed = set(range(0x8400, 0x8800)) | set(range(0xfec0, 0xff04))
                unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                assert not unknown, (name, [hex(i) for i in unknown])
                return isolated[-1]

            def case(kind, name, **changes):
                setup(kind, **changes)
                return call(kind, name)

            def clock():
                nonlocal clock_only, clock_done
                clock_only, clock_done = True, False
                prepare(0x19c96)
                hook(observer)
                execute(100000)
                hook(None)
                clock_only = False
                assert clock_done and not errors, errors

            for index, completed, facing, y, vx in itertools.product(range(8), [0, 1, 2, 255], [0, 1, 2, 255],
                    [0x013fffff, 0x01405678, 0x01410000], [0x40001, -0x40001, -0x80000000]):
                case(0x11, f'retreat-{index}-{completed}-{facing}-{y}-{vx}', index=index, completed=completed, facing=facing, y=y, vx=vx)
            for index, ticks, pressed, completed in itertools.product([0, 1, 2, 13, 14, 15, 255], [0, 1, 255],
                    [0, 0x10, 0x40, 0x50, 0x20], [0, 1, 2, 3, 128, 255]):
                case(0x2d, f'repeat-{index}-{ticks}-{pressed}-{completed}', index=index, ticks=ticks, pressed=pressed, completed=completed)
            for completed, index, contact, pose, ticks in itertools.product([0, 1, 2, 255], [0, 1, 2, 255],
                    [0, 1, 4, 12], [0xa4, 0xa6, 0, 255], [0, 255]):
                case(0x2e, f'combination-{completed}-{index}-{contact}-{pose}-{ticks}', completed=completed, index=index, contact=contact, pose=pose, ticks=ticks)
            for kind, facing, variant, vx, position, identity in itertools.product([0x2f, 0x30, 0x32], [0, 1, 2, 255],
                    [0, 1, 2, 3, 255], [-0x80000000, -0x28001, 0x7fffffff], [0x0064abcd, 0x7fffffff, -0x80000000], [0, 15, 255]):
                case(kind, f'launch-{kind}-{facing}-{variant}-{vx}-{position}-{identity}', facing=facing, gravity=variant << 24 | 0x551234,
                     vx=vx, vy=-vx if vx != -0x80000000 else vx, x=position, y=position, team=identity, target_bit=identity,
                     attrs={0: 0x4321, 15: 0x9ff0, 255: 0xfff0}[identity], layer={0: 0, 15: 3, 255: 255}[identity],
                     child_gravity=0x83123456, child_owner=0x97ab)
            for kind, completed, index, ticks, size in itertools.product([0x2f, 0x30, 0x32], [0, 1, 2, 255], [0, 1, 255], [0, 1, 255], [0, 1, 256, 65535]):
                case(kind, f'launch-gate-{kind}-{completed}-{index}-{ticks}-{size}', completed=completed, index=index, ticks=ticks, child_size=size)
            for kind, facing, extra, gravity in itertools.product([0x2f, 0x30, 0x32], [0, 1, 2, 255], [0, 1, 255], [0, 0x7fffffff]):
                case(kind, f'complete-{kind}-{facing}-{extra}-{gravity}', completed=1, facing=facing, extra_jump=extra, gravity=gravity)
            for counter, variant, facing, size in itertools.product([-32768, -1, 0, 1, 29, 30, 31, 32, 32766, 32767, 65535],
                    [0, 1, 2, 3, 128, 255], [0, 1, 2, 255], [0, 1, 256, 65535]):
                case(0x31, f'charge-{counter}-{variant}-{facing}-{size}', alt=(counter & 65535) << 16 | 0xabcd,
                     gravity=variant << 24 | 0x551234, facing=facing, charge_size=size, charge_vx=0x7654ffff, charge_vy=0x1234abcd)
            boundary_count = len(isolated)
            configs = []
            for facing, y in itertools.product([0, 1], [0x01405678, 0x01305678]):
                configs.append((0x11, f'retreat-{facing}-{y}', {'facing': facing, 'y': y}, 0 if y >> 16 == 320 else 2))
            for pressed in [0, 0x40]:
                configs.append((0x2d, f'repeat-{pressed}', {}, 0))
            for pose, contact in itertools.product([0xa4, 0xa6], [0, 4]):
                configs.append((0x2e, f'combination-{pose}-{contact}', {'pose': pose, 'contact': contact}, 3 if contact else 0))
            for kind, variant, facing, occupied in itertools.product([0x2f, 0x31, 0x32], [0, 1, 2], [0, 1], [0, 256]):
                configs.append((kind, f'projectile-{kind}-{variant}-{facing}-{occupied}',
                    {'gravity': variant << 24 | 0x551234, 'facing': facing, 'child_size': occupied,
                     'charge_size': occupied, 'alt': 0x1234, 'pose': 0xa3 if kind == 0x31 else 0xa0 + variant}, 0))
            for facing in [0, 1]:
                configs.append((0x30, f'air-projectile-{facing}', {'facing': facing, 'extra_jump': 255}, 2))
            for kind, name, changes, terminal in configs:
                setup(kind, **changes)
                write(0xff845b, 255)
                add_record()
                initial, steps = snapshot(ref.ram_cpu(), 0xff8400), []
                for index in range(1200):
                    pressed = 0x40 if kind == 0x2d and name == 'repeat-64' and index == 5 else 0
                    write(0xff8471, 0)
                    write(0xff8472, pressed)
                    clock()
                    active = ref.ram_cpu()[0x845d]
                    assert active in ENTRIES.values(), (name, active)
                    row = call(active, f'stream-{name}-{index}')
                    row['advance_clock'], row['external_input'] = True, {'held': 0, 'pressed': pressed}
                    row['record_after'] = add_record()
                    row['after_record'] = snapshot(ref.ram_cpu(), 0xff8400)
                    steps.append(row)
                    if row['after_record']['body']['action_kind'] not in ENTRIES.values():
                        break
                else:
                    raise AssertionError(('source5 MAIN did not reach endpoint', name))
                assert row['after_record']['body']['action_kind'] == terminal, (name, terminal, row['after_record']['body'])
                sequences.append({'kind': kind, 'name': name, 'initial': initial, 'steps': steps, 'terminal': terminal})
        identity = ref.identity

    baseline = {'input': str(input_path.relative_to(ROOT)), 'input_sha256': sha(input_path.read_bytes()),
                'reference': str(reference_path.relative_to(ROOT)), 'reference_raw_sha256': sha(control_raw),
                'reference_manifest': str(manifest_path.relative_to(ROOT)), 'reference_manifest_sha256': sha(manifest_path.read_bytes()),
                'video_steps': len(expected), 'entry_frame_limit': entry_limit, 'all_ram_video_match': True}
    outputs = [('fighter5-motion-oracle', natural, 'Actual source5 MAIN entries under independently controlled real-menu physical inputs')]
    parameter_raw = None
    if not args.natural_only:
        raw_record = rom[binding['parameter_pointer']:binding['parameter_pointer'] + 16]
        parameter_raw = (json.dumps({'rom_sha256': sha(rom), 'records': [{'source': binding['parameter_pointer'], 'raw_bytes': raw_record.hex(),
            'data': {key: n(raw_record, off, 4, True) for key, off in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}}]}, indent=2) + '\n').encode()
        (out / 'fighter5-motion-parameters.json').write_bytes(parameter_raw)
        outputs += [('fighter5-motion-boundaries', isolated[:boundary_count], 'Explicit source5 MAIN private contexts; complete 64KiB write audit'),
                    ('fighter5-motion-sequences', sequences, 'Single-initial-state CPU-clock and immutable original animation timing streams')]
    for name, rows, scope in outputs:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':')) + '\n').encode()
        packed = gzip.compress(raw, mtime=0)
        (out / (name + '.json.gz')).write_bytes(packed)
        calls = [row for stream in rows for row in stream['steps']] if name.endswith('sequences') else rows
        report = {'pass': True, 'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(rows), 'calls': len(calls),
                  'kind_counts': dict(Counter(str(row['kind']) for row in calls)),
                  'character_counts': dict(Counter(str(row['before']['character_id']) for row in calls)),
                  'allocation_attempts': sum(len(row['allocations']) for row in calls),
                  'external_queue_calls': sum(bool(row['external_queue_clears']) for row in calls),
                  'interrupt_calls': sum(len(row['interrupts']) for row in calls),
                  'external_change_calls': sum(bool(row['external_changes']) for row in calls),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'baselines': [baseline],
                  'frames_match_original_core': frames == expected, 'rng_calls': sum(row['rng_calls'] for row in calls),
                  'unexplained_change_calls': sum(bool(row['unexplained_changes']) for row in calls),
                  'private_unknown_ram_writes': 0, 'errors': errors, 'incomplete_calls': 0,
                  'retreat_table': {'address': '0x1cb20', 'bytes': rom[0x1cb20:0x1cb30].hex(), 'sha256': sha(rom[0x1cb20:0x1cb30])},
                  'limits': ['MAIN stops before15afc; no shared tail, child/FX callbacks or world scheduling.',
                             'Only Duration/Flags are refreshed from actual immutable animation records; no intermediate state injection.']}
        if parameter_raw is not None:
            report.update(binding=binding, movement_parameters_sha256=sha(parameter_raw), library_frames=len(library))
        (out / (name + '-manifest.json')).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': True}), flush=True)


if __name__ == '__main__':
    main()
