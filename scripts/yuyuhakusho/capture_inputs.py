#!/usr/bin/env python3
"""Capture complete original controller-normalization calls and prove no drift."""
import argparse
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    pcs = (C.c_uint * 4)(0x2dce, 0x2e32, 0x2d5e, 0x2d94)
    rows, pending, errors = [], {}, []
    with ReferenceSession(ROOT, out, core=core) as ref:
        def fields(ram, slot):
            base = 0xf780 + slot
            return [ram[base+i*8] for i in range(11)]
        @callback_type
        def observe(kind, width, pc, value):
            try:
                ram = ref.ram_cpu()
                a0 = ref.get_register(8) & 0xffff
                a1 = ref.get_register(9) & 0xffff
                slot = a1 - 0xf780
                if not 0 <= slot < 8:
                    raise ValueError(f'Unexpected input channel {a1:x}')
                if pc in (0x2dce, 0x2d5e):
                    if slot in pending:
                        raise ValueError('Overlapping controller call')
                    pending[slot] = {'frame': ref.frame, 'channel': slot, 'entry': hex(pc),
                                     'six_buttons': pc == 0x2dce, 'raw_nibbles': list(ram[a0+2:a0+5]),
                                     'entry_d0': ref.get_register(0), 'entry_d1': ref.get_register(1),
                                     'swap_ac': ram[(a1+0x7d0) & 0xffff] != 0,
                                     'before': fields(ram, slot)}
                else:
                    row = pending.pop(slot)
                    row['exit'] = hex(pc)
                    row['after'] = fields(ram, slot)
                    rows.append(row)
            except Exception as exc:
                errors.append(str(exc))
        assert bridge.yyh_set_observer(observe, pcs, len(pcs)) == 1
        ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frame_records = []
        for frame in range(1, 3601):
            frame_records.append(ref.step(frame, buttons_at(events, frame))['reference'])
        ref.api('set_cpu_hook', None, C.c_void_p)(None)
        report = {'scope': 'Natural input calls, no RAM edits', 'identity': ref.identity,
                  'filter_sha256': sha(bridge_path.read_bytes()), 'frames': 3600,
                  'complete_calls': len(rows), 'incomplete_calls': list(pending.values()),
                  'errors': errors, 'frames_match_original_core': frame_records == expected,
                  'six_button_calls': sum(r['six_buttons'] for r in rows),
                  'three_button_calls': sum(not r['six_buttons'] for r in rows),
                  'pass': not errors and not pending and len(rows) > 0 and frame_records == expected}
        (out/'inputs-oracle.json').write_text(json.dumps(rows, separators=(',', ':'))+'\n')
        report['fixture_sha256'] = sha((out/'inputs-oracle.json').read_bytes())
        (out/'report.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))
    assert report['pass'], 'Observer drift or incomplete evidence; inspect report'


if __name__ == '__main__':
    main()
