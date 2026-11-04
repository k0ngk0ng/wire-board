#!/usr/bin/env python3
"""Capture complete original ordered body-separation calls, no injected outcomes."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def number(raw, offset, size):
    return int.from_bytes(raw[offset:offset+size], 'big')


def snapshot(ram):
    count = number(ram, 0xa00a, 2)
    assert count <= 256 and count % 16 == 0, count
    owners = sorted({number(ram, 0xa50c+i, 2) for i in range(0, count, 16)})
    assert all(0x8000 <= owner <= 0xff00 for owner in owners), owners
    return {'count': count, 'shape_bytes': ram[0xa500:0xa600].hex(),
            'actor_bytes': {str(owner): ram[owner:owner+256].hex() for owner in owners}}


def validate(before_ram, after_ram, before):
    allowed = set()
    for owner in before['actor_bytes']:
        owner = int(owner)
        allowed.update(range(owner+0x40, owner+0x44))
        allowed.add(owner+0x89)
    for offset in range(0, before['count'], 16):
        allowed.update(range(0xa500+offset, 0xa504+offset))
    return [{'offset': i, 'before': a, 'after': b}
            for i, (a, b) in enumerate(zip(before_ram, after_ram))
            if a != b and i not in allowed]


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
    pcs = (C.c_uint*2)(0x1ee1e, 0x1eee0)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows, boundaries, errors = [], [], []
    pending = None
    private = False
    sample = None
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, sample
            try:
                if private:
                    if pc == 0x1eee0:
                        assert sample is None
                        sample = ref.ram_cpu()
                        ref.cpu.cycles = 2000000
                    return
                ram = ref.ram_cpu()
                if pc == 0x1ee1e:
                    assert pending is None
                    pending = (ram, {'frame': ref.frame, 'before': snapshot(ram)})
                else:
                    assert pending is not None
                    previous, row = pending
                    pending = None
                    row.update(after=snapshot(ram), external_changes=validate(previous, ram, row['before']))
                    rows.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 2000000
        assert bridge.yyh_set_observer(observe, pcs, 2)
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames = []
        for f in range(1, 3601):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == baseline and pending is None and not errors, (errors[:8], pending)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, value, size=1):
            raw = (value & ((1 << (size*8))-1)).to_bytes(size, 'big')
            for i, b in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def case(name, records, teams=(0,1,2,3), grounds=(0x1401234,)*4, actor_x=(0x100abcd, 0x100abcd, 0x100abcd, 0x100abcd)):
            nonlocal sample
            ref.restore(checkpoint, restore_refresh_cycles=True)
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            for n, owner in enumerate((0x8400, 0x9000, 0x9400, 0x9800)):
                for i in range(256):
                    write(0xff0000+owner+i, (i*37+n*17+11)&255)
                write(0xff0000+owner+0x6e, teams[n])
                write(0xff0000+owner+0x9c, grounds[n], 4)
                write(0xff0000+owner+0x40, actor_x[n], 4)
                write(0xff0000+owner+0x89, 0xa4)
            for i in range(256):
                write(0xffa500+i, (i*19+13)&255)
            for i, (x, y, width, height, owner_index) in enumerate(records):
                base = 0xffa500+i*16
                write(base, x, 4)
                write(base+4, y, 4)
                write(base+8, width)
                write(base+9, height)
                write(base+12, (0x8400, 0x9000, 0x9400, 0x9800)[owner_index], 2)
            write(0xffa00a, len(records)*16, 2)
            previous = ref.ram_cpu()
            before = snapshot(previous)
            sample = None
            setreg(16, 0x1ee1e)
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
            execute(1000000)
            hook(None)
            assert sample is not None and not errors, (name, errors[:3])
            external = validate(previous, sample, before)
            assert not external, (name, external[:8])
            boundaries.append({'scope':'isolated-original-cpu', 'name':name, 'before':before, 'after':snapshot(sample), 'external_changes':external})
        case('empty', [])
        case('one', [(0,0,5,5,0)])
        # Exhaust byte sums, signed word absolute/overflow predicates, contact
        # equality and low-word comparisons independently in both axes.
        for axis, difference, dimensions in itertools.product([0,1], [-32768,-32767,-256,-255,-10,-1,0,1,10,255,256,32767], [(0,0),(5,5),(127,128),(128,128),(255,255)]):
            a = [0x1001234,0x1005678,dimensions[0],dimensions[0],0]
            b = [0x100abcd,0x100def0,dimensions[1],dimensions[1],1]
            b[axis] = (a[axis] + (difference << 16)) & 0xffffffff
            case(f'axis-{axis}-difference-{difference}-size-{dimensions}', [tuple(a),tuple(b)])
        for x1, x2 in [(0,0),(0,1),(1,0),(-1,0),(0,-1),(0x7fffffff,-0x80000000),(-0x80000000,0x7fffffff)]:
            case(f'full-x-{x1}-{x2}', [(x1,0,10,10,0),(x2,0,10,10,1)], actor_x=(0x7fffffff,-0x80000000,0,0))
        common = [(0x100aaaa,0x100bbbb,15,15,0),(0x100aaaa,0x100bbbb,15,15,1)]
        case('same-team', common, teams=(9,9,2,3))
        case('ground-high-mismatch', common, grounds=(0x140ffff,0x141ffff,0,0))
        case('ground-low-mismatch', common, grounds=(0x1400000,0x140ffff,0,0))
        case('same-owner-many', [(0,0,20,20,0),(1,0,20,20,0),(0x10000,0,20,20,1)])
        for ordering in itertools.permutations(range(4)):
            case('four-order-'+''.join(map(str,ordering)), [(0x100aaaa,0x100bbbb,15,15,i) for i in ordering])
        case('four-different-coordinates', [(0x100aaaa+i*0x70000,0x100bbbb,15,15,i) for i in range(4)])
        case('full-buffer', [(0x100aaaa+(i%4)*0x50000,0x100bbbb,15,15,i%4) for i in range(16)])
        identity = ref.identity
    for name, calls, scope in [('body-separation-oracle',rows,'Natural complete original body-separation calls'),('body-separation-boundaries',boundaries,'Isolated original ordered-pair and arithmetic boundary cases')]:
        raw = (json.dumps(calls,separators=(',',':'))+'\n').encode()
        compressed = gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        report = {'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(calls),'nonempty_calls':sum(r['before']['count']>0 for r in calls),'external_change_calls':sum(bool(r['external_changes']) for r in calls),'frames_match_original_core':frames==baseline,'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'errors':errors,'incomplete_calls':0,'pass':bool(calls) and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps(report,indent=2))


if __name__ == '__main__':
    main()
