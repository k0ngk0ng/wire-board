#!/usr/bin/env python3
"""Observe complete original animation-clock and residual-motion calls."""
import argparse
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def clock_fields(ram, actor):
    return {"frame_index": ram[actor+0x59], "duration": ram[actor+0x5a],
            "ticks": ram[actor+0x5b], "flags": ram[actor+0x5c],
            "completed": ram[actor+0x60]}


def drift_fields(ram, actor):
    return {"position": int.from_bytes(ram[actor+0x40:actor+0x44], 'big', signed=True),
            "residual": int.from_bytes(ram[actor+0x98:actor+0x9c], 'big', signed=True),
            "facing": ram[actor+0x61]}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    out = args.out.resolve()
    if not out.is_relative_to(ROOT):
        raise ValueError('output must remain inside worktree')
    out.mkdir(parents=True, exist_ok=True)
    events_path = ROOT/'docs/research/md-yuyuhakusho/inputs.json'
    baseline_path = ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json'
    events = json.loads(events_path.read_text())
    expected = json.loads(baseline_path.read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    definitions = {
        0x19c96: ('clock', 0x19cc2, clock_fields, (0x59, 0x5b, 0x60)),
        0x19cc4: ('drift', 0x19cec, drift_fields, tuple(range(0x40, 0x44)) + tuple(range(0x98, 0x9c))),
    }
    exits = {definition[1]: entry for entry, definition in definitions.items()}
    addresses = list(definitions) + list(exits)
    pcs = (C.c_uint * len(addresses))(*addresses)
    rows, pending, errors = [], {}, []
    with ReferenceSession(ROOT, out, core=core) as ref:
        @callback_type
        def observe(kind, width, pc, value):
            try:
                actor = ref.get_register(13) & 0xffff
                if not 0x8000 <= actor <= 0x9f00:
                    raise ValueError(f'Unexpected actor pointer {actor:x}')
                ram = ref.ram_cpu()
                if pc in definitions:
                    name, exit_pc, fields, offsets = definitions[pc]
                    key = (pc, actor)
                    if key in pending:
                        raise ValueError(f'Overlapping {name} call at {actor:x}')
                    pending[key] = {'frame': ref.frame, 'actor': hex(actor), 'entry': hex(pc),
                                    'exit': hex(exit_pc), 'kind': name, name+'_before': fields(ram, actor),
                                    '_ram': ram, '_sp': ref.get_register(15)}
                else:
                    entry = exits[pc]
                    name, _, fields, offsets = definitions[entry]
                    row = pending.pop((entry, actor))
                    before = row.pop('_ram')
                    if ref.get_register(15) != row.pop('_sp'):
                        raise ValueError('Unexpected stack change before RTS')
                    changed = [i for i, (a, b) in enumerate(zip(before, ram)) if a != b]
                    allowed = {actor+i for i in offsets}
                    # The frontend can finish a video frame in the middle of
                    # a 68000 call; Z80/VDP work may change unrelated RAM before
                    # the next instruction resumes. Preserve those observations
                    # rather than attributing them to this bounded function.
                    unexpected = [hex(i) for i in changed if actor <= i < actor+0x100 and i not in allowed]
                    if unexpected:
                        raise ValueError(f'{name} wrote outside known fields: {unexpected[:12]}')
                    row['exit_frame'] = ref.frame
                    row[name+'_after'] = fields(ram, actor)
                    row['changed_offsets'] = [hex(i-actor) for i in changed if actor <= i < actor+0x100]
                    row['external_changes'] = [[hex(i), before[i], ram[i]] for i in changed if not actor <= i < actor+0x100]
                    rows.append(row)
            except Exception as exc:
                if len(errors) < 50:
                    errors.append(str(exc))
        assert bridge.yyh_set_observer(observe, pcs, len(pcs)) == 1
        ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
        ref.api('set_cpu_hook', None, C.c_void_p)(None)
        fixture = out/'animation-oracle.json'
        fixture.write_text(json.dumps(rows, separators=(',', ':'))+'\n')
        clocks = [r for r in rows if r['kind'] == 'clock']
        drifts = [r for r in rows if r['kind'] == 'drift']
        coverage = {
            'clock_calls': len(clocks), 'drift_calls': len(drifts),
            'clock_tick_byte_wraps': sum(r['clock_before']['ticks'] == 255 for r in clocks),
            'clock_zero_duration': sum(r['clock_before']['duration'] == 0 for r in clocks),
            'clock_index_byte_wraps': sum(r['clock_before']['frame_index'] == 255 and r['clock_after']['frame_index'] == 0 for r in clocks),
            'clock_loop_completion': sum(r['clock_after']['completed'] == 1 and r['clock_before']['completed'] != 1 for r in clocks),
            'clock_preserved_completion': sum(r['clock_before']['completed'] != 0 and r['clock_after']['completed'] == r['clock_before']['completed'] for r in clocks),
            'drift_positive': sum(r['drift_before']['residual'] > 0 for r in drifts),
            'drift_negative': sum(r['drift_before']['residual'] < 0 for r in drifts),
            'drift_zero': sum(r['drift_before']['residual'] == 0 for r in drifts),
            'drift_crosses_zero': sum(0 < r['drift_before']['residual'] < 0x4800 for r in drifts),
            'drift_facing_values': sorted({r['drift_before']['facing'] for r in drifts}),
        }
        report = {'scope': 'Two bounded original functions, no RAM edits; not a full character update',
                  'identity': ref.identity, 'filter_sha256': sha(bridge_path.read_bytes()),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'inputs_sha256': sha(events_path.read_bytes()),
                  'baseline_sha256': sha(baseline_path.read_bytes()), 'fixture_sha256': sha(fixture.read_bytes()),
                  'frames': 3600, 'complete_calls': len(rows), 'incomplete_calls': len(pending),
                  'errors': errors, 'all_actor_ram_writes_within_declared_fields': not errors,
                  'calls_with_external_ram_changes': sum(bool(r['external_changes']) for r in rows),
                  'calls_crossing_video_frame': sum(r['exit_frame'] != r['frame'] for r in rows),
                  'external_changes_crossing_video_frame': sum(bool(r['external_changes']) and r['exit_frame'] != r['frame'] for r in rows),
                  'limitations': ['Per-call external RAM changes may include intervening Z80/VDP work; they are retained, not attributed to the observed function.',
                                  'Natural samples do not establish unobserved arithmetic boundaries or complete animation behavior.'],
                  'frames_match_original_core': frames == expected, 'coverage': coverage,
                  'pass': bool(clocks) and bool(drifts) and not errors and not pending and frames == expected}
        (out/'animation-oracle-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))
    assert report['pass'], 'Observer drift, incomplete calls or unexpected side effects'


if __name__ == '__main__':
    main()
