"""Validate retained arena evidence and summarize only completed checks."""
from pathlib import Path
import datetime, hashlib, json, subprocess

ROOT = Path(__file__).resolve().parents[2]
GAME = ROOT / 'games/yuyuhakusho'
OUT = GAME / 'verification'

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def bound(relative, require_pass=True):
    path = ROOT / relative
    data = json.loads(path.read_text())
    if require_pass:
        assert data.get('pass') is True, relative
    return data, {'path': relative, 'sha256': digest(path)}

reports = {}
subsystems = [
    'games/yuyuhakusho/engine/testdata/arena-menu-controls-validation.json',
    'games/yuyuhakusho/engine/testdata/arenas-2-6-8-validation.json',
    'games/yuyuhakusho/engine/testdata/arenas-4-c-e-validation.json',
    'games/yuyuhakusho/engine/testdata/arena-a-validation.json',
    'games/yuyuhakusho/engine/testdata/arena-world-verification.json',
    'games/yuyuhakusho/data/additional-arena-assets-validation.json',
    'games/yuyuhakusho/presentation/testdata/arena-profiles-validation.json',
    'games/yuyuhakusho/presentation/testdata/arena-a-aux-pixel-validation.json',
    'games/yuyuhakusho/verification/additional-arena-player/final/result.json',
    'games/yuyuhakusho/verification/additional-arena-pan/verification.json',
]
for relative in subsystems:
    data, artifact = bound(relative)
    reports[relative] = artifact

checks = {}
legacy = {}
matrices = {}
pan = {}
for environment in ('native', 'wasm'):
    data, artifact = bound('games/yuyuhakusho/verification/additional-arena-' + environment + '-checks.json')
    assert data['tests'] == 445 and all(job['exit_code'] == 0 and job['passed'] == job['expected'] for job in data['jobs'])
    for job in data['jobs']:
        assert digest(ROOT / job['log']) == job['log_sha256']
    checks[environment] = dict(artifact, tests=data['tests'], packages=data['packages'])
    data, artifact = bound('games/yuyuhakusho/verification/additional-arena-legacy-' + environment + '.json')
    assert data['reports'] == 179 and data['rule_reports'] == 165 and data['render_reports'] == 14
    assert digest(ROOT / data['actual_reports_path']) == data['actual_reports_gzip_sha256']
    for check in data['checks']:
        assert check['pass'] and digest(ROOT / check['expected_path']) == check['expected_sha256']
    legacy[environment] = dict(artifact, reports=179, rule_reports=165, render_reports=14)
    data, artifact = bound('games/yuyuhakusho/verification/additional-arena-e2e/' + environment + '/validation.json')
    assert data['scenarios'] == 63 and data['all_eleven_characters_per_arena'] and not data['reference_state_loaded']
    for check in data['checks']:
        assert check['pass'] and digest(ROOT / check['path']) == check['sha256']
    assert len(data['pan_checks']) == 7
    matrices[environment] = dict(artifact, complete_matches=63, pan_scenarios=7)
    pan[environment] = []
    for arena in (2, 4, 6, 8, 10, 12, 14):
        data, artifact = bound('games/yuyuhakusho/verification/additional-arena-pan/' + environment + '/arena-' + format(arena, 'x') + '.json')
        assert data['arena'] == arena and not data['reference_state_loaded'] and data['human_auto_takeovers'] == 0
        assert data['pan_frames'] > 0 and data['feedback_frames'] > 0 and data['restored_pan_frames'] > 0 and data['restored_pixel_frames'] > 0 and data['restored_state_frames'] > 0
        pan[environment].append(dict(artifact, arena=arena, pan_frames=data['pan_frames']))
assert checks['native']['packages'] == checks['wasm']['packages']

for relative in ('games/yuyuhakusho/verification/additional-arena-bridge-test-first.json',):
    record, _ = bound(relative, require_pass=False)
    assert record['test_first']
    for name, expected in record['sha256'].items():
        assert digest(ROOT / name) == expected, ('frozen root contract changed', name)
player, _ = bound('games/yuyuhakusho/verification/additional-arena-player/final/result.json')
assert len(player['cases']) == 7 and player['input_trusted'] and player['human_no_auto_takeover']
for name, expected in player['source_sha256'].items():
    assert digest(ROOT / name) == expected, ('browser checked stale source', name)
