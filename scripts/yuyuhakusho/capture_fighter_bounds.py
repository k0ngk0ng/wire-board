#!/usr/bin/env python3
"""Complete original X bounds and shared camera-position sampling."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*2)(0x208d8, 0x20916)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, sequences, errors = [], [], [], []; pending = {}; private = False; label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        def snapshot(address):
            ram = ref.ram_cpu(); off = address&0xffff
            return {'actor': ram[off:off+256].hex(), 'samples': ram[0xa800:0xa880].hex(),
                    'used': int.from_bytes(ram[0xa00e:0xa010], 'big'), 'left': int.from_bytes(ram[0xa010:0xa012], 'big', signed=True),
                    'right': int.from_bytes(ram[0xa012:0xa014], 'big', signed=True)}

        @cbtype
        def observe(kind, width, pc, value):
            try:
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15); key = (address, sp)
                if pc == 0x208d8:
                    assert key not in pending
                    pending[key] = {'object_id': address&0xffff, 'frame': ref.frame, 'before': snapshot(address)}
                elif key in pending:
                    row = pending.pop(key); row.update(after=snapshot(address), exit_frame=ref.frame)
                    before, after = bytes.fromhex(row['before']['actor']), bytes.fromhex(row['after']['actor'])
                    assert all(a == b or i in (0x40, 0x41) for i, (a, b) in enumerate(zip(before, after)))
                    if private:
                        row['name'] = label; isolated.append(row); ref.cpu.cycles = 4000000
                    else:
                        natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, 2)
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400: checkpoint = ref.save()
        hook(None); assert frames == expected and not errors and not pending, (errors, len(pending))
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, raw):
            for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def setup(seed, left=32, right=600, used=0, x=0, y=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            write_bytes(0xff8400, bytes((i*73+seed*19+17)&255 for i in range(256)))
            write(0xff8440, x, 4); write(0xff8444, y, 4); write(0xff8478, -1024 if seed%2 else 223, 2)
            write_bytes(0xffa800, bytes((i*47+seed*17+11)&255 for i in range(128)))
            write(0xffa00e, used, 2); write(0xffa010, left, 2); write(0xffa012, right, 2)

        def invoke(name):
            nonlocal label
            label = name; setreg(17, 0x2700); setreg(15, 0xffffe0); write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xffff8400); setreg(16, 0x208d8); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu(); count = len(isolated); hook(observer); execute(2000000); hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, errors)
            after = ref.ram_cpu(); allowed = {0x8440, 0x8441, 0xa00e, 0xa00f} | set(range(0xa800, 0xa880)) | set(range(0xff80, 0xffe4))
            assert not [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed], name
            return isolated[-1]
        bounds = [(-32768, 32767), (-1, 1), (32, 600), (0, 0), (32767, -32768), (600, 32)]
        variants = itertools.product(bounds, [-32768, -32767, -1, 0, 1, 31, 32, 33, 32766, 32767], [0, 1, 65535], [0, 4, 12, 56, 60, 63, 64, 65, 255, 32768, 65535])
        for seed, ((left, right), high, fraction, used) in enumerate(variants):
            x = (high << 16) | fraction; y = [-0x80000000, -1, 0, 0x7fffffff][seed%4]
            setup(seed, left, right, used, x, y); invoke(f'bounds-{left}-{right}/x-{high}-{fraction}/used-{used}')
        boundary_count = len(isolated)
        for seed, (left, right, used) in enumerate([(32, 600, 0), (-32768, 32767, 60), (600, 32, 64), (0, 0, 65535)]):
            setup(seed, left, right, used)
            rows = []
            for step in range(32):
                x = ([-32768, -1, 0, 32, 600, 32767][step%6] << 16) | ((step*2003)&65535)
                y = ((step*7919) ^ 0x80001234)&0xffffffff
                write(0xff8440, x, 4); write(0xff8444, y, 4)
                row = invoke(f'stream-{seed}/{step}')
                a = bytes.fromhex(row['before']['actor']); row['external_position'] = {'x': int.from_bytes(a[0x40:0x44], 'big', signed=True), 'y': int.from_bytes(a[0x44:0x48], 'big', signed=True)}
                rows.append(row)
            sequences.append({'initial': rows[0]['before'], 'calls': rows})
        identity = ref.identity
    for name, records, scope in [('fighter-bounds-oracle', natural, 'Natural complete208d8 calls'), ('fighter-bounds-boundaries', isolated[:boundary_count], 'Declared isolated originalCPU calls'), ('fighter-bounds-sequences', sequences, 'Singleinitial buffer with external upstream positions only')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        rows = [r for s in records for r in s['calls']] if name.endswith('sequences') else records
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(records), 'calls': len(rows),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected, 'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'limits': ['Health=-1024 callfilter remains the outer caller responsibility', 'Isolated position streams are not fullworld steps', 'Malformed unaligned low counters outside typed sample storage are rejected independently']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
