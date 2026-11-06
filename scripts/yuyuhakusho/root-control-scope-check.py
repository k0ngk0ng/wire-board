"""Verify the retained Root-control rules evidence without running a core."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def read_json(path):
    data = path.read_bytes()
    if path.name.endswith('.gz'):
        data = gzip.decompress(data)
    return json.loads(data)


def bound_path(relative):
    path = (ROOT / relative).resolve()
    assert path.is_relative_to(ROOT), relative
    return path


def checked_hash(relative, expected):
    path = bound_path(relative)
    assert hashlib.sha256(path.read_bytes()).hexdigest() == expected, relative
    return path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, default=ROOT / 'games/yuyuhakusho/verification/root-control/validation.json')
    args = parser.parse_args()
    path = args.manifest.resolve()
    assert path.is_relative_to(ROOT)
    data = read_json(path)
    assert data['version'] == 1 and data['pass']
    assert data['persistent_fields'] == ['Priority', 'Selected', 'ChoiceBits', 'Fade', 'PauseRestore']
    assert not data['full_audio_complete'] and not data['full_world_IRQ_verified']
    assert not data['complete_root_cycle_cost_verified']
    assert data['native_frames'] == 1115 and data['rules_frames'] == 1113
    assert data['rules_requests'] == 8480 and data['excluded_ordinary_start_frames'] == 2
    for relative, expected in data['files'].items():
        checked_hash(relative, expected)
    for entry in data['component_manifests']:
        child = read_json(checked_hash(entry['path'], entry['sha256']))
        if entry['hash_map']:
            for relative, expected in child[entry['hash_map']].items():
                checked_hash(relative, expected)
    preserved = read_json(bound_path(data['preserved_native_manifest']))
    source_map = {}
    for entry in preserved['files']:
        retained = checked_hash(entry['retained'], entry['sha256'])
        assert retained.stat().st_size == entry['bytes']
        source_map[entry['source']] = retained
    native_frames = 0
    for source in data['current_native_captures']:
        capture = read_json(source_map[source])
        assert capture['pass'] and capture['actual_PSG_byte_store_coverage']
        for case in capture['cases']:
            assert case['pass']
            observed_path = case['artifact']['path']
            observed = read_json(source_map[observed_path])
            control_path = observed_path.removesuffix('.json.gz') + '-control.json.gz'
            control = read_json(source_map[control_path])
            assert observed['frames'] == control['frames'], case['id']
            assert not observed['observer_errors'], case['id']
            assert len(observed['frames']) == case['frames']
            checked_hash(str(source_map[observed_path].relative_to(ROOT)), case['artifact']['sha256'])
            checked_hash(str(source_map[control_path].relative_to(ROOT)), case['control_sha256'])
            native_frames += case['frames']
    assert native_frames == data['native_frames']
    rules_frames, rules_requests = 0, 0
    for entry in data['rule_fixtures']:
        fixture = read_json(bound_path(entry['path']))
        assert len(fixture['fight_candidate_ids']) == 139
        assert 0x7f not in fixture['fight_candidate_ids']
        for case in fixture['cases']:
            first = entry['start_at'].get(case['id'], 0)
            frames = case['frames'][first:]
            rules_frames += len(frames)
            rules_requests += sum(len(frame['requests']) for frame in frames)
    assert (rules_frames, rules_requests) == (1113, 8480)
    tests = ['TestOrchestratorRetainedNative785FramesV3', 'TestOrchestratorFightNative180FramesV3',
             'TestOrchestratorFightActivePCM04TypedCheckpointV3',
             'TestOrchestratorLeasePrecedesEveryRootAndScoreCallback']
    for relative in data['final_execution_logs']:
        rows = [json.loads(line) for line in bound_path(relative).read_text().splitlines()]
        assert not any(row['Action'] == 'fail' for row in rows), relative
        assert rows[-1]['Action'] == 'pass' and 'Test' not in rows[-1], relative
        passed = {row.get('Test') for row in rows if row['Action'] == 'pass'}
        assert set(tests).issubset(passed), relative
    print(json.dumps({'pass': True, 'files': len(data['files']), 'preserved_native_files': len(source_map),
                      'native_frames': native_frames, 'rules_frames': rules_frames,
                      'rules_requests': rules_requests, 'no_core_or_Go_execution': True}, indent=2))


if __name__ == '__main__':
    main()