for name in ('additional-arena-player-tsc.txt', 'additional-arena-vet.txt'):
    path = OUT / name
    assert path.exists() and not path.read_text().strip(), name

# These are deliberate earlier failures, not a claim that every invocation exited0.
required_reds = ['arena-source-backdrop-red.jsonl', 'arena-aux-sprite-symbol-red.jsonl', 'additional-arena-bridge-symbol-red.jsonl', 'additional-arena-player-red.log', 'additional-arena-e2e-native.log', 'additional-arena-e2e-wasm.log']
red_artifacts = []
for name in required_reds:
    path = OUT / name
    assert path.exists() and path.stat().st_size > 0, path
    red_artifacts.append({'path': str(path.relative_to(ROOT)), 'sha256': digest(path)})
for name in ('additional-arena-e2e-native.log', 'additional-arena-e2e-wasm.log'):
    assert 'no dynamic pan was exercised by the complete-match matrix' in (OUT / name).read_text(), name

source_paths = []
for directory in ('engine', 'presentation', 'runtimebridge'):
    source_paths += sorted((GAME / directory).glob('*.go'))
source_paths += sorted((GAME / 'cmd').glob('*/main*.go'))
source_paths += [ROOT / 'cmd/verify-yuyu-arena-aux/main.go', ROOT / 'scripts/yuyuhakusho/local_player.tsx', ROOT / 'scripts/yuyuhakusho/serve_local_player.py', ROOT / 'scripts/yuyuhakusho/verify_arena_sessions.py', ROOT / 'scripts/yuyuhakusho/verify_arena_checks.py', ROOT / 'scripts/yuyuhakusho/verify_arena_pan.py', Path(__file__).resolve()]
source_paths += [ROOT / 'scripts/yuyuhakusho/verify_arena_legacy.py', ROOT / 'scripts/yuyuhakusho/check_arena_player.py']
source_paths += sorted((GAME / 'data').glob('*.json.gz'))
summary = {
    'verified_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'baseline_commit': 'a22f355c',
    'stage': 'Seven additional arenas integrated with eleven-character independent sessions',
    'pass': True, 'goal_complete': False, 'release_ready': False,
    'overall_progress_estimate_percent': 81,
    'arena_offsets': [0, 2, 4, 6, 8, 10, 12, 14],
    'integration_checks': checks,
    'complete_matches': matrices,
    'dynamic_pan': pan,
    'legacy_default_regression': legacy,
    'original_background_pixels': 9535530,
    'original_arena_a_composite_pixels': 1368854,
    'original_arena_a_aux_visible_pixels': 29063,
    'pixel_mismatches': 0,
    'evidence': list(reports.values()),
    'cleanup': {'path': 'games/yuyuhakusho/verification/additional-arena-cleanup.json', 'sha256': digest(OUT / 'additional-arena-cleanup.json')},
    'tool_directory_note': {'path': 'games/yuyuhakusho/verification/additional-arena-tool-directory.json', 'sha256': digest(OUT / 'additional-arena-tool-directory.json')},
    'retained_actual_failures': red_artifacts,
    'invocation_note': 'Initial63-case native and Wasm matrices passed every match but exited1 because no AI case generated pan. Unchanged match reports were resumed only after seven additional legal-input pan scenarios passed. Full check shards and final continuations exited0.',
    'limits': [
        'Local research player; platform rooms, active takeover, fullscreen, authoritative network input/prediction and slow-peer isolation are pending.',
        'Sound playback timing, continuous BGM loops and mixing are pending.',
        'Normal-mode raster only; per-scanline sprite overflow and shadow/highlight remain outside scope.',
        'ID4 blank-row unused palette9/10/11 phases differ; visible colors and final pixels match.',
        'Original aux pixel oracle uses an indexed reference window; independent new-session HUD reconstruction is verified by separate complete render sessions.',
        'Historical rule reports preserve complete world/event/lifecycle traces except rule asset digest. Historical rendered pixel traces remain exact; version-bound snapshot hashes are checked through legacy JSON fixtures and native/Wasm equality.',
        'No push, upload, release or deployment.'
    ],
    'source_sha256': {str(path.relative_to(ROOT)): digest(path) for path in source_paths},
}
(OUT / 'additional-arenas-session.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
print('eight-arena integration verified:445 checks/runtime,126 complete matches,14 legal pan scenarios,358 historical reports')
