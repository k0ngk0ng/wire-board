"""Verify the final source4 evidence, including the isolated schema metadata recheck."""
from datetime import datetime, timezone
from pathlib import Path
import gzip
import hashlib
import json
import shutil

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT/'games/yuyuhakusho/verification'
LOCAL = ROOT/'.local/md-yuyuhakusho'
ENGINE = 'github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine'
RECHECK = 'TestFighter0RuleVersionAndInvalidSequences'
def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()
def read(path): return json.loads(path.read_text())
def bound(path): return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path)}
def copy_log(name):
    src, dst = LOCAL/name, OUT/name
    shutil.copyfile(src, dst)
    return dst, [json.loads(line) for line in dst.read_text().splitlines()]

def top_pass(rows):
    return [(r['Package'], r['Test']) for r in rows if r.get('Action') == 'pass' and r.get('Test') and '/' not in r['Test']]

inventory = read(OUT/'hiei-native-shards.json')
expected = {(p['package'], n) for p in inventory['packages'] for n in p['tests']}
assert len(expected) == 327
change = read(OUT/'hiei-source0-version-recheck.json')
changed = {p for p, h in inventory['source_sha256'].items() if sha(ROOT/p) != h}
assert changed == {change['source_path']}, changed
assert inventory['source_sha256'][change['source_path']] == change['source_before_sha256']
assert sha(ROOT/change['source_path']) == change['source_after_sha256']
original = (ROOT/change['source_path']).read_text().replace(change['exact_replacement']['after'], change['exact_replacement']['before'])
assert hashlib.sha256(original.encode()).hexdigest() == change['source_before_sha256']
assert not change['production_changed']

integration = []
for target in ('native', 'wasm'):
    names = [f'hiei-native-shard-{i}.jsonl' for i in range(4)] + ['hiei-native-animation.jsonl'] if target == 'native' else ['hiei-integration-wasm.jsonl']
    rows, logs = [], []
    for name in names:
        path, entries = copy_log(name)
        assert not any(r.get('Action') == 'build-fail' for r in entries), name
        assert any(r.get('Action') in ('pass', 'fail') and not r.get('Test') for r in entries), name
        rows += entries
        logs.append(bound(path))
    failures = {(r['Package'], r['Test']) for r in rows if r.get('Action') == 'fail' and r.get('Test')}
    assert failures == {(ENGINE, RECHECK)}, failures
    passed = top_pass(rows)
    assert len(passed) == len(set(passed)) == 326 and set(passed) == expected - failures
    path, retry = copy_log(f'hiei-source0-version-{target}.jsonl')
    assert not any(r.get('Action') in ('fail', 'build-fail') for r in retry)
    assert set(top_pass(retry)) == failures
    assert any(r.get('Action') == 'pass' and not r.get('Test') for r in retry)
    integration.append({'environment': target, 'pass': True, 'final_test_count': len(expected), 'initial_pass_count': len(passed),
                        'initial_failure': RECHECK, 'resolution': 'Version metadata guard accepts schema2/3; all existing invalid-sequence and schema1 checks remain unchanged.',
                        'initial_invocations': logs, 'targeted_recheck': bound(path),
                        'final_passed_tests': [{'package': p, 'test': n} for p, n in sorted(set(passed) | set(top_pass(retry)))]})

assets = []
for target in ('native', 'wasm'):
    passed, logs = [], []
    for name in (f'hiei-presentation-{target}.jsonl', f'hiei-assets-{target}.jsonl'):
        path, rows = copy_log(name)
        assert not any(r.get('Action') in ('fail', 'build-fail') for r in rows), name
        assert any(r.get('Action') == 'pass' and not r.get('Test') for r in rows), name
        passed += top_pass(rows)
        logs.append(bound(path))
    assert len(passed) == len(set(passed)) == 27
    assets.append({'environment': target, 'pass': True, 'test_count': len(passed), 'invocations': logs})

