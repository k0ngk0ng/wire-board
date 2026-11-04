#!/usr/bin/env python3
"""Original complete attack/hurt calls and deliberate isolated CPU cases."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

FIELDS = {
    'x': (0x40, 4), 'y': (0x44, 4), 'ground': (0x9c, 4),
    'velocity_x': (0x10, 4), 'velocity_y': (0x14, 4), 'gravity': (0x4c, 4),
    'character_id': (0x58, 1), 'frame_index': (0x59, 1), 'frame_duration': (0x5a, 1),
    'frame_ticks': (0x5b, 1), 'frame_flags': (0x5c, 1), 'action_kind': (0x5d, 1),
    'pose': (0x5f, 1), 'completed': (0x60, 1), 'facing': (0x61, 1),
    'guard_mask': (0x63, 1), 'attack_flags': (0x64, 1), 'attack_value': (0x65, 1),
    'stun_damage': (0x66, 1), 'effect_kind': (0x67, 1), 'reaction': (0x68, 1),
    'hit_stop_ticks': (0x69, 1), 'shake': (0x6a, 1), 'action_flags': (0x6c, 1),
    'cancel_ticks': (0x6d, 1), 'team': (0x6e, 1), 'target_bit': (0x70, 1),
    'retained_held': (0x71, 1), 'input_retention': (0x75, 1), 'auxiliary77': (0x77, 1),
    'health': (0x78, 2), 'stun_value': (0x7e, 2), 'stun_ticks': (0x84, 2),
    'status_delay': (0x86, 1), 'attack_delay': (0x88, 1), 'contact_flags': (0x89, 1),
    'target_mask': (0x94, 4), 'residual': (0x98, 4), 'auxiliary_b8': (0xb8, 4), 'combo_count': (0xc6, 1)}
WRITTEN = {0x26, 0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60, 0x61, 0x64, 0x68, 0x69, 0x6a,
           0x6c, 0x6d, 0x6e, 0x75, 0x77, 0x86, 0x88, 0x89, 0xc6}
for off, size in [(0x10, 8), (0x4c, 4), (0x78, 2), (0x7e, 2), (0x84, 2), (0x94, 8), (0xb8, 4)]:
    WRITTEN.update(range(off, off+size))


def number(raw, off, size):
    return int.from_bytes(raw[off:off+size], 'big')


def snapshot(ram):
    attack_count, hurt_count = number(ram, 0xa004, 2), number(ram, 0xa008, 2)
    assert all(n <= 256 and n % 16 == 0 for n in (attack_count, hurt_count))
    owners = sorted({number(ram, base+i+12, 2) for base, count in [(0xa200, attack_count), (0xa400, hurt_count)] for i in range(0, count, 16)})
    assert all(0x8000 <= owner <= 0xff00 for owner in owners), owners
    return {'attack_count': attack_count, 'hurt_count': hurt_count,
            'attack_bytes': ram[0xa200:0xa300].hex(), 'hurt_bytes': ram[0xa400:0xa500].hex(),
            'actors': {str(owner): ram[owner:owner+256].hex() for owner in owners},
            'effects': ram[0x9400:0x9c00].hex(), 'cursor': number(ram, 0xa036, 2),
            'pending_sound': ram[0xa030], 'slow_ticks': number(ram, 0xf558, 2), 'combo_flags': ram[0xa0a4],
            'arena_left': number(ram, 0xa010, 2), 'arena_right': number(ram, 0xa012, 2)}


def external_changes(before, after, state, stack):
    allowed = set(range(0x9400, 0x9c00)) | {0xa030, 0xa036, 0xa037, 0xf558, 0xf559, 0xa0a4}
    allowed.update(range(max(0, stack-128), min(65536, stack+4)))
    for key in state['actors']:
        owner = int(key)
        allowed.update(owner+off for off in WRITTEN)
    return [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    filter_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(filter_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pc_values = [0x1f21e, 0x1f522, 0x1f2a6, 0x1f32c, 0x1f414, 0x1fa70, 0x1f524]
    pcs = (C.c_uint*len(pc_values))(*pc_values)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows, isolated, errors = [], [], []
    pending = None
    private = False
    name = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x1f21e:
                    assert pending is None, 'Nested attack/hurt entry'
                    raw = ref.ram_cpu()
                    pending = (raw, ref.get_register(15)&0xffff,
                               {'frame': ref.frame, 'before': snapshot(raw), 'contacts': 0, 'guards': 0, 'hits': 0, 'knockouts': 0, 'reactions': []})
                elif pc == 0x1f522:
                    assert pending is not None, 'Return without attack/hurt entry'
                    before, stack, row = pending
                    raw = ref.ram_cpu()
                    row.update(after=snapshot(raw), external_changes=external_changes(before, raw, row['before'], stack))
                    assert row['before']['attack_bytes'] == row['after']['attack_bytes']
                    assert row['before']['hurt_bytes'] == row['after']['hurt_bytes']
                    if private:
                        row['name'] = name
                        isolated.append(row)
                        ref.cpu.cycles = 8000000
                    else:
                        rows.append(row)
                    pending = None
                elif pending is not None:
                    row = pending[2]
                    if pc == 0x1f2a6:
                        row['contacts'] += 1
                    elif pc == 0x1f32c:
                        row['guards'] += 1
                    elif pc == 0x1f414:
                        row['hits'] += 1
                    elif pc == 0x1fa70:
                        row['knockouts'] += 1
                    elif pc == 0x1f524:
                        ram = ref.ram_cpu()
                        row['reactions'].append(ram[(ref.get_register(11)&0xffff)+0x68])
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 8000000
        assert bridge.yyh_set_observer(observe, pcs, len(pcs))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        callback = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(callback)
        frames = []
        for f in range(1, 3601):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not errors, (len(rows), errors[:5])
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value & ((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def actor(owner, index=0, **changes):
            result = dict(owner=owner, x=100<<16, y=0x1400000, ground=0x1400000,
                          velocity_x=0x1245678, velocity_y=-0x2345678, gravity=0x8901,
                          character_id=index%11, frame_index=83, frame_duration=91, frame_ticks=0,
                          frame_flags=0, action_kind=0, pose=0, completed=7, facing=index&1,
                          guard_mask=0, attack_flags=0, attack_value=17, stun_damage=12, effect_kind=1,
                          reaction=0, hit_stop_ticks=7, shake=9, action_flags=0x93, cancel_ticks=0x47,
                          team=index, target_bit=index, retained_held=0, input_retention=19, auxiliary77=0xb1,
                          health=100, stun_value=80, stun_ticks=0x8765, status_delay=13, attack_delay=97,
                          contact_flags=0x80, target_mask=0xffffffff, residual=-12345,
                          auxiliary_b8=0x100abcd, combo_count=0)
            result.update(changes)
            return result

        def shape(owner, x=100, y=320, width=18, height=25):
            return owner, x, y, width, height

        def case(case_name, actors, attacks=None, hurts=None, busy=(), cursor=0, seed=0, left=0, right=320):
            nonlocal name
            name = case_name
            attacks = [shape(actors[0]['owner'])] if attacks is None else attacks
            hurts = [shape(actors[-1]['owner'], 112)] if hurts is None else hurts
            ref.restore(checkpoint, restore_refresh_cycles=True)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            for reg in range(15):
                setreg(reg, 0)
            for n, obj in enumerate(actors):
                owner = obj['owner']
                write_bytes(0xff0000+owner, bytes((i*37+seed*19+n*29+11)&255 for i in range(256)))
                for key, (off, size) in FIELDS.items():
                    write(0xff0000+owner+off, obj[key], size)
            for base, counter, records in [(0xa200, 0xa004, attacks), (0xa400, 0xa008, hurts)]:
                raw = bytearray((i*19+seed*23+13)&255 for i in range(256))
                for i, (owner, x, y, width, height) in enumerate(records):
                    for off, value, size in [(0,x,2),(4,y,2),(8,width,1),(9,height,1),(12,owner,2)]:
                        raw[i*16+off:i*16+off+size] = (value&((1<<(size*8))-1)).to_bytes(size,'big')
                write_bytes(0xff0000+base, raw)
                write(0xff0000+counter, len(records)*16, 2)
            raw = bytearray((i*73+seed*31+17)&255 for i in range(0x800))
            for n in range(16):
                raw[n*128+4:n*128+6] = (0x80 if n in busy else 0).to_bytes(2,'big')
            write_bytes(0xff9400, raw)
            write(0xffa036, cursor, 2)
            write(0xffa030, 0xc9)
            write(0xfff558, 0x1234, 2)
            write(0xffa0a4, 0xa2)
            write(0xffa010, left, 2)
            write(0xffa012, right, 2)
            previous = len(isolated)
            setreg(16, 0x1f21e)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            hook(callback)
            execute(4000000)
            hook(None)
            assert len(isolated) == previous+1 and pending is None and not errors, (name, errors[:4])
            assert not isolated[-1]['external_changes'], (name, isolated[-1]['external_changes'][:8])

        for i, (reaction, posture, strength, effect) in enumerate(itertools.product(range(11), [0,4,8], range(4), range(3))):
            a = actor(0x8400, reaction=reaction, attack_flags=strength | (0x10 if i&1 else 0), effect_kind=effect,
                      facing=[0,1,2,255][i%4], frame_ticks=[0,1,255][i%3], y=0x1400000-(i%3))
            b = actor(0x9000, 1, frame_flags=posture, pose=[0,0x20,0x27,0x28,0x2d,0x2e,0x30,0x31][i%8],
                      health=100, stun_value=80, combo_count=[0,14,15,16,254,255][i%6],
                      auxiliary_b8=[0x100abcd,0xde1234,0xdf5678,0x7fffabcd,-0x80000000][i%5])
            case(f'reaction-{reaction}/posture-{posture}/strength-{strength}/effect-{effect}', [a,b],
                 busy=range(16) if i%7 == 0 else (), cursor=0xff80 if i%5 == 0 else 0, seed=i)
        for i, (strength, mask, flags, held, health) in enumerate(itertools.product(range(4), [4,8,12], [0,0x10,0x20,0x30], [0,2], [-1,0,63])):
            a = actor(0x8400, attack_flags=strength|mask|flags, attack_value=255, effect_kind=i%3, y=0x1400000-(i%2))
            b = actor(0x9000,1,guard_mask=mask,retained_held=held,health=health,x=([0,16,17,303,304,320][i%6]<<16)|0xabcd)
            case(f'guard-{strength}-{mask}-{flags}-{held}-{health}', [a,b], seed=i)
        for i, (health, stun, damage) in enumerate(itertools.product([-32768,-1,0,1,17,255,32767], [-32768,-1,0,1,12,32767], [0,1,17,255])):
            a = actor(0x8400,attack_value=damage,stun_damage=damage,reaction=i%11,effect_kind=i%3)
            b = actor(0x9000,1,health=health,stun_value=stun,character_id=i%11)
            case(f'damage-{health}-{stun}-{damage}', [a,b], seed=i)
        base = [actor(0x8400),actor(0x9000,1)]
        for bit, mask in itertools.product([0,1,31,32,33,255], [0,1,2,0x80000000,0xffffffff]):
            case(f'target-{bit}-{mask:x}', [actor(0x8400,target_mask=mask),actor(0x9000,1,target_bit=bit)])
        for field, value in [('team',0),('team',255),('ground',0x140ffff),('ground',0x1410000),('ground',-0x80000000)]:
            case(f'filter-{field}-{value}', [base[0],actor(0x9000,1,**{field:value})])
        geometry = [(100,136,18,18),(100,137,18,18),(100,100,0,0),(100,100,255,1),(100,101,255,1),
                    (0,-32768,1,1),(32767,-32768,4,4),(-32768,-32767,128,128),(32767,32767,255,255)]
        for axis in [0,1]:
            for i, (first, second, wa, wb) in enumerate(geometry):
                a, b = [100,320,18,25],[112,320,18,25]
                a[axis],b[axis],a[axis+2],b[axis+2]=first,second,wa,wb
                case(f'geometry-{axis}-{i}',base,[(0x8400,*a)],[(0x9000,*b)],seed=i)
        for count_a, count_b in [(0,0),(0,1),(1,0),(1,1),(4,4),(16,16)]:
            actors = [actor(owner,i,reaction=i+5) for i,owner in enumerate([0x8400,0x8500,0x9000,0x9100])]
            attacks=[shape(actors[i%4]['owner'],100+i%3) for i in range(count_a)]
            hurts=[shape(actors[(i+1)%4]['owner'],100+i%3) for i in range(count_b)]
            case(f'four-actors-{count_a}-{count_b}',actors,attacks,hurts,seed=count_a)
        for suffix,busy in [('full',range(16)),('current-free',range(1,16)),('next-free',set(range(16))-{1}),('last-free',set(range(16))-{15})]:
            case('pool-'+suffix,[actor(0x8400),actor(0x9000,1,pose=0x20)],busy=busy)
        identity = ref.identity
    for fixture_name, records, scope in [('attack-contacts-oracle', rows, 'Complete passive original calls'),
                                         ('attack-contacts-boundaries', isolated, 'Explicit isolated original CPU calls')]:
        raw=(json.dumps(records,separators=(',',':'))+'\n').encode()
        compressed=gzip.compress(raw,mtime=0)
        (out/(fixture_name+'.json.gz')).write_bytes(compressed)
        report={'scope':scope,'identity':identity,'records':len(records),'contacts':sum(r['contacts']for r in records),
                'guards':sum(r['guards']for r in records),'hits':sum(r['hits']for r in records),'knockouts':sum(r['knockouts']for r in records),
                'reactions':sorted({x for r in records for x in r['reactions']}),'external_change_calls':sum(bool(r['external_changes'])for r in records),
                'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(filter_path.read_bytes()),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==expected,
                'incomplete_calls':0,'errors':errors,'pass':bool(records)and not errors,
                'limits':['Attack/hurt contact resolution and reached helpers, not subsequent reaction motion/effect lifetimes or complete matches',
                          'Isolated original CPU private inputs deliberately replaced; natural full frames untouched']}
        (out/(fixture_name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps(report,indent=2))


if __name__ == '__main__':
    main()
