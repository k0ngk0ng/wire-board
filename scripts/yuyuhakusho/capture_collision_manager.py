#!/usr/bin/env python3
"""Full original collision manager, including all consumers and tail state."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_clash import number


def snapshot(ram, owners=None):
    counts = [number(ram, 0xa004+i*2, 2) for i in range(5)]
    assert all(c <= 256 and c % 16 == 0 for c in counts[:4])
    assert counts[4] <= 512 and counts[4] % 32 == 0
    if owners is None:
        owners = set(range(0x8400, 0x9400, 256))
        for group in range(4):
            owners.update(number(ram, 0xa200+group*256+i+12, 2) for i in range(0, counts[group], 16))
        owners.update(number(ram, 0xa600+i+12, 2) for i in range(0, counts[4], 32))
    assert all(0x8000 <= owner <= 0xff00 for owner in owners)
    return {'actors': {str(owner): ram[owner:owner+256].hex() for owner in sorted(owners)},
            'buffers': ram[0xa200:0xa600].hex(), 'throws': ram[0xa600:0xa800].hex(), 'counts': counts,
            'effects': ram[0x9400:0x9c00].hex(), 'cursor': number(ram, 0xa036, 2), 'pending_sound': ram[0xa030],
            'sound_queue': list(ram[0xf80a:0xf80e]), 'pause_flags': ram[0xf550],
            'slow_ticks': number(ram, 0xf558, 2), 'slow_flag': ram[0xf55a], 'combo_flags': ram[0xa0a4],
            'arena_left': number(ram, 0xa010, 2), 'arena_right': number(ram, 0xa012, 2)}


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
    points = [0x1edac, 0x1ee1c, 0x37c56, 0x37cac]
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows, cases, sequences, errors = [], [], [], []
    pending = None
    private = False
    label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x1edac:
                    assert pending is None, 'Nested manager entry'
                    pending = {'frame': ref.frame, 'before': snapshot(ref.ram_cpu()), 'external_queue_clears': []}
                elif pc in (0x37c56, 0x37cac):
                    if pending is not None:
                        address = ref.get_register(9)&0xffff
                        pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(address), 'before': ref.ram_cpu()[address]})
                elif pc == 0x1ee1c:
                    assert pending is not None, 'Manager return without entry'
                    owners = [int(owner) for owner in pending['before']['actors']]
                    pending.update(after=snapshot(ref.ram_cpu(), owners), exit_frame=ref.frame)
                    if private:
                        pending['name'] = label
                        cases.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        rows.append(pending)
                    pending = None
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
        assert frames == expected and not pending and not errors, (len(rows), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            data = (value & ((1 << (size*8))-1)).to_bytes(size, 'big')
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def invoke(name):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xff8100)
            setreg(16, 0x1edac)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before_ram = ref.ram_cpu()
            count = len(cases)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(cases) == count+1 and pending is None and not errors, (name, errors)
            after_ram = ref.ram_cpu()
            allowed = set(range(0x8400, 0x9c00)) | set(range(0xa004, 0xa00e)) | set(range(0xa200, 0xa800)) | {0xa030, 0xa036, 0xa037, 0xa0a4, 0xf558, 0xf559, 0xf55a} | set(range(0xf80a, 0xf80e)) | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before_ram, after_ram)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return cases[-1]

        def setup(presence=0, pause=0, slow=0, flag=0, queue=(0, 0, 0, 0), guard=False, health=100, same_team=False, lane=False, seed=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            for index, owner in enumerate([0x8400, 0x9000]):
                raw = bytearray((i*73+index*47+seed*11+17)&255 for i in range(256))
                write_bytes(0xff0000+owner, raw)
                for off, value, size in [(0x40, ((100+index*12)<<16)|0xabcd, 4), (0x44, 320<<16, 4), (0x9c, (320+(index if lane else 0))<<16, 4),
                                         (0x58, index*9, 1), (0x59, 0, 1), (0x5a, 8, 1), (0x5b, 0, 1), (0x5c, 0, 1), (0x5d, 0, 1), (0x5f, 0x20 if seed%3 == 0 else 0, 1),
                                         (0x61, index, 1), (0x63, 4 if guard else 0, 1), (0x64, 4 if guard else seed%4, 1), (0x65, 17, 1), (0x66, 9, 1), (0x67, 1, 1), (0x68, seed%11, 1),
                                         (0x6c, 0, 1), (0x6e, 0 if same_team else index, 1), (0x70, index, 1), (0x71, 8, 1), (0x78, health, 2), (0x7e, 100, 2),
                                         (0x94, 0xffffffff, 4), (0xc6, 0, 1), (0xc8, 0, 1)]:
                    write(0xff0000+owner+off, value, size)
            raw = bytearray((i*19+seed*31+91)&255 for i in range(0x600))
            counts = [16 if presence&4 else 0, 32 if presence&2 else 0, 16 if presence&5 else 0, 32 if presence&8 else 0, 32 if presence&1 else 0]
            for group in range(4):
                for i in range(counts[group]//16):
                    owner = 0x9000 if group == 2 or i == 1 else 0x8400
                    pos = group*256+i*16
                    for off, value, size in [(0, 112 if owner == 0x9000 else 100, 2), (4, 320, 2), (8, 16, 1), (9, 24, 1), (12, owner, 2)]:
                        raw[pos+off:pos+off+size] = value.to_bytes(size, 'big')
            for off, value, size in [(0, 100, 2), (2, 320, 2), (4, 16, 1), (5, 24, 1), (6, 0x46, 1), (7, 0x47, 1), (8, 12, 1), (9, 13, 1), (10, 1, 1), (12, 0x8400, 2), (16, 20, 2), (18, 0, 2)]:
                raw[1024+off:1024+off+size] = value.to_bytes(size, 'big')
            write_bytes(0xffa200, raw)
            for i, count in enumerate(counts):
                write(0xffa004+i*2, count, 2)
            raw_pool = bytearray((i*37+seed*23+13)&255 for i in range(0x800))
            for i in range(16):
                raw_pool[i*128+4:i*128+6] = (0x80 if seed%5 == 4 else 0).to_bytes(2, 'big')
            write_bytes(0xff9400, raw_pool)
            write(0xffa036, 0xff80 if seed%3 == 0 else 0, 2)
            write(0xffa030, 0x97 if seed%2 else 0)
            write_bytes(0xfff80a, bytes(queue))
            write(0xfff550, pause)
            write(0xfff558, slow, 2)
            write(0xfff55a, flag)
            write(0xffa0a4, 0xc3)
            write(0xffa010, 32, 2)
            write(0xffa012, 600, 2)

        for i, presence in enumerate(range(16)):
            for variant in range(6):
                setup(presence=presence, guard=variant == 1, health=0 if variant == 2 else 100,
                      same_team=variant == 3, lane=variant == 4, pause=0x80 if variant == 5 else 0,
                      queue=(0x80, 0x81, 0x82, 0x83), slow=3, flag=255, seed=i*6+variant)
                invoke(f'consumer-mask-{presence:x}/variant-{variant}')
        for i, (slow, flag, queue, pause) in enumerate(itertools.product([0, 1, 2, 3, 4, 0xa0, 0xffff], [0, 1, 255],
                                                   [(0, 0, 0, 0), (0x80, 0x81, 0x82, 0x83), (0x80, 0x97, 0x82, 0x83), (0x80, 0x81, 0, 0x83)], [0, 0x80])):
            setup(slow=slow, flag=flag, queue=queue, pause=pause, seed=i)
            invoke(f'tail-slow-{slow}/flag-{flag}/queue-{queue}/pause-{pause}')
        for presence in [0, 4, 6, 13, 15]:
            setup(presence=presence, slow=0xa0, flag=255, seed=presence)
            stream = []
            for step in range(10):
                # No restoration or expected-state injection between calls.
                stream.append(invoke(f'sequence-{presence}/step-{step}'))
            sequences.append({'name': f'initial-mask-{presence:x}', 'initial': stream[0]['before'], 'calls': stream})
        identity = ref.identity
    for fixture_name, records, scope in [('collision-manager-oracle', rows, 'Passive complete original manager calls'),
                                         ('collision-manager-boundaries', cases[:-50], 'Declared isolated complete original manager calls'),
                                         ('collision-manager-sequences', sequences, 'Single-initial-state isolated manager streams')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(fixture_name+'.json.gz')).write_bytes(compressed)
        calls = [call for stream in records for call in stream['calls']] if fixture_name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'records': len(records), 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(bridge_path.read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
                  'frames_match_original_core': frames == expected, 'incomplete_calls': 0, 'errors': errors,
                  'external_queue_calls': sum(bool(r['external_queue_clears']) for r in calls), 'pass': bool(records) and not errors,
                  'limits': ['Complete manager calls, not full actor/world steps or matches', 'The 512-byte throw capture is not producer-capacity evidence']}
        (out/(fixture_name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