matches = []
for kind, wanted in (('rules', 21), ('render', 12)):
    paths = sorted((OUT/'hiei-e2e/native'/kind).glob('*.json'))
    assert len(paths) == wanted
    for path in paths:
        other = OUT/'hiei-e2e/wasm'/kind/path.name
        a, b = read(path), read(other)
        assert a['pass'] and b['pass'] and 4 in a['characters'] and not a['reference_state_loaded']
        assert a.pop('environment') != b.pop('environment') and a == b, path
        if kind == 'rules':
            assert a['completed'] and a['human_auto_takeovers'] == 0
            assert 0 < a['first_damage_tick'] <= a['first_ko_tick'] <= a['retirement_tick'] <= a['result_tick'] <= a['clear_tick']
        else:
            assert a['complete'] and a['hud_window_complete'] and a['restored_pixel_frames'] == 30
        matches.append({'case': kind+'/'+path.stem, 'native': bound(path), 'wasm': bound(other), 'ticks': a['ticks'],
                        'characters': a['characters'], 'final_state_sha256': a['final_state_sha256'],
                        **({'trace_sha256': a['trace_sha256']} if kind == 'rules' else {'pixel_trace_sha256': a['pixel_trace_sha256']})})
legacy_path = OUT/'hiei-legacy-regression.json'
legacy = read(legacy_path)
assert legacy['pass'] and len(legacy['checks']) == 192 and all(row['pass'] for row in legacy['checks'])
packed = (ROOT/legacy['actual_reports_path']).read_bytes()
raw = gzip.decompress(packed)
assert hashlib.sha256(raw).hexdigest() == legacy['actual_reports_sha256'] and hashlib.sha256(packed).hexdigest() == legacy['actual_reports_gzip_sha256']
actuals = json.loads(raw)
for row in legacy['checks']:
    a, b = actuals[row['actual_key']].copy(), read(ROOT/row['expected_path'])
    assert a.pop('rules_sha256') == row['rules_sha256'] and b.pop('rules_sha256') == row['previous_rules_sha256']
    assert a == b, row['case']

browser_path = OUT/'hiei-local-player/result.json'
browser = read(browser_path)
assert browser['pass'] and browser['input_trusted'] and browser['viewport'] == 320 and browser['human_idle_no_takeover']
assert browser['final_view']['players'][0]['character_id'] == 4 and browser['final_view']['players'][0]['input_channel'] == 0
assert browser['source4_forward_attack']['Action'] == 3 and browser['source4_forward_attack']['Pose'] == 131
for path, digest in browser['source_sha256'].items(): assert sha(ROOT/path) == digest
checks = []
for name in ('hiei-vet.txt', 'hiei-wasm-vet.txt', 'hiei-player-tsc.txt'):
    p = LOCAL/name
    assert p.is_file() and not p.read_text().strip(), p
    dst = OUT/name
    shutil.copyfile(p, dst)
    checks.append({**bound(dst), 'pass': True})
shutil.copyfile(LOCAL/'hiei-player.txt', OUT/'hiei-player.txt')

worlds = []
for name, counts in [('chu-world-fighter4', (3032, 39782, None)), ('chu-world-fighter4-special', (1801, 24154, 12114)), ('chu-world-fighter4-duo', (3001, 34236, 18901))]:
    p = ROOT/f'games/yuyuhakusho/engine/testdata/{name}-manifest.json'
    m = read(p)
    assert m['pass'] and m['frames_match_original_core'] and m['incomplete_calls'] == 0 and not m['errors']
    assert (m['records'], m['object_calls']) == counts[:2]
    if counts[2] is not None: assert m['draws'] == counts[2]
    fixture = p.with_name(name+'.json.gz')
    data = fixture.read_bytes()
    assert hashlib.sha256(data).hexdigest() == m['gzip_sha256'] and hashlib.sha256(gzip.decompress(data)).hexdigest() == m['fixture_sha256']
    worlds.append({'manifest': bound(p), 'fixture': bound(fixture), 'passes': counts[0], 'callbacks': counts[1],
                   'ordered_draws': counts[2], 'first_video_label': m['first_frame'], 'last_video_label': m['last_frame'],
                   'control_frames': m['frames_checked'], 'single_initial_state': True})

