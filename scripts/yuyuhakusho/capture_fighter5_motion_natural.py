#!/usr/bin/env python3
"""Append another independently controlled natural source5 MAIN observation.

Frozen production, tests, boundaries and single-initial streams remain unchanged.
The original producer only observes the pinned hook core; it never records a new
unobserved-original control.
"""
import argparse
from collections import Counter
import gzip
import json
import os
from pathlib import Path
import subprocess
import sys
from reference import ROOT, sha


def main():
    parser = argparse.ArgumentParser()
    for name in ['input', 'reference', 'manifest', 'out']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    input_path, reference_path, manifest_path, out = [p.resolve() for p in
        [args.input, args.reference, args.manifest, args.out]]
    assert all(p.is_relative_to(ROOT) for p in [input_path, reference_path, manifest_path, out])
    out.mkdir(parents=True, exist_ok=True)
    target = ROOT / 'games/yuyuhakusho/engine/testdata'
    owned = [ROOT / 'games/yuyuhakusho/engine/fighter5_motion.go',
             ROOT / 'games/yuyuhakusho/engine/fighter5_motion_test.go',
             ROOT / 'scripts/yuyuhakusho/capture_fighter5_motion.py',
             target / 'fighter5-motion-boundaries.json.gz', target / 'fighter5-motion-boundaries-manifest.json',
             target / 'fighter5-motion-sequences.json.gz', target / 'fighter5-motion-sequences-manifest.json',
             target / 'fighter5-motion-parameters.json']
    frozen = {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in owned}
    freeze = json.loads((target / 'fighter5-motion-freeze.json').read_text())
    test_path = 'games/yuyuhakusho/engine/fighter5_motion_test.go'
    assert frozen[test_path] == freeze['files'][test_path]
    oracle_path = target / 'fighter5-motion-oracle.json.gz'
    oracle_manifest_path = target / 'fighter5-motion-oracle-manifest.json'
    old_packed, old_manifest_raw = oracle_path.read_bytes(), oracle_manifest_path.read_bytes()
    old_raw, old_manifest = gzip.decompress(old_packed), json.loads(old_manifest_raw)
    assert sha(old_raw) == old_manifest['fixture_sha256'] and sha(old_packed) == old_manifest['gzip_sha256']
    old = json.loads(old_raw)
    assert old_manifest['pass'] and old_manifest['frames_match_original_core']
    command = [sys.executable, str(ROOT / 'scripts/yuyuhakusho/capture_fighter5_motion.py'),
               '--input', str(input_path), '--reference', str(reference_path), '--manifest', str(manifest_path),
               '--out', str(out / 'observation'), '--natural-only']
    subprocess.run(command, check=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE='1'))
    appended_packed = (out / 'observation/fighter5-motion-oracle.json.gz').read_bytes()
    appended_manifest = json.loads((out / 'observation/fighter5-motion-oracle-manifest.json').read_text())
    appended_raw = gzip.decompress(appended_packed)
    assert appended_manifest['pass'] and appended_manifest['frames_match_original_core']
    assert sha(appended_raw) == appended_manifest['fixture_sha256'] and sha(appended_packed) == appended_manifest['gzip_sha256']
    assert appended_manifest['capture_sha256'] == frozen['scripts/yuyuhakusho/capture_fighter5_motion.py']
    appended = json.loads(appended_raw)
    assert not appended['library'] and appended['calls']
    source = str(input_path.relative_to(ROOT))
    scenario = input_path.name
    assert all(row['scenario'] == scenario and row['before']['character_id'] == 5 for row in appended['calls'])
    assert len(appended_manifest['baselines']) == 1 and appended_manifest['baselines'][0]['input'] == source
    previous = [row for row in old['calls'] if row['scenario'] != scenario]
    combined = previous + appended['calls']
    raw = (json.dumps({'library': old['library'], 'calls': combined}, separators=(',', ':')) + '\n').encode()
    packed = gzip.compress(raw, mtime=0)
    report = dict(old_manifest)
    report.update(scope='Actual source5 MAIN entries under ordinary and appended independently controlled real-menu physical inputs',
                  records=len(combined), calls=len(combined), kind_counts=dict(Counter(str(row['kind']) for row in combined)),
                  character_counts=dict(Counter(str(row['before']['character_id']) for row in combined)),
                  fixture_sha256=sha(raw), gzip_sha256=sha(packed),
                  allocation_attempts=sum(len(row['allocations']) for row in combined),
                  external_queue_calls=sum(bool(row['external_queue_clears']) for row in combined),
                  interrupt_calls=sum(len(row['interrupts']) for row in combined),
                  external_change_calls=sum(bool(row['external_changes']) for row in combined),
                  rng_calls=sum(row['rng_calls'] for row in combined),
                  unexplained_change_calls=sum(bool(row['unexplained_changes']) for row in combined),
                  baselines=[b for b in old_manifest['baselines'] if b['input'] != source] + appended_manifest['baselines'])
    report['natural_append'] = {'producer': str(Path(__file__).relative_to(ROOT)), 'producer_sha256': sha(Path(__file__).read_bytes()),
                                'scenario': source, 'added_calls': len(appended['calls']),
                                'added_kind_counts': appended_manifest['kind_counts'],
                                'original_producer_unchanged': True, 'frozen_motion_and_tests_unchanged': True}
    assert all(sha((ROOT / path).read_bytes()) == digest for path, digest in frozen.items())
    ledger_path = target / 'fighter5-motion-natural-append.json'
    prior_ledger = json.loads(ledger_path.read_text()) if ledger_path.exists() else None
    ledger = {'scope': 'Natural evidence appended only; frozen motion, tests, boundaries and continuous flows are unchanged',
              'producer_sha256': sha(Path(__file__).read_bytes()), 'command': command,
              'previous_oracle_raw_sha256': sha(old_raw), 'previous_oracle_gzip_sha256': sha(old_packed),
              'previous_manifest_sha256': sha(old_manifest_raw), 'previous_manifest_snapshot': old_manifest,
              'appended_manifest': appended_manifest, 'appended_frame_range': [min(row['frame'] for row in appended['calls']), max(row['frame'] for row in appended['calls'])],
              'combined_calls': len(combined), 'combined_raw_sha256': sha(raw), 'combined_gzip_sha256': sha(packed), 'frozen_bindings': frozen}
    if prior_ledger is not None:
        ledger.update(previous_append_sha256=sha(ledger_path.read_bytes()), previous_append_snapshot=prior_ledger)
    oracle_path.write_bytes(packed)
    oracle_manifest_path.write_text(json.dumps(report, indent=2) + '\n')
    ledger_path.write_text(json.dumps(ledger, indent=2) + '\n')
    print(json.dumps({'pass': True, 'appended_calls': len(appended['calls']), 'appended_kinds': appended_manifest['kind_counts'],
                      'combined_calls': len(combined), 'frozen_motion_tests_boundaries_streams_unchanged': True}), flush=True)


if __name__ == '__main__':
    main()
