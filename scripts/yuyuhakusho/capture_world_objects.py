#!/usr/bin/env python3
"""Complete original 0x3556 clears for bounded typed-object prefix storage."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


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
    points = [0x3556, 0x3574]
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, excluded, errors = [], [], [], []
    pending = {}
    private = False
    label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            try:
                full_address = ref.get_register(13)
                address = full_address&0xffff
                key = (full_address, ref.get_register(15))
                if pc == 0x3556:
                    assert key not in pending, 'Nested clear'
                    ram = ref.ram_cpu()
                    size = int.from_bytes(ram[address+4:address+6], 'big')
                    count = ((size if size else 64)>>4)*16
                    row = {'object_id': address, 'a5': full_address, 'frame': ref.frame, 'size': size, 'cleared_bytes': count}
                    if not 0x8000 <= address < 0xa000 or not 0 < count <= 256:
                        row['excluded_scope'] = True
                    else:
                        row['before_bytes'] = ram[address:address+256].hex()
                    pending[key] = row
                elif pc == 0x3574 and key in pending:
                    row = pending.pop(key)
                    if row.get('excluded_scope'):
                        excluded.append(row)
                    else:
                        ram = ref.ram_cpu()
                        row['after_bytes'] = ram[address:address+256].hex()
                        before = bytes.fromhex(row['before_bytes'])
                        count = row['cleared_bytes']
                        assert bytes.fromhex(row['after_bytes']) == b'\0'*count+before[count:]
                        if private:
                            row['name'] = label
                            isolated.append(row)
                        else:
                            natural.append(row)
                    if private:
                        ref.cpu.cycles = 4000000
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
        assert frames == expected and not pending and not errors, (errors, len(pending))
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte
        sizes = [0, *range(16, 257, 16), 17, 31, 63, 127, 255, 257, 271]
        for address in [0x8400, 0x8500, 0x9400]:
            for size in sizes:
                for seed in range(4):
                    ref.restore(checkpoint, restore_refresh_cycles=True)
                    setreg(17, 0x2700)
                    setreg(15, 0xffff00)
                    write(0xffff00, (0x3d8).to_bytes(4, 'big'))
                    setreg(13, 0xffff0000|address)
                    raw = bytearray((i*73+seed*47+17)&255 for i in range(256))
                    raw[4:6] = size.to_bytes(2, 'big')
                    write(0xff0000+address, raw)
                    before = ref.ram_cpu()
                    count = len(isolated)
                    label = f'address-{address:x}/size-{size}/seed-{seed}'
                    setreg(16, 0x3556)
                    ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                    hook(observer); execute(200000); hook(None)
                    assert len(isolated) == count+1 and not pending and not errors, (label, errors)
                    after = ref.ram_cpu()
                    unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and not address <= i < address+256 and not 0xfed0 <= i < 0xff04]
                    assert not unexpected, (label, unexpected)
        identity = ref.identity
    for name, rows in [('world-objects-oracle', natural), ('world-objects-boundaries', isolated)]:
        raw = (json.dumps(rows, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        report = {'scope': 'Original 0x3556 complete clear with logical prefix no larger than256 bytes', 'identity': identity,
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(rows), 'counts': dict(Counter(r['cleared_bytes'] for r in rows)),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'excluded_natural': excluded, 'incomplete_calls': 0, 'errors': errors, 'pass': bool(rows)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
