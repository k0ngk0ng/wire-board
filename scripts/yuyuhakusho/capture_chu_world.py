#!/usr/bin/env python3
"""Actual original whole-object-pass order and bounded ready-match snapshots."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def number(raw, off, n):
    return int.from_bytes(raw[off:off+n], 'big')


def controllers(ram):
    return [{'current': ram[0xf780+i] | ram[0xf7a8+i]<<8,
             'previous': ram[0xf788+i] | ram[0xf7b0+i]<<8,
             'changed': ram[0xf790+i] | ram[0xf7b8+i]<<8,
             'pressed': ram[0xf798+i] | ram[0xf7c0+i]<<8,
             'released': ram[0xf7a0+i] | ram[0xf7c8+i]<<8} for i in range(8)]


def snapshot(ram):
    return {'objects': ram[0x8000:0xa000].hex(), 'globals': ram[0xa000:0xa200].hex(),
            'shapes': ram[0xa200:0xa800].hex(), 'stream': ram[0x6000:0x6400].hex(),
            'clock': ram[0xf550:0xf590].hex(), 'seed': number(ram, 0xff18, 4),
            'palette_registers': ram[0xf500:0xf514].hex(), 'palettes': ram[0xe000:0xe180].hex(),
            'cells': ram[0xe600:0xec00].hex(), 'samples': ram[0xa800:0xa880].hex(),
            'registration': ram[0xa880:0xa900].hex(), 'stats': ram[0xff60:0x10000].hex(),
            'options': ram[0xff10:0xff18].hex(), 'queue': list(ram[0xf80a:0xf80e]), 'audio_pause': ram[0xf807],
            'hud_cursor': number(ram, 0xf518, 2), 'controllers': controllers(ram),
            'vertical': ram[0xe500:0xe540].hex(), 'pan_a': ram[0x6000:0x61c0].hex(), 'pan_b': ram[0x6200:0x63c0].hex(),
            'sources': ram[0xf598:0xf5a0].hex(), 'scroll_mode': ram[0xf6e7],
            'resource_list': number(ram, 0xf530, 4), 'resource_busy': ram[0xf54e]}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--start', type=int, default=1800)
    p.add_argument('--end', type=int, default=5500)
    p.add_argument('--trace-only', action='store_true')
    p.add_argument('--inputs', type=Path, default=ROOT/'docs/research/md-yuyuhakusho/inputs.json')
    p.add_argument('--name')
    p.add_argument('--allow-open-end', action='store_true')
    args = p.parse_args(); out = args.out.resolve()
    assert out.is_relative_to(ROOT) and 0 < args.start <= args.end <= 5500
    out.mkdir(parents=True, exist_ok=True)
    events_path = args.inputs.resolve()
    assert events_path.is_relative_to(ROOT)
    events = json.loads(events_path.read_text())
    if events_path == ROOT/'docs/research/md-yuyuhakusho/inputs.json':
        expected_path = ROOT/'games/yuyuhakusho/engine/testdata/match-lifecycle-reference-frames.json.gz'
        expected = json.loads(gzip.decompress(expected_path.read_bytes()))[:args.end]
    else:
        expected_path = out/'chu-world-special-reference-frames.json.gz'
        with ReferenceSession(ROOT, out/'unobserved') as original:
            expected = [original.step(f, buttons_at(events, f))['reference'] for f in range(1, args.end+1)]
        baseline = (json.dumps(expected, separators=(',', ':'))+'\n').encode()
        expected_path.write_bytes(gzip.compress(baseline, mtime=0))
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [0x3592, 0x35a2, 0x35a4, 0x3a2e, 0x3a6a, 0x37c56, 0x37cac, 0x3982]
    pcs = (C.c_uint*len(points))(*points)
    pending = None; rows = []; errors = []; kinds = {}; closed = False; driver_entries = []
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending, closed
            try:
                if not args.start <= ref.frame <= args.end or closed:
                    return
                sp = ref.get_register(15)
                if pc == 0x3a2e:
                    driver_entries.append(ref.frame)
                elif pc == 0x3592:
                    assert pending is None, 'Overlapping object pass'
                    ram = ref.ram_cpu()
                    assert number(ram, 0x8000, 4) == 0x9ad0 and number(ram, 0x8006, 2) == 20, 'Not ready-match wait container'
                    pending = {'frame': ref.frame, 'calls': [], 'rng_calls': 0, 'queue_clears': [],
                               'manager_stage_before': number(ram, 0x8046, 2), 'stack': sp,
                               'driver_entries': list(driver_entries)}
                    driver_entries.clear()
                    if not args.trace_only: pending['before'] = snapshot(ram)
                elif pending is not None and pc == 0x35a2:
                    ram = ref.ram_cpu(); full = ref.get_register(13); address = full&0xffff
                    call = {'object_id': address, 'a5': full, 'behavior': number(ram, address, 4),
                            'size': number(ram, address+4, 2), 'stage': number(ram, address+6, 2),
                            'saved_stride': number(ram, sp&0xffff, 2)}
                    assert call['saved_stride'] == call['size']
                    pending['calls'].append(call)
                    key = str(call['behavior'])
                    stat = kinds.setdefault(key, {'behavior': call['behavior'], 'calls': 0, 'ids': set(), 'stages': set(), 'first_frame': ref.frame, 'last_frame': ref.frame})
                    stat['calls'] += 1; stat['ids'].add(address); stat['stages'].add(call['stage']); stat['last_frame'] = ref.frame
                elif pending is not None and pc == 0x35a4:
                    assert pending['calls']
                    pending['calls'][-1]['a5_after'] = ref.get_register(13)
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                elif pending is not None and pc in [0x37c56, 0x37cac]:
                    address = ref.get_register(9)&0xffff
                    pending['queue_clears'].append({'pc': hex(pc), 'address': address, 'before': ref.ram_cpu()[address], 'sr': ref.get_register(17)})
                elif pending is not None and pc == 0x3a6a:
                    assert sp == pending['stack']+4, 'Object pass return stack mismatch'
                    ram = ref.ram_cpu(); pending['manager_stage_after'] = number(ram, 0x8046, 2)
                    pending['cleared_world'] = number(ram, 0x8040, 4) == 0
                    if not args.trace_only: pending['after'] = snapshot(ram)
                    rows.append(pending); pending = None
                    if rows[-1]['cleared_world']: closed = True
            except Exception as exc:
                errors.append(str(exc))
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, args.end+1)]
        hook(None)
        assert frames == expected and not pending and not errors, (errors[:10], bool(pending), len(rows))
        identity = ref.identity
    for stat in kinds.values(): stat['ids'] = sorted(stat['ids']); stat['stages'] = sorted(stat['stages'])
    raw = (json.dumps(rows, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
    name = args.name or ('chu-world-trace' if args.trace_only else 'chu-world-oracle')
    assert name.replace('-', '').isalnum() and name.startswith('chu-world-')
    (out/(name+'.json.gz')).write_bytes(zipped)
    report = {'scope': 'Actual whole3592 ready-match passes; no Go execution or world completion claim',
              'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'inputs_sha256': sha(events_path.read_bytes()),
              'inputs_path': str(events_path.relative_to(ROOT)),
              'baseline_sha256': sha(expected_path.read_bytes()), 'frames_checked': len(frames), 'records': len(rows),
              'first_frame': rows[0]['frame'] if rows else None, 'last_frame': rows[-1]['frame'] if rows else None,
              'object_calls': sum(len(r['calls'])for r in rows), 'kinds': sorted(kinds.values(), key=lambda k:k['behavior']),
              'queue_interleaved_passes': sum(bool(r['queue_clears'])for r in rows), 'closed_after_clear': closed,
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
              'incomplete_calls': 0, 'errors': errors, 'allow_open_end': args.allow_open_end,
              'pass': bool(rows)and(closed or args.allow_open_end)and not errors}
    (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items()if k!='kinds'},indent=2))
    for stat in report['kinds']: print(hex(stat['behavior']), stat['calls'], [hex(i)for i in stat['ids']], stat['stages'])


if __name__ == '__main__': main()
