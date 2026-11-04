#!/usr/bin/env python3
"""Passive manager-object/header inventory against the pinned original run."""
import argparse
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def n(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    p.add_argument('--frames', type=int, default=3600)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    headers, previous, changes = {}, {}, []
    with ReferenceSession(ROOT, out) as ref:
        frames = []
        for frame in range(1, args.frames+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            ram = ref.ram_cpu()
            row = {'frame': frame, 'objects': {}, 'battle': n(ram, 0xa03a, 2), 'pause': ram[0xf550]}
            for owner in [0x8000, 0x8040, 0x8080, 0x80c0, 0x8100, 0x8200, 0x8300]:
                value = (n(ram, owner, 4), n(ram, owner+4, 2), n(ram, owner+6, 2))
                row['objects'][hex(owner)] = value
                headers.setdefault(hex(owner), {})[str(value)] = headers.setdefault(hex(owner), {}).get(str(value), 0)+1
            row['fighters'] = [[n(ram, a, 4), n(ram, a+0x78, 2, True), ram[a+0x6f]] for a in [0x8400, 0x8800, 0x8c00, 0x9000]]
            row['winner'] = n(ram, 0xa074, 2, True)
            identity = {'objects': row['objects'], 'battle': row['battle'], 'pause': row['pause'], 'fighters': row['fighters'], 'winner': row['winner']}
            if identity != previous:
                changes.append(row)
                previous = identity
            if frame in [1500, 1800, 2400, 3600]:
                (out/f'ram-{frame}.bin').write_bytes(ram)
        assert frames[:3600] == expected
        report = {'scope': 'Passive object headers over original input then released controls', 'identity': ref.identity,
                  'headers': headers, 'changes': changes, 'frames_match_original_core': True,
                  'script_sha256': sha(Path(__file__).read_bytes()), 'frames': args.frames, 'pass': True,
                  'limits': ['Headers identify candidates only, not complete lifecycle semantics or all modes']}
    (out/'manager-headers.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'pass': True, 'change_rows': len(changes), 'headers': headers}, indent=2))


if __name__ == '__main__':
    main()
