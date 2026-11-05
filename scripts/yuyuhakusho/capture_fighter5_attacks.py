#!/usr/bin/env python3
"""Source5 complete selectors/constructors, dirty inputs and full-RAM audit."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_attacks import snapshot as chu_snapshot

ENTRIES = {0x1705c: 'ground', 0x1719e: 'air', 0x17214: 'ground-special', 0x172c4: 'air-special'}
DIRECT = {0x1926a: 'combination-high', 0x192ae: 'combination-low',
          0x19408: 'final-0', 0x19426: 'final-1', 0x19444: 'final-2'}
SPECIAL = [0x19216, 0x19240, 0x19284, 0x192c8, 0x192f6, 0x19324,
           0x19364, 0x193a4, 0x193bc, 0x193d4]
SPECIAL_COST = dict(zip(SPECIAL, [8, 16, 16, 4, 4, 4, 4, 50, 50, 50]))
CONSTRUCTORS = set(SPECIAL + [0x1710c, 0x18812, 0x18828, 0x1883e, 0x18854,
                             0x1886a, 0x18880, 0x188d8, 0x188ee, 0x18930])
SELECTOR_RETURNS = {0x15b18, 0x15b24, 0x15b30, 0x15b3c}
DIRECT_RETURNS = {0x1b6ec, 0x1b6f4, 0x1b9a8, 0x1b9b0, 0x1b9b8}
IRQ_START = 0xfff000
IRQ_END = {0xfff09a, 0xfff0a8, 0xfff13a}
FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]
ACTOR_WRITES = {0x48, 0x49, 0x4c, 0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60,
                0x61, 0x64, 0x6b, 0x6c, 0x6d, 0x6e, 0x77, 0xb8, 0xb9} | set(range(0x10, 0x18))
DEPENDENCIES = ['scripts/yuyuhakusho/capture_chu_attacks.py', 'scripts/yuyuhakusho/capture_ground_idle.py',
                'scripts/yuyuhakusho/capture_jump.py', 'scripts/yuyuhakusho/capture_effect_pool.py',
                'scripts/yuyuhakusho/reference.py', 'docs/research/md-yuyuhakusho/probe.py']


def changes(before, after):
    return [{'offset': i, 'before': a, 'after': b}
            for i, (a, b) in enumerate(zip(before, after)) if a != b]


class Rows:
    def __init__(self, path):
        self.path = path
        self.file = gzip.open(path, 'wt')
        self.count = 0
        self.kinds, self.constructors, self.sources = Counter(), Counter(), Counter()
        self.successful = Counter()
        self.interrupts = 0

    def add(self, row):
        self.file.write(json.dumps(row, separators=(',', ':'))+'\n')
        self.count += 1
        self.kinds[row['kind']] += 1
        self.constructors.update(row['constructors'])
        spent = row['before']['energy']-row['after']['energy']
        self.successful.update(pc for pc in row['constructors']
                               if int(pc, 16) in DIRECT or SPECIAL_COST.get(int(pc, 16)) == spent)
        self.sources[hex(row['object_id'])] += 1
        self.interrupts += len(row['interrupts'])

    def close(self):
        self.file.close()


def snapshot(ram, address, rom):
    off = address & 0xffff
    actor = ram[off:off+256]
    return chu_snapshot(ram, address, rom) | {
        'world_bytes': ram[0x8400:0x9400].hex(),
        'children_bytes': ram[off+256:off+1024].hex(),
        'energy_fraction': int.from_bytes(actor[0xba:0xbc], 'big'),
        'random_seed': int.from_bytes(ram[0xff18:0xff1c], 'big'),
        'alternate_velocity': int.from_bytes(actor[0x48:0x4c], 'big', signed=True),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--reference-manifest', required=True, type=Path)
    parser.add_argument('--natural-only', action='store_true')
    parser.add_argument('--natural-name', default='fighter5-attacks-natural')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    assert args.natural_name in ['fighter5-attacks-natural', 'fighter5-attacks-special-natural']
    out.mkdir(parents=True, exist_ok=True)
    control_path = args.reference_manifest.resolve()
    assert control_path.is_relative_to(ROOT)
    control = json.loads(control_path.read_text())
    assert control['observer_attached'] is False and control['frames'] == 3602
    assert control['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    input_path, baseline_path = (ROOT/control[k]['path'] for k in ['input', 'output'])
    assert input_path.resolve().is_relative_to(ROOT) and baseline_path.resolve().is_relative_to(ROOT)
    assert sha(input_path.read_bytes()) == control['input']['sha256']
    assert sha(baseline_path.read_bytes()) == control['output']['sha256']
    expected = json.loads(baseline_path.read_text())
    events = json.loads(input_path.read_text())
    assert len(expected) == 3602
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES) | set(DIRECT) | CONSTRUCTORS | SELECTOR_RETURNS | DIRECT_RETURNS |
                    {0x3d8, 0x3982, IRQ_START} | IRQ_END | {pc | 0xff000000 for pc in {IRQ_START} | IRQ_END})
    assert len(points) <= 64, ('observer capacity exceeded; split scopes', len(points))
    pcs = (C.c_uint*len(points))(*points)
    natural, oracle, boundaries = [Rows(out/(name+'.jsonl.gz')) for name in ['natural', 'oracle', 'boundaries']]
    streams, errors = [], []
    pending = None
    private = False
    destination = None
    label = ''
    last = None
    rng_calls = 0

    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        @cbtype
        def observe(kind, width, raw_pc, value):
            nonlocal pending, last, rng_calls
            try:
                pc = raw_pc & 0xffffff
                if pc == IRQ_START:
                    if pending is not None:
                        pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                    return
                if pc in IRQ_END:
                    if pending is not None and pending['_irq']:
                        irq = pending['_irq'].pop()
                        irq.update(exit=hex(pc), changes=changes(irq.pop('_ram'), ref.ram_cpu()))
                        pending['interrupts'].append(irq)
                    return
                address, sp = ref.get_register(13) & 0xffff, ref.get_register(15)
                role = (ENTRIES | DIRECT).get(pc)
                if role and pending is None:
                    if not private and ref.frame > 3600:
                        return
                    ram = ref.ram_cpu()
                    if not private and (address not in FIGHTERS or ram[address+0x58] != 5):
                        return
                    assert ram[address+0x58] == 5
                    ret = int.from_bytes(ram[sp & 0xffff:(sp & 0xffff)+4], 'big')
                    assert ret == 0x3d8 if private else ret in SELECTOR_RETURNS | DIRECT_RETURNS, hex(ret)
                    pending = {'kind': role, 'object_id': address, 'source_character_id': 5,
                               'source_register': hex(ref.get_register(13)), 'entry_pc': hex(pc),
                               'entry_sp': sp, 'return_pc': hex(ret), 'frame': ref.frame,
                               'before': snapshot(ram, address, rom), 'constructors': [], 'rng_calls': 0,
                               'interrupts': [], '_irq': [], '_ram': ram,
                               'context': 'declared-private' if private else 'natural'}
                    if pc in DIRECT:
                        pending['constructors'].append(hex(pc))
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                    rng_calls += 1
                elif pending is not None and pc in CONSTRUCTORS:
                    pending['constructors'].append(hex(pc))
                elif pending is not None and pc == int(pending['return_pc'], 16):
                    assert sp == pending['entry_sp']+4 and address == pending['object_id']
                    assert not pending.pop('_irq')
                    ram = ref.ram_cpu()
                    pending.update(after=snapshot(ram, address, rom), exit=hex(pc), exit_sp=sp, exit_frame=ref.frame)
                    before_ram = pending.pop('_ram')
                    stack = {(pending['entry_sp']-i) & 0xffff for i in range(1, 0x100)}
                    allowed = {address+i for i in ACTOR_WRITES} | set(range(0x9400, 0x9c00)) | \
                        set(range(0xa600, 0xa800)) | {0xa00c, 0xa00d, 0xa036, 0xa037} | stack
                    pending['ram_before_sha256'], pending['ram_after_sha256'] = sha(before_ram), sha(ram)
                    pending['ram_changes'] = changes(before_ram, ram)
                    pending['stack_changes'] = [d for d in pending['ram_changes'] if d['offset'] in stack]
                    pending['external_changes'] = [d for d in pending['ram_changes'] if d['offset'] not in allowed]
                    unexplained = []
                    for change in pending['external_changes']:
                        current, steps = change['before'], 0
                        for irq in pending['interrupts']:
                            for delta in irq['changes']:
                                if delta['offset'] == change['offset']:
                                    assert delta['before'] == current
                                    current, steps = delta['after'], steps+1
                        if not steps or current != change['after']:
                            unexplained.append(change)
                    pending['unexplained_changes'] = unexplained
                    assert not unexplained, unexplained
                    assert pending['before']['children_bytes'] == pending['after']['children_bytes']
                    assert pending['before']['energy_fraction'] == pending['after']['energy_fraction']
                    assert pending['before']['random_seed'] == pending['after']['random_seed']
                    assert pending['rng_calls'] == 0
                    last, pending = pending, None
                    if private:
                        assert not last['external_changes'] and not last['interrupts']
                        last['name'] = label
                        if destination is not None:
                            destination.add(last)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.add(last)
            except Exception as exc:
                errors.append((label, repr(exc)))
                if private:
                    ref.cpu.cycles = 4000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames, checkpoint = [], None
        for frame in range(1, len(expected)+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not errors, errors[:3]

        if not args.natural_only:
            assert checkpoint is not None
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i) & 0xffff) ^ 1)).value = byte

            def write(address, value, size=1):
                write_bytes(address, (value & ((1 << (size*8))-1)).to_bytes(size, 'big'))

            serial = 0

            def setup(request=0, direction=0, press=0, changed=0, facing=0, pose=0, energy=255,
                      health=100, pool_mode=None, cursor=None, fraction=None, child=0, source=None,
                      alternate=None, gravity=None, x=None, y=None, ground=None):
                nonlocal serial
                serial += 1
                ref.restore(checkpoint, restore_refresh_cycles=True)
                for reg in range(15):
                    setreg(reg, 0)
                source = source or FIGHTERS[serial % 4]
                cursor = [0, 0x780, 0xff80][serial % 3] if cursor is None else cursor
                pool_mode = serial % 4 if pool_mode is None else pool_mode
                fraction = [0, 1, 0xffff][serial % 3] if fraction is None else fraction
                write_bytes(0x8400, bytes((i*73+serial*29+17) & 255 for i in range(4096)))
                fields = [(0x58, 5, 1), (0x5f, pose, 1), (0x61, facing, 1), (0x6b, request, 1),
                          (0x71, direction | [0, 0x10, 0x70, 0xf0][serial % 4], 1), (0x72, press, 1),
                          (0x73, changed, 1), (0x78, health, 2), (0xb8, energy, 2), (0xba, fraction, 2),
                          (0x90, 0xff7000, 4), (0x40, [0x7fff1234, 0x80004321, 0xffffabcd, 0x00107654][serial % 4] if x is None else x, 4),
                          (0x44, (0x01401234 if y is None else y), 4), (0x9c, (0x01405678 if ground is None else ground), 4)]
                if alternate is not None:
                    fields.append((0x48, alternate, 4))
                if gravity is not None:
                    fields.append((0x4c, gravity, 4))
                for offset, value, size in fields:
                    write(source+offset, value, size)
                write(source+0x104, child, 2)
                write(0x7008, [0x3600, -1, -0x80000000, 0x7fffffff][serial % 4], 4)
                write(0xff18, (serial*0x9e3779b9) & 0xffffffff, 4)
                write_bytes(0xa600, bytes((i*19+serial*17+91) & 255 for i in range(512)))
                write(0xa00c, (serial % 16)*32, 2)
                raw = bytearray((i*37+serial*23+13) & 255 for i in range(2048))
                current = (cursor & 0x7ff)//128
                for i in range(16):
                    available = pool_mode == 0 or pool_mode == 2 and i == (current+1) % 16 or pool_mode == 3 and i == current
                    raw[i*128+4:i*128+6] = (0 if available else 0x80).to_bytes(2, 'big')
                write_bytes(0x9400, raw)
                write(0xa036, cursor, 2)
                return source

            def invoke(role, source, name):
                nonlocal label, last
                label, last = name, None
                setreg(17, 0x2700)
                setreg(15, 0xffff00)
                write(0xff00, 0x3d8, 4)
                setreg(13, 0xffff0000+source)
                setreg(16, next(pc for pc, value in (ENTRIES | DIRECT).items() if value == role))
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                hook(observer)
                execute(2000000)
                hook(None)
                assert last is not None and pending is None and not errors, (name, errors[:3])
                return last

            costs = [('ground-special', 1, 0, 16, 0), ('ground-special', 1, 0, 16, 2),
                     ('ground-special', 3, 16, 8, 0), ('ground-special', 3, 64, 8, 0),
                     ('ground-special', 4, 64, 4, 0), ('ground-special', 4, 16, 4, 0),
                     ('ground-special', 4, 32, 4, 0), ('air-special', 4, 112, 4, 0),
                     ('ground-special', 6, 64, 50, 0), ('ground-special', 6, 16, 50, 0),
                     ('ground-special', 6, 32, 50, 0)]
            destination = oracle
            plans = [('ground', dict(direction=8, press=64)), ('ground', dict(request=7)),
                     ('ground', dict(request=8)), ('ground', dict(direction=2, press=80)),
                     ('air', dict(direction=8, press=64)), ('air', dict(direction=8, press=80)),
                     ('air', dict(direction=0, press=64))]
            for role, request, changed, cost, direction in costs:
                for energy in [cost-1, cost]:
                    plans.append((role, dict(request=request, changed=changed, direction=direction,
                                             health=111, energy=energy)))
            plans += [(role, dict(request=7, energy=-1, changed=255)) for role in DIRECT.values()]
            for index, (role, values) in enumerate(plans):
                source = setup(**values)
                invoke(role, source, f'complete-{index}-{role}')

            destination = boundaries
            faces = [0, 1, 2, 3, 128, 255]
            for request, pose, direction, press, facing in itertools.product([0, 7, 8], [0, 11], range(16), [0, 16, 64, 80], faces):
                source = setup(request=request, pose=pose, direction=direction, press=press, facing=facing)
                invoke('ground', source, f'ground-{request}-{pose}-{direction}-{press}-{facing}')
            for direction, press, facing in itertools.product(range(16), [0, 16, 32, 64, 48, 80, 96, 112], faces):
                source = setup(direction=direction, press=press, facing=facing)
                invoke('air', source, f'air-{direction}-{press}-{facing}')
            for role, request, direction, changed, facing in itertools.product(ENTRIES.values(), range(9), [0, 1, 2, 4, 8, 15], [0, 16, 32, 64, 48, 80, 96, 112, 255], faces):
                if role in ['ground', 'air']:
                    continue
                source = setup(request=request, direction=direction, changed=changed, facing=facing,
                               energy=[-1, 0, 255][serial % 3], health=[-1, 111, 112][serial % 3],
                               child=[0, 1, 0x100, 0xffff][serial % 4])
                invoke(role, source, f'special-{role}-{request}-{direction}-{changed}-{facing}')
            for role, request, changed, cost, direction in costs:
                for energy, health, child, facing, fraction in itertools.product([-32768, -1, 0, cost-1, cost, cost+1, 32767], [-32768, 110, 111, 112, 32767] if request == 6 else [111], [0, 1, 0x100, 0xffff], faces, [0, 1, 65535]):
                    source = setup(request=request, changed=changed, direction=direction, energy=energy,
                                   health=health, child=child, facing=facing, fraction=fraction)
                    invoke(role, source, f'cost-{role}-{request}-{direction}-{changed}-{energy}-{health}-{child}-{facing}-{fraction}')
            for request, facing, pool_mode, cursor in itertools.product([7, 8], faces, range(4), [0, 0x80, 0x780, 0xff80]):
                source = setup(request=request, facing=facing, pool_mode=pool_mode, cursor=cursor)
                invoke('ground', source, f'dash-pool-{request}-{facing}-{pool_mode}-{cursor}')
            coordinates = [(0x7fff1234, 0x80004321), (0x80004321, 0x7fff1234),
                           (0xffffabcd, 0x00207654), (0x00107654, 0xffffabcd)]
            for direction, facing, index, (x, y) in itertools.product([4, 5, 8, 9], faces, range(16), coordinates):
                source = setup(direction=direction, press=64, facing=facing, x=x, y=y)
                write(0xa00c, index*32, 2)
                invoke('ground', source, f'throw-{direction}-{facing}-{index}-{x}-{y}')
            for role, channel, byte in itertools.product(ENTRIES.values(), ['held', 'pressed', 'changed'], range(256)):
                source = setup(request=4, press=64, changed=112)
                write(source+{'held': 0x71, 'pressed': 0x72, 'changed': 0x73}[channel], byte)
                invoke(role, source, f'input-byte-{role}-{channel}-{byte}')
            priority_plans = [('ground-special', 1, 0), ('ground-special', 1, 2),
                              ('ground-special', 3, 0), ('ground-special', 4, 0),
                              ('ground-special', 6, 0), ('air-special', 4, 0)]
            for (role, request, direction), byte in itertools.product(priority_plans, range(256)):
                source = setup(request=request, direction=direction, changed=byte, health=111)
                invoke(role, source, f'priority-byte-{role}-{request}-{direction}-{byte}')
            for byte in range(256):
                source = setup(request=1, changed=0)
                write(source+0x71, byte)
                invoke('ground-special', source, f'combination-held-byte-{byte}')
            for high, relative, fraction in itertools.product([-32768, -1, 0, 127, 128, 32767], [-1, 0, 1], [0, 0xffff]):
                threshold = ((high-128+32768) & 65535)-32768
                yhigh = ((threshold+relative+32768) & 65535)-32768
                source = setup(request=4, changed=112, y=(yhigh << 16) | fraction, ground=(high << 16) | (fraction ^ 65535))
                invoke('air-special', source, f'height-{high}-{relative}-{fraction}')
            for role, alternate, gravity, fraction in itertools.product(['ground-special', 'air-special', *DIRECT.values()], [0, 1, -1, -2147483648, 2147483647, 0xa5d3c791], [0, 1, -1, -2147483648, 2147483647, 0x89123456], [0, 1, 65535]):
                source = setup(request=6 if role == 'ground-special' else 4, changed=112, health=111,
                               alternate=alternate, gravity=gravity, fraction=fraction)
                invoke(role, source, f'alias-{role}-{alternate}-{gravity}-{fraction}')
            for role, request, facing, energy, child in itertools.product(DIRECT.values(), range(9), faces, [-32768, -1, 0, 255, 32767], [0, 1, 0x100, 0xffff]):
                source = setup(request=request, facing=facing, energy=energy, child=child, changed=255)
                invoke(role, source, f'followup-{role}-{request}-{facing}-{energy}-{child}')

            destination = None
            stream_plans = [('ground', 8, 0, 0), ('air', 0, 0, 0)] + \
                [(role, request, changed, direction) for role, request, changed, _, direction in costs] + \
                [(role, 7, 255, 0) for role in DIRECT.values()]
            for stream_index, (role, request, initial_changed, direction) in enumerate(stream_plans):
                for facing in faces:
                    source = setup(request=request, direction=direction, changed=initial_changed,
                                   energy=255, health=111, facing=facing, pool_mode=0, cursor=0xff80)
                    write(0xa00c, 0, 2)
                    calls = []
                    for step in range(16):
                        held = direction if step == 0 else [0, 2, 4, 8, 10][step % 5]
                        press = [0, 64, 80, 16][step % 4]
                        changed = initial_changed if step == 0 else [0, 16, 64, 112][step % 4]
                        for offset, value in [(0x71, held), (0x72, press), (0x73, changed)]:
                            write(source+offset, value)
                        row = invoke(role, source, f'stream-{stream_index}-{role}-{facing}/{step}')
                        row['external_controls'] = {'held': held, 'pressed': press, 'changed': changed}
                        calls.append(row)
                    streams.append({'kind': role, 'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity

    for rows in [natural, oracle, boundaries]:
        rows.close()
    datasets = [(args.natural_name, natural, 'Passive natural source5 complete selector and followup returns'),
                ('fighter5-attacks-oracle', oracle, 'Declared-private representative complete source5 returns'),
                ('fighter5-attacks-boundaries', boundaries, 'Declared-private source5 complete boundary matrix'),
                ('fighter5-attacks-sequences', streams, 'Declared-private single-initial-state source5 streams')]
    for name, source, scope in datasets:
        if args.natural_only and name != args.natural_name:
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
                    with gzip.open(source.path, 'rb') as rows:
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
                  'input': {'path': str(input_path.relative_to(ROOT)), 'sha256': sha(input_path.read_bytes())},
                  'baseline': {'path': str(baseline_path.relative_to(ROOT)), 'sha256': sha(baseline_path.read_bytes()), 'frames': len(frames)},
                  'reference_manifest': {'path': str(control_path.relative_to(ROOT)), 'sha256': sha(control_path.read_bytes())},
                  'records': records, 'calls': records if isinstance(source, Rows) else sum(len(stream['calls']) for stream in source),
                  'kind_counts': dict(source.kinds) if isinstance(source, Rows) else dict(Counter(stream['kind'] for stream in source)),
                  'constructor_counts': dict(source.constructors) if isinstance(source, Rows) else {},
                  'successful_constructor_counts': dict(source.successful) if isinstance(source, Rows) else {},
                  'source_objects': dict(source.sources) if isinstance(source, Rows) else {},
                  'fixture_sha256': digest.hexdigest(), 'gzip_sha256': sha(path.read_bytes()),
                  'frames_match_original_core': frames == expected, 'video_steps': len(frames),
                  'full_ram_audit_bytes': 65536, 'interrupts': source.interrupts if isinstance(source, Rows) else 0,
                  'observer_points': points, 'observer_point_count': len(points),
                  'incomplete_calls': 0, 'errors': errors, 'rng_calls': rng_calls,
                  'entry_frame_limit': 3600, 'drain_frames': 2,
                  'pass': frames == expected and not errors and bool(records),
                  'limits': ['Private dirty inputs do not establish natural gameplay or character resource initialization',
                             'Selection and initialization only; no continued motion, collision or full matches',
                             'Streams change declared base controls only; no expected intermediate state or RNG injection',
                             'RAM audit retains every changed byte and separately explains interrupt changes']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps({key: value for key, value in report.items() if key not in ['capture_dependencies', 'limits', 'observer_points']}, indent=2), flush=True)


if __name__ == '__main__':
    main()
