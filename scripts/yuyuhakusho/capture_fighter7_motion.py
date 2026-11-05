#!/usr/bin/env python3
"""Pinned source7 MAIN evidence; independent rules never load this collector."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path

from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data
from capture_effect_pool import slot_index
from capture_ground_idle import actor as body_state

ENTRIES = {0x1ca9c: '0f', 0x1cb08: '11', 0x1cbc8: '13', 0x1cc28: '14',
           0x1bd6e: '39', 0x1be08: '3a', 0x1be64: '3b', 0x1bfce: '3c',
           0x1c032: '3d', 0x1c6e2: '45'}
POSES = {'0f': 11, '11': 12, '13': 0x87, '14': 0xb1, '39': 0xa0,
         '3a': 0xa4, '3b': 0xa6, '3c': 0xa7, '3d': 0xaa, '45': 0xae}
PARENTS = (0x8400, 0x8800, 0x8c00, 0x9000)
HOOK_CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
ORIGINAL_CORE_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off + size], 'big', signed=signed)


def changes(before, after):
    assert len(before) == len(after) == 65536
    return [{'offset': i, 'before': a, 'after': b}
            for i, (a, b) in enumerate(zip(before, after)) if a != b]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--inputs', required=True, type=Path)
    parser.add_argument('--reference', required=True, type=Path)
    parser.add_argument('--reference-manifest', required=True, type=Path)
    parser.add_argument('--natural-only', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    input_path = args.inputs.resolve()
    reference_path, control_path = args.reference.resolve(), args.reference_manifest.resolve()
    input_raw, reference_raw = input_path.read_bytes(), reference_path.read_bytes()
    control_raw = control_path.read_bytes()
    control, events, expected = json.loads(control_raw), json.loads(input_raw), json.loads(reference_raw)
    assert control['identity']['core_sha256'] == ORIGINAL_CORE_SHA
    assert control['input']['sha256'] == sha(input_raw) and control['output']['sha256'] == sha(reference_raw)
    assert len(expected) == 3602
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x19c96, 0x19cc2, 0x15afc, 0x1fae6, 0x1faee,
              0x3982, 0x37c56, 0x37cac, 0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a,
              0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint * len(points))(*points)
    natural, isolated, streams, errors, library = [], [], [], [], {}
    pending, private, requested, label = None, False, '', ''

    def snapshot(ram, parent=0x8400):
        obj = ram[parent:parent + 256]
        pointer = n(obj, 0x90, 4) & 0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer & 65535)
        return {'object_id': parent, 'character_id': obj[0x58], 'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71] | obj[0xc0] << 8, 'pressed': obj[0x72] | obj[0xc1] << 8,
                             'changed': obj[0x73] | obj[0xc2] << 8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'energy': n(obj, 0xb8, 4, True), 'power': n(obj, 0x7c, 2, True),
                'alternate_velocity': n(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[parent + 256:parent + 1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'left': n(ram, 0xa010, 2, True),
                'others': {str(p): ram[p:p + 256].hex() for p in PARENTS if p != parent},
                'movement_source': n(obj, 0x90, 4), 'animation_set': n(obj, 0x50, 4), 'shape_set': n(obj, 0x8c, 4),
                'parameters': {key: n(memory, index + delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'random_seed': n(ram, 0xff18, 4), 'pending_sound': ram[0xa030],
                'sound_queue': list(ram[0xf80a:0xf80e]),
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a],
                                'palette_attributes': n(ram, 0xa03e), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True),
                                'shake': n(ram, 0xf566, 2, True)}}

    with ReferenceSession(ROOT, out / 'observed', core=core) as ref:
        assert ref.identity['core_sha256'] == HOOK_CORE_SHA

        @cbtype
        def observe(kind, width, raw_pc, value):
            nonlocal pending
            try:
                pc = raw_pc & 0xffffff
                if pc == 0xfff000:
                    if pending is not None:
                        pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                    return
                if pc in (0xfff09a, 0xfff0a8, 0xfff13a):
                    if pending is not None and pending['_irq']:
                        irq = pending['_irq'].pop()
                        irq.update(exit=hex(pc), changes=changes(irq.pop('_ram'), ref.ram_cpu()))
                        pending['interrupts'].append(irq)
                        pending['_external_events'].append({'kind': 'irq', 'changes': irq['changes']})
                    return
                if pc == 0x3982:
                    if pending is not None:
                        pending['rng_calls'] += 1
                    return
                if pc in (0x37c56, 0x37cac):
                    if pending is not None:
                        address = ref.get_register(9) & 65535
                        event = {'pc': hex(pc), 'address': address, 'before': ref.ram_cpu()[address], 'after': 0}
                        pending['external_queue_clears'].append(event)
                        pending['_external_events'].append({'kind': 'audio', 'changes': [{'offset': address, 'before': event['before'], 'after': 0}]})
                    return
                parent = ref.get_register(13) & 65535
                ram, sp = ref.ram_cpu(), ref.get_register(15)
                role = 'clock' if private and requested == 'clock' and pc == 0x19c96 else ENTRIES.get(pc)
                if role and parent in PARENTS and ram[parent + 0x58] == 7 and (private or ref.frame <= 3600):
                    assert pending is None, 'overlapping source7 MAIN'
                    assert role == 'clock' or ram[parent + 0x5d] == int(role, 16)
                    pending = {'kind': role, 'frame': ref.frame, 'object_id': parent, 'entry': hex(pc), 'entry_sp': sp,
                               'source_register': hex(ref.get_register(13)), 'before': snapshot(ram, parent), 'allocations': [],
                               'rng_calls': 0, 'interrupts': [], 'external_queue_clears': [], '_irq': [], '_external_events': [], '_ram': ram}
                elif pending is not None:
                    if pc in (0x1fae6, 0x1faee):
                        pending['allocations'].append(slot_index(ref.get_register(8)))
                    done = pc == 0x19cc2 if pending['kind'] == 'clock' else pc == 0x15afc
                    if done:
                        row, pending = pending, None
                        assert sp == row['entry_sp'] and parent == row['object_id'] and not row.pop('_irq')
                        row.update(after=snapshot(ram, parent), exit=hex(pc), exit_frame=ref.frame, return_sp=sp)
                        row['ram_changes'] = changes(row.pop('_ram'), ram)
                        owned = (set(range(parent, parent + 1024)) | set(range(0x9400, 0x9c00)) |
                                 {0xa036, 0xa037} | {(sp - i) & 65535 for i in range(0x180)})
                        if row['kind'] == '3d':
                            owned |= {p + 0x69 for p in PARENTS if p != parent}
                        row['external_changes'] = [delta for delta in row['ram_changes'] if delta['offset'] not in owned]
                        row['unexplained_changes'] = []
                        external_events = row.pop('_external_events')
                        for delta in row['external_changes']:
                            current, count = delta['before'], 0
                            for event in external_events:
                                for change in event['changes']:
                                    if change['offset'] == delta['offset']:
                                        assert current == change['before'], ('broken external event chain', delta, event)
                                        current, count = change['after'], count + 1
                            if count == 0 or current != delta['after']:
                                row['unexplained_changes'].append(delta)
                        assert not row['unexplained_changes'], row['unexplained_changes'][:8]
                        assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                        if private:
                            assert not row['external_changes'] and not row['external_queue_clears'] and not row['interrupts']
                            row['name'] = label
                            isolated.append(row)
                            ref.cpu.cycles = 8000000
                        else:
                            natural.append(row)
            except Exception as exc:
                errors.append(repr(exc))
                if private:
                    ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook, observer = ref.api('set_cpu_hook', None, C.c_void_p), C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, len(expected) + 1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None)
        assert frames == expected and pending is None and not errors, (errors[:8], len(natural))
        assert base_ram[0x8458] == 7
        binding = {'source_tag': 7, 'checkpoint_frame': 2400, 'source_actor_sha256': sha(base_ram[0x8400:0x8500]),
                   'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4), 'movement_source': n(base_ram, 0x8490, 4)}
        identity = ref.identity
        if not args.natural_only:
            private = True
            setreg, execute = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint), ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte

            def write(address, value, size=1):
                write_bytes(address, (value & ((1 << (size * 8)) - 1)).to_bytes(size, 'big'))

            def setup(role, seed=0, **options):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, bytes((i * 73 + seed * 29 + 17) & 255 for i in range(4096)))
                write_bytes(0xff8450, base_ram[0x8450:0x8454])
                write_bytes(0xff848c, base_ram[0x848c:0x8494])
                write_bytes(0xff9400, bytes((i * 37 + seed * 23 + 13) & 255 for i in range(2048)))
                v = {'index': 0, 'duration': 8, 'ticks': 0, 'flags': 0, 'completed': 0, 'facing': 0,
                     'pose': POSES[role], 'x': 100 << 16 | 0xabcd, 'y': 300 << 16 | 0x5678,
                     'ground': 320 << 16 | 0x1234, 'vx': 0x40001, 'vy': -0x20000, 'gravity': 0x3800,
                     'aux': 0x00123456, 'energy': 220 << 16 | 0xabcd, 'contact': 0, 'held': 0,
                     'pressed': 0, 'changed': 0, 'team': 2, 'base_team': 1, 'target': 255,
                     'attributes': 0xfff0, 'layer': 2, 'children': 0, 'pool': 'empty', 'cursor': 0, 'left': -17,
                     'peer_teams': (1, 2, 255), 'peer_sizes': (0, 0x100, 0x100), 'peer_health': (-1, 0, 111)}
                v.update(options)
                fields = {0: (0x12668, 4), 4: (256, 2), 6: (2, 2), 0x58: (7, 1), 0x5d: (int(role, 16), 1),
                          0x5f: (v['pose'], 1), 0x59: (v['index'], 1), 0x5a: (v['duration'], 1), 0x5b: (v['ticks'], 1),
                          0x5c: (v['flags'], 1), 0x60: (v['completed'], 1), 0x61: (v['facing'], 1), 0x6b: (0x91, 1),
                          0x6c: (0x5a, 1), 0x6e: (v['team'], 1), 0x6f: (v['base_team'], 1), 0x70: (v['target'], 1),
                          0x71: (v['held'], 1), 0x72: (v['pressed'], 1), 0x73: (v['changed'], 1),
                          0xc0: (0xa1, 1), 0xc1: (0xb2, 1), 0xc2: (0xc3, 1), 0x78: (100, 2),
                          0x40: (v['x'], 4), 0x44: (v['y'], 4), 0x9c: (v['ground'], 4), 0x10: (v['vx'], 4),
                          0x14: (v['vy'], 4), 0x4c: (v['gravity'], 4), 0x48: (v['aux'], 4), 0xb8: (v['energy'], 4),
                          0x89: (v['contact'], 1), 0x24: (v['attributes'], 2), 0x26: (v['layer'], 1), 0x87: (255, 1)}
                for off, (value, count) in fields.items():
                    write(0xff8400 + off, value, count)
                for i in range(3):
                    write(0xff8504 + i * 256, 0x100 if v['children'] & (1 << i) else 0, 2)
                for i, parent in enumerate(PARENTS[1:]):
                    write(0xff0000 + parent + 4, v['peer_sizes'][i], 2)
                    write(0xff0000 + parent + 0x6e, v['peer_teams'][i])
                    write(0xff0000 + parent + 0x78, v['peer_health'][i], 2)
                    write(0xff0000 + parent + 0x69, [0, 1, 254][i])
                for i in range(16):
                    write(0xff9404 + i * 128, 0 if v['pool'] == 'empty' or v['pool'] == 'single' and i == 1 else 128, 2)
                write(0xffa036, v['cursor'], 2)
                write(0xffa010, v['left'], 2)
                write(0xffa030, 0x97)
                write(0xffff18, 0x12345678, 4)
                write_bytes(0xfff80a, bytes([0, 0x80, 0, 0x81]))

            def run(role, name):
                nonlocal requested, label
                requested, label = role, name
                for reg in range(16):
                    setreg(reg, 0)
                setreg(17, 0x2700)
                setreg(15, 0xffffff00)
                write(0xffff00, 0x3d8, 4)
                setreg(13, 0xffff8400)
                setreg(16, 0x19c96 if role == 'clock' else next(pc for pc, value in ENTRIES.items() if value == role))
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                count = len(isolated)
                hook(observer)
                execute(8000000)
                hook(None)
                assert len(isolated) == count + 1 and pending is None and not errors, (name, errors[:8])
                return isolated[-1]

            def case(role, name, **options):
                setup(role, len(isolated), **options)
                return run(role, name)

            def record(name):
                before, ram = snapshot(ref.ram_cpu()), ref.ram_cpu()
                key, data, origin = selected_data(rom, ram, ram[0x8400:0x8500])
                identity_key = json.dumps(key, sort_keys=True)
                assert identity_key not in library or library[identity_key]['data'] == data
                library[identity_key] = {'key': key, 'data': data, 'source': origin}
                write(0xff845a, data['duration'])
                write(0xff845c, data['flags'])
                return {'kind': 'record', 'name': name, 'before': before, 'after': snapshot(ref.ram_cpu()), 'record_key': key}

            for completed, index, facing, y in itertools.product([0, 1, 2, 255], range(8), [0, 1, 2, 255], [300 << 16, 320 << 16 | 0x5678]):
                case('11', f'11-{completed}-{index}-{facing}-{y}', completed=completed, index=index, facing=facing, y=y)
            for role, completed, index, ticks, facing, pool in itertools.product(['0f', '13', '14'], [0, 1, 2], [0, 1, 2, 255], [0, 1, 255], [0, 1, 2, 255], ['empty', 'full']):
                case(role, f'{role}-{completed}-{index}-{ticks}-{facing}-{pool}', completed=completed, index=index, ticks=ticks, facing=facing, pool=pool, flags=4 if index & 1 else 0, y=320 << 16 | 0x5678)
            for index, ticks, facing, contact, y in itertools.product([0, 1, 255], [0, 1, 4], [0, 1, 2, 255], [0, 1, 4, 8, 12], [255, 256, 319, 320, 321]):
                case('45', f'45-{index}-{ticks}-{facing}-{contact}-{y}', index=index, ticks=ticks, facing=facing, contact=contact, y=y << 16 | 0xabcd, vy=0x40001)
            for facing, index, contact, aux, y, vy in itertools.product([0, 1, 2, 255], [0, 1, 255], [0, 1, 4, 8, 13], [0x00123456, 0x01123456], [300 << 16 | 0xffff, 320 << 16 | 0xffff, 321 << 16], [-0x40000, 0, 0x40000]):
                case('39', f'39-{facing}-{index}-{contact}-{aux}-{y}-{vy}', facing=facing, index=index, contact=contact, aux=aux, y=y, vy=vy, completed=2)
            for completed, variant, left in itertools.product([0, 1, 2, 255], [0, 1, 2, 3, 255], [-32768, -17, 0, 32767]):
                case('3a', f'3a-{completed}-{variant}-{left}', completed=completed, aux=variant << 24 | 0x123456, left=left)
            for completed, children, facing, target, attributes, layer in itertools.product([0, 1, 2], range(8), [0, 1, 2, 255], [0, 255], [0, 0x9ff0, 0xfff0], [0, 255]):
                case('3b', f'3b-{completed}-{children}-{facing}-{target}-{attributes}-{layer}', completed=completed, children=children, facing=facing, target=target, attributes=attributes, layer=layer, vx=0x0030abcd)
            for energy, children, held, pressed in itertools.product([-0x80000000, -1, 0, 0x3fff, 0x4000, 0x4001, 0x7fff, 0xffff, 0x10000, 0x7fffffff], range(8), [0, 0x91, 255], [0, 0x20, 0x50, 255]):
                case('3c', f'3c-{energy}-{children}-{held}-{pressed}', energy=energy, children=children, held=held, pressed=pressed)
            for completed, index, ticks, children, team, peers in itertools.product([0, 1, 2, 255], [0, 1, 2, 3, 255], [0, 1, 255], [0, 4], [0, 1, 255], [(0, 0, 0), (1, 1, 1), (0, 1, 255)]):
                case('3d', f'3d-{completed}-{index}-{ticks}-{children}-{team}-{peers}', completed=completed, index=index, ticks=ticks, children=children, team=team, peer_teams=peers)
            for role, x, y, vx, vy, gravity in itertools.product(['39'], [-0x80000000, 0x7fffffff], [-0x80000000, 0x7fffffff], [-0x80000000, 0x7fffffff], [-0x80000000, 0x7fffffff], [-0x80000000, 0x7fffffff]):
                case(role, f'wrap-{x}-{y}-{vx}-{vy}-{gravity}', x=x, y=y, vx=vx, vy=vy, gravity=gravity)
            for role, cursor, pool in itertools.product(['0f', '13', '14', '45'], [0x780, 0xff80], ['empty', 'full', 'single']):
                case(role, f'pool-{role}-{cursor}-{pool}', index=2, ticks=0, cursor=cursor, pool=pool, y=300 << 16, vy=0x30000)
            boundary_count = len(isolated)
            configs = [
                ('0f', 'dash', {'y': 320 << 16 | 0x5678}),
                ('11', 'retreat', {'y': 320 << 16 | 0x5678}),
                ('13', 'slide', {'y': 320 << 16 | 0x5678}),
                ('14', 'landing', {'y': 320 << 16 | 0x5678}),
                ('39', 'normal-land', {'vx': 0x40000, 'vy': -0x20000, 'aux': 0x00123456}),
                ('39', 'contact-land', {'vx': -0x40000, 'vy': -0x30000, 'contact': 4, 'aux': 0x00123456}),
                ('39', 'advance-contact0', {'vx': 0x40000, 'vy': 0x10000, 'contact': 1, 'aux': 0x00123456}),
                ('3a', 'warp-left', {'aux': 0x00123456}),
                ('3a', 'warp-center', {'aux': 0x01123456}),
                ('3a', 'warp-right', {'aux': 0x02123456}),
                ('3b', 'dual-control-exit', {'vx': 0x0030abcd, 'energy': 0x0020abcd}),
                ('3c', 'first-slot-empty', {'children': 0, 'energy': 0x0020abcd}),
                ('3c', 'energy-zero-exit', {'children': 3, 'energy': 0x00018001}),
                ('3d', 'all-parents-stop', {}),
                ('3d', 'busy-pan-stop', {'children': 4}),
                ('45', 'flight-land', {'vy': 0x40000}),
                ('45', 'collision-bounce', {'vy': 0x30000, 'contact': 4, 'pose': 0xaf}),
            ]
            for stream, ((initial_role, name, options), pool) in enumerate(itertools.product(configs, ['empty', 'full'])):
                setup(initial_role, stream, pool=pool, **options)
                write(0xff845b, 255)
                timing = record('initial-record')
                initial, calls, external = timing['after'], [], []
                control_ticks = 0
                for step in range(512):
                    role = f'{ref.ram_cpu()[0x845d]:02x}'
                    if role == '3c' and name == 'dual-control-exit':
                        control_ticks += 1
                        pressed = 0x20 if control_ticks >= 5 else 0x40
                        held = 0x91 if control_ticks & 1 else 0x82
                        before_input = snapshot(ref.ram_cpu())
                        write(0xff8471, held)
                        write(0xff8472, pressed)
                        calls.append({'kind': 'input', 'name': f'{stream}/{step}-input', 'before': before_input,
                                      'after': snapshot(ref.ram_cpu()), 'external_input': {'held': held, 'pressed': pressed}})
                    calls.append(run('clock', f'{stream}/{step}-clock'))
                    calls.append(run(role, f'{stream}/{step}-main'))
                    calls.append(record(f'{stream}/{step}-record'))
                    after_kind = f'{ref.ram_cpu()[0x845d]:02x}'
                    if after_kind not in ENTRIES.values():
                        break
                    if initial_role != '3b' and after_kind != initial_role:
                        break
                else:
                    raise AssertionError(('MAIN trajectory did not finish', initial_role, name, pool))
                streams.append({'name': f'{initial_role}-{name}-{pool}-{stream}', 'initial_kind': initial_role,
                                'initial_record': timing, 'initial': initial, 'calls': calls,
                                'milestone': 'action-transition', 'final_kind': after_kind})

    baseline = {'input': str(input_path.relative_to(ROOT)), 'input_sha256': sha(input_raw),
                'reference': str(reference_path.relative_to(ROOT)), 'reference_raw_sha256': sha(reference_raw),
                'reference_manifest': str(control_path.relative_to(ROOT)), 'reference_manifest_sha256': sha(control_raw),
                'video_steps': len(expected), 'all_ram_video_match': frames == expected}
    groups = [('fighter7-motion-oracle', natural, 'Natural source7 MAIN before shared tail under the unique original 3602-frame control')]
    if not args.natural_only:
        groups += [('fighter7-motion-boundaries', isolated[:boundary_count], 'Dirty source7 MAIN and shared support CPU boundaries'),
                   ('fighter7-motion-sequences', streams, 'One initial source7 state; original clock plus immutable own-record timing refreshes')]
    for name, rows, scope in groups:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':')) + '\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out / (name + '.json.gz')).write_bytes(zipped)
        calls = [row for stream in rows for row in stream['calls']] if name.endswith('sequences') else rows
        report = {'pass': bool(rows) and not errors, 'scope': scope, 'identity': identity, 'binding': binding,
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'records': len(rows), 'calls': len(calls),
                  'kind_counts': dict(Counter(row['kind'] for row in calls)), 'library_frames': len(library),
                  'frames_match_original_core': frames == expected, 'baselines': [baseline],
                  'incomplete_calls': 0, 'errors': errors,
                  'rng_calls': sum(row.get('rng_calls', 0) for row in calls),
                  'external_queue_calls': sum(bool(row.get('external_queue_clears')) for row in calls),
                  'external_change_calls': sum(bool(row.get('external_changes')) for row in calls),
                  'unexplained_change_calls': sum(bool(row.get('unexplained_changes')) for row in calls),
                  'interrupt_calls': sum(len(row.get('interrupts', [])) for row in calls),
                  'full_ram_delta_calls': sum('ram_changes' in row for row in calls),
                  'limits': ['CPU phase ends at15afc; no shared tail, active child, FX or whole actor simulation.',
                             'Natural records cover only calls selected by the declared real menu input.',
                             'Private single-initial-state trajectories refresh only Duration/Flags from their own immutable animation key.',
                             'External audio clears and IRQ writes retain exact before/after event chains; no history RAM exemption.']}
        (out / (name + '-manifest.json')).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': report['pass']}), flush=True)


if __name__ == '__main__':
    main()
