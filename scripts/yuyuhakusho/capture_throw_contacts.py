#!/usr/bin/env python3
"""Complete original grab-contact consumer, natural and declared CPU inputs."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

GRAB_BYTES = 512
ALLOWED_ACTOR = {0x26, 0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60, 0x61, 0x64, 0x6c, 0x6d, 0x77, 0x89} | set(range(0x40, 0x48)) | set(range(0x94, 0x98))


def number(raw, offset, size):
    return int.from_bytes(raw[offset:offset+size], 'big')


def snapshot(ram):
    grabs, hurts = number(ram, 0xa00c, 2), number(ram, 0xa008, 2)
    assert grabs <= GRAB_BYTES and grabs % 32 == 0, grabs
    assert hurts <= 256 and hurts % 16 == 0, hurts
    owners = {number(ram, 0xa60c+i, 2) for i in range(0, grabs, 32)}
    owners.update(number(ram, 0xa40c+i, 2) for i in range(0, hurts, 16))
    assert all(0x8000 <= owner <= 0xff00 for owner in owners), owners
    return {'grab_offset': grabs, 'hurt_offset': hurts,
            'grabs': ram[0xa600:0xa600+GRAB_BYTES].hex(), 'hurts': ram[0xa400:0xa500].hex(),
            'actors': {str(owner): ram[owner:owner+256].hex() for owner in sorted(owners)},
            'left': number(ram, 0xa010, 2), 'right': number(ram, 0xa012, 2)}


def validate(before, after):
    for key in ['grab_offset', 'hurt_offset', 'grabs', 'hurts', 'left', 'right']:
        assert before[key] == after[key], ('Unexpected throw buffer/global write', key)
    assert before['actors'].keys() == after['actors'].keys()
    for key, encoded in before['actors'].items():
        old, new = bytes.fromhex(encoded), bytes.fromhex(after['actors'][key])
        unknown = [i for i, (a, b) in enumerate(zip(old, new)) if a != b and i not in ALLOWED_ACTOR]
        assert not unknown, (key, unknown)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    out = parser.parse_args().out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*3)(0x1eee2, 0x1ef86, 0x1f08e)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows, boundaries, errors = [], [], []
    pending = None
    private = False
    case_name = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x1eee2:
                    assert pending is None
                    pending = {'frame': ref.frame, 'before': snapshot(ref.ram_cpu()), 'contacts': 0}
                elif pc == 0x1ef86:
                    assert pending is not None
                    pending['contacts'] += 1
                elif pc == 0x1f08e:
                    assert pending is not None
                    pending['after'] = snapshot(ref.ram_cpu())
                    validate(pending['before'], pending['after'])
                    if private:
                        pending['name'] = case_name
                        boundaries.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        rows.append(pending)
                    pending = None
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(pcs))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == baseline and pending is None and not errors, (errors[:5], pending is not None)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value & ((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def write_bytes(address, raw):
            for i, b in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def actor(owner, team):
            return {'owner': owner, 'team': team, 'x': 0x100abcd, 'y': 0x1401234, 'ground': 0x1400000,
                    'delay': 0, 'stages': 0, 'flags': 0, 'input': 0, 'facing': 0x83}

        def grab(owner=0x8400, x=100, y=320, width=16, height=24, dx=12, dy=-3, facing=0):
            return {'owner': owner, 'x': x, 'y': y, 'width': width, 'height': height, 'dx': dx, 'dy': dy,
                    'source_pose': 0x62, 'target_pose': 0x67, 'source_action': 0x23, 'target_action': 0x24, 'facing': facing}

        def hurt(owner=0x9000, x=110, y=320, width=16, height=24):
            return {'owner': owner, 'x': x, 'y': y, 'width': width, 'height': height}

        def run(name, actors, grabs, hurts, left=8, right=312, seed=0):
            nonlocal case_name
            case_name = name
            ref.restore(checkpoint, restore_refresh_cycles=True)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            for reg in range(15):
                setreg(reg, 0)
            for index, a in enumerate(actors):
                base = 0xff0000+a['owner']
                write_bytes(base, bytes((i*73+seed*29+index*31+17)&255 for i in range(256)))
                for key, off, size in [('team',0x6e,1), ('x',0x40,4), ('y',0x44,4), ('ground',0x9c,4),
                                       ('delay',0xc8,1), ('stages',0x6c,1), ('flags',0x5c,1), ('input',0x71,1), ('facing',0x61,1)]:
                    write(base+off, a[key], size)
            raw_grabs = bytearray((i*19+seed*7+91)&255 for i in range(GRAB_BYTES))
            for i, g in enumerate(grabs):
                for key, off, size in [('x',0,2), ('y',2,2), ('width',4,1), ('height',5,1), ('source_pose',6,1),
                                       ('target_pose',7,1), ('source_action',8,1), ('target_action',9,1), ('facing',10,1),
                                       ('owner',12,2), ('dx',16,2), ('dy',18,2)]:
                    raw_grabs[i*32+off:i*32+off+size] = (g[key]&((1 << (size*8))-1)).to_bytes(size,'big')
            raw_hurts = bytearray((i*23+seed*11+69)&255 for i in range(256))
            for i, h in enumerate(hurts):
                for key, off, size in [('x',0,2), ('y',4,2), ('width',8,1), ('height',9,1), ('owner',12,2)]:
                    raw_hurts[i*16+off:i*16+off+size] = (h[key]&((1 << (size*8))-1)).to_bytes(size,'big')
            write_bytes(0xffa600, raw_grabs)
            write_bytes(0xffa400, raw_hurts)
            write(0xffa00c, len(grabs)*32, 2)
            write(0xffa008, len(hurts)*16, 2)
            write(0xffa010, left, 2)
            write(0xffa012, right, 2)
            previous = ref.ram_cpu()
            count = len(boundaries)
            setreg(16, 0x1eee2)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(boundaries) == count+1 and pending is None and not errors, (name, errors[:5], pending is not None)
            allowed = set(range(0xffb0, 0xffe4))
            for a in actors:
                allowed.update(range(a['owner'], a['owner']+256))
            changed = [i for i, (a,b) in enumerate(zip(previous, ref.ram_cpu())) if a != b and i not in allowed]
            assert not changed, (name, changed[:10])

        base = [actor(0x8400, 0), actor(0x9000, 1)]
        run('empty', base, [], [])
        run('no-grabs', base, [], [hurt()])
        run('no-hurts', base, [grab()], [])
        for i, (source_input, target_facing, ground) in enumerate(itertools.product([0,8,0x80,0xff], [0,1,2,0x80,0xff], [0x1300000,0x130ffff,0x1400000])):
            actors = [dict(a) for a in base]
            actors[0]['input'] = source_input
            actors[0]['ground'] = actors[1]['ground'] = ground
            run(f'facing-input-{source_input}-target-{target_facing}-ground-{ground:x}', actors, [grab(facing=target_facing)], [hurt()], seed=i)
        for field, values in [('delay',[0,1,255]), ('team',[0,1,255]), ('ground',[0x1400000,0x140ffff,0x1410000]), ('stages',[0,0x20,0xff]), ('flags',[0,8,0xff])]:
            for value in values:
                actors = [dict(a) for a in base]
                actors[1][field] = value
                run(f'filter-{field}-{value}', actors, [grab()], [hurt()])
        for source_flag, target_flag in itertools.product([0,8,0xf7,0xff], repeat=2):
            actors = [dict(a) for a in base]
            actors[0]['flags'], actors[1]['flags'] = source_flag, target_flag
            run(f'frame-flags-{source_flag}-{target_flag}', actors, [grab()], [hurt()])
        for i, (source_x, dx, dy, left, right, source_input) in enumerate([
                (0x100abcd,12,-3,8,312,0), (0x100abcd,12,-3,8,312,8),
                (0x0000ffff,12,32767,8,312,0), (0x01380001,20,-32768,8,312,8),
                (0x7fff1234,1,1,-32768,32767,8), (-0x80001234,-1,-1,-32768,32767,0),
                (0x100abcd,32767,-32768,300,8,8), (0x7fffabcd,-32768,32767,32767,-32768,0)]):
            actors = [dict(a) for a in base]
            actors[0]['x'], actors[0]['input'] = source_x, source_input
            actors[0]['y'] = 0x7fffdef0 if i&1 else -0x80001234
            run(f'coordinates-{i}', actors, [grab(dx=dx,dy=dy)], [hurt()], left=left, right=right, seed=i)
        for axis, difference, sizes in itertools.product(['x','y'],[-32768,-32767,-256,-255,-32,-31,-1,0,1,31,32,33,255,256,32767],[(0,0),(16,16),(127,128),(128,128),(255,255)]):
            g, h = grab(), hurt()
            size = 'width' if axis == 'x' else 'height'
            g[axis], h[axis] = 100, 100+difference
            g[size], h[size] = sizes
            run(f'geometry-{axis}-{difference}-{sizes}', base, [g], [h])
        owners = [0x8400,0x8500,0x9000,0x9100]
        for ordering in itertools.permutations(range(4)):
            actors = [actor(owner,i) for i,owner in enumerate(owners)]
            run('four-order-'+''.join(map(str,ordering)), actors,
                [grab(owner=owners[i],dx=i*7,dy=i) for i in ordering],
                [hurt(owner=owners[i]) for i in reversed(ordering)])
        actors = [actor(owner,i) for i,owner in enumerate(owners)]
        run('duplicate-shapes', actors, [grab(owner=owners[0]),grab(owner=owners[0])], [hurt(owner=owners[1]) for _ in range(4)])
        run('full-buffers', actors, [grab(owner=owners[i%4],dx=i) for i in range(16)], [hurt(owner=owners[i%4]) for i in range(16)])
        identity = ref.identity
    for name, calls, scope in [('throw-contacts-oracle',rows,'Natural complete original grab-contact calls'),
                               ('throw-contacts-boundaries',boundaries,'Explicit isolated original CPU input matrix')]:
        raw = (json.dumps(calls,separators=(',',':'))+'\n').encode()
        zipped = gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        report = {'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
                  'records':len(calls),'contacts':sum(row['contacts'] for row in calls),'nonempty_calls':sum(row['before']['grab_offset']>0 and row['before']['hurt_offset']>0 for row in calls),
                  'fixture_sha256':sha(raw),'gzip_sha256':sha(zipped),'frames_match_original_core':frames==baseline,'errors':errors,'incomplete_calls':0,'pass':bool(calls) and not errors,
                  'limits':['Consumer only; A600 producer capacity and subsequent throw/damage stages are not recovered',
                            'Natural frames untouched; isolated CPU cases explicitly replace private inputs',
                            'Captured A600 bytes span 512 bytes; production semantic slice does not impose that as an original capacity']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps(report,indent=2))


if __name__ == '__main__':
    main()
