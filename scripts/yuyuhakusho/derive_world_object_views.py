#!/usr/bin/env python3
"""Bounded representative original-CPU rows for canonical presentation adapters.

This derives checks from existing source-bound CPU fixtures; it performs no
new target execution and never invents original outputs.
"""
import argparse
from collections import Counter
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def original(name):
    td = ROOT/'games/yuyuhakusho/engine/testdata'
    zipped = (td/(name+'.json.gz')).read_bytes()
    raw = gzip.decompress(zipped)
    mp = td/(name+'-manifest.json')
    m = json.loads(mp.read_text())
    assert m['pass'] and m['frames_match_original_core']
    assert m['gzip_sha256'] == sha(zipped) and m['fixture_sha256'] == sha(raw)
    data = json.loads(raw)
    return data, {'fixture': str((td/(name+'.json.gz')).relative_to(ROOT)), 'gzip_sha256': sha(zipped),
                  'fixture_sha256': sha(raw), 'manifest': str(mp.relative_to(ROOT)), 'manifest_sha256': sha(mp.read_bytes())}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    args = p.parse_args(); out = args.out.resolve()
    assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    for name, target in [('match-overlays-boundaries', 'world-objects-overlay-samples'),
                         ('default-arena-boundaries', 'world-objects-sky-samples')]:
        data, source = original(name)
        selected, seen = [], Counter()
        for row in data['calls']:
            kind = row['kind']
            if name.startswith('default-arena'):
                if kind not in ['sun', 'moon', 'spark'] or row.get('reference_only'):
                    continue
                off = {'sun': 0, 'moon': 128, 'spark': 256}[kind]
                b = bytes.fromhex(row['before']['sky'])[off:off+128]
            else:
                if kind not in ['intro', 'intro-part', 'draw', 'win', 'character', 'team', 'partner', 'shadow']:
                    continue
                if kind == 'shadow':
                    b = bytes.fromhex(row['before']['shadow_bytes'])
                else:
                    off = row['object_id']-0x8100
                    b = bytes.fromhex(row['before']['objects'])[off:off+64]
            stage = int.from_bytes(b[6:8], 'big')
            key = (kind, stage, bool(row['before']['pause']&128), bool(row['before']['slow']),
                   row['draw'], row.get('cleared_bytes', 0), row.get('spawned', False))
            if seen[key] < 2:
                selected.append(row); seen[key] += 1
        payload = {'calls': selected}
        if 'library' in data:
            payload = {'library': data['library'], **payload}
        raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(target+'.json.gz')).write_bytes(zipped)
        m = {'scope': 'Derived representative existing original-CPU cases, not a new execution or world simulation',
             'source': source, 'records': len(selected), 'kind_counts': dict(Counter(r['kind'] for r in selected)),
             'selection': 'First two rows per(kind,stage,pause-sign,slow-nonzero,draw,clear-prefix,spawned); overlay only intro/intro-part/draw/win/character/team/partner/shadow; sky only sun/moon/spark, reference-only rows excluded',
             'derive_script_sha256': sha(Path(__file__).read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
             'frames_match_original_core': True, 'pass': bool(selected)}
        (out/(target+'-manifest.json')).write_text(json.dumps(m, indent=2)+'\n')
        print(json.dumps({'name': target, 'records': len(selected), 'kinds': m['kind_counts']}))


if __name__ == '__main__':
    main()
