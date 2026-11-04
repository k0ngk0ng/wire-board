#!/usr/bin/env python3
"""Verify/copy source0 projectile evidence and export semantic read-only data."""
import argparse
import gzip
import json
from pathlib import Path
from reference import ROOT, sha


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--from', dest='source', type=Path, required=True); args = parser.parse_args()
    source = args.source.resolve(); assert source.is_relative_to(ROOT)
    target = ROOT/'games/yuyuhakusho/engine/testdata'; data = ROOT/'games/yuyuhakusho/data'
    capture_sha = sha((ROOT/'scripts/yuyuhakusho/capture_fighter0_projectiles.py').read_bytes())
    input_sha = sha((target/'fighter0-projectile-inputs.json').read_bytes())
    bindings = {}
    for name in ['fighter0-projectile-oracle', 'fighter0-projectile-boundaries', 'fighter0-projectile-sequences']:
        packed = (source/(name+'.json.gz')).read_bytes(); raw = gzip.decompress(packed)
        manifest_raw = (source/(name+'-manifest.json')).read_bytes(); manifest = json.loads(manifest_raw)
        assert manifest['pass'] and manifest['frames_match_original_core'] and not manifest['errors'] and manifest['incomplete_calls'] == 0
        assert manifest['external_queue_calls'] == 0 and manifest['external_change_calls'] == 0
        assert manifest['capture_sha256'] == capture_sha and manifest['inputs_sha256'] == input_sha
        assert manifest['fixture_sha256'] == sha(raw) and manifest['gzip_sha256'] == sha(packed)
        assert manifest['library_frames'] == 25 and manifest['short_sequences'] == 8
        (target/(name+'.json.gz')).write_bytes(packed); (target/(name+'-manifest.json')).write_bytes(manifest_raw)
        bindings[name] = {'fixture_sha256': sha(raw), 'gzip_sha256': sha(packed), 'calls': manifest['calls']}
    original = json.loads((source/'fighter0-projectile-oracle-manifest.json').read_text())
    frames_raw = (source/'fighter0-projectile-natural-reference-frames.json').read_bytes(); frames = json.loads(frames_raw)
    assert len(frames) == original['video_steps'] == 3602
    for i, row in enumerate(frames): assert row['frame'] == i+1
    packed = gzip.compress(frames_raw, mtime=0)
    (target/'fighter0-projectile-reference-frames.json.gz').write_bytes(packed)
    reference = {'pass': True, 'scope': 'Source0 special-flow passive observed/unobserved per-video RAM/video baseline',
                 'frames': len(frames), 'frames_match_original_core': True, 'identity': original['identity'],
                 'unobserved_identity': original['unobserved_identity'], 'inputs_sha256': input_sha,
                 'capture_sha256': capture_sha, 'fixture_sha256': sha(frames_raw), 'gzip_sha256': sha(packed),
                 'capture_manifest_sha256': sha((source/'fighter0-projectile-oracle-manifest.json').read_bytes()),
                 'export_sha256': sha(Path(__file__).read_bytes()),
                 'limits': ['Video labels are not logical tick counts', 'The accepted projectile entry boundary is3600; two extra video steps drain pending callbacks']}
    (target/'fighter0-projectile-reference-manifest.json').write_text(json.dumps(reference, indent=2)+'\n')
    library = json.loads((source/'fighter0-projectile-library.json').read_text())
    semantic = {'animations': [{'key': row['key'], 'data': row['data']} for row in library['animations']], 'sequences': library['sequences']}
    rules = (json.dumps(semantic, separators=(',', ':'))+'\n').encode()
    (data/'fighter0-projectile-rules.json').write_bytes(rules)
    report = {'pass': True, 'scope': 'Source0 child/projectile semantic data without CPU inputs, raw RAM or original instruction bytes',
              'animations': len(semantic['animations']), 'sequences': len(semantic['sequences']), 'rules_bytes': len(rules),
              'rules_sha256': sha(rules), 'capture_sha256': capture_sha, 'export_sha256': sha(Path(__file__).read_bytes()),
              'inputs_sha256': input_sha, 'baseline_gzip_sha256': sha(packed), 'evidence': bindings}
    (data/'fighter0-projectile-rules-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
