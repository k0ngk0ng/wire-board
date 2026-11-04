#!/usr/bin/env python3
"""Paired original-CPU proof that an FFE0 test stack aliases real statistics."""
import argparse
import ctypes as C
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
    bridge = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = (C.c_uint*2)(0xf84c, 0xf850)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())[:2400]
    rows, pending = [], None
    with ReferenceSession(ROOT, out, core=core) as ref:
        frames = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 2401)]
        assert frames == expected
        checkpoint = ref.save()
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)
        hook = ref.api('set_cpu_hook', None, C.c_void_p)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            ram = ref.ram_cpu()
            if pc == 0xf84c and pending is None:
                index = ref.get_register(4)&0xffff
                index = index if index < 32768 else index-65536
                address = (ref.get_register(9)+0x40+index)&0xffffff
                sp = ref.get_register(15)&0xffffff
                pending = {'writer_pc': hex(pc), 'address': hex(address), 'stack_at_writer': hex(sp),
                           'before': ram[address&0xffff], 'return_before': ram[sp&0xffff:(sp&0xffff)+4].hex(),
                           'writes_return_address': sp <= address < sp+4}
            elif pc == 0xf850 and pending is not None:
                address, sp = int(pending['address'], 16), int(pending['stack_at_writer'], 16)
                pending.update(after=ram[address&0xffff], return_after=ram[sp&0xffff:(sp&0xffff)+4].hex())
                rows.append(pending)
                pending = None
                ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, points, len(points))
        for stack in [0xffffffe0, 0xffffff00]:
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for i in range(15):
                setreg(i, 0)
            for owner, team in zip([0x8400, 0x8800, 0x8c00, 0x9000], [1, 2, 3, 4]):
                write(0xff0000+owner, 0x12668, 4)
                write(0xff0000+owner+0x78, 223 if team == 1 else -1024, 2)
                write(0xff0000+owner+0x6f, team)
            for i, byte in enumerate([11, 10, 9, 8, 1, 2, 3, 4]):
                write(0xffa100+i, byte)
            for i in range(160):
                write(0xffff60+i, 0)
            for address, value, size in [(0xff8046, 20, 2), (0xffa062, 2, 2), (0xffa088, 2, 2),
                                         (0xffa096, 0, 1), (0xfff550, 0, 1), (0xffa03a, 0, 2)]:
                write(address, value, size)
            setreg(17, 0x2700)
            setreg(13, 0xffff8040)
            setreg(15, stack)
            write(stack, 0x3d8, 4)
            setreg(16, 0xf2a4)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            old_count = len(rows)
            hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
            execute(2000000)
            hook(None)
            assert len(rows) == old_count+1
            rows[-1]['initial_stack'] = hex(stack)
        assert rows[0]['writes_return_address'] and not rows[1]['writes_return_address']
        assert rows[0]['return_before'] != rows[0]['return_after'] and rows[1]['return_before'] == rows[1]['return_after']
        report = {'scope': 'Declared paired private CPU inputs stopped after first actual loss-counter write, not complete calls',
                  'identity': ref.identity, 'script_sha256': sha(Path(__file__).read_bytes()), 'baseline_frames_unchanged': len(frames),
                  'statistics_range': 'FFFFC0..FFFFFF', 'comparisons': rows, 'pass': True,
                  'conclusion': 'Use canonical FF00-or-lower stack; FFE0 test trampoline aliases statistics and corrupts its return'}
    (out/'stack-diagnostics.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
