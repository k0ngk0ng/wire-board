#!/usr/bin/env python3
"""Complete Chu throw submission and its tail-called action constructor."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

AREA = 0xa600
SIZE = 0x400
ALLOWED = {0x59, 0x5a, 0x5b, 0x5d, 0x5f, 0x60, 0x61, 0x64, 0x6c, 0x6d, 0x6e, 0x77}


def number(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram, owner):
    return {'object_id': owner, 'actor_bytes': ram[owner:owner+256].hex(),
            'records': ram[AREA:AREA+SIZE].hex(), 'offset': number(ram, 0xa00c, 2)}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    filter_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(filter_path)); cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint*2)(0x17ec2, 0x19c44)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, errors = [], [], []
    pending = {}; private = False; label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                owner = ref.get_register(13)&0xffff; sp = ref.get_register(15); key = (owner, sp)
                if pc == 0x17ec2:
                    assert key not in pending, 'Nested throw producer'
                    ram = ref.ram_cpu()
                    pending[key] = ({'frame': ref.frame, 'before': snapshot(ram, owner)}, ram)
                elif key in pending:
                    row, before = pending.pop(key); after = ref.ram_cpu(); old = before[owner:owner+256]; new = after[owner:owner+256]
                    unexpected = [i for i, (a, b) in enumerate(zip(old, new)) if a != b and i not in ALLOWED]
                    assert not unexpected, ('Unknown actor writes', unexpected)
                    permitted = set(range(owner, owner+256)) | set(range(AREA, AREA+SIZE)) | {0xa00c, 0xa00d}
                    external = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in permitted]
                    assert not external, ('External RAM changes', external)
                    row.update(after=snapshot(after, owner), exit='0x19c44', exit_frame=ref.frame)
                    if private:
                        row['name'] = label; isolated.append(row); ref.cpu.cycles = 200000
                    else: natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 200000
        assert bridge.yyh_set_observer(observe, pcs, len(pcs))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400: checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors and natural, (len(natural), errors)
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, data):
            for i, byte in enumerate(data): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        values = [(0x00641234, 0x01405678), (0x7fffabcd, 0x8000cdef), (0x80000000, 0x00281234), (0xffffffff, 0x0027ffff)]
        for i, (offset, facing, held, position) in enumerate(itertools.product([0, 32, 96, 480, 512, 992], [0, 1, 2, 255], range(16), values)):
            label = f'offset-{offset}/facing-{facing}/held-{held}/position-{values.index(position)}'
            ref.restore(checkpoint, restore_refresh_cycles=True)
            setreg(17, 0x2700); setreg(15, 0xffffe0); write(0xffffe0, 0x3d8, 4)
            owner = [0x8400, 0x9000][i%2]; setreg(13, 0xff0000+owner)
            for reg in range(8): setreg(reg, 0)
            raw = bytes((n*73+i*29+17)&255 for n in range(256)); write_bytes(0xff0000+owner, raw)
            write(0xff0000+owner+0x40, position[0], 4); write(0xff0000+owner+0x44, position[1], 4)
            write(0xff0000+owner+0x61, facing); write(0xff0000+owner+0x71, held)
            write(0xff0000+owner+0x6e, [0, 128, 255][i%3]); write(0xff0000+owner+0x6f, [255, 0, 1][i%3])
            write_bytes(0xffa600, bytes((n*19+i*31+91)&255 for n in range(SIZE))); write(0xffa00c, offset, 2)
            count = len(isolated); setreg(16, 0x17ec2); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            hook(observer); execute(100000); hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (label, errors)
        identity = ref.identity
    for name, records, scope in [('throw-records-oracle', natural, 'Complete natural Chu submit branch and reset'),
                                  ('throw-records-boundaries', isolated, 'Declared isolated original Chu submit branch and reset')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode(); compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        report = {'scope': scope, 'identity': identity, 'records': len(records), 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(filter_path.read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
                  'frames_match_original_core': frames == expected, 'incomplete_calls': len(pending), 'errors': errors,
                  'max_input_offset': max(r['before']['offset'] for r in records), 'pass': bool(records) and not errors,
                  'limits': ['Direct Chu submit branch, not its selector or all character producers',
                             'No capacity guard exists in the original branch; 1024-byte declared private capture is not original allocated-capacity proof',
                             'Not subsequent throw contact, motion or damage']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
