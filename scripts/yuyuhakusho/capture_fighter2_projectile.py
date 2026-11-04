#!/usr/bin/env python3
"""Source2 stationary projectile, scatter creator contexts and immutable poses."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x12dd4: 'beam', 0x12c96: 'scatter'}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, 0x1ac58, 0x1adc8, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac, 0x3982,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    sources = [('docs/research/md-yuyuhakusho/source2-next-inputs.json', 'fighter2-attacks-reference-frames.json', 'fighter2-attacks-reference-manifest.json'),
               ('games/yuyuhakusho/engine/testdata/fighter2-projectile-inputs.json', 'fighter2-attacks-special-reference-frames.json', 'fighter2-attacks-special-reference-manifest.json')]
    natural, isolated, streams, baselines, errors = [], [], [], [], []
    library = {}; pending = None; private = False; label = ''; scenario = ''

    def snapshot(ram, parent):
        obj = ram[parent:parent+256]; pointer = n(obj, 0x90, 4)&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()}, 'buffers': ram[0xa200:0xa600].hex(),
                'offsets': [n(ram, 0xa004+i*2) for i in range(4)], 'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]),
                'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    for source_index, (input_name, reference_name, manifest_name) in enumerate(sources):
        input_path = ROOT/input_name; reference_path = ROOT/'games/yuyuhakusho/engine/testdata'/reference_name
        manifest_path = ROOT/'games/yuyuhakusho/engine/testdata'/manifest_name
        reference_raw = reference_path.read_bytes(); expected = json.loads(reference_raw); events = json.loads(input_path.read_text())
        original = json.loads(manifest_path.read_text())
        assert original['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert original['output']['sha256'] == sha(reference_raw) and original['input']['sha256'] == sha(input_path.read_bytes())
        scenario = Path(input_name).name
        with ReferenceSession(ROOT, out/('observed-'+str(source_index)), core=core) as ref:
            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending
                try:
                    raw_pc = pc; pc &= 0xffffff
                    if pc == 0xfff000:
                        if pending is not None: pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), 'frame': ref.frame, '_ram': ref.ram_cpu()})
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
                    address, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                    parent = (address&65535)&0xfc00; ram = ref.ram_cpu(); role = ENTRIES.get(pc)
                    if private and pc in [0x1ac58, 0x1adc8]: role = 'creator_beam' if pc == 0x1ac58 else 'creator_scatter'
                    if role and ram[parent+0x58] == 2 and (private or ref.frame <= 3600):
                        assert pending is None
                        pending = {'kind': role, 'frame': ref.frame, 'scenario': scenario, 'object_id': address&65535, 'entry_sp': sp,
                                   'before': snapshot(ram, parent), 'source_register': hex(ref.get_register(13)), 'draw': False, 'cleared_bytes': 0,
                                   'rng_calls': 0, 'external_queue_clears': [], 'interrupts': [], '_irq': [], '_ram': ram}
                    elif pending is not None:
                        if pc == 0x1eb40:
                            off = address&65535; key, data, origin = selected_data(rom, ram, ram[off:off+256]); identity = json.dumps(key, sort_keys=True)
                            assert identity not in library or library[identity]['data'] == data
                            library[identity] = {'key': key, 'data': data, 'source': origin}
                        elif pc == 0x3556:
                            size = n(ram, (address&65535)+4); pending['cleared_bytes'] = (size or 64)//16*16
                            assert 0 < pending['cleared_bytes'] <= 256
                        finish = (pc == 0x15afc and pending['kind'].startswith('creator_') or
                                  pc == 0x1580 and pending['kind'] in ['beam', 'scatter'] or pc == 0x3574 and pending['cleared_bytes'])
                        if finish:
                            row = pending; pending = None; assert sp == row['entry_sp'] and not row.pop('_irq')
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
                                        assert delta['before'] == current; current, steps = delta['after'], steps+1
                                if steps == 0 or current != change['after']: row['unexplained_changes'].append(change)
                            assert not row['unexplained_changes'] and row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                            if private:
                                assert not row['external_changes'] and not row['external_queue_clears']; row['name'] = label
                                isolated.append(row); ref.cpu.cycles = 8000000
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
            hook(None); assert frames == expected and pending is None and not errors, (scenario, errors[:3])
            baselines.append({'input': input_name, 'input_sha256': sha(input_path.read_bytes()), 'reference': str(reference_path.relative_to(ROOT)),
                              'reference_raw_sha256': sha(reference_raw), 'reference_manifest': str(manifest_path.relative_to(ROOT)),
                              'reference_manifest_sha256': sha(manifest_path.read_bytes()), 'video_steps': len(expected), 'all_ram_video_match': True})
            identity = ref.identity
            if source_index == 0: continue
            assert base_ram[0x8458] == 2
            binding = {'source_tag': 2, 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4),
                       'parameter_pointer': n(base_ram, 0x8490, 4), 'source_actor_sha256': sha(base_ram[0x8400:0x8500])}
            private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))

            def setup(seed=0, role='beam', **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, bytes((i*73+seed*29+17)&255 for i in range(1024)))
                write_bytes(0xff8450, base_ram[0x8450:0x8454]); write_bytes(0xff848c, base_ram[0x848c:0x8494]); write(0xff8458, 2)
                write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048))); write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                values = {'stage': 2, 'size': 256, 'pose': 12 if role == 'beam' else 13, 'index': 0, 'duration': 4, 'ticks': 0, 'flags': 0,
                          'completed': 0, 'hitstop': 0, 'shake': 5, 'x': 100<<16|0xabcd, 'y': 260<<16|0x5678,
                          'vx': 0x7fffffff, 'vy': -1, 'ground': 320<<16|0x1234, 'facing': 0, 'offset_x': -3, 'offset_y': 4,
                          'pool': 'empty', 'cursor': 0, 'pause': 0, 'slow': 0, 'contact': 0, 'left': 0, 'right': 600}
                values.update(changes)
                fields = {0: (0x12dd4 if role == 'beam' else 0x12c96, 4), 4: (values['size'], 2), 6: (values['stage'], 2),
                          0x1c: (0xd8f1a, 4), 0x20: (0x240000, 4), 0x50: (0xd92ca, 4), 0x8c: (0xd9a3a, 4),
                          0x5f: (values['pose'], 1), 0x59: (values['index'], 1), 0x5a: (values['duration'], 1), 0x5b: (values['ticks'], 1),
                          0x5c: (values['flags'], 1), 0x60: (values['completed'], 1), 0x69: (values['hitstop'], 1), 0x6a: (values['shake'], 1),
                          0x40: (values['x'], 4), 0x44: (values['y'], 4), 0x10: (values['vx'], 4), 0x14: (values['vy'], 4),
                          0x9c: (values['ground'], 4), 0x61: (values['facing'], 1), 0x54: (values['offset_x'], 2), 0x56: (values['offset_y'], 2), 0x89: (values['contact'], 1)}
                for off, (value, count) in fields.items(): write(0xff8500+off, value, count)
                for i in range(16):
                    busy = values['pool'] == 'full' or values['pool'] == 'single' and i != 1
                    write(0xff9404+i*128, 128 if busy else 0, 2)
                for i in range(4): write(0xffa004+i*2, [0, 240, 256][(i+seed)%3], 2)
                write(0xffa036, values['cursor'], 2); write(0xffa03e, [0, 0x2000, 0xffff][seed%3], 2); write(0xffa030, 0x97)
                write_bytes(0xfff80a, bytes([0, 0x80, 0, 0x81])); write(0xffff18, 0x12345678, 4)
                for off, value in [(0xa010, values['left']), (0xa012, values['right']), (0xa014, 31), (0xa016, 23), (0xf566, -3)]: write(0xff0000+off, value, 2)
                write(0xfff550, values['pause']); write(0xfff55a, values['slow'])

            def run(role, name, address=0x8500):
                nonlocal label
                label = name
                for reg in range(8): setreg(reg, 0)
                setreg(17, 0x2700); setreg(15, 0xffffff00); write(0xffff00, 0x3d8, 4); setreg(13, 0xffff0000|address)
                setreg(16, {'beam': 0x12dd4, 'scatter': 0x12c96, 'creator_beam': 0x1ac58, 'creator_scatter': 0x1adc8}[role])
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896; count = len(isolated)
                hook(observer); execute(8000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors, (name, errors[:3])
                return isolated[-1]

            def creator(role, seed=0, ticks=0, facing=0, pool='empty', cursor=0):
                setup(seed, pool=pool, cursor=cursor)
                write_bytes(0xff8500, bytes(768)); write(0xff8400, 0x12668, 4); write(0xff8404, 256, 2); write(0xff8406, 2, 2)
                write(0xff845d, 0x1e if role == 'beam' else 0x20); write(0xff845f, 0xa6 if role == 'beam' else 0xa8)
                write(0xff8459, 4 if role == 'beam' else 5); write(0xff845b, ticks); write(0xff8460, 0); write(0xff8461, facing)
                write(0xff8440, (100 if facing == 0 else 500)<<16|0xabcd, 4); write(0xff8444, 320<<16|0x5678, 4); write(0xff849c, 320<<16|0x1234, 4)
                write(0xff8410, 0x300000 if role == 'beam' else 0x40000, 4); write(0xff844c, 0x1234, 4)
                write(0xff8471, 0); write(0xff8472, 0); write(0xff8473, 0)
                return run('creator_'+role, f'creator-{role}-{seed}-{ticks}-{facing}', 0x8400)

            for pose in [9, 12, 13]:
                start = 0xd92ca+n(rom, 0xd92ca+pose*2, 2, True)
                for index in range(64):
                    for facing in [0, 1]:
                        role = 'scatter' if pose == 13 else 'beam'
                        setup(pose+index+facing, role=role, pose=pose, index=index, facing=facing, hitstop=1 if role == 'beam' else 0, ticks=0, duration=255, vx=0)
                        run(role, f'animation-{pose}-{index}-{facing}')
                    if rom[start+index*8+2]&128: break
                else: raise ValueError('animation has no bounded terminal')
            for seed, (stage, pose, index, ticks, paused, completed, hitstop) in enumerate(itertools.product([0, 2], [9, 12], [0, 1], [0, 1, 255], range(3), [0, 1, 2], [0, 1, 2, 255])):
                setup(seed, stage=stage, pose=pose, index=index, ticks=ticks, pause=128 if paused == 1 else 0, slow=255 if paused == 2 else 0, completed=completed, hitstop=hitstop, flags=128 if index else 0)
                run('beam', f'gates-{stage}-{pose}-{index}-{ticks}-{paused}-{completed}-{hitstop}')
            for seed, (hitstop, shake, offset, facing) in enumerate(itertools.product([1, 2, 3, 4, 254, 255], [0, 1, 2, 3, 128, 255], [-32768, -1, 0, 32767], [0, 1, 2, 255])):
                setup(seed, hitstop=hitstop, shake=shake, offset_x=offset, facing=facing, completed=1)
                run('beam', f'hitstop-{hitstop}-{shake}-{offset}-{facing}')
            for seed, (x, left, right, pause) in enumerate(itertools.product([-32768, -1, 0, 1, 599, 600, 601, 32767], [-32768, 0, 300], [-1, 200, 600, 32767], [0, 128])):
                setup(seed, stage=0, x=x<<16|0xabcd, left=left, right=right, pause=pause, vx=-0x80000000, vy=0x7fffffff)
                run('beam', f'clamp-{x}-{left}-{right}-{pause}')
            for seed, (pool, cursor, facing) in enumerate(itertools.product(['empty', 'single', 'full'], [0, 0xff80, 0x780], [0, 1, 3])):
                setup(seed, stage=0, pose=9, pool=pool, cursor=cursor, facing=facing)
                run('beam', f'paired-dust-{pool}-{cursor}-{facing}')
            for seed, size in enumerate([0, 31, 271, *range(16, 257, 16)]):
                setup(seed, size=size, completed=1)
                run('beam', f'clear-{size}')
            boundary_count = len(isolated)
            for stream_id, (role, pool, facing, hitstop) in enumerate(itertools.product(['beam', 'scatter', 'dust'], ['empty', 'full'], [0, 1], [False, True])):
                creator_row = None
                if role == 'dust': setup(stream_id, stage=0, pose=9, pool=pool, facing=facing, vx=0x7fffffff)
                else: creator_row = creator(role, stream_id, facing=facing, pool=pool)
                address = 0x8500 if role != 'scatter' else 0x8600
                actual_role = 'beam' if role == 'dust' else role
                initial = snapshot(ref.ram_cpu(), 0x8400); calls = []
                for step in range(512):
                    external = {}
                    if hitstop and step == 3:
                        write(0xff0000+address+0x69, 4); write(0xff0000+address+0x6a, 5); external.update(hitstop=4, shake=5)
                    if step == 1: write(0xfff550, 128); external['pause_flags'] = 128
                    if step == 2: write(0xfff550, 0); external['pause_flags'] = 0
                    row = run(actual_role, f'stream-{role}-{pool}-{facing}-{hitstop}/{step}', address)
                    row['external_input'] = external; calls.append(row)
                    if row['cleared_bytes']: break
                else: raise AssertionError(('projectile stream did not delete', role, pool, facing, hitstop))
                streams.append({'name': f'{role}-{pool}-{facing}-{hitstop}', 'creator': creator_row, 'initial': initial, 'calls': calls})
            identity = ref.identity

    for name, rows, scope in [('fighter2-projectile-oracle', natural, 'Actual source2 new/static and reused-scatter full callbacks'),
                              ('fighter2-projectile-boundaries', isolated[:boundary_count], 'Original complete callbacks for timing, clamp, hit-stop, paired dust and prefix clear'),
                              ('fighter2-projectile-sequences', streams, 'Original source2 creators followed by single-initial-state child lifecycles')]:
        raw = (json.dumps({'library': list(library.values()), 'calls': rows}, separators=(',', ':'))+'\n').encode(); packed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed)
        calls = [r for stream in rows for r in ([stream['creator']] if stream['creator'] else [])+stream['calls']] if name.endswith('sequences') else rows
        report = {'pass': bool(rows) and not errors, 'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'records': len(rows), 'calls': len(calls), 'kind_counts': dict(Counter(r['kind'] for r in calls)), 'library_frames': len(library),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'baselines': baselines, 'binding': binding, 'frames_match_original_core': all(b['all_ram_video_match'] for b in baselines),
                  'rng_calls': sum(r['rng_calls'] for r in calls), 'external_queue_calls': sum(bool(r['external_queue_clears']) for r in calls),
                  'external_change_calls': sum(bool(r['external_changes']) for r in calls), 'unexplained_change_calls': sum(bool(r['unexplained_changes']) for r in calls),
                  'interrupt_calls': sum(len(r['interrupts']) for r in calls), 'clears': sum(r['cleared_bytes'] != 0 for r in calls), 'errors': errors, 'incomplete_calls': 0,
                  'limits': ['Stops at draw request or completed clear, without hardware sprites or active FX/collision callbacks',
                             'Source2 parent creators are executed once; parent then freezes during the complete child lifecycle',
                             'Private external hit-stop/pause pulses are explicit; no intermediate expected-state, clock or data-record reinjection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps({'name': name, 'records': len(rows), 'calls': len(calls), 'kinds': report['kind_counts'], 'pass': report['pass']}))
    assert len(library) == 23
    frames = sorted(library.values(), key=lambda r: (r['key']['animation_set'], r['key']['shape_set'], r['key']['pose'], r['key']['index']))
    semantic = (json.dumps({'animations': [{'key': row['key'], 'data': row['data']} for row in frames]}, separators=(',', ':'))+'\n').encode()
    (out/'fighter2-projectile-rules.json').write_bytes(semantic)
    (out/'fighter2-projectile-rules-manifest.json').write_text(json.dumps({'pass': True, 'scope': 'Semantic animation/shape records without CPU examples, RAM or instruction bytes',
        'animations': len(frames), 'poses': [9, 12, 13], 'rules_bytes': len(semantic), 'rules_sha256': sha(semantic), 'capture_sha256': sha(Path(__file__).read_bytes()),
        'rom_sha256': identity['rom_sha256'], 'evidence': {suffix: sha((out/('fighter2-projectile-'+suffix+'.json.gz')).read_bytes()) for suffix in ['oracle', 'boundaries', 'sequences']}}, indent=2)+'\n')


if __name__ == '__main__': main()