evidence_names = ['fighter4-attacks-validation.json', 'fighter4-motion-validation.json', 'fighter4-motion-duo-validation.json',
                  'fighter4-composition-validation.json', 'fighter4-duo-composition-validation.json', 'fighter4-projectile-validation.json',
                  'fighter4-storage-test-first.json', 'fighter4-movement-test-first.json', 'fighter4-world-test-first.json',
                  'fighter4-rules-test-first.json', 'fighter4-dragon-world-test-first.json', 'fighter4-special-world-initial-binding.json',
                  'fighter4-session-save-test-first.json', 'fighter4-duo-world-boundary.json']
evidence = [ROOT/'games/yuyuhakusho/engine/testdata'/name for name in evidence_names]
evidence += [OUT/name for name in ('hiei-test-first.json', 'hiei-native-shards.json', 'hiei-source0-version-recheck.json', 'fighter4-dragon-assets.json')]
evidence += [ROOT/'docs/research/md-yuyuhakusho'/name for name in ('source4-special-discovery-validation.json', 'source4-special-discovery-duo-lifecycle.json', 'source4-special-discovery.md')]
evidence += [ROOT/'games/yuyuhakusho/data/chu-world-rules-manifest.json', ROOT/'games/yuyuhakusho/engine/FIGHTER4_WORLD_VERIFICATION.md']
evidence.append(ROOT/'games/yuyuhakusho/engine/testdata/fighter4-world-reference-frames.json.gz')
for name in ('fighter4-attacks-validation.json', 'fighter4-motion-validation.json', 'fighter4-motion-duo-validation.json', 'fighter4-composition-validation.json', 'fighter4-duo-composition-validation.json', 'fighter4-projectile-validation.json'):
    assert read(ROOT/'games/yuyuhakusho/engine/testdata'/name)['pass'], name
assert read(OUT/'fighter4-dragon-assets.json')['pass']
for path, digest in read(OUT/'hiei-test-first.json')['files'].items(): assert sha(ROOT/path) == digest, path
if (OUT/'hiei-cleanup.json').exists(): evidence.append(OUT/'hiei-cleanup.json')
sources = {str(p.relative_to(ROOT)): sha(p) for folder in ('engine', 'runtimebridge', 'cmd') for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)): sha(p) for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir()) if p.is_file() and (p.suffix in ('.py', '.sh', '.tsx', '.css', '.mjs') or p.name == 'tsconfig.json')})
report = {'verified_at_utc': datetime.now(timezone.utc).isoformat(), 'baseline_commit': '605efb55',
          'stage': 'Hiei complete shared rules and nine-character independent matches', 'pass': True,
          'goal_complete': False, 'release_ready': False, 'overall_progress_estimate_percent': 71,
          'integration_checks': integration, 'asset_checks': assets, 'fresh_matches': matches,
          'legacy_regression': bound(legacy_path), 'browser_e2e': bound(browser_path), 'targeted_checks': checks,
          'original_world': worlds, 'supporting_evidence': [bound(p) for p in evidence], 'current_source_sha256': sources,
          'limits': ['Only source IDs0/1/2/3/4/6/8/9/10 and the default arena are playable; source5/source7 and seven arenas remain.',
                     'Two special worlds remain active at their retained boundary; only the ordinary source4 world claims complete clear.',
                     'Duo original startup is excluded; retained practice ready-match boundary is2500..5500 under actuala01e/stage32.',
                     'Original AI4 natural full-world evidence is not claimed; source4 full AI uses existing isolated/composition evidence plus independent complete matches.',
                     'Sprite hardware cache/VDP consumers remain a separate presentation boundary; callback-time ordered Draw fields are compared explicitly.',
                     'Schema3 adds9 typed AN records,1344 signed words and one alternate movement; schema1/2 remain readable. Legacy world/event/lifecycle traces remain exact.',
                     'One historical schema2 metadata guard failed initially; only that guard changed, and all its checks passed in targeted native/Wasm rechecks. Original full logs are retained.',
                     'No platform room/network/prediction/takeover/audio-playback acceptance, CDN upload, release or deployment.']}
(OUT/'hiei-session.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'pass': True, 'rule_tests_per_environment': 327, 'asset_tests_per_environment': 27, 'full_native_wasm_scenarios': len(matches), 'legacy_reports': 192, 'progress_estimate_percent': 71}))
