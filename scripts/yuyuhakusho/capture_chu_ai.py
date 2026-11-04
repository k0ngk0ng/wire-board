#!/usr/bin/env python3
"""Complete original Chu AI calls: passive baseline, private boundaries, streams."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]
BEHAVIORS = [0x1d2fa, 0x1d312, 0x1cf4c, 0x1cf78, 0x1cfa4, 0x1cfd2, 0x1d000, 0x1d032,
             0x1d074, 0x1d0b6, 0x1d0cc, 0x1d0e2, 0x1d114, 0x1d14c, 0x1d186, 0x1d1cc, 0x1d212]
DECISIONS = [0x1e562, 0x1e5be, 0x1e5d0, 0x1e5fc, 0x1e60e, 0x1e64c, 0x1e65e, 0x1e670,
             0x1e682, 0x1e6ee, 0x1e748, 0x1e766, 0x1e7a2, 0x1e7fc, 0x1e836]
ALLOWED = {0x6b, 0x71, 0x72, 0x73, 0xce} | set(range(0xd0, 0xd8))


def number(raw, offset, size, signed=False):
    return int.from_bytes(raw[offset:offset+size], 'big', signed=signed)


def snapshot(ram, address):
    off = address&0xffff
    return {'actor': ram[off:off+256].hex(), 'world': ram[0x8400:0x9400].hex(),
            'seed': number(ram, 0xff18, 4), 'difficulty': number(ram, 0xff12, 2),
            'left': number(ram, 0xa010, 2), 'right': number(ram, 0xa012, 2)}


def next_random(seed):
    if seed == 0:
        seed = 0x2a6d365a
    value = (seed*41)&0xffffffff
    mixed = ((value&65535)+(value>>16))&65535
    return (mixed<<16)|(value&65535), mixed


def decision_seeds():
    """Select private initial seeds only; no expected decision is injected."""
    ordinary, rare = {}, []
    for seed in range(1, 100000):
        current = seed
        values = []
        for _ in range(5):
            current, value = next_random(current)
            values.append(value)
        if max(values[:2]) >= 58399:
            continue
        if values[2]&127 == 0:
            if len(rare) < 8:
                rare.append(seed)
        elif values[3]&7 == 0:
            ordinary.setdefault(values[4]&15, seed)
        if len(ordinary) == 16 and len(rare) == 8:
            break
    assert len(ordinary) == 16 and len(rare) == 8
    return list(ordinary.values())+rare


def probe_pointer_encoding(out):
    """Preserve the full-register distinction hidden by the 24-bit bus alias."""
    input_path = ROOT/'games/yuyuhakusho/engine/testdata/chu-ai-pointer-diagnostics.json'
    payload = json.loads(input_path.read_text())
    original_input = payload['input']
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*2)(0x1cc8a, 0x3d8)
    private = False
    natural_registers, samples = set(), []
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            if pc == 0x1cc8a and not private:
                natural_registers.add(ref.get_register(13))
            elif private and pc == 0x3d8:
                samples.append(snapshot(ref.ram_cpu(), 0x9000))
                ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, 2)
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
        expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

        cases = []
        for source_register in [0x00ff9000, 0xffff9000]:
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write(0xff8400, bytes.fromhex(original_input['world']))
            for address, field, size in [(0xffff18, 'seed', 4), (0xffff12, 'difficulty', 2), (0xffa010, 'left', 2), (0xffa012, 'right', 2)]:
                write(address, original_input[field].to_bytes(size, 'big'))
            write(0xffffe0, (0x3d8).to_bytes(4, 'big'))
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            setreg(13, source_register)
            setreg(16, 0x1cc8a)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = snapshot(ref.ram_cpu(), 0x9000)
            assert before == original_input
            count = len(samples)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(samples) == count+1
            cases.append({'source_register': hex(source_register), 'bus_address': hex(source_register&0xffffff),
                          'before': before, 'after': samples[-1], 'follow_target': hex(number(bytes.fromhex(samples[-1]['actor']), 0xd6, 2))})
        payload.update(identity=ref.identity, capture_sha256=sha(Path(__file__).read_bytes()),
                       frames_match_original_core=True, natural_source_registers=[hex(x) for x in sorted(natural_registers)], cases=cases,
                       limits=['The positive 24-bit source-register alias is a declared private diagnostic, not the original scheduler encoding',
                               'Game rule object identity uses canonical fighter IDs, not arbitrary CPU-register bus aliases'])
    (out/'chu-ai-pointer-diagnostics.json').write_text(json.dumps(payload, indent=2)+'\n')
    print(json.dumps({'natural_source_registers': payload['natural_source_registers'], 'follow_targets': [c['follow_target'] for c in cases], 'frames_match_original_core': True}))


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--pointer-diagnostic', action='store_true')
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    if args.pointer_diagnostic:
        probe_pointer_encoding(out)
        return
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [0x1cc8a, 0x156ee, 0x3d8, 0x3982, 0x1cd70, 0x1e1e4]+BEHAVIORS+DECISIONS
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, cases, streams, errors = [], [], [], []
    pending = None
    private = False
    label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @callback_type
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x1cc8a:
                    assert pending is None, 'Nested AI entry'
                    address = ref.get_register(13)&0xffffff
                    ram = ref.ram_cpu()
                    assert ram[(address&0xffff)+0x58] == 9, 'Only Chu AI is in scope'
                    pending = {'object_id': address&0xffff, 'source_register': hex(ref.get_register(13)), 'frame': ref.frame, 'entry_sp': ref.get_register(15),
                               'before': snapshot(ram, address), 'paths': [], 'rng_calls': 0}
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                elif pending is not None and pc not in (0x156ee, 0x3d8):
                    pending['paths'].append(hex(pc))
                elif pending is not None and pc == (0x3d8 if private else 0x156ee):
                    assert ref.get_register(15) == pending['entry_sp']+4, 'AI did not reach its real outer return'
                    ram = ref.ram_cpu()
                    pending.update(after=snapshot(ram, pending['object_id']), exit=hex(pc), exit_frame=ref.frame)
                    before, after = bytes.fromhex(pending['before']['actor']), bytes.fromhex(pending['after']['actor'])
                    unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in ALLOWED]
                    assert not unexpected, unexpected
                    if private:
                        pending['name'] = label
                        cases.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.append(pending)
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
        assert frames == baseline and not pending and not errors, (len(natural), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

        def write_bytes(address, raw):
            for i, byte in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

        def setup(serial=0, behavior=2, difficulty=6, cooldown=10, elapsed=9, facing=0, energy=100,
                  dx=100, dy=0, source_air=0, seed=1, cursor=6, eligible=1, attack=0, projectile=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write_bytes(0xff8400, bytes((i*73+serial*29+17)&255 for i in range(0x1000)))
            for i, owner in enumerate(FIGHTERS):
                for off, value, size in [(4, 256 if i == 3 or eligible&(1 << i) else 0, 2), (0x58, 9, 1), (0x6e, 0 if i == 3 else 1, 1),
                                         (0x78, 223, 2), (0x40, 100+(dx if i != 3 else 0), 2), (0x44, 320+source_air+(dy if i != 3 else 0), 2),
                                         (0x9c, 320, 2), (0x5c, 0x10 if attack else 0, 1), (0x64, attack, 1)]:
                    write(0xff0000+owner+off, value, size)
                for off, value, size in [(0x40, 100+projectile, 2), (0x44, 300 if projectile else 0, 2), (0x9c, 320, 2), (0x64, attack, 1)]:
                    write(0xff0000+owner+0x100+off, value, size)
            for off, value, size in [(0x61, facing, 1), (0x5f, [0, 0xa6][serial%2], 1), (0xb8, energy, 2),
                                     (0xce, behavior, 1), (0xd0, cursor, 2), (0xd2, cooldown, 2), (0xd4, elapsed, 2), (0xd6, 0x8400, 2)]:
                write(0xff9000+off, value, size)
            write(0xffff12, difficulty, 2)
            write(0xffff18, seed, 4)
            write(0xffa010, 32, 2)
            write(0xffa012, 600, 2)

        def invoke(name, source_id=0x9000):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xffff0000+source_id)
            setreg(16, 0x1cc8a)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(cases)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(cases) == count+1 and pending is None and not errors, (name, errors, pending is not None)
            after = ref.ram_cpu()
            allowed = set(range(source_id, source_id+256)) | set(range(0xff18, 0xff1c)) | set(range(0xff80, 0xffe4))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return cases[-1]

        serial = 0
        for behavior, difficulty, facing, cooldown, energy in itertools.product(range(17), [0, 2, 4, 6], [0, 1, 3], [0, 1, 2, 32768], [32, 100]):
            setup(serial, behavior=behavior, difficulty=difficulty, facing=facing, cooldown=cooldown, energy=energy,
                  elapsed=[0xffff, 0, 1, 7, 8, 0x7fff][serial%6], seed=[0, 1, 0x12345678, 0xffffffff][serial%4])
            invoke(f'behavior-{behavior}/difficulty-{difficulty}/facing-{facing}/cooldown-{cooldown}/energy-{energy}')
            serial += 1
        for cursor, eligible, variant in itertools.product([0, 2, 4, 6], range(8), ['ordinary', 'same-team', 'negative-health']):
            setup(serial, cursor=cursor, eligible=eligible, seed=serial*7919+1)
            if variant != 'ordinary':
                for i, owner in enumerate(FIGHTERS[:3]):
                    write(0xff0000+owner+(0x6e if variant == 'same-team' else 0x78), 0 if variant == 'same-team' else -1, 1 if variant == 'same-team' else 2)
            invoke(f'target-filter-{cursor}/{eligible}/{variant}')
            serial += 1
        seeds = decision_seeds()
        geometry = [(dx, dy, air) for dx in [-32768, -192, -128, -96, -64, -48, -32, -16, -1, 0, 15, 16, 31, 32, 47, 48, 63, 64, 95, 96, 127, 128, 191, 192, 32767]
                    for dy, air in [(0, 0), (-20, 0), (-72, 0), (-80, 0), (-20, -30), (20, -30), (64, -30)]]
        for (dx, dy, air), seed, facing in itertools.product(geometry, seeds, [0, 1, 3]):
            setup(serial, dx=dx, dy=dy, source_air=air, seed=seed, facing=facing)
            invoke(f'geometry-{dx}/{dy}/{air}/seed-{seed}/face-{facing}')
            serial += 1
        for attack, projectile, distance, seed in itertools.product([0, 4, 8, 12], [0, 1, 64, 65], [0, 127, 128, -32768], seeds[:4]):
            setup(serial, attack=attack, projectile=projectile, dx=distance, seed=seed)
            invoke(f'reaction-{attack}/{projectile}/{distance}/seed-{seed}')
            serial += 1
        for source_id, behavior, seed in itertools.product(FIGHTERS, range(17), seeds[:4]):
            setup(serial, behavior=behavior, elapsed=0xffff, seed=seed)
            source_bytes = ref.ram_cpu()[0x9000:0x9100]
            write_bytes(0xff0000+source_id, source_bytes)
            for i, owner in enumerate(FIGHTERS):
                write(0xff0000+owner+4, 256, 2)
                write(0xff0000+owner+0x6e, 0 if owner == source_id else 1)
                write(0xff0000+owner+0x40, 100 if owner == source_id else 180+i*16, 2)
            enemy_id = FIGHTERS[(FIGHTERS.index(source_id)+1)%4]
            write(0xff0000+source_id+0xd0, (FIGHTERS.index(enemy_id)*2-2)&7, 2)
            write(0xff0000+source_id+0xd6, enemy_id, 2)
            invoke(f'all-source-slots-{source_id:x}/{behavior}/seed-{seed}', source_id)
            serial += 1
        for behavior, difficulty, elapsed, lane in itertools.product([6, 9, 12, 16], [0, 2, 4, 6], [0xffff, 0, 1, 2, 7, 8, 15, 16, 31, 32, 0x7fff, 0x8000], [304, 320]):
            setup(serial, behavior=behavior, difficulty=difficulty, elapsed=elapsed, seed=seeds[serial%16])
            write(0xff849c, lane, 2)
            invoke(f'cross-lane-{behavior}/{difficulty}/{elapsed}/{lane}')
            serial += 1
        for team_pattern, seed in itertools.product([(0, 0, 1), (0, 1, 0), (1, 0, 0), (0, 0, 0)], seeds[:16]):
            setup(serial, behavior=16, eligible=7, elapsed=0xffff, seed=seed)
            for owner, team in zip(FIGHTERS[:3], team_pattern):
                write(0xff0000+owner+0x6e, team)
            invoke(f'follow-team-register-alias-{team_pattern}/seed-{seed}')
            serial += 1
        for health, energy, difficulty, cooldown in itertools.product([-32768, -1, 0, 223, 32767], [-32768, -1, 0, 32, 33, 222, 223, 32767], [0, 6], [0, 1]):
            setup(serial, behavior=12 if serial%2 else 2, difficulty=difficulty, energy=energy, cooldown=cooldown, seed=serial*391+1)
            write(0xff9078, health, 2)
            invoke(f'signed-resources-{health}/{energy}/{difficulty}/{cooldown}')
            serial += 1
        period_seeds = {}
        for seed in range(1, 1000000):
            _, value = next_random(seed)
            if value in (0, 0xfe00, 0xff00, 0xffff):
                period_seeds.setdefault(value, seed)
            if len(period_seeds) == 4:
                break
        assert len(period_seeds) == 4, period_seeds
        for value, seed in period_seeds.items():
            setup(serial, cooldown=0, difficulty=6, seed=seed)
            invoke(f'period-byte-wrap-{value:x}/seed-{seed}')
            serial += 1
        boundary_count = len(cases)
        for behavior in range(17):
            setup(serial, behavior=behavior, cooldown=0, elapsed=0xffff, seed=behavior*391+1)
            calls = []
            for step in range(24):
                # Only declared target position changes externally, not AI/RNG outputs.
                write(0xff8440, 80+(step%7)*24, 2)
                calls.append(invoke(f'stream-{behavior}/{step}'))
            streams.append({'initial': calls[0]['before'], 'calls': calls,
                            'external_target_x': [80+(step%7)*24 for step in range(24)]})
            serial += 1
        identity = ref.identity
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    tables = {name: [number(rom, off+i*2, 2) for i in range(4)] for name, off in
              [('period_scales', 0x1cc82), ('decision_thresholds', 0x1cd60), ('reaction_thresholds', 0x1cd68), ('lane_masks', 0x1d294)]}
    for name, records, scope in [('chu-ai-oracle', natural, 'Complete natural Chu AI returns'),
                                  ('chu-ai-boundaries', cases[:boundary_count], 'Declared isolated complete Chu AI returns'),
                                  ('chu-ai-sequences', streams, 'Single-initial-state AI streams with declared target movement')]:
        raw = (json.dumps({'tables': tables, 'calls': records}, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        rows = [call for stream in records for call in stream['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'filter_sha256': sha(bridge_path.read_bytes()),
                  'records': len(records), 'calls': len(rows), 'paths': dict(Counter(path for row in rows for path in row['paths'])),
                  'rng_calls': dict(Counter(str(row['rng_calls']) for row in rows)), 'frames_match_original_core': frames == baseline,
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'errors': errors, 'incomplete_calls': 0, 'pass': bool(records) and not errors,
                  'limits': ['Source9 complete AI functions only; not all-character strategies, actor/world steps or complete matches',
                             'Isolated calls declare private original CPU input replacement; streams change only target X externally']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
