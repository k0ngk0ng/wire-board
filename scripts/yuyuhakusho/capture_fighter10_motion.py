#!/usr/bin/env python3
"""Original CPU source-10 continuation evidence, not a runtime dependency."""
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

ENTRIES = {0x1cb08: 0x11, 0x1b67a: 0x2d, 0x1c85e: 0x47, 0x1c8e0: 0x48, 0x1c9aa: 0x49}


def number(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off + size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--inputs', type=Path, default=ROOT / 'docs/research/md-yuyuhakusho/inputs.json')
    parser.add_argument('--end', type=int, default=3600)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and args.end >= 2400
    out.mkdir(parents=True, exist_ok=True)
    inputs_path = args.inputs.resolve()
    input_raw = inputs_path.read_bytes()
    events = json.loads(input_raw)
    (out / 'fighter10-motion-inputs.json').write_bytes(input_raw)
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = list(ENTRIES) + [0x15afc, 0x1fae6, 0x1faee, 0x19cc2]
    pcs = (C.c_uint * len(points))(*points)
    with ReferenceSession(ROOT, out / 'unobserved') as ref:
        expected = [ref.step(frame, buttons_at(events, frame))['reference'] for frame in range(1, args.end + 1)]
        unobserved_identity = ref.identity
    natural, isolated, sequences, errors = [], [], [], []
    pending, private, clock_only, clock_done, label = {}, False, False, False, ''

    def snapshot(ram, address):
        off = address & 65535
        obj = ram[off:off + 256]
        pointer = number(obj, 0x90, 4) & 0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer & 65535)
        return {'object_id': off, 'character_id': obj[0x58], 'body': body_state(obj), 'actor_bytes': obj.hex(),
                'controls': {'held': obj[0x71] | obj[0xc0] << 8, 'pressed': obj[0x72] | obj[0xc1] << 8,
                             'changed': obj[0x73] | obj[0xc2] << 8, 'raw_held': obj[0x74], 'retain_ticks': obj[0x75]},
                'aux_48': number(obj, 0x48, 4, True), 'contact_flags': obj[0x89],
                'children': ram[off + 256:off + 1024].hex(),
                'pool': {'cursor': number(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'parameters': {name: number(memory, index + delta, 4, True) for name, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]}}

    with ReferenceSession(ROOT, out, core=core) as ref:
        @callback_type
        def observe(kind, width, pc, value):
            nonlocal clock_done
            try:
                if clock_only:
                    if pc == 0x19cc2:
                        clock_done = True
                        ref.cpu.cycles = 8000000
                    return
                address = ref.get_register(13) & 0xffffff
                ram = ref.ram_cpu()
                if pc in ENTRIES:
                    assert address not in pending, 'overlapping continuation'
                    pending[address] = {'kind': ENTRIES[pc], 'frame': ref.frame, 'before': snapshot(ram, address),
                                        'entry_sp': ref.get_register(15), 'allocations': []}
                elif address in pending:
                    row = pending[address]
                    if pc in [0x1fae6, 0x1faee]:
                        row['allocations'].append(slot_index(ref.get_register(8)))
                    elif pc == 0x15afc:
                        assert ref.get_register(15) == row['entry_sp'], 'continuation did not restore complete return SP'
                        row = pending.pop(address)
                        row.update(after=snapshot(ram, address), exit='0x15afc', exit_frame=ref.frame)
                        if private:
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
        for frame in range(1, args.end + 1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not errors and not pending, (errors, pending)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value & ((1 << (8 * size)) - 1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte

        def fill(address, size, seed):
            for i in range(size):
                write(address + i, (i * 73 + seed * 29 + 17) & 255)

        def setup(kind, pool='empty', cursor=0, **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            fill(0xff8400, 1024, kind)
            fill(0xff9400, 2048, kind + 1)
            for i in range(16):
                write(0xff9404 + i * 128, 0 if pool == 'empty' or pool == 'single' and i == 1 else 128, 2)
            fields = {0x58: (10, 1), 0x5d: (kind, 1), 0x5f: (0xa0, 1), 0x59: (1, 1), 0x5a: (8, 1), 0x5b: (0, 1),
                      0x5c: (0, 1), 0x60: (0, 1), 0x61: (0, 1), 0x6b: (0, 1), 0x71: (0, 1), 0x72: (0, 1), 0x73: (0, 1),
                      0xc0: (0xa1, 1), 0xc1: (0xb2, 1), 0xc2: (0xc3, 1), 0x90: (0xff7000, 4),
                      0x40: (0x0064abcd, 4), 0x44: (0x012c5678, 4), 0x9c: (0x01401234, 4),
                      0x10: (0x40001, 4), 0x14: (0x38000, 4), 0x48: (0x03123456, 4), 0x4c: (0x3800, 4), 0x89: (0, 1)}
            aliases = {'pose': 0x5f, 'index': 0x59, 'duration': 0x5a, 'ticks': 0x5b, 'flags': 0x5c, 'completed': 0x60,
                       'facing': 0x61, 'pressed': 0x72, 'x': 0x40, 'y': 0x44, 'ground': 0x9c,
                       'vx': 0x10, 'vy': 0x14, 'gravity': 0x4c, 'aux': 0x48, 'contact': 0x89}
            for key, value in changes.items():
                offset = aliases[key]
                fields[offset] = (value, fields[offset][1])
            for offset, (value, size) in fields.items():
                write(0xff8400 + offset, value, size)
            for offset, value in [(4, -0x58001), (8, 0x3600), (12, 0x1c000)]:
                write(0xff7000 + offset, value, 4)
            write(0xffa036, cursor, 2)

        def prepare_cpu(entry):
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
            prepare_cpu(next(pc for pc, slot in ENTRIES.items() if slot == kind))
            before, count = ref.ram_cpu(), len(isolated)
            hook(observer)
            execute(6000000)
            hook(None)
            assert len(isolated) == count + 1 and not pending and not errors, (name, errors)
            after = ref.ram_cpu()
            allowed = set(range(0x8400, 0x8800)) | set(range(0x9400, 0x9c00)) | {0xa036, 0xa037} | set(range(0xfec0, 0xff04))
            unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unknown, (name, unknown)
            return isolated[-1]

        def case(kind, name, pool='empty', cursor=0, **changes):
            setup(kind, pool, cursor, **changes)
            return call(kind, name)

        def tick_clock():
            nonlocal clock_only, clock_done
            clock_only, clock_done = True, False
            prepare_cpu(0x19c96)
            hook(observer)
            execute(100000)
            hook(None)
            clock_only = False
            assert clock_done and not errors, errors

        for index, completed, facing, y, vx in itertools.product(range(8), [0, 1, 2, 255], [0, 1, 2, 255],
                                                               [0x013fffff, 0x01405678, 0x01410000], [0x40001, -0x40001, -0x80000000]):
            case(0x11, f'retreat-{index}-{completed}-{facing}-{y}-{vx}', index=index, completed=completed, facing=facing, y=y, vx=vx)
        for index, ticks, pressed, completed in itertools.product([0, 1, 2, 13, 14, 15, 255], [0, 1, 255], [0, 0x10, 0x40, 0x50, 0x20], [0, 1, 2, 3, 128, 255]):
            case(0x2d, f'repeat-{index}-{ticks}-{pressed}-{completed}', index=index, ticks=ticks, pressed=pressed, completed=completed)
        for completed, index, ticks, facing, contact, pool in itertools.product([0, 1, 2, 255], [0, 1, 2, 255], [0, 1, 3, 4, 255], [0, 1, 2, 255], [0, 1, 4, 5], ['empty', 'full', 'single']):
            case(0x47, f'dash-{completed}-{index}-{ticks}-{facing}-{contact}-{pool}', pool, completed=completed, index=index, ticks=ticks, facing=facing, contact=contact,
                 y=0x01405678 if index & 1 else 0x012c5678, vx=-0x40001 if facing == 2 else 0x40001)
        for index, ticks, facing, y, vy, pool in itertools.product([0, 1, 255], [0, 1, 4], [0, 1, 2, 255], [255, 256, 319, 320, 321], [-0x38000, 0, 0x38000], ['empty', 'full', 'single']):
            case(0x48, f'flight-{index}-{ticks}-{facing}-{y}-{vy}-{pool}', pool, index=index, ticks=ticks, facing=facing, y=(y << 16) | 0xabcd, vy=vy, contact=5)
        for completed, index, counter, facing, pool in itertools.product([0, 1, 2, 255], [0, 3, 4, 27, 28, 29, 30, 31, 255], [0, 3, 255], [0, 1, 2, 255], ['empty', 'full', 'single']):
            case(0x49, f'ultimate-{completed}-{index}-{counter}-{facing}-{pool}', pool, completed=completed, index=index, aux=(counter << 24) | 0x123456, facing=facing)
        for kind, facing, x, cursor in itertools.product([0x47, 0x48, 0x49], [0, 1, 2, 255], [0x7fffffff, -0x80000000], [0x780, 0xff80]):
            case(kind, f'wrap-{kind:x}-{facing}-{x}-{cursor}', 'single', cursor, index=4, ticks=0, facing=facing, x=x, aux=0xff123456,
                 y=0x7fffffff, ground=-0x80000000, vy=0x40001)
        boundary_count = len(isolated)
        configs = [
            (0x11, 'normal-seven-frames', {'flags': 0, 'duration': 2, 'index': 0, 'ticks': 255}, 'frame-six'),
            (0x11, 'completed-recovery', {'flags': 128, 'duration': 3, 'index': 3, 'ticks': 0, 'y': 0x01405678}, 'action-transition'),
            (0x2d, 'repeat-to-frame14', {'flags': 0, 'duration': 2, 'index': 0, 'ticks': 255}, 'frame-fourteen'),
            (0x2d, 'completed-recovery', {'flags': 128, 'duration': 3, 'index': 2, 'ticks': 0}, 'action-transition'),
            (0x47, 'ground-dash', {'flags': 128, 'duration': 5, 'index': 0, 'ticks': 0, 'y': 0x01405678}, 'action-transition'),
            (0x47, 'air-contact-dash', {'flags': 128, 'duration': 5, 'index': 0, 'ticks': 0, 'contact': 1}, 'action-transition'),
            (0x48, 'land', {'flags': 0, 'duration': 2, 'index': 1, 'ticks': 0, 'y': 0x01345678}, 'action-transition'),
            (0x49, 'counter-and-decay', {'flags': 0, 'duration': 1, 'index': 3, 'ticks': 0, 'aux': 0xff123456}, 'frame-thirty-one'),
            (0x49, 'completed-recovery', {'flags': 128, 'duration': 4, 'index': 27, 'ticks': 0, 'aux': 0x03123456}, 'action-transition'),
        ]
        for (initial_kind, name, options, milestone), pool in itertools.product(configs, ['empty', 'full', 'single']):
            setup(initial_kind, pool, **options)
            initial, steps = snapshot(ref.ram_cpu(), 0xff8400), []
            for index in range(256):
                external = {}
                if name == 'repeat-to-frame14':
                    pressed = 0x40 if index == 5 else 0
                    write(0xff8472, pressed)
                    external = {'pressed': pressed}
                tick_clock()
                kind = ref.ram_cpu()[0x845d]
                assert kind in ENTRIES.values()
                row = call(kind, f'sequence-{name}-{pool}-{index}')
                row['advance_clock'], row['external_input'] = True, external
                steps.append(row)
                after = row['after']['body']
                if milestone == 'action-transition' and after['action_kind'] != initial_kind:
                    break
                if milestone == 'frame-six' and after['clock']['frame_index'] == 6:
                    break
                if milestone == 'frame-fourteen' and after['clock']['frame_index'] == 14:
                    break
                if milestone == 'frame-thirty-one' and after['clock']['frame_index'] == 31:
                    break
            else:
                raise AssertionError(('no declared trajectory milestone', name, pool))
            sequences.append({'kind': initial_kind, 'name': name, 'pool': pool, 'initial': initial, 'steps': steps, 'milestone': milestone})
        identity = ref.identity
    for name, records, scope in [
        ('fighter10-motion-oracle', natural, 'Actual natural target-slot calls; count may be zero if declared inputs select another source'),
        ('fighter10-motion-boundaries', isolated[:boundary_count], 'Explicit source10 private phase inputs, full parent/child/pool writes'),
        ('fighter10-motion-sequences', sequences, 'Single-initial-state isolated original-clock trajectories with declared milestones'),
    ]:
        raw = (json.dumps(records, separators=(',', ':')) + '\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out / (name + '.json.gz')).write_bytes(zipped)
        calls = [step for sequence in records for step in sequence['steps']] if name.endswith('sequences') else records
        report = {'scope': scope, 'identity': identity, 'unobserved_identity': unobserved_identity,
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'inputs_sha256': sha(input_raw), 'video_steps': args.end, 'records': len(records), 'calls': len(calls),
                  'kind_counts': dict(Counter(str(row['kind']) for row in calls)),
                  'character_counts': dict(Counter(str(row['before']['character_id']) for row in calls)),
                  'allocation_attempts': sum(len(row['allocations']) for row in calls),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors,
                  'retreat_table': {'address': '0x1cb20', 'bytes': rom[0x1cb20:0x1cb30].hex(), 'sha256': sha(rom[0x1cb20:0x1cb30])},
                  'pass': not errors and (bool(records) or name.endswith('oracle')),
                  'limits': ['Motion stops before shared action tail; no animation record refresh or FX lifecycle.',
                             'Private timing trajectories are not natural full-attack or match evidence.',
                             'Retreat frame7 is a CPU-verified structural entry; source10 normal pose has only frames0..6.']}
        (out / (name + '-manifest.json')).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
