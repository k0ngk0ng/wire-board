#!/usr/bin/env python3
"""Physical two-controller source7 menu discovery, with no state injection."""
import argparse
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def number(ram, off, size=2):
    return int.from_bytes(ram[off:off + size], 'big')


def state(ref):
    ram = ref.ram_cpu()
    return {'frame': ref.frame, 'main_behavior': hex(number(ram, 0x8000, 4)), 'main_stage': number(ram, 0x8006),
            'header_a100': ram[0xa100:0xa120].hex(), 'characters': list(ram[0xa100:0xa104]),
            'controllers': list(ram[0xa104:0xa108]), 'mode_a01e': number(ram, 0xa01e),
            'sources': [ram[p + 0x58] for p in [0x8400, 0x8800, 0x8c00, 0x9000]],
            'channels': [ram[p + 0x76] for p in [0x8400, 0x8800, 0x8c00, 0x9000]],
            'parent_sizes': [number(ram, p + 4) for p in [0x8400, 0x8800, 0x8c00, 0x9000]]}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, default=ROOT / '.local/md-yuyuhakusho/source7-duo-discovery/menu')
    parser.add_argument('--inputs', type=Path, default=ROOT / 'docs/research/md-yuyuhakusho/source4-special-discovery-duo-menu-inputs.json')
    parser.add_argument('--mode', choices=['inspect', 'scan', 'replay'], default='inspect')
    parser.add_argument('--start', type=int, default=1305)
    parser.add_argument('--end', type=int, default=1374)
    parser.add_argument('--button', type=int, default=6)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = [dict(e, port=e.get('port', 0)) for e in json.loads(args.inputs.read_text())]
    failures = ['Wrong controller port or menu stage', 'P2 character confirmed before directional input',
                'Source/channel/mode/header does not describe two human actors', 'State injection or retagging',
                'Fixed input cold replay differs from discovered menu', 'Screenshot does not show tree as P2']
    (out / 'failure-checklist.json').write_text(json.dumps({'scope': 'Menu discovery only, no reference control producer', 'scenarios': failures}, indent=2) + '\n')
    if args.mode == 'replay':
        with ReferenceSession(ROOT, out / 'replay') as ref:
            for frame in range(1, 2401):
                ref.step(frame, buttons_at(events, frame))
            result = state(ref)
            result.update(identity=ref.identity, input_sha256=sha(args.inputs.read_bytes()), reference_control_generated=False)
            ref.screenshot(out / 'selection.png')
        (out / 'selection.json').write_text(json.dumps(result, indent=2) + '\n')
        print(json.dumps(result), flush=True)
        return
    with ReferenceSession(ROOT, out / 'inspect') as ref:
        checkpoints, samples = {}, []
        for frame in range(1, 2401):
            ref.step(frame, buttons_at(events, frame))
            if frame in [1000, 1100, 1200, 1256, 1299, 1305, 1350, 1399, 1406, 1506, 1700, 1800, 2000, 2400]:
                row = state(ref)
                samples.append(row)
                ref.screenshot(out / f'menu-{frame}.png')
                print(json.dumps(row), flush=True)
            if frame in [1106, 1200, 1256, 1299, 1506]:
                checkpoints[frame] = ref.save()
        (out / 'inspection.json').write_text(json.dumps({'identity': ref.identity, 'input_sha256': sha(args.inputs.read_bytes()), 'samples': samples}, indent=2) + '\n')
        if args.mode == 'scan':
            trials = []
            for button in [args.button]:
                for start in [args.start]:
                    for duration in range(1, args.end - args.start + 2):
                        checkpoint_frame = max(f for f in checkpoints if f < start)
                        ref.restore(checkpoints[checkpoint_frame], restore_refresh_cycles=True)
                        candidate = events + [{'start': start, 'end': min(start + duration - 1, 1845), 'buttons': [button], 'port': 1}]
                        for frame in range(checkpoint_frame + 1, 2401):
                            ref.step(frame, buttons_at(candidate, frame))
                        row = state(ref)
                        row.update(start=start, duration=duration, button=button)
                        trials.append(row)
                        print(json.dumps(row), flush=True)
                        if row['sources'][3] == 7 and row['sources'][0] == 3 and row['channels'][0] == 0 and row['channels'][3] == 1:
                            p = out / 'candidate-inputs.json'
                            candidate.sort(key=lambda e: (e['start'], e['port']))
                            p.write_text(json.dumps(candidate, indent=2) + '\n')
                            ref.screenshot(out / 'candidate.png')
                            (out / 'candidate-selection.json').write_text(json.dumps(row, indent=2) + '\n')
                            (out / 'trials.json').write_text(json.dumps(trials, indent=2) + '\n')
                            return
            (out / 'trials.json').write_text(json.dumps(trials, indent=2) + '\n')


if __name__ == '__main__':
    main()
