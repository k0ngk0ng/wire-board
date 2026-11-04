#!/usr/bin/env python3
"""Complete source8 selector returns; explicit private inputs and passive baseline."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_attacks import snapshot as chu_snapshot

ENTRIES = {0x17adc: 'ground', 0x17c1e: 'air', 0x17c94: 'ground-special', 0x17d5e: 'air-special'}
SPECIAL = [0x197c2, 0x197ec, 0x19804, 0x1981c, 0x19846, 0x19860, 0x19896, 0x198b0, 0x198ca, 0x198fc]
CONSTRUCTORS = set(SPECIAL + [0x17b8c, 0x18812, 0x18828, 0x1883e, 0x18854, 0x1886a, 0x18880, 0x188d8, 0x188ee, 0x18930])
NATURAL_RETURNS = {0x15b18, 0x15b24, 0x15b30, 0x15b3c}
FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]
ACTOR_WRITES = {0x48, 0x49, 0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60, 0x61, 0x64, 0x6b, 0x6c, 0x6d, 0x6e, 0x77, 0xb8, 0xb9} | set(range(0x10, 0x18))
DEPENDENCIES = ['scripts/yuyuhakusho/capture_chu_attacks.py', 'scripts/yuyuhakusho/capture_ground_idle.py',
                'scripts/yuyuhakusho/capture_jump.py', 'scripts/yuyuhakusho/capture_ground_actions.py', 'scripts/yuyuhakusho/capture_effect_pool.py',
                'scripts/yuyuhakusho/reference.py', 'docs/research/md-yuyuhakusho/probe.py']


class Rows:
    def __init__(self, path):
        self.path = path
        self.file = path.open('w')
        self.count = 0
        self.kinds, self.constructors, self.sources = Counter(), Counter(), Counter()

    def add(self, row):
        self.file.write(json.dumps(row, separators=(',', ':'))+'\n')
        self.count += 1
        self.kinds[row['kind']] += 1
        self.constructors.update(row['constructors'])
        self.sources[hex(row['object_id'])] += 1

    def close(self):
        self.file.close()


def snapshot(ram, address, rom):
    offset = address & 0xffff
    actor = ram[offset:offset+256]
    return chu_snapshot(ram, address, rom) | {
        'world_bytes': ram[0x8400:0x9400].hex(),
        'children_bytes': ram[offset+256:offset+1024].hex(),
        'energy_fraction': int.from_bytes(actor[0xba:0xbc], 'big'),
        'random_seed': int.from_bytes(ram[0xff18:0xff1c], 'big'),
        'aux48': int.from_bytes(actor[0x48:0x4c], 'big'),
        'aux48_high': actor[0x48],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--inputs', type=Path, default=ROOT/'docs/research/md-yuyuhakusho/source8-next-inputs.json')
    parser.add_argument('--baseline', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter8-attacks-reference-frames.json')
    parser.add_argument('--natural-only', action='store_true')
    parser.add_argument('--natural-entry-limit', type=int, default=3600)
    parser.add_argument('--private-context', type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    assert args.inputs.resolve().is_relative_to(ROOT) and args.baseline.resolve().is_relative_to(ROOT)
    assert not (args.private_context and args.natural_only)
    private_bundle = None
    if args.private_context:
        assert args.private_context.resolve().is_relative_to(ROOT)
        private_bundle = json.loads(gzip.decompress(args.private_context.read_bytes()))
    expected = None if private_bundle else json.loads(args.baseline.read_text())
    events = None if private_bundle else json.loads(args.inputs.read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES) | CONSTRUCTORS | NATURAL_RETURNS | {0x3d8, 0x3982})
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    natural = Rows(out/'natural.jsonl')
    oracle = Rows(out/'oracle.jsonl')
    boundaries = Rows(out/'boundaries.jsonl')
    streams = []
    pending = None
    private = False
    destination = None
    label = ''
    last = None
    errors = []
    rng_calls = 0
    initializing = False
    initialization = None
    bindings = None

    with ReferenceSession(ROOT, out, core=core) as ref:
        @callback_type
        def observe(kind, width, pc, value):
            nonlocal pending, last, rng_calls, initialization
            try:
                address = ref.get_register(13) & 0xffffff
                sp = ref.get_register(15)
                if initializing:
                    if pc == 0x3d8:
                        assert sp == 0xffff04
                        initialization = {'actor': ref.ram_cpu()[0x8400:0x8500].hex(), 'pc': hex(pc), 'sp': sp}
                        ref.cpu.cycles = 4000000
                    return
                if pc in ENTRIES:
                    if not private and args.natural_entry_limit and ref.frame > args.natural_entry_limit:
                        return
                    assert pending is None, 'Overlapping source8 selector'
                    ram = ref.ram_cpu()
                    assert ram[(address & 0xffff)+0x58] == 8
                    ret = int.from_bytes(ram[sp & 0xffff:(sp & 0xffff)+4], 'big')
                    assert ret == 0x3d8 if private else ret in NATURAL_RETURNS, hex(ret)
                    pending = {'kind': ENTRIES[pc], 'object_id': address & 0xffff,
                               'source_character_id': 8, 'source_register': hex(ref.get_register(13)),
                               'entry_pc': hex(pc), 'entry_sp': sp, 'return_pc': hex(ret),
                               'frame': ref.frame, 'before': snapshot(ram, address, rom),
                               'constructors': [], 'rng_calls': 0, 'context': 'declared-private' if private else 'natural'}
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                    rng_calls += 1
                elif pending is not None and pc in CONSTRUCTORS:
                    pending['constructors'].append(hex(pc))
                elif pending is not None and pc == int(pending['return_pc'], 16):
                    assert sp == pending['entry_sp']+4 and address & 0xffff == pending['object_id']
                    pending.update(after=snapshot(ref.ram_cpu(), address, rom), exit=hex(pc), exit_sp=sp, exit_frame=ref.frame)
                    before = bytes.fromhex(pending['before']['actor_bytes'])
                    after = bytes.fromhex(pending['after']['actor_bytes'])
                    assert not [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in ACTOR_WRITES]
                    assert pending['before']['children_bytes'] == pending['after']['children_bytes']
                    assert pending['before']['energy_fraction'] == pending['after']['energy_fraction']
                    assert pending['before']['random_seed'] == pending['after']['random_seed']
                    assert pending['rng_calls'] == 0
                    last = pending
                    pending = None
                    if private:
                        last['name'] = label
                        if destination is not None:
                            destination.add(last)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.add(last)
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        checkpoint = None
        for frame in range(1, len(expected)+1 if expected is not None else 1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        if private_bundle:
            checkpoint = dict(private_bundle['checkpoint'], data=bytes.fromhex(private_bundle['checkpoint']['data']))
            assert checkpoint['core_sha256'] == ref.identity['core_sha256']
            assert pending is None and not errors
        else:
            assert frames == expected and pending is None and not errors, errors[:3]

        if not args.natural_only:
            assert checkpoint is not None
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write(address, value, size=1):
                for i, byte in enumerate((value & ((1 << (size*8))-1)).to_bytes(size, 'big')):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i) & 0xffff) ^ 1)).value = byte

            def write_bytes(address, raw):
                for i, byte in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i) & 0xffff) ^ 1)).value = byte

            # Source8 resources come from its actual original initializer,
            # never from a manually chosen data pointer or a menu claim.
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            setreg(17, 0x2700)
            setreg(15, 0xffff00)
            write(0xffff00, 0x3d8, 4)
            setreg(13, 0xffff8400)
            write(0xff8458, 8)
            setreg(16, 0x126c8)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            initializing = True
            hook(observer)
            execute(2000000)
            hook(None)
            initializing = False
            assert initialization is not None and not errors
            raw_actor = bytes.fromhex(initialization['actor'])
            offsets = {'graphics_set': 0x1c, 'tiles_base': 0x20, 'animation_set': 0x50, 'shape_set': 0x8c, 'movement_parameters': 0x90}
            bindings = {name: int.from_bytes(raw_actor[off:off+4], 'big') for name, off in offsets.items()}
            pinned = json.loads((ROOT/'games/yuyuhakusho/data/animation-catalog-manifest.json').read_text())['initializer_checks'][8]['observed']
            assert bindings == pinned
            (out/'fighter8-attacks-initializer.json').write_text(json.dumps({'scope': 'Declared private original source8 initialization, not natural menu selection', 'entry': '0x126c8', 'source_id': 8, 'before_checkpoint_frame': checkpoint['frame'], 'result': initialization, 'bindings': bindings, 'identity': ref.identity}, indent=2)+'\n')

            serial = 0

            def setup(request=0, direction=0, press=0, changed=0, facing=0, pose=0, energy=255, health=100,
                      pool_mode=None, cursor=None, fraction=None, aux=None, source=None, child=None):
                nonlocal serial
                serial += 1
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for reg in range(15):
                    setreg(reg, 0)
                source = source or FIGHTERS[serial % 4]
                cursor = [0, 0x780, 0xff80][serial % 3] if cursor is None else cursor
                pool_mode = serial % 4 if pool_mode is None else pool_mode
                fraction = [0, 1, 0xffff][serial % 3] if fraction is None else fraction
                write_bytes(0xff8400, bytes((i*73+serial*29+17) & 255 for i in range(4096)))
                for offset, value, size in [(0x58, 8, 1), (0x5f, pose, 1), (0x61, facing, 1), (0x6b, request, 1),
                                             (0x71, direction | [0, 0x10, 0x70, 0xf0][serial % 4], 1),
                                             (0x72, press, 1), (0x73, changed, 1), (0x78, health, 2),
                                             (0xb8, energy, 2), (0xba, fraction, 2),
                                             (0x40, [0x7fff1234, 0x80004321, 0xffffabcd, 0x00107654][serial % 4], 4),
                                             (0x44, [0x80004321, 0x7fff1234, 0x00207654, 0xffffabcd][serial % 4], 4)]:
                    write(0xff0000+source+offset, value, size)
                for name, offset in offsets.items():
                    write(0xff0000+source+offset, bindings[name], 4)
                write(0xff0000+source+0x104, (0 if serial % 3 else 0x100) if child is None else child, 2)
                if aux is not None:
                    write(0xff0000+source+0x48, aux, 4)
                write(0xffff18, (serial*0x9e3779b9) & 0xffffffff, 4)
                write_bytes(0xffa600, bytes((i*19+serial*17+91) & 255 for i in range(512)))
                write(0xffa00c, (serial % 16)*32, 2)
                raw = bytearray((i*37+serial*23+13) & 255 for i in range(2048))
                current = (cursor & 0x7ff)//128
                for i in range(16):
                    available = pool_mode == 0 or pool_mode == 2 and i == (current+1) % 16 or pool_mode == 3 and i == current
                    raw[i*128+4:i*128+6] = (0 if available else 0x80).to_bytes(2, 'big')
                write_bytes(0xff9400, raw)
                write(0xffa036, cursor, 2)
                return source

            def invoke(kind, source, name):
                nonlocal label, last
                label, last = name, None
                setreg(17, 0x2700)
                setreg(15, 0xffff00)
                write(0xffff00, 0x3d8, 4)
                setreg(13, 0xffff0000+source)
                setreg(16, next(pc for pc, value in ENTRIES.items() if value == kind))
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                before = ref.ram_cpu()
                hook(observer)
                execute(2000000)
                hook(None)
                assert last is not None and pending is None and not errors, (name, errors[:3])
                after = ref.ram_cpu()
                allowed = set(range(source, source+256)) | set(range(0x9400, 0x9c00)) | set(range(0xa600, 0xa800)) | {0xa00c, 0xa00d, 0xa036, 0xa037} | set(range(0xfed0, 0xff04))
                assert not [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed], name
                return last

            destination = oracle
            plans = [('ground', dict(direction=8, press=64)), ('ground', dict(request=7)),
                     ('ground', dict(request=8)), ('ground', dict(direction=2, press=80)),
                     ('air', dict(direction=8, press=64)), ('air', dict(direction=8, press=80)),
                     ('air', dict(direction=0, press=64))]
            for kind, request, changed in [('ground-special', 1, 0),
                                            ('ground-special', 2, 32), ('ground-special', 2, 16), ('ground-special', 2, 64),
                                            ('ground-special', 3, 16), ('ground-special', 3, 64),
                                            ('ground-special', 4, 32), ('ground-special', 4, 16), ('ground-special', 4, 64),
                                            ('ground-special', 6, 64), ('air-special', 3, 16), ('air-special', 3, 64)]:
                plans.append((kind, dict(request=request, changed=changed, health=111, child=0, aux=0xa5d3c791)))
                plans.append((kind, dict(request=request, changed=changed, energy=0, health=111, child=0, aux=0xa5d3c791)))
            for index, (kind, values) in enumerate(plans):
                source = setup(**values)
                invoke(kind, source, f'complete-{index}-{kind}')

            destination = boundaries
            for request, pose, direction, press, facing in itertools.product([0, 7, 8], [0, 11], range(16), [0, 16, 64, 80], [0, 1, 3, 128]):
                source = setup(request=request, pose=pose, direction=direction, press=press, facing=facing)
                invoke('ground', source, f'ground-{request}-{pose}-{direction}-{press}-{facing}')
            for direction, press, facing in itertools.product(range(16), [0, 16, 32, 64, 48, 80, 96, 112], [0, 1, 3, 128]):
                source = setup(direction=direction, press=press, facing=facing)
                invoke('air', source, f'air-{direction}-{press}-{facing}')
            for kind, request, direction, changed, facing in itertools.product(['ground-special', 'air-special'], range(9), [0, 1, 4, 8, 15], [0, 16, 32, 64, 48, 80, 96, 112, 255], [0, 1, 3, 128]):
                source = setup(request=request, direction=direction, changed=changed, facing=facing,
                               energy=[-1, 0, 255][serial % 3], health=[-1, 111, 112][serial % 3])
                invoke(kind, source, f'special-{kind}-{request}-{direction}-{changed}-{facing}')
            costs = [('ground-special', 1, 0, 24),
                     ('ground-special', 2, 32, 8), ('ground-special', 2, 16, 16), ('ground-special', 2, 64, 24),
                     ('ground-special', 3, 16, 16), ('ground-special', 3, 64, 16),
                     ('ground-special', 4, 32, 8), ('ground-special', 4, 16, 8), ('ground-special', 4, 64, 8),
                     ('ground-special', 6, 64, 200), ('air-special', 3, 16, 16), ('air-special', 3, 64, 16)]
            for kind, request, changed, cost in costs:
                for energy, health, facing, fraction, child in itertools.product([-32768, -1, 0, cost-1, cost, cost+1, 32767], [-32768, 111, 112], [0, 1, 3, 128], [0, 1, 65535], [0, 0x100]):
                    source = setup(request=request, changed=changed, energy=energy, health=health,
                                   facing=facing, fraction=fraction, child=child, aux=[0, 0xff123456, 0xa5d3c791][fraction % 3])
                    invoke(kind, source, f'cost-{kind}-{request}-{changed}-{energy}-{health}-{facing}-{fraction}-{child}')
            for aux, energy, health, changed, facing in itertools.product([0, 0x12345678, 0xffffffff, 0x80008000, 0x0000ffff, 0xffff0000], [0, 199, 200, 201], [111, 112], [0, 64, 112], [0, 1, 3]):
                source = setup(request=6, aux=aux, energy=energy, health=health, changed=changed, facing=facing)
                invoke('ground-special', source, f'aux48-word-{aux}-{energy}-{health}-{changed}-{facing}')
            for request, facing, pool_mode, cursor in itertools.product([7, 8], [0, 1, 3, 128], range(4), [0, 0x80, 0x780, 0xff80]):
                source = setup(request=request, facing=facing, pool_mode=pool_mode, cursor=cursor)
                invoke('ground', source, f'dash-pool-{request}-{facing}-{pool_mode}-{cursor}')
            for kind, channel, byte in itertools.product(['ground', 'air', 'ground-special', 'air-special'], ['held', 'pressed', 'changed'], range(256)):
                source = setup(request=2 if kind == 'ground-special' else 3 if kind == 'air-special' else 0, press=64, changed=64)
                write(0xff0000+source+{'held': 0x71, 'pressed': 0x72, 'changed': 0x73}[channel], byte)
                invoke(kind, source, f'input-byte-{kind}-{channel}-{byte}')

            destination = None
            for kind, facing in itertools.product(ENTRIES.values(), [0, 1, 3, 128]):
                source = setup(request=8 if kind == 'ground' else 6 if kind == 'ground-special' else 3 if kind == 'air-special' else 0,
                               energy=255, health=111, facing=facing, pool_mode=0, cursor=0xff80, child=0, aux=0xa5d3c791)
                write(0xffa00c, 0, 2)
                calls = []
                for step in range(20):
                    held = [0, 2, 4, 8, 10][step % 5]
                    press = [0, 64, 80, 16][step % 4]
                    changed = [0, 16, 64, 112][step % 4]
                    for offset, value in [(0x71, held), (0x72, press), (0x73, changed)]:
                        write(0xff0000+source+offset, value)
                    row = invoke(kind, source, f'stream-{kind}-{facing}/{step}')
                    row['external_controls'] = {'held': held, 'pressed': press, 'changed': changed}
                    calls.append(row)
                streams.append({'kind': kind, 'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity

    for rows in [natural, oracle, boundaries]:
        rows.close()
    datasets = [('fighter8-attacks-natural', natural, 'Passive natural source8 complete selector returns'),
                ('fighter8-attacks-oracle', oracle, 'Declared-private representative complete source8 selector returns'),
                ('fighter8-attacks-boundaries', boundaries, 'Declared-private source8 complete selector boundary matrix'),
                ('fighter8-attacks-sequences', streams, 'Declared-private single-initial-state source8 selector streams')]
    for name, source, scope in datasets:
        if private_bundle and name.endswith('-natural'):
            continue
        if args.natural_only and not name.endswith('-natural'):
            continue
        path = out/(name+'.json.gz')
        digest = __import__('hashlib').sha256()
        records = source.count if isinstance(source, Rows) else len(source)
        with path.open('wb') as file:
            with gzip.GzipFile(fileobj=file, mode='wb', mtime=0) as zipped:
                def emit(value):
                    digest.update(value)
                    zipped.write(value)
                emit(b'{"calls":[')
                if isinstance(source, Rows):
                    with source.path.open('rb') as rows:
                        for index, line in enumerate(rows):
                            if index:
                                emit(b',')
                            emit(line.rstrip(b'\n'))
                else:
                    for index, row in enumerate(source):
                        if index:
                            emit(b',')
                        emit(json.dumps(row, separators=(',', ':')).encode())
                emit(b']}\n')
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'capture_dependencies': [{'path': dep, 'sha256': sha((ROOT/dep).read_bytes())} for dep in DEPENDENCIES],
                  'input': None if private_bundle else {'path': str(args.inputs.resolve().relative_to(ROOT)), 'sha256': sha(args.inputs.read_bytes())},
                  'baseline': None if private_bundle else {'path': str(args.baseline.resolve().relative_to(ROOT)), 'sha256': sha(args.baseline.read_bytes()), 'frames': len(frames)},
                  'private_context': {'path': str(args.private_context.resolve().relative_to(ROOT)), 'sha256': sha(args.private_context.read_bytes()), 'original_checkpoint_frame': checkpoint['frame']} if private_bundle else None,
                  'private_context_verified': bool(private_bundle and bindings is not None), 'source8_initializer_bindings': bindings,
                  'records': records, 'calls': records if isinstance(source, Rows) else sum(len(stream['calls']) for stream in source),
                  'kind_counts': dict(source.kinds) if isinstance(source, Rows) else dict(Counter(stream['kind'] for stream in source)),
                  'constructor_counts': dict(source.constructors) if isinstance(source, Rows) else {},
                  'source_objects': dict(source.sources) if isinstance(source, Rows) else {},
                  'fixture_sha256': digest.hexdigest(), 'gzip_sha256': sha(path.read_bytes()),
                  'natural_entry_limit': args.natural_entry_limit or len(frames), 'frames_match_original_core': expected is not None and frames == expected, 'incomplete_calls': 0, 'errors': errors,
                  'rng_calls': rng_calls, 'pass': (bool(private_bundle and bindings is not None) or frames == expected) and not errors and (bool(records) or name.endswith('-natural')),
                  'limits': ['Private contexts do not establish natural gameplay', 'Natural source8 count is explicit, including zero',
                             'Selection and initialization only; no continued motion, collision or full matches',
                             'Streams change declared base controls only; no expected intermediate state or RNG injection']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps({key: value for key, value in report.items() if key not in ['capture_dependencies', 'input', 'baseline', 'limits']}, indent=2), flush=True)


if __name__ == '__main__':
    main()
