#!/usr/bin/env python3
"""Research-only original68000 dragon/head/trail/boost callbacks and flows."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x13bb6: 'head', 0x13e9a: 'trail', 0x13fc8: 'boost'}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x1b2d4, 0x1b48a, 0x19c96, 0x1eb40, 0x15afc, 0x1580, 0x3556, 0x3574, 0x3d8,
              0x13c64, 0x13d0c, 0x13d1e, 0x1fae6, 0x1faee, 0x3982, 0x37c56, 0x37cac,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    sources = [('docs/research/md-yuyuhakusho/source4-next-inputs.json',
                'games/yuyuhakusho/engine/testdata/fighter4-attacks-reference-frames.json',
                'games/yuyuhakusho/engine/testdata/fighter4-attacks-reference-manifest.json')]
    special = ('games/yuyuhakusho/engine/testdata/fighter4-special-inputs.json',
               'games/yuyuhakusho/engine/testdata/fighter4-attacks-special-reference-frames.json',
               'games/yuyuhakusho/engine/testdata/fighter4-attacks-special-reference-manifest.json')
    if all((ROOT/name).is_file() for name in special): sources.append(special)
    natural, isolated, streams, errors, baselines, library = [], [], [], [], [], {}
    pending = None; private = False; direct = ''; label = ''; sequence = 0; scenario = ''
    curve_reads = []

    def draw(ram, off):
        return {'ObjectID': off, 'GraphicsSet': n(ram, off+0x1c, 4), 'TileBank': n(ram, off+0x20, 4),
                'Parameter': n(ram, off+0x3c), 'ScreenX': n(ram, off+8), 'ScreenY': n(ram, off+12),
                'OffsetX': n(ram, off+0x28, 2, True), 'OffsetY': n(ram, off+0x2a, 2, True),
                'Attributes': n(ram, off+0x24), 'Depth': n(ram, off+0x3e), 'Layer': ram[off+0x26],
                'SkipTiles': ram[off+0x27], 'Cache': [n(ram, off+0x2c+i*4, 4) for i in range(4)]}

    def snapshot(ram, parent=0x8400):
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)],
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'history': ram[0x7000:0x7800].hex(), 'bank_cursor': n(ram, 0xa038),
                'parameters': {key: n(rom, n(ram, parent+0x90, 4)+delta, 4, True) for key, delta in
                               [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    for source_index, (input_name, reference_name, manifest_name) in enumerate(sources):
        input_path, reference_path, manifest_path = (ROOT/name for name in (input_name, reference_name, manifest_name))
        reference_raw = reference_path.read_bytes(); manifest = json.loads(manifest_path.read_text())
        assert manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert manifest['input']['sha256'] == sha(input_path.read_bytes()) and manifest['output']['sha256'] == sha(reference_raw)
        expected, events, scenario = json.loads(reference_raw), json.loads(input_path.read_text()), input_name
        with ReferenceSession(ROOT, out/('observed-'+str(source_index)), core=core) as ref:
            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending, sequence
                try:
                    raw_pc = pc; pc &= 0xffffff
                    if pc == 0xfff000:
                        if pending is not None: pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                        return
                    if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                        if pending is not None and pending['_irq']:
                            irq = pending['_irq'].pop(); ram = ref.ram_cpu()
                            irq.update(exit=hex(pc), changes=[{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                            pending['interrupts'].append(irq)
                        return
                    if pc in [0x37c56, 0x37cac]:
                        if pending is not None:
                            a = ref.get_register(9)&65535; pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                        return
                    if pc == 0x3982:
                        if pending is not None: pending['rng_calls'] += 1
                        return
                    address, sp = ref.get_register(13)&65535, ref.get_register(15)
                    ram = ref.ram_cpu(); role = ENTRIES.get(pc)
                    if private and direct:
                        role = {0x1b2d4: 'creator_head', 0x1b48a: 'creator_boost', 0x19c96: 'clock', 0x1eb40: 'record'}.get(pc, role) if pending is None else None
                    parent = (address&0xfc00) if address < 0x9400 else (n(ram, address+0x48)&0xfc00)
                    if role and (private or parent in range(0x8400, 0x9400, 1024) and ram[parent+0x58] == 4):
                        assert pending is None, ('nested', pc, address)
                        if private: parent = 0x8400
                        pending = {'kind': role, 'frame': ref.frame, 'scenario': scenario, 'object_id': address, 'family_base': parent,
                                   'entry_sp': sp, 'source_register': hex(ref.get_register(13)), 'before': snapshot(ram, parent),
                                   'draw': False, 'cleared_bytes': 0, 'rng_calls': 0, 'external_queue_clears': [], 'interrupts': [],
                                   'allocations': [], 'curve_reads': [], '_irq': [], '_ram': ram}
                    elif pending is not None:
                        if pc in [0x13c64, 0x13d0c, 0x13d1e]:
                            index = ref.get_register(0)&65535; index = index if index < 32768 else index-65536
                            addr = (ref.get_register(8)+index)&0xffffff
                            row = {'pc': hex(pc), 'address': addr, 'index': (addr-0x34000)//2, 'value': n(rom, addr, 2, True)}
                            pending['curve_reads'].append(row); curve_reads.append(row)
                        if pc in [0x1fae6, 0x1faee]: pending['allocations'].append(ref.get_register(8)&65535)
                        if pc == 0x1eb40:
                            key, data, origin = selected_data(rom, ram, ram[address:address+256]); identity = json.dumps(key, sort_keys=True)
                            assert identity not in library or library[identity]['data'] == data
                            library[identity] = {'key': key, 'data': data, 'source': origin}
                        if pc == 0x3556:
                            pending['cleared_bytes'] = (n(ram, address+4) or 64)//16*16
                        finish = pc == 0x1580 or pc == 0x3574 and pending['cleared_bytes'] or pc == 0x15afc and pending['kind'].startswith('creator_') or pc == 0x3d8 and pending['kind'] in ['clock', 'record']
                        if finish:
                            row = pending; pending = None; assert sp == row['entry_sp']+(4 if pc == 0x3d8 else 0) and not row.pop('_irq')
                            sequence += 1; row.update(after=snapshot(ram, row['family_base']), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame, sequence=sequence)
                            if row['draw']: row['draw_request'] = draw(ram, address)
                            parent = row['family_base']
                            history_allowed = {0x7000+b*512+i for b in range(4) for i in [*range(0x140), *range(0x140, 0x180, 2)]}
                            allowed = set(range(parent, parent+1024))|set(range(0x9400, 0x9c00))|history_allowed|set(range(0xa200, 0xa600))|set(range(0xa004, 0xa00c))|{0xa030,0xa036,0xa037,0xa038,0xa039}|set(range(0xf80a,0xf80e))|{(row['entry_sp']-i)&65535 for i in range(0x180)}
                            row['ram_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(row.pop('_ram'), ram)) if a != b]
                            row['external_changes'] = [c for c in row['ram_changes'] if c['offset'] not in allowed]
                            row['unexplained_changes'] = []
                            for change in row['external_changes']:
                                current, steps = change['before'], 0
                                for irq in row['interrupts']:
                                    for delta in irq['changes']:
                                        if delta['offset'] == change['offset']:
                                            assert delta['before'] == current; current, steps = delta['after'], steps+1
                                if steps == 0 or current != change['after']: row['unexplained_changes'].append(change)
                            assert not row['unexplained_changes'] and row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed'], (row['kind'], row['unexplained_changes'])
                            if private:
                                assert not row['external_changes'] and not row['external_queue_clears']; row['name'] = label
                                isolated.append(row); ref.cpu.cycles = 8000000
                            else: natural.append(row)
                except Exception as exc:
                    errors.append((label, repr(exc)))
                    if private: ref.cpu.cycles = 8000000

            assert bridge.yyh_set_observer(observe, pcs, len(points))
            hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
            hook(observer); frames = []
            for frame in range(1, len(expected)+1):
                frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
                if frame == 2400: checkpoint, base_ram = ref.save(), ref.ram_cpu()
            hook(None); assert frames == expected and pending is None and not errors, (scenario, errors[:3])
            baselines.append({'input': input_name, 'input_sha256': sha(input_path.read_bytes()), 'reference': reference_name,
                              'reference_raw_sha256': sha(reference_raw), 'reference_manifest': manifest_name,
                              'reference_manifest_sha256': sha(manifest_path.read_bytes()), 'video_steps': len(expected), 'all_ram_video_match': True})
            identity = ref.identity
            if source_index != len(sources)-1: continue
            assert base_ram[0x8458] == 4
            private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))

            def setup(role='head', seed=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for address, count, mul in [(0x8400, 1024, 73), (0x9400, 2048, 37), (0x7000, 2048, 11), (0xa200, 1024, 19)]:
                    write_bytes(address, bytes((i*mul+seed*29+17)&255 for i in range(count)))
                write_bytes(0x8450, base_ram[0x8450:0x8454]); write_bytes(0x848c, base_ram[0x848c:0x8494]); write(0x8458, 4)
                v = {'stage': 2, 'size': 128 if role == 'trail' else 256, 'index': 0, 'duration': 8, 'ticks': 0, 'flags': 128, 'completed': 0,
                     'x': 100<<16|0xabcd, 'y': 260<<16|0x5678, 'ground': 320<<16|0x1234, 'direction': 0x1001234, 'phase': 0, 'age': 0,
                     'sound_clock': 0, 'facing': 0, 'held': 0, 'contact': 0, 'bank': 0, 'pause': 0, 'slow': 0, 'cursor': 0, 'pool': 'empty',
                     'parent_energy': 223<<16|0xabcd, 'parent_hp': 100, 'parent_pose': 0, 'parent_facing': 0, 'parent_attack': 0x83,
                     'left': 0, 'right': 600, 'layer': 253, 'skip': 0xa5, 'depth': 0x9876, 'attrs': 0x63cd}
                v.update(changes); off = 0x9480 if role == 'trail' else 0x8500
                for i in range(16):
                    free = v['pool'] == 'empty' or v['pool'] == 'single' and i == 1 or v['pool'] == 'current' and i == (v['cursor']&0x7ff)//128
                    write(0x9404+i*128, 0 if free else 128, 2)
                fields = {0: (next(pc for pc, r in ENTRIES.items() if r == role), 4), 4: (v['size'], 2), 6: (v['stage'], 2),
                          0x1c: (0xd8f1a, 4), 0x20: (0x240000, 4), 0x24: (v['attrs'], 2), 0x26: (v['layer'], 1), 0x27: (v['skip'], 1), 0x3e: (v['depth'], 2),
                          0x50: (0xd92ca, 4), 0x8c: (0xd9a3a, 4), 0x5f: (0x13 if role == 'boost' else 0x14, 1),
                          0x59: (v['index'], 1), 0x5a: (v['duration'], 1), 0x5b: (v['ticks'], 1), 0x5c: (v['flags'], 1), 0x60: (v['completed'], 1),
                          0x40: (v['x'], 4), 0x44: (v['y'], 4), 0x10: (0x1234beef, 4), 0x14: ((v['age']<<16)|0x4321, 4),
                          0x4c: (((v['bank'] if role == 'trail' else v['phase'])<<16)|0xfedc, 4), 0x48: (v['direction'], 4),
                          0x9c: (v['ground'], 4), 0x61: (v['facing'], 1), 0x71: (v['held'], 1), 0x89: (v['contact'], 1),
                          0xb8: ((v['sound_clock']<<16)|0xcafe, 4), 0xfe: (v['bank'], 2)}
                for delta, (value, size) in fields.items():
                    if role != 'trail' or delta < 128: write(off+delta, value, size)
                for delta, value, size in [(0x40, 300<<16|0x1111, 4), (0x44, 280<<16|0x2222, 4), (0x61, v['parent_facing'], 1),
                                           (0x5f, v['parent_pose'], 1), (0x78, v['parent_hp'], 2), (0x65, v['parent_attack'], 1), (0xb8, v['parent_energy'], 4)]: write(0x8400+delta, value, size)
                for i in range(4): write(0xa004+i*2, [0, 240, 256][(i+seed)%3], 2)
                for address, value, size in [(0xa036, v['cursor'], 2), (0xa038, 0x600, 2), (0xa03e, 0x2000, 2), (0xa030, 0x97, 1), (0xff18, 0x12345678, 4),
                                             (0xa010, v['left'], 2), (0xa012, v['right'], 2), (0xa014, 31, 2), (0xa016, 23, 2), (0xf566, -3, 2), (0xf550, v['pause'], 1), (0xf55a, v['slow'], 1)]: write(address, value, size)
                write_bytes(0xf80a, bytes([0, 0x80, 0, 0x81])); return off

            def call(role, name, address=None):
                nonlocal label, direct
                label, direct = name, role
                if address is None: address = 0x9480 if role == 'trail' else 0x8400 if role.startswith('creator') or role in ['clock', 'record'] else 0x8500
                entry = {'creator_head': 0x1b2d4, 'creator_boost': 0x1b48a, 'clock': 0x19c96, 'record': 0x1eb40}.get(role)
                if entry is None: entry = next(pc for pc, r in ENTRIES.items() if r == role)
                for reg in range(8): setreg(reg, 0)
                setreg(17, 0x2700); setreg(15, 0xffffff00); write(0xff00, 0x3d8, 4); setreg(13, 0xffff0000|address); setreg(16, entry)
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896; count = len(isolated)
                hook(observer); execute(8000000); hook(None); direct = ''
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors[:3])
                return isolated[-1]

            def case(role, name, **values): setup(role, len(isolated), **values); return call(role, name)

            for role, stage, pause, slow, completed in itertools.product(ENTRIES.values(), [0, 2], [0, 0x7f, 0x80, 0xff], [0, 255], [0, 1, 2]):
                case(role, f'gates-{role}-{stage}-{pause}-{slow}-{completed}', stage=stage, pause=pause, slow=slow, completed=completed)
            for held, direction, phase in itertools.product(range(16), [0, 1, 0x1ff, 0x200, 0x201, 0x3ff], [0, 224, 480, 736, 992]):
                case('head', f'steer-{held}-{direction}-{phase}', held=held|0xa0, direction=direction<<16|0xffff, phase=phase)
            for age, bank, pool, cursor in itertools.product([0, 2, 3, 27, 28, 30, 31, 32, 2046, 2047, 32766, 32767, 32768, 65534, 65535], [0, 0x200, 0x400, 0x600], ['empty', 'full', 'current'], [0, 0x780]):
                case('head', f'age-{age}-{bank}-{pool}-{cursor}', age=age, bank=bank, pool=pool, cursor=cursor)
            for x, y, ground, contact in [(100, 320, 320, 0), (100, 321, 320, 0), (100, -1, 320, 0x18), (-65, 200, 320, 0x18), (-64, 200, 320, 0), (664, 200, 320, 0), (665, 200, 320, 0x18), (100, 200, 320, 8), (100, 200, 320, 16), (100, 200, 320, 4), (-32768, -32768, 32767, 0)]:
                case('head', f'exit-{x}-{y}-{ground}-{contact}', x=(x<<16)|1, y=(y<<16)|0xffff, ground=ground<<16, contact=contact, direction=0, phase=992, age=3)
            for clock in [0, 1, 254, 255, 256, 65535]: case('head', f'sound-{clock}', sound_clock=clock)
            for angle in range(0,1024,128): case('head', f'pose-bucket-{angle}', direction=angle<<16|0x1234,phase=992)
            for bank, age, angle, status in itertools.product([0, 0x200, 0x400, 0x600], [0, 30, 31, 65535], [0, 63, 64, 255, 256, 511, 512, 513, 767, 768, 1023, 32768, 65535], [0, 1, 2, 3, 255]):
                setup('trail', len(isolated), bank=bank, age=age); pos=0x7000+bank+((age+1)&31)*2
                for delta, val, width in [(0, 0x8000, 2), (0x40, 0x7fff, 2), (0x80, angle, 2), (0xc0, 0x1234, 2), (0x100, 0xfda5, 2), (0x140, status, 1)]: write(pos+delta, val, width)
                call('trail', f'trail-{bank}-{age}-{angle}-{status}')
            for energy, hp, pose, facing in itertools.product([-0x80000000, -1, 0, 0x4fff, 0x5000, 0xffff, 1<<16, (219<<16)|0xffff, 220<<16, (223<<16)|0xabcd, 0x7fffffff], [-1, 0, 100], [0, 0xf, 0x10, 0x11], [0, 1, 2, 255]):
                case('boost', f'energy-{energy}-{hp}-{pose}-{facing}', parent_energy=energy, parent_hp=hp, parent_pose=pose, parent_facing=facing)
            for role, size in itertools.product(ENTRIES.values(), [0, 16, 31, 32, 48, 64, 80, 96, 112, 128] ):
                if role == 'head': case(role, f'clear-{role}-{size}', size=size, contact=16)
                elif role == 'boost': case(role, f'clear-{role}-{size}', size=size, parent_energy=-1)
                else:
                    setup(role, len(isolated), size=size); write(0x7142, 1); call(role, f'clear-{role}-{size}')
            boundary_count = len(isolated)

            for kind, bank, facing, pool in itertools.product(['head', 'boost'], [0, 0x600], [0, 2], ['empty', 'full']):
                setup(kind, len(streams), stage=0, bank=bank, pool=pool, parent_facing=facing)
                write_bytes(0x8500, bytes(768)); write(0x8504, 0, 2)
                # The creator receives one original clock step into its actual
                # creation frame. No intermediate expected child is injected.
                write(0x8400, 0x13b18, 4); write(0x8404, 256, 2); write(0x8406, 2, 2)
                write(0x845d, 0x29 if kind == 'head' else 0x2b); write(0x845f, 0xa6 if kind == 'head' else 0xab)
                write(0x8459, 1 if kind == 'head' else 0); write(0x845a, 8); write(0x845b, 7); write(0x845c, 0); write(0x8460, 0)
                write(0x8440, 300<<16|0xabcd, 4); write(0x8444, 280<<16|0x5678, 4); write(0x849c, 320<<16|0x1234, 4)
                write(0x8471, 0); write(0x8472, 0); write(0x8473, 0); write(0x8410, 0, 4)
                write(0xa038, bank, 2); write(0xa010, -30000, 2); write(0xa012, 30000, 2)
                initial=snapshot(ref.ram_cpu()); calls=[call('clock', 'stream-clock'), call('creator_'+kind, 'stream-create-'+kind)]
                assert n(ref.ram_cpu(), 0x8504) == 256
                for step in range(900):
                    ram=ref.ram_cpu()
                    if not n(ram,0x8504) and not any(n(ram,a,4)==0x13e9a and n(ram,a+4) for a in range(0x9400,0x9c00,128)): break
                    for address in [0x8500,*range(0x9400,0x9c00,128)]:
                        ram=ref.ram_cpu()
                        if not n(ram,address+4) or address != 0x8500 and n(ram,address,4)!=0x13e9a: continue
                        role=kind if address==0x8500 else 'trail'
                        external={}
                        if step in [1,2]: write(0xf550, 128 if step==1 else 0); external['pause_flags']=128 if step==1 else 0
                        if role=='head' and step==80: write(address+0x89, 0x10); external['contact_flags']=0x10
                        row=call(role, f'stream-{kind}-{bank}-{facing}-{pool}-{step}-{address:x}', address); row['external_input']=external; calls.append(row)
                else: raise AssertionError(('flow never cleared',kind,bank,facing,pool))
                streams.append({'name':f'{kind}-{bank}-{facing}-{pool}','initial':initial,'calls':calls})

    curve = {'first_index': -32, 'values': [n(rom, 0x34000+i*2, 2, True) for i in range(-32,1312)]}
    assert curve_reads and min(r['index'] for r in curve_reads) >= -32 and max(r['index'] for r in curve_reads) <= 1311
    for name, rows, scope in [('fighter4-projectile-oracle', natural, 'Actual full source4 dragon callbacks under real-menu input'),
                              ('fighter4-projectile-boundaries', isolated[:boundary_count], 'Original full dirty callbacks and signed-word lookup addresses'),
                              ('fighter4-projectile-sequences', streams, 'Original creator/clock/head/trail/history/clear single-initial-state flows')]:
        raw=(json.dumps({'library':list(library.values()),'curve':curve,'calls':rows},separators=(',',':'))+'\n').encode(); packed=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed)
        calls=[r for s in rows for r in s['calls']] if name.endswith('sequences') else rows
        report={'pass':not errors and (bool(rows) or name.endswith('oracle')), 'scope':scope, 'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
                'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(r['kind'] for r in calls)),'library_frames':len(library),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(packed),'baselines':baselines,'frames_match_original_core':all(b['all_ram_video_match'] for b in baselines),
                'curve_read_count':len(curve_reads),'curve_min_index':min(r['index'] for r in curve_reads),'curve_max_index':max(r['index'] for r in curve_reads),
                'rng_calls':sum(r['rng_calls'] for r in calls),'unexplained_change_calls':sum(bool(r['unexplained_changes']) for r in calls),'errors':errors,
                'limits':['Draw is captured at1580 entry; hardware sprite consumers are outside callback scope.','Non-dragon impact/dust callbacks are not scheduled in isolated flows.','Only declared pause/contact external inputs; no intermediate state/clock/AN reinjection.']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n'); print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':report['pass']}),flush=True)
    semantic={'animations':[{'key':r['key'],'data':r['data']} for r in sorted(library.values(),key=lambda r:tuple(r['key'].values())) if r['key']['animation_set']==0xd92ca and 14<=r['key']['pose']<=19], 'curve':curve}
    semantic_raw=(json.dumps(semantic,separators=(',',':'))+'\n').encode(); (out/'fighter4-projectile-rules.json').write_bytes(semantic_raw)
    (out/'fighter4-projectile-rules-manifest.json').write_text(json.dumps({'pass':True,'rom_sha256':sha(rom),'rules_sha256':sha(semantic_raw),'animations':len(semantic['animations']),'curve_words':len(curve['values']),
        'curve_cpu_min_index':min(r['index'] for r in curve_reads),'curve_cpu_max_index':max(r['index'] for r in curve_reads),'capture_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n')


if __name__ == '__main__': main()
