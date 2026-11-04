#!/usr/bin/env python3
"""Original combo digits and attached stun-effect rules/lifetimes."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0x14930: 'combo', 0x14ec6: 'status'}
RETURNS = {0x14f3c, 0x14f5a, 0x14f78, 0x14fb6}


def num(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, address, kind):
    off = address&0xffff
    parent = num(ram, off+0x48, 2) if kind == 'status' else 0x8400
    assert 0x8000 <= parent <= 0xff00
    return {'object_id': off, 'actor_bytes': ram[off:off+128].hex(), 'pool': ram[0x9400:0x9c00].hex(),
            'parent_id': parent, 'parent_bytes': ram[parent:parent+256].hex(), 'cursor': num(ram, 0xa036, 2),
            'world': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'attribute_mask': num(ram, 0xa03e, 2),
                      'camera_x': num(ram, 0xa014, 2, True), 'camera_y': num(ram, 0xa016, 2, True), 'shake': num(ram, 0xf566, 2, True)}}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = list(ENTRIES)+list(RETURNS)+[0x1580, 0x3556, 0x3574]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    base_events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    spam = [e for e in base_events if e['end'] <= 1800]
    for frame in range(1850, 3550, 4):
        direction = 7 if (frame-1850)//280%2 == 0 else 6
        spam.append({'start': frame, 'end': frame+1, 'buttons': [direction, 0 if frame%32 else 1]})
    spam.sort(key=lambda x: x['start'])
    profiles = [('baseline', base_events), ('attack-pulses', spam)]
    input_raw = (json.dumps(profiles, indent=2)+'\n').encode()
    (out/'attached-effects-inputs.json').write_bytes(input_raw)
    immutable = {'sequences': [], 'orbit_x': [], 'orbit_y': [], 'offset_y': [], 'sources': []}
    for i in range(3):
        address = 0x15082+i*6
        raw = rom[address:address+6]
        immutable['sequences'].append({'id': address, 'frames': [{'parameter': raw[j]*2, 'duration': raw[j+1]} for j in range(0, 6, 2)]})
        immutable['sources'].append({'address': hex(address), 'bytes': raw.hex(), 'sha256': sha(raw)})
    for key, address in [('orbit_x', 0x34000), ('orbit_y', 0x34200)]:
        raw = rom[address:address+2048]
        immutable[key] = [raw[i]-256 if raw[i] >= 128 else raw[i] for i in range(0, 2048, 2)]
        immutable['sources'].append({'address': hex(address), 'bytes': raw.hex(), 'sha256': sha(raw), 'sample_stride': 2})
    raw = rom[0x1506c:0x15082]
    immutable['offset_y'] = [num(raw, i, 2, True) for i in range(0, 22, 2)]
    immutable['sources'].append({'address': '0x1506c', 'bytes': raw.hex(), 'sha256': sha(raw)})
    natural, isolated, streams, errors, baselines = [], [], [], [], []
    pending = {}
    private = False
    label = ''
    profile_name = ''
    for profile_name, events in profiles:
        with ReferenceSession(ROOT, out/('unobserved-'+profile_name)) as ref:
            expected = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 3601)]
            unobserved_identity = ref.identity
        with ReferenceSession(ROOT, out/('observed-'+profile_name), core=core) as ref:
            @cbtype
            def observe(kind, width, pc, value):
                try:
                    address = ref.get_register(13)&0xffffff
                    key = (address, ref.get_register(15))
                    if pc in ENTRIES:
                        assert not pending, 'Nested attached FX callback'
                        pending[key] = {'kind': ENTRIES[pc], 'profile': profile_name, 'frame': ref.frame,
                                        'before': snapshot(ref.ram_cpu(), address, ENTRIES[pc]), 'cleared_bytes': 0}
                    elif key in pending:
                        row = pending[key]
                        ram = ref.ram_cpu()
                        if pc == 0x3556:
                            size = num(ram, (address&0xffff)+4, 2)
                            assert size == 0 or 16 <= size < 144
                            row['cleared_bytes'] = (size if size else 64)//16*16
                        if pc == 0x1580 or pc in RETURNS or pc == 0x3574 and row['cleared_bytes']:
                            # A cleared status owner word must not become a new
                            # parent address when retaining the final snapshot.
                            off = address&0xffff
                            after = snapshot(ram, address, 'combo')
                            after['parent_id'] = row['before']['parent_id']
                            owner = after['parent_id']
                            after['parent_bytes'] = ram[owner:owner+256].hex()
                            row.update(after=after, draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                            assert row['before']['parent_bytes'] == row['after']['parent_bytes']
                            pending.pop(key)
                            if private:
                                row['name'] = label
                                isolated.append(row)
                                ref.cpu.cycles = 4000000
                            else:
                                natural.append(row)
                except Exception as exc:
                    errors.append(str(exc))
                    if private:
                        ref.cpu.cycles = 4000000
            assert bridge.yyh_set_observer(observe, pcs, len(points))
            hook = ref.api('set_cpu_hook', None, C.c_void_p)
            observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
            hook(observer)
            frames = []
            for frame in range(1, 3601):
                frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
                if frame == 2400:
                    checkpoint = ref.save()
            hook(None)
            assert frames == expected and not pending and not errors, (profile_name, errors, list(pending))
            baselines.append({'name': profile_name, 'frames': 3600, 'frames_match_original_core': True, 'inputs_sha256': sha((json.dumps(events, separators=(',', ':'))+'\n').encode()), 'unobserved_identity': unobserved_identity})
            if profile_name != profiles[-1][0]:
                continue
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write(address, value, size=1):
                for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

            def fill(address, size, seed):
                for i in range(size):
                    write(address+i, (i*73+seed*29+17)&255)

            def setup(kind, seed=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                fill(0xff9400, 0x800, seed)
                for owner in [0x8400, 0x8800, 0x8c00, 0x9000]:
                    fill(0xff0000+owner, 256, seed+1)
                owner = changes.get('owner', 0x8400 if kind == 'combo' else [0x8400, 0x8800, 0x8c00, 0x9000][seed%4])
                fields = {0:(0x14930 if kind == 'combo' else 0x14ec6, 4), 4:(128, 2), 6:(2, 2), 8:(0x180, 2), 12:(0x140, 2),
                          0x10:((changes.get('phase', 0)<<16)|0xabcd, 4), 0x1c:(0x27f618, 4), 0x20:(0x240000, 4), 0x24:(0xbfff, 2),
                          0x27:(1, 1), 0x28:(-3, 2), 0x2a:(7, 2), 0x3c:(0x104, 2), 0x3e:(0x100, 2),
                          0x40:(0x7fff1234, 4), 0x44:(-0x80004321, 4), 0x48:(changes.get('lifetime', 0) if kind == 'combo' else owner, 2),
                          0x50:(0x15082, 4), 0x54:(-13, 2), 0x56:(17, 2), 0x58:(0, 1), 0x59:(0, 1), 0x5a:(0, 1), 0x5b:(1, 1), 0x5c:(0, 1)}
                aliases = {'stage':6, 'size':4, 'parameter':0x3c, 'type':0x58, 'index':0x59, 'delay':0x5a, 'ticks':0x5b, 'flags':0x5c}
                for key, value in changes.items():
                    if key in aliases:
                        off = aliases[key]
                        fields[off] = (value, fields[off][1])
                for off, (value, size) in fields.items():
                    write(0xff9400+off, value, size)
                for off, value, size in [(0x58, changes.get('character', seed%11), 1), (0x59, changes.get('parent_index', 1), 1),
                                         (0x5b, changes.get('parent_ticks', 1), 1), (0x5d, changes.get('action', 0), 1),
                                         (0x84, changes.get('stun', 128), 2), (0x40, -0x80004321, 4), (0x44, 0x7fff1234, 4)]:
                    write(0xff0000+owner+off, value, size)
                for off, value, size in [(0xf550, changes.get('pause', 0), 1), (0xf55a, changes.get('slow', 0), 1),
                                         (0xa03e, [0, 0x2000, 0xffff][seed%3], 2), (0xa014, [0, 0x7fff, -32768, -1][seed%4], 2),
                                         (0xa016, [-32768, -1, 0, 32767][seed%4], 2), (0xf566, [-32768, -1, 0, 32767][(seed+1)%4], 2)]:
                    write(0xff0000+off, value, size)
                return owner

            def invoke(kind, name):
                nonlocal label
                label = name
                setreg(17, 0x2700)
                setreg(15, 0xffffe0)
                write(0xffffe0, 0x3d8, 4)
                setreg(13, 0xff9400)
                for reg in range(8):
                    setreg(reg, 0)
                setreg(16, 0x14930 if kind == 'combo' else 0x14ec6)
                ref.cpu.cycles = 0
                ref.cpu.refresh_cycles = 896
                before = ref.ram_cpu()
                count = len(isolated)
                hook(observer)
                execute(2000000)
                hook(None)
                assert len(isolated) == count+1 and not pending and not errors, (name, errors, list(pending))
                after = ref.ram_cpu()
                allowed = set(range(0x9400, 0x9480)) | set(range(0xff80, 0xffe4))
                unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                assert not unexpected, (name, unexpected)
                return isolated[-1]

            for seed, (stage, pause, slow, lifetime, size) in enumerate(itertools.product([0, 2, 1, 65535], [0, 128, 1], [0, 1, 255], [0, 1, 30, 31, 32, 33, 65534, 65535], [0, 16, 31, 64, 128])):
                setup('combo', seed, stage=stage, pause=pause, slow=slow, lifetime=lifetime, size=size)
                invoke('combo', f'combo-{stage}-{pause}-{slow}-{lifetime}-{size}')
            parent_configs = [(5, 0, 0, 65535), (5, 0, 1, 0), (5, 1, 0, 1), (0, 0, 0, 65535), (0, 0, 0, 1), (0, 0, 0, 128)]
            for seed, (stage, type_id, pause, slow, delay, config) in enumerate(itertools.product([0, 2, 65535], range(3), [0, 128, 1], [0, 1], [0, 1, 21, 42, 255], parent_configs)):
                action, index, ticks, stun = config
                setup('status', seed, stage=stage, type=type_id, pause=pause, slow=slow, delay=delay, action=action, parent_index=index, parent_ticks=ticks, stun=stun)
                invoke('status', f'status-{stage}-{type_id}-{pause}-{slow}-{delay}-{config}')
            for phase in range(1024):
                setup('status', phase, phase=phase, type=phase%3, character=phase%11, stun=1)
                invoke('status', f'orbit-phase-{phase}')
            for seed, (config, phase, size, stun) in enumerate(itertools.product([(0, 0), (0, 1), (0, 2), (1, 1), (1, 255), (2, 2), (255, 1)], [0, 0xffff], [0, 16, 31, 64, 128], [0, 0x8000, 1])):
                # The tuple already contains both animation index and countdown.
                child_index, child_ticks = config
                setup('status', seed, index=child_index, ticks=child_ticks, phase=phase, size=size, stun=stun)
                invoke('status', f'status-clock-{child_index}-{child_ticks}-{phase}-{size}-{stun}')
            boundary_count = len(isolated)
            for kind in ['combo', 'status']:
                for variant in range(6):
                    owner = setup(kind, variant, stage=0, type=variant%3, delay=[0, 21, 42][variant%3], lifetime=31 if variant == 4 else 65535 if variant == 5 else 0)
                    initial = snapshot(ref.ram_cpu(), 0xff9400, kind)
                    calls = []
                    for step in range(180):
                        external = {'x': ((100+step)<<16)|0xabcd, 'y': (320<<16)|0x1234, 'stun': 128 if step < 60 else 0,
                                    'action': 5 if variant == 4 and step == 6 else 0, 'parent_index': 0, 'parent_ticks': 0 if variant == 4 and step == 6 else 1,
                                    'pause': 128 if variant&1 and step%4 == 1 else 0, 'slow': 1 if variant&2 and step%4 == 2 else 0}
                        for off, value, size in [(0x40, external['x'], 4), (0x44, external['y'], 4), (0x84, external['stun'], 2),
                                                 (0x5d, external['action'], 1), (0x59, external['parent_index'], 1), (0x5b, external['parent_ticks'], 1)]:
                            write(0xff0000+owner+off, value, size)
                        write(0xfff550, external['pause'])
                        write(0xfff55a, external['slow'])
                        row = invoke(kind, f'stream-{kind}-{variant}/{step}')
                        row['external'] = external
                        calls.append(row)
                        if row['cleared_bytes']:
                            break
                    assert step < 179, (kind, variant)
                    streams.append({'kind': kind, 'initial': initial, 'calls': calls})
            identity = ref.identity
    for name, calls, scope in [('attached-effects-oracle', natural, 'Natural rule stages before renderer or complete deletion'),
                               ('attached-effects-boundaries', isolated[:boundary_count], 'Explicit original CPU effect rules'),
                               ('attached-effects-sequences', streams, 'Single-initial-state effect lifetimes with declared parent/world inputs')]:
        payload = {'library': immutable, 'calls': calls}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        rows = [call for stream in calls for call in stream['calls']] if name.endswith('-sequences') else calls
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'records': len(calls), 'calls': len(rows), 'kind_counts': dict(Counter(r['kind'] for r in rows)),
                  'draw_calls': sum(r['draw'] for r in rows), 'delete_calls': sum(bool(r['cleared_bytes']) for r in rows),
                  'silent_returns': sum(not r['draw'] and not r['cleared_bytes'] for r in rows), 'baselines': baselines,
                  'frames_match_original_core': all(b['frames_match_original_core'] for b in baselines), 'inputs_sha256': sha(input_raw),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'incomplete_calls': 0, 'errors': errors, 'pass': bool(calls) and not errors,
                  'limits': ['Rules stop before0x1580 renderer or after0x3556 deletion; not full sprite callback/world step',
                             'One retained effect slot is128bytes; wrapped/larger original clear aliasing is not modeled',
                             'Lifetimes consume declared external parent/world inputs without expected-state reinjection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
