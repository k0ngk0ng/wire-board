#!/usr/bin/env python3
"""Source3 air22 and reused child callbacks in exact parent/family contexts."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x1afc8: 'air', 0x1313e: 'main', 0x132de: 'charge', 0x134c2: 'release'}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(bridge_path)); cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x1a430, 0x19c96, 0x19cc2, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac, 0x3982,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint*len(points))(*points)
    sources = [('docs/research/md-yuyuhakusho/source3-next-inputs.json', 'fighter3-attacks-reference-frames.json', 'fighter3-attacks-reference-manifest.json'),
               ('games/yuyuhakusho/engine/testdata/fighter3-special-inputs.json', 'fighter3-attacks-special-reference-frames.json', 'fighter3-attacks-special-reference-manifest.json')]
    natural, isolated, streams, baselines, errors = [], [], [], [], []
    library = {}; pending = None; private = False; requested = ''; label = ''; scenario = ''

    def snapshot(ram, parent=0x8400):
        obj = ram[parent:parent+256]; pointer = n(obj, 0x90, 4)&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)],
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    for source_index, (input_name, reference_name, manifest_name) in enumerate(sources):
        input_path = ROOT/input_name; ref_path = ROOT/'games/yuyuhakusho/engine/testdata'/reference_name
        ref_manifest = ROOT/'games/yuyuhakusho/engine/testdata'/manifest_name
        reference_raw = ref_path.read_bytes(); expected = json.loads(reference_raw); events = json.loads(input_path.read_text())
        original_manifest = json.loads(ref_manifest.read_text())
        assert original_manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert original_manifest['output']['sha256'] == sha(reference_raw) and original_manifest['input']['sha256'] == sha(input_path.read_bytes())
        scenario = Path(input_name).name
        with ReferenceSession(ROOT, out/('observed-'+str(source_index)), core=core) as ref:
            def remember_frame(ram, address):
                off = address&65535
                key, data, origin = selected_data(rom, ram, ram[off:off+256]); identity = json.dumps(key, sort_keys=True)
                assert identity not in library or library[identity]['data'] == data
                library[identity] = {'key': key, 'data': data, 'source': origin}
                return key, data

            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending
                try:
                    raw_pc = pc; pc &= 0xffffff
                    if pc == 0xfff000:
                        if pending is not None:
                            pending['_irq_before'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), 'frame': ref.frame, '_ram': ref.ram_cpu()})
                        return
                    if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                        if pending is not None and pending['_irq_before']:
                            irq = pending['_irq_before'].pop(); ram = ref.ram_cpu()
                            irq.update(exit=hex(pc), exit_frame=ref.frame,
                                       changes=[{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                            pending['interrupts'].append(irq)
                        return
                    if pc in [0x37c56, 0x37cac]:
                        if pending is not None:
                            a = ref.get_register(9)&65535
                            pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                        return
                    if pc == 0x3982:
                        if pending is not None: pending['rng_calls'] += 1
                        return
                    address, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                    parent = (address&65535)&0xfc00
                    role = ENTRIES.get(pc)
                    if private and pc == 0x1a430: role = 'charge_motion'
                    if private and requested == 'clock' and pc == 0x19c96: role = 'clock'
                    ram = ref.ram_cpu()
                    if role and ram[parent+0x58] == 3 and (private or ref.frame <= 3600):
                        assert pending is None, 'overlapping source3 family callback'
                        pending = {'kind': role, 'frame': ref.frame, 'scenario': scenario, 'object_id': address&65535,
                                   'before': snapshot(ram, parent), 'entry_sp': sp, 'source_register': hex(ref.get_register(13)),
                                   'external_queue_clears': [], 'rng_calls': 0, 'draw': False, 'cleared_bytes': 0, '_ram': ram,
                                   'interrupts': [], '_irq_before': []}
                    elif pending is not None:
                        if pc == 0x1eb40:
                            remember_frame(ram, address)
                        elif pc == 0x3556:
                            size = n(ram, (address&65535)+4)
                            pending['cleared_bytes'] = (size or 64)//16*16
                            assert 0 < pending['cleared_bytes'] <= 256
                        finish = (pc == 0x15afc and pending['kind'] in ['air', 'charge_motion'] or
                                  pc == 0x19cc2 and pending['kind'] == 'clock' or
                                  pc == 0x1580 and pending['kind'] in ['main', 'charge', 'release'] or
                                  pc == 0x3574 and pending['cleared_bytes'] != 0)
                        if finish:
                            assert sp == pending['entry_sp']
                            row = pending; pending = None
                            assert not row.pop('_irq_before')
                            row.update(after=snapshot(ram, parent), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                            allowed = (set(range(parent, parent+1024)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa600)) |
                                       set(range(0xa004, 0xa00c)) | {0xa030, 0xa036, 0xa037} | set(range(0xf80a, 0xf80e)) |
                                       {(row['entry_sp']-i)&65535 for i in range(0x180)})
                            row['external_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(row.pop('_ram'), ram)) if a != b and i not in allowed]
                            row['unexplained_changes'] = []
                            for change in row['external_changes']:
                                current, steps = change['before'], 0
                                for irq in row['interrupts']:
                                    for delta in irq['changes']:
                                        if delta['offset'] != change['offset']: continue
                                        if delta['before'] != current: raise AssertionError(('IRQ write chain disagrees', change, delta, current))
                                        current, steps = delta['after'], steps+1
                                if steps == 0 or current != change['after']: row['unexplained_changes'].append(change)
                            assert not row['unexplained_changes'], (row['kind'], row['frame'], row['unexplained_changes'])
                            assert row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                            if private:
                                assert not row['external_changes'] and not row['external_queue_clears']
                                row['name'] = label; isolated.append(row); ref.cpu.cycles = 8000000
                            else: natural.append(row)
                except Exception as exc:
                    errors.append(repr(exc))
                    if private: ref.cpu.cycles = 8000000

            assert bridge.yyh_set_observer(observe, pcs, len(points))
            hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
            hook(observer); frames = []
            for frame in range(1, len(expected)+1):
                frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
                if frame == 2400: checkpoint, base_ram = ref.save(), ref.ram_cpu()
            hook(None); assert frames == expected and not errors and pending is None, (scenario, errors[:3], pending is not None)
            baselines.append({'input': input_name, 'input_sha256': sha(input_path.read_bytes()), 'reference': str(ref_path.relative_to(ROOT)),
                              'reference_raw_sha256': sha(reference_raw), 'reference_manifest': str(ref_manifest.relative_to(ROOT)),
                              'reference_manifest_sha256': sha(ref_manifest.read_bytes()), 'video_steps': len(expected), 'all_ram_video_match': True})
            identity = ref.identity
            if source_index == 0: continue
            assert base_ram[0x8458] == 3
            binding = {'source_tag': 3, 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4),
                       'parameter_pointer': n(base_ram, 0x8490, 4), 'source_actor_sha256': sha(base_ram[0x8400:0x8500])}
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))

            def setup(seed=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                values = {'index': 2, 'ticks': 0, 'completed': 0, 'facing': 0, 'power': 127, 'child_power': 7, 'children': 0,
                          'x': 100<<16|0xabcd, 'y': 280<<16|0x5678, 'ground': 320<<16|0x1234, 'vx': 28<<16|0xabcd,
                          'vy': -48<<16|0x5678, 'alt': 0x40000, 'gravity': 0x123456, 'extra_jump': 0,
                          'pool': 'empty', 'cursor': 0, 'pause': 0, 'slow': 0, 'mode': 0x22, 'pose': 0xac,
                          'clear_children': False, 'held': 0, 'pressed': 0, 'parameters': None}
                values.update(changes)
                write_bytes(0xff8400, bytes((i*73+seed*29+17)&255 for i in range(1024)))
                if values['clear_children']: write_bytes(0xff8500, bytes(768))
                write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048)))
                write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                write_bytes(0xff8450, base_ram[0x8450:0x8454]); write_bytes(0xff848c, base_ram[0x848c:0x8494])
                fields = {0: (0x12668, 4), 4: (256, 2), 6: (2, 2), 0x58: (3, 1), 0x59: (values['index'], 1), 0x5a: (8, 1),
                          0x5b: (values['ticks'], 1), 0x5c: (0x10, 1), 0x5d: (values['mode'], 1), 0x5f: (values['pose'], 1),
                          0x60: (values['completed'], 1), 0x61: (values['facing'], 1), 0x40: (values['x'], 4), 0x44: (values['y'], 4),
                          0x9c: (values['ground'], 4), 0x10: (values['vx'], 4), 0x14: (values['vy'], 4), 0x48: (values['alt'], 4),
                          0x4c: (values['gravity'], 4), 0x87: (values['extra_jump'], 1), 0x7c: (values['power'], 2),
                          0xb8: (223<<16|0xabcd, 4), 0x6b: (0, 1), 0x71: (values['held'], 1), 0x72: (values['pressed'], 1),
                          0x73: (0, 1), 0xc0: (0xa1, 1), 0xc1: (0xb2, 1), 0xc2: (0xc3, 1)}
                for off, (value, count) in fields.items(): write(0xff8400+off, value, count)
                for i in range(3): write(0xff8504+i*256, 256 if values['children']&(1<<i) else 0, 2)
                write(0xff857c, values['child_power'], 2)
                for i in range(16): write(0xff9404+i*128, 128 if values['pool'] == 'full' else 0, 2)
                for i in range(4): write(0xffa004+i*2, [0, 240, 256][(i+seed)%3], 2)
                write(0xffa036, values['cursor'], 2); write(0xffa030, 0x97)
                write(0xffa03e, [0, 0x2000, 0xffff][seed%3], 2)
                for off, value in [(0xa010, 0), (0xa012, 600), (0xa014, 31), (0xa016, 23), (0xf566, -3)]: write(0xff0000+off, value, 2)
                write(0xfff550, values['pause']); write(0xfff55a, values['slow']); write(0xffff18, 0x12345678, 4)
                write_bytes(0xfff80a, bytes([0, 0x80, 0, 0x81]))
                if values['parameters'] is not None:
                    write(0xff8490, 0xffff7000, 4)
                    for off, key in [(4, 'vertical_velocity'), (8, 'gravity'), (12, 'horizontal_velocity')]: write(0xff7000+off, values['parameters'][key], 4)

            def run(role, name, address=0x8400):
                nonlocal label, requested
                label, requested = name, role
                for reg in range(8): setreg(reg, 0)
                setreg(17, 0x2700); setreg(15, 0xffffff00); write(0xffff00, 0x3d8, 4); setreg(13, 0xffff0000|address)
                entry = {'air': 0x1afc8, 'charge_motion': 0x1a430, 'clock': 0x19c96, 'main': 0x1313e, 'charge': 0x132de, 'release': 0x134c2}[role]
                setreg(16, entry); ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896; count = len(isolated)
                hook(observer); execute(8000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors[:3], pending is not None)
                return isolated[-1]

            def record(name):
                before = snapshot(ref.ram_cpu()); key, data = remember_frame(ref.ram_cpu(), 0xff8400)
                write(0xff845a, data['duration']); write(0xff845c, data['flags'])
                row = {'kind': 'record', 'name': name, 'object_id': 0x8400, 'before': before, 'after': snapshot(ref.ram_cpu()),
                       'record_key': key, 'external_queue_clears': [], 'rng_calls': 0, 'draw': False, 'cleared_bytes': 0, 'external_changes': [],
                       'interrupts': [], 'unexplained_changes': []}
                isolated.append(row); return row

            for seed, (completed, index, ticks, children, facing) in enumerate(itertools.product([0, 1, 2, 255], [0, 1, 2, 3, 255], [0, 1, 255], range(8), [0, 1, 2, 255])):
                setup(seed, completed=completed, index=index, ticks=ticks, children=children, facing=facing)
                run('air', f'gates-{completed}-{index}-{ticks}-{children}-{facing}')
            for seed, (power, child_power, children, facing) in enumerate(itertools.product([-32768, -1, 0, 1, 31, 32, 127, 223, 255, 256, 8192, 32767], [-32768, -1, 0, 3, 7, 32767], [0, 4], [0, 1, 2, 255])):
                setup(seed, power=power, child_power=child_power, children=children, facing=facing)
                run('air', f'power-{power}-{child_power}-{children}-{facing}')
            for seed, (x, dx, dy, alt, facing) in enumerate(itertools.product([-0x80000000, -1, 0x7fffabcd], [-32768, -1, 28, 32767], [-32768, -48, 32767], [-0x80000000, -1, 0, 0x40000], [0, 1, 2, 255])):
                setup(seed, x=x, y=x, vx=dx<<16|0xabcd, vy=dy<<16|0x5678, alt=alt, facing=facing, cursor=1)
                run('air', f'word-offsets-{x}-{dx}-{dy}-{alt}-{facing}')
            for seed, (vertical, horizontal, gravity, extra, facing) in enumerate(itertools.product([-0x80000000, -7, -3, -1, 0, 1, 3, 7, 0x7fffffff], [-0x80000000, -1, 0, 1, 0x7fffffff], [-0x80000000, 0, 0x7fffffff], [0, 1, 255], [0, 1, 2, 255])):
                parameters = {'vertical_velocity': vertical, 'horizontal_velocity': horizontal, 'gravity': gravity}
                setup(seed, completed=1, extra_jump=extra, facing=facing, parameters=parameters)
                run('air', f'completion-{vertical}-{horizontal}-{gravity}-{extra}-{facing}')
            # These child contexts are initialized by the real source3 parent
            # callback before the existing callback runs, not by relabeling9/0.
            for seed, (role, power, facing, paused) in enumerate(itertools.product(['main', 'charge', 'release'], [0, 32, 96, 224], [0, 1, 2], range(3))):
                setup(seed, clear_children=True, child_power=7, power=power, facing=facing)
                if role == 'charge':
                    write(0xff845d, 0x17); write(0xff845f, 0xa4); write(0xff8459, 4); write(0xff844c, 1)
                    write(0xff8471, 16); run('charge_motion', f'child-preparation-{role}-{power}-{facing}-{paused}')
                    address = 0x8600
                else:
                    run('air', f'child-preparation-{role}-{power}-{facing}-{paused}'); address = 0x8500 if role == 'main' else 0x8700
                write(0xfff550, 128 if paused == 1 else 0); write(0xfff55a, 255 if paused == 2 else 0)
                run(role, f'child-context-{role}-{power}-{facing}-{paused}', address)
            boundary_count = len(isolated)
            configs = []
            for power, facing, pool in itertools.product([0, 32, 96, 224], [0, 1], ['empty', 'full']): configs.append(('air', power, facing, pool, 0))
            for mode, facing, pool in itertools.product([1, 2], [0, 1], ['empty', 'full']): configs.append(('ground', 0, facing, pool, mode))
            for stream_id, (mode, power, facing, pool, charge_mode) in enumerate(configs):
                setup(stream_id, index=0, ticks=255, power=power, child_power=0, clear_children=True, facing=facing, pool=pool,
                      x=(100 if facing == 0 else 500)<<16|0xabcd, mode=0x22 if mode == 'air' else 0x17,
                      pose=0xac if mode == 'air' else 0xa4, gravity=charge_mode<<24|0x123456, held=16 if mode == 'ground' else 0)
                record('initial-timing'); initial = snapshot(ref.ram_cpu()); calls = []
                for step in range(512):
                    external = {}
                    if mode == 'ground' and step == 40: write(0xff8471, 0); external['held'] = 0
                    active = ref.ram_cpu()[0x845d] == (0x22 if mode == 'air' else 0x17)
                    rows = []
                    if active:
                        rows.append(run('clock', f'{mode}-{stream_id}/{step}-clock'))
                        rows.append(run('air' if mode == 'air' else 'charge_motion', f'{mode}-{stream_id}/{step}-parent'))
                        rows.append(record(f'{mode}-{stream_id}/{step}-timing'))
                    for offset in [0x100, 0x200, 0x300]:
                        ram = ref.ram_cpu(); address = 0x8400+offset
                        if n(ram, address+4) == 0: continue
                        behavior = n(ram, address, 4)
                        role = {0x1313e: 'main', 0x132de: 'charge', 0x134c2: 'release'}[behavior]
                        rows.append(run(role, f'{mode}-{stream_id}/{step}-{role}', address))
                    assert rows
                    rows[0]['external_input'] = external; calls.extend(rows)
                    ram = ref.ram_cpu()
                    if ram[0x845d] != (0x22 if mode == 'air' else 0x17) and all(n(ram, 0x8504+i*256) == 0 for i in range(3)): break
                else: raise AssertionError(('family stream did not finish', mode, power, facing, pool))
                streams.append({'name': f'{mode}-{power}-{facing}-{pool}-{charge_mode}', 'mode': mode, 'initial': initial, 'calls': calls})

    for name, rows, scope in [('fighter3-air-motion-oracle', natural, 'Actual source3 air22 and existing child callbacks under two real input flows'),
                              ('fighter3-air-motion-boundaries', isolated[:boundary_count], 'Complete original air22 callbacks and real-parent-created child contexts'),
                              ('fighter3-air-motion-sequences', streams, 'Single-initial-state original parent-clock/data and scheduled-child trajectories')]:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':'))+'\n').encode(); packed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed)
        calls = [row for stream in rows for row in stream['calls']] if name.endswith('sequences') else rows
        report = {'pass': bool(rows) and not errors, 'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'records': len(rows), 'calls': len(calls), 'kind_counts': dict(Counter(row['kind'] for row in calls)),
                  'library_frames': len(library), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'baselines': baselines, 'binding': binding,
                  'frames_match_original_core': all(b['all_ram_video_match'] for b in baselines), 'rng_calls': sum(row['rng_calls'] for row in calls),
                  'external_queue_calls': sum(bool(row['external_queue_clears']) for row in calls), 'external_change_calls': sum(bool(row['external_changes']) for row in calls),
                  'interrupt_calls': sum(len(row['interrupts']) for row in calls), 'unexplained_change_calls': sum(bool(row['unexplained_changes']) for row in calls),
                  'draws': sum(row['draw'] for row in calls), 'clears': sum(row['cleared_bytes'] != 0 for row in calls), 'errors': errors, 'incomplete_calls': 0,
                  'limits': ['Callbacks stop before sprite emission or shared parent tail, or after prefix clear',
                             'Private parent timing records refresh only Duration/Flags from its actual immutable keys; not shape/tail outputs',
                             'Recovered parent freezes after leaving its covered motion while remaining child callbacks continue to deletion',
                             'No collision consumers, active FX callbacks, full actor/world or platform execution in these family streams']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': report['pass']}))


if __name__ == '__main__': main()
