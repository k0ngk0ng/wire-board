#!/usr/bin/env python3
"""Unique, observer-free source6 original-core3602 reference producer."""
import argparse
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--inputs', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--selection', required=True, type=Path)
    args = parser.parse_args()
    for path in [args.inputs, args.out, args.selection]:
        assert path.resolve().is_relative_to(ROOT)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    inputs = json.loads(args.inputs.read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
    assert sha(core.read_bytes()) == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    frames = []
    with ReferenceSession(ROOT, ROOT/'.local/md-yuyuhakusho/fighter6-reference', core=core) as ref:
        for frame in range(1, 3603):
            frames.append(ref.step(frame, buttons_at(inputs, frame))['reference'])
        identity = ref.identity
    raw = (json.dumps(frames, separators=(',', ':'))+'\n').encode()
    args.out.write_bytes(raw)
    report = {
        'scope': 'Shared unobserved original source6 real-menu3602 baseline; entries<=3600+2drain, not Go acceptance',
        'identity': identity,
        'input': {'path': str(args.inputs.resolve().relative_to(ROOT)), 'sha256': sha(args.inputs.read_bytes())},
        'frames': len(frames), 'entry_frame_limit': 3600, 'drain_frames': 2,
        'output': {'path': str(args.out.resolve().relative_to(ROOT)), 'sha256': sha(raw)},
        'observer_attached': False,
        'capture': {'path': str(Path(__file__).resolve().relative_to(ROOT)), 'sha256': sha(Path(__file__).read_bytes())},
        'selection_discovery': {'path': str(args.selection.resolve().relative_to(ROOT)), 'sha256': sha(args.selection.read_bytes())},
    }
    manifest = args.out.with_name(args.out.name.replace('-frames.json', '-manifest.json'))
    assert manifest != args.out
    manifest.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
