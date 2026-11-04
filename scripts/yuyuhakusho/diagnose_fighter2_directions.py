#!/usr/bin/env python3
"""Bounded real-CPU diagnostics for source2's nonstructural direction indexes."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, sha
from capture_fighter2_attacks import snapshot


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--context', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and args.context.resolve().is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    context = json.loads(gzip.decompress(args.context.read_bytes()))
    checkpoint = dict(context['checkpoint'], data=bytes.fromhex(context['checkpoint']['data']))
    seed = context['natural_seed']
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    typ = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [typ, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    vectors = {}
    for index in [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, *range(32, 48)]:
        address = int.from_bytes(rom[index*4:index*4+4], 'big') & 0xffffff
        vectors.setdefault(address, []).append(index)
    targets = {index: 0x1690c+index*2+int.from_bytes(rom[0x1690c+index*2:0x1690e+index*2], 'big', signed=True)
               for index in range(11, 16)}
    points = sorted({0x168cc, 0x3d8, *targets.values(), *vectors})
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    records = []
    with ReferenceSession(ROOT, out, core=core) as ref:
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)
        observed = []
        stop = None
        boundary = None

        @typ
        def observe(kind, width, pc, value):
            nonlocal stop, boundary
            observed.append({'pc': hex(pc), 'sp': ref.get_register(15), 'sr': ref.get_register(17)})
            if pc == 0x3d8:
                stop = 'returned' if ref.get_register(15) == 0xffff04 else 'sentinel-with-wrong-sp'
            elif pc in vectors:
                stop = 'exception-vector-target'
            if stop is not None:
                boundary = {'ram': ref.ram_cpu(), 'registers': [ref.get_register(i) for i in range(21)]}
                ref.cpu.cycles = 1000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)

        def write(address, value, count=1):
            for i, byte in enumerate((value & ((1 << (count*8))-1)).to_bytes(count, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i) & 0xffff) ^ 1)).value = byte

        def write_ram(raw):
            native = bytearray(65536)
            native[0::2], native[1::2] = raw[1::2], raw[0::2]
            C.memmove(ref.ram_pointer, bytes(native), len(native))

        def ranges(before, after):
            result = []
            start = None
            for i in range(65537):
                changed = i < 65536 and before[i] != after[i]
                if changed and start is None:
                    start = i
                elif not changed and start is not None:
                    result.append({'start': hex(start), 'end_exclusive': hex(i),
                                   'before': before[start:i].hex(), 'after': after[start:i].hex()})
                    start = None
            return result

        for profile in ['zero-registers', 'natural-register-transplant', 'nonzero-d4-a4']:
            for direction in range(11, 16):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_ram(bytes.fromhex(seed['ram']))
                source = seed['source_register'] & 0xffff
                for index in range(15):
                    setreg(index, seed['registers'][index] if profile == 'natural-register-transplant' else 0)
                if profile == 'nonzero-d4-a4':
                    setreg(4, 0x12345678)
                    setreg(12, 0xffff8400)
                # Keep captured CCR only in the declared natural-register
                # profile. Interrupts are disabled for every private attempt.
                setreg(17, 0x2700 | (seed['registers'][17] & 0xff if profile == 'natural-register-transplant' else 0))
                setreg(15, 0xffff00)
                write(0xffff00, 0x3d8, 4)
                setreg(13, 0xffff0000+source)
                setreg(16, 0x168cc)
                write(0xff0000+source+0x58, 2)
                write(0xff0000+source+0x6b, 1)
                write(0xff0000+source+0x71, direction)
                write(0xff0000+source+0x72, 0)
                write(0xff0000+source+0x73, 0)
                write(0xff0000+source+0xb8, 100, 2)
                before = ref.ram_cpu()
                registers_before = [ref.get_register(i) for i in range(21)]
                before_state = ref.save()['data']
                ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                observed, stop, boundary = [], None, None
                hook(observer)
                execute(100000)
                hook(None)
                execution_end_ram = ref.ram_cpu()
                execution_end_registers = [ref.get_register(i) for i in range(21)]
                # The hook precedes the sentinel instruction. The core may
                # execute that one instruction before seeing our stop budget;
                # use the boundary snapshot for selector output, and retain
                # execution-end state separately without conflating the two.
                after = boundary['ram'] if boundary is not None else execution_end_ram
                registers_after = boundary['registers'] if boundary is not None else execution_end_registers
                records.append({'profile': profile, 'direction': direction, 'structural_table_entry': False,
                                'computed_target': hex(targets[direction]), 'source_id': 2, 'object_id': source,
                                'source_object': hex(source), 'cycle_budget': 100000,
                                'status': stop or 'not-returned-within-cycle-budget', 'observer_events': observed,
                                'exception_vectors_at_stop': vectors.get(int(observed[-1]['pc'], 16), []) if stop == 'exception-vector-target' else [],
                                'registers_before': registers_before, 'registers_after': registers_after,
                                'ram_before': before.hex(), 'ram_after': after.hex(), 'changed_ranges': ranges(before, after),
                                'before': snapshot(before, 0xff0000+source, rom), 'after': snapshot(after, 0xff0000+source, rom),
                                'before_core_state': before_state.hex(), 'execution_end_core_state': ref.save()['data'].hex(),
                                'execution_end_ram': execution_end_ram.hex(), 'execution_end_registers': execution_end_registers,
                                'context_limit': 'Declared private RAM/register transplant with private SP/return and interrupts disabled; not natural gameplay'})
        identity = ref.identity
    raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
    path = out/'fighter2-attacks-direction-diagnostics.json.gz'
    path.write_bytes(gzip.compress(raw, mtime=0))
    manifest = {'scope': 'Independent actual-CPU diagnostics; never mixed into normal source2 fixtures',
                'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
                'capture_dependencies': [{'path': 'scripts/yuyuhakusho/capture_fighter2_attacks.py', 'sha256': sha((ROOT/'scripts/yuyuhakusho/capture_fighter2_attacks.py').read_bytes())}],
                'context': {'path': str(args.context.resolve().relative_to(ROOT)), 'sha256': sha(args.context.read_bytes())},
                'records': len(records), 'cycle_budget_per_attempt': 100000,
                'fixture_sha256': sha(raw), 'gzip_sha256': sha(path.read_bytes()),
                'outcomes': [{'profile': row['profile'], 'direction': row['direction'], 'target': row['computed_target'],
                              'status': row['status'], 'pc': hex(row['registers_after'][16]),
                              'changed_bytes': sum(len(bytes.fromhex(part['before'])) for part in row['changed_ranges'])}
                             for row in records],
                'limits': ['Stopped at sentinel or first known exception-vector target, otherwise cycle budget',
                           'An observed vector-handler PC does not alone distinguish overlapping vector identities',
                           'Raw out-of-table target computation is not a structural table entry',
                           'No Go fallback, noop or strategy is inferred by this producer']}
    path.with_name('fighter2-attacks-direction-diagnostics-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
    print(json.dumps(manifest, indent=2), flush=True)


if __name__ == '__main__':
    main()
