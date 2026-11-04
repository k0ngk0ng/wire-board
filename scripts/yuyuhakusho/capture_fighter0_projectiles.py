#!/usr/bin/env python3
"""Original Genkai scatter projectile and independent release-light rules."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data


ENTRIES = {0x12c96: 'scatter', 0x134c2: 'release'}
DRAWS = {0x12d82, 0x135e6}


def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address):
    off = address & 0xffff
    parent = off & 0xfc00
    return {'actor_bytes': ram[off:off+256].hex(), 'parent_bytes': ram[parent:parent+256].hex(),
            'pool': {'cursor': num(ram, 0xa036, 2), 'bytes': ram[0x9400:0x9c00].hex()},
            'buffers': ram[0xa200:0xa600].hex(), 'offsets': [num(ram, 0xa004+i*2, 2) for i in range(4)],
            'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'rng_bytes': ram[0xff18:0xff1c].hex(),
            'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': num(ram, 0xa03e, 2),
                            'arena_left': num(ram, 0xa010, 2, True), 'arena_right': num(ram, 0xa012, 2, True),
                            'camera_x': num(ram, 0xa014, 2, True), 'camera_y': num(ram, 0xa016, 2, True), 'shake': num(ram, 0xf566, 2, True)}}


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--inputs', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter0-projectile-inputs.json')
    args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    input_path = args.inputs.resolve()
    events = json.loads(input_path.read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, *DRAWS, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac]
    pcs = (C.c_uint*len(points))(*points)
    with ReferenceSession(ROOT, out/'unobserved') as control:
        expected = [control.step(f, buttons_at(events, f))['reference'] for f in range(1, 3603)]
        control_identity = control.identity
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    short_sequences = [{'identity': 0x135ee+n*6, 'frames': [
        {'parameter': rom[0x135ee+n*6+i*2]*2, 'duration': rom[0x135ef+n*6+i*2]} for i in range(3)]}
        for n in range(8)]
    natural, boundaries, streams, errors = [], [], [], []
    library, pending = {}, {}
    private, label = False, ''
    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        def allowed(address, sp):
            off = address & 0xffff
            return (set(range(off, off+256)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa600)) |
                    set(range(0xa004, 0xa00c)) | {0xa030, 0xa036, 0xa037} | set(range(0xf80a, 0xf80e)) |
                    {(sp-i) & 0xffff for i in range(0x180)})

        @cbtype
        def observe(kind, width, pc, value):
            try:
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                key = (address, sp)
                if pc in ENTRIES and (private or ref.frame <= 3600):
                    assert not pending, 'Overlapping projectile callback'
                    ram = ref.ram_cpu()
                    pending[key] = {'kind': ENTRIES[pc], 'frame': ref.frame, 'object_id': address&0xffff,
                                    'before': snapshot(ram, address), 'external_queue_clears': [], 'deleted': False,
                                    '_ram': ram, '_allowed': allowed(address, sp)}
                elif pc in (0x37c56, 0x37cac):
                    for row in pending.values():
                        a = ref.get_register(9)&0xffff
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                elif pc == 0x1eb40:
                    for (owner, _), row in pending.items():
                        if owner == address:
                            ram = ref.ram_cpu(); off = address&0xffff
                            frame_key, data, source = selected_data(rom, ram, ram[off:off+256])
                            identity = json.dumps(frame_key, sort_keys=True)
                            assert identity not in library or library[identity]['data'] == data
                            library[identity] = {'key': frame_key, 'data': data, 'source': source}
                            row['animation_key'] = frame_key
                elif pc == 0x3556 and key in pending:
                    row = pending[key]; row['deleted'] = True
                    size = num(bytes.fromhex(row['before']['actor_bytes']), 4, 2)
                    row['cleared_bytes'] = ((size or 64)>>4)*16
                    assert 0 < row['cleared_bytes'] <= 256
                elif key in pending and (pc in DRAWS or pc == 0x3574 and pending[key]['deleted']):
                    row = pending.pop(key); ram = ref.ram_cpu()
                    row.update(after=snapshot(ram, address), draw=pc in DRAWS, exit=hex(pc), exit_frame=ref.frame,
                               cleared_bytes=row.get('cleared_bytes', 0))
                    row['external_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(row.pop('_ram'), ram))
                                               if a != b and i not in row['_allowed']]
                    row.pop('_allowed')
                    assert row['before']['parent_bytes'] == row['after']['parent_bytes']
                    assert row['before']['rng_bytes'] == row['after']['rng_bytes']
                    if private:
                        assert not row['external_changes'], (label, row['external_changes'])
                        row['name'] = label; boundaries.append(row); ref.cpu.cycles = 4000000
                    else:
                        natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for f in range(1, 3603):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors, (len(natural), list(pending), errors[:4])
        (out/'fighter0-projectile-natural-reference-frames.json').write_text(json.dumps(frames, separators=(',', ':'))+'\n')
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def fill(address, count, seed):
            for i in range(count): write(address+i, (i*73+seed*29+17)&255)

        def setup(kind, seed=0, **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            fill(0xff8500, 256, seed); fill(0xff9400, 2048, seed+1); fill(0xffa200, 1024, seed+2)
            values = {'stage': 2, 'size': 256, 'power': 0, 'pose': 0, 'index': 0, 'duration': 4, 'ticks': 0, 'flags': 0x10,
                      'completed': 0, 'x': (100<<16)|0xabcd, 'y': (260<<16)|0x5678, 'ground': (320<<16)|0x1234,
                      'vx': 0x60000, 'vy': -0x10000, 'facing': 0, 'contact': 0, 'pause': 0, 'slow': 0,
                      'pool': 'empty', 'left': 0, 'right': 600, 'camera_x': 31, 'camera_y': 23, 'shake': 0,
                      'offset_x': -3, 'offset_y': 4, 'sequence': 0x135ee}
            if kind == 'release': values.update(index=255, ticks=1)
            values.update(changes)
            fields = {'stage': (6, 2), 'size': (4, 2), 'power': (0x7c, 2), 'pose': (0x5f, 1), 'index': (0x59, 1),
                      'duration': (0x5a, 1), 'ticks': (0x5b, 1), 'flags': (0x5c, 1), 'completed': (0x60, 1),
                      'x': (0x40, 4), 'y': (0x44, 4), 'ground': (0x9c, 4), 'vx': (0x10, 4), 'vy': (0x14, 4),
                      'facing': (0x61, 1), 'contact': (0x89, 1), 'offset_x': (0x54, 2), 'offset_y': (0x56, 2)}
            for name, (off, count) in fields.items(): write(0xff8500+off, values[name], count)
            write(0xff8500, 0x12c96 if kind == 'scatter' else 0x134c2, 4)
            for off, value in [(0x1c, 0xd8f1a), (0x20, 0x240000), (0x50, 0xd92ca if kind == 'scatter' else values['sequence']), (0x8c, 0xd9a3a)]:
                write(0xff8500+off, value, 4)
            for i in range(16):
                busy = values['pool'] == 'full' or values['pool'] == 'single' and i != 1
                write(0xff9404+i*128, 128 if busy else 0, 2)
            for i in range(4): write(0xffa004+i*2, [0, 240, 256][(i+seed)%3], 2)
            write(0xffa036, 0xff80 if seed%2 else 0, 2); write(0xffa03e, [0, 0x2000, 0xffff][seed%3], 2)
            for a, name in [(0xffa010, 'left'), (0xffa012, 'right'), (0xffa014, 'camera_x'), (0xffa016, 'camera_y'), (0xfff566, 'shake')]: write(a, values[name], 2)
            write(0xfff550, values['pause']); write(0xfff55a, values['slow']); write(0xffa030, 0x97 if seed%2 else 0)
            for i, value in enumerate([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]): write(0xfff80a+i, value)

        def invoke(kind, name):
            nonlocal label
            label = name
            for reg in range(8): setreg(reg, 0)
            setreg(17, 0x2700); setreg(15, 0xffff00); write(0xffff00, 0x3d8, 4)
            setreg(13, 0xff8500); setreg(16, 0x12c96 if kind == 'scatter' else 0x134c2)
            ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896; count = len(boundaries)
            hook(observer); execute(2000000); hook(None)
            assert len(boundaries) == count+1 and not pending and not errors, (name, errors[:4], list(pending))
            return boundaries[-1]

        # Every immutable d92ca record required by this source and shared1313e.
        for pose in [0, 1, 2, 3, 4, 5, 6, 7, 8, 33, 34, 35]:
            base = 0xd92ca+num(rom, 0xd92ca+pose*2, 2, True)
            for index in range(64):
                for facing in [0, 1]:
                    setup('scatter', pose+index+facing, pose=pose, index=index, ticks=0, flags=0, facing=facing)
                    invoke('scatter', f'animation-{pose}-{index}-{facing}')
                if rom[base+index*8+2]&0x80: break
            else: raise ValueError('unterminated original animation')
        for seed, (stage, pose, index, ticks, paused, pool, facing, completed) in enumerate(itertools.product(
                [0, 2], [0, 2, 8], [0, 1], [0, 1, 255], range(3), ['empty', 'full'], [0, 1, 3], [0, 1, 2])):
            setup('scatter', seed, stage=stage, pose=pose, index=index, ticks=ticks, flags=0x10|(0x80 if index else 0),
                  pause=128 if paused == 1 else 0, slow=255 if paused == 2 else 0, pool=pool, facing=facing, completed=completed)
            invoke('scatter', f'scatter-matrix-{stage}-{pose}-{index}-{ticks}-{paused}-{pool}-{facing}-{completed}')
        geometry = [(-32768, -32768, -1), (-1, 260, -1), (31, 319, 0), (100, 319, 0), (100, 320, 0),
                    (100, 321, 0), (632, 260, -1), (633, 260, -1), (32767, 32767, -1), (700, 320, 0), (700, 321, 1)]
        for seed, ((x, y, vy), contact, pool, completed) in enumerate(itertools.product(geometry, [0, 1, 4, 8, 16, 24], ['empty', 'single', 'full'], [0, 1, 2])):
            setup('scatter', seed, x=(x<<16)|0xabcd, y=(y<<16)|0x5678, vx=0, vy=vy, contact=contact, pool=pool, completed=completed)
            invoke('scatter', f'scatter-geometry-{x}-{y}-{vy}-{contact}-{pool}-{completed}')
        for seed, (size, cause) in enumerate(itertools.product([0, 31, 271, *range(16, 257, 16)], ['bound', 'contact', 'land', 'completed'])):
            setup('scatter', seed, size=size, x=700<<16 if cause == 'bound' else 100<<16, y=321<<16 if cause == 'land' else 260<<16,
                  vy=0 if cause == 'land' else -1, contact=8 if cause == 'contact' else 0, completed=1 if cause == 'completed' else 0)
            invoke('scatter', f'scatter-clear-{size}-{cause}')
        powers = [0, 31, 32, 63, 64, 96, 128, 160, 192, 224, 255, 256, 0x2000, 0x2020, 0x20e0, 0x7fff, 0x8000, 0x8020, 0xffff]
        for seed, (stage, power, index, ticks, paused, completed) in enumerate(itertools.product([0, 2], powers, [255, 0, 1], [0, 1, 255], range(3), [0, 1, 2])):
            setup('release', seed, stage=stage, power=power, index=index, ticks=ticks, pause=128 if paused == 1 else 0,
                  slow=255 if paused == 2 else 0, completed=completed, flags=255, sequence=0x135fa)
            invoke('release', f'release-matrix-{stage}-{power}-{index}-{ticks}-{paused}-{completed}')
        for seed, (size, power) in enumerate(itertools.product([0, 31, 271, *range(16, 257, 16)], [0, 32, 96, 224, 0x2020])):
            setup('release', seed, size=size, power=power, index=1, ticks=1, completed=255)
            invoke('release', f'release-clear-{size}-{power}')
        boundary_count = len(boundaries)
        for pose, facing, terminal in itertools.product([0, 3, 8], [0, 1], ['animation', 'bound', 'land', 'contact']):
            setup('scatter', pose+facing, stage=0, pose=pose, facing=facing, x=(100 if facing == 0 else 500)<<16,
                  y=260<<16, vx=(6 if facing == 0 else -6)<<16, vy=0x10000 if terminal == 'land' else -1)
            initial = snapshot(ref.ram_cpu(), 0xff8500); calls = []
            for step in range(128):
                external = {}
                # Declared external animation metadata is never reinjected:
                # only contacts or world pause are controlled during the flow.
                if terminal == 'contact' and step == 2: write(0xff8589, 8); external['contact_flags'] = 8
                if step == 1: write(0xfff550, 128); external['pause_flags'] = 128
                if step == 2: write(0xfff550, 0); external['pause_flags'] = 0
                row = invoke('scatter', f'scatter-stream-{pose}-{facing}-{terminal}/{step}')
                row['external_input'] = external; calls.append(row)
                if row['deleted']: break
            assert calls[-1]['deleted']
            streams.append({'kind': 'scatter', 'initial': initial, 'calls': calls})
        for power, paused in itertools.product([0, 32, 64, 96, 128, 160, 192, 224, 0x2020, 0x8020], [False, True]):
            setup('release', power, stage=0, power=power, completed=2)
            initial = snapshot(ref.ram_cpu(), 0xff8500); calls = []
            for step in range(64):
                external = {}
                if paused and step == 2: write(0xfff55a, 255); external['slow_flag'] = 255
                if paused and step == 4: write(0xfff55a, 0); external['slow_flag'] = 0
                row = invoke('release', f'release-stream-{power}-{paused}/{step}')
                row['external_input'] = external; calls.append(row)
                if row['deleted']: break
            assert calls[-1]['deleted']
            streams.append({'kind': 'release', 'initial': initial, 'calls': calls})
        identity = ref.identity
    semantic = {'animations': list(library.values()), 'sequences': short_sequences}
    (out/'fighter0-projectile-library.json').write_text(json.dumps(semantic, separators=(',', ':'))+'\n')
    for name, records, scope in [('fighter0-projectile-oracle', natural, 'Natural original callbacks for the declared source0 menu/input flow'),
                                  ('fighter0-projectile-boundaries', boundaries[:boundary_count], 'Isolated complete original callbacks including dirty input and prefix clears'),
                                  ('fighter0-projectile-sequences', streams, 'Single-initial-state original lifecycles ending in deletion')]:
        payload = {'library': semantic, 'calls': records}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        calls = [c for s in records for c in s['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'unobserved_identity': control_identity,
                  'inputs_sha256': sha(input_path.read_bytes()), 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'records': len(records), 'calls': len(calls), 'kind_counts': dict(Counter(c['kind'] for c in calls)),
                  'draws': sum(c['draw'] for c in calls), 'deletions': sum(c['deleted'] for c in calls), 'library_frames': len(library), 'short_sequences': len(short_sequences),
                  'external_queue_calls': sum(bool(c['external_queue_clears']) for c in calls),
                  'external_change_calls': sum(bool(c['external_changes']) for c in calls),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'video_steps': len(frames), 'accepted_entry_last_frame': 3600, 'errors': errors, 'incomplete_calls': 0,
                  'pass': bool(records) and not errors,
                  'limits': ['Stop at draw request before hardware sprite emission or after completed prefix clear',
                             'Natural counts are reported per actual callback kind; absent kinds are not claimed observed',
                             'Parent/world collision scheduling and source0 actor composition are separate',
                             'Private cases execute a restored original CPU with declared external inputs; no rules are executed by this capture script']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
