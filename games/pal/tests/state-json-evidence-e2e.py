import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'native-json-source'
PERFORMANCE = ROOT / '.local/pal/performance'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


def quantile(values, fraction):
    values = sorted(values)
    return values[min(len(values)-1, int((len(values)-1)*fraction+.5))]


before = load(PERFORMANCE / 'native-json-before/result.json')
after = load(PERFORMANCE / 'native-json-after/result.json')
assert before['measurementComplete'] and after['measurementComplete'] and after['passed']
assert not before['fullPerformanceAcceptance'] and not after['fullPerformanceAcceptance']
assert before['budgets'] == after['budgets'] == {'responseP50MS': 33, 'responseP95MS': 50, 'responseMaxMS': 100, 'rafP95MS': 35, 'fieldGameP95Bytes': 4096, 'ruleP95MS': 20}
assert before['fixture'] == after['fixture'] and after['fixture']['buildID'] == '4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3'
assert after['fixture']['players'] == 1 and after['fixture']['profile'] == 'original-98' and after['fixture']['ruleOnlyBundle']
expected = {'normal', 'rtt-200', 'fast-independent-room-while-a-delayed', 'phone-320'}
assert {row['name'] for row in before['cases']} == {row['name'] for row in after['cases']} == expected
comparison = []
for value in [before, after]:
    assert len(value['roomIDs']) == len(set(value['roomIDs'])) == 2
    assert abs(value['phoneCanvas']['width']-min(320, value['phoneCanvas']['viewportHeight']*1.6)) < 1
    for case in value['cases']:
        assert len(case['samples']) == 12 and all(row['trusted'] and row['informativePixels'] >= 128 for row in case['samples'])
        assert quantile([row['responseMS'] for row in case['samples']], .95) == case['responseMS']['p95']
        assert all(row['players'] == 1 and not row['failed'] for row in case['ruleSamples'])
        assert quantile([row['elapsedMS'] for row in case['ruleSamples']], .95) == case['ruleMS']['p95']
        if value is after:
            assert case['passed'] and case['ruleMS']['p95'] <= 20
            assert case['responseMS']['p50'] <= 33 and case['responseMS']['p95'] <= 50 and case['responseMS']['max'] <= 100
            assert case['rafMS']['p95'] <= 35 and case['gameBytes']['p95'] <= 4096
        if case['name'] == 'rtt-200':
            assert 150 <= case['connectionRTTMS']['p95'] <= 300
for case in after['cases']:
    old = next(row for row in before['cases'] if row['name'] == case['name'])
    assert case['ruleMS']['p95'] < old['ruleMS']['p95']
    comparison.append({'case': case['name'], 'beforeRuleP95MS': old['ruleMS']['p95'], 'afterRuleP95MS': case['ruleMS']['p95'], 'responseP95MS': case['responseMS']['p95'], 'rafP95MS': case['rafMS']['p95'], 'gameP95Bytes': case['gameBytes']['p95'], 'measuredRTTP95MS': case['connectionRTTMS']['p95']})
progress = VERIFY / 'hidden-progress.json'
assert progress.read_bytes() == (SOURCE / 'prior-hidden-progress.json').read_bytes()
baseline_path = SOURCE / 'prior-artifact-baseline.json'
baseline = load(baseline_path)
assert baseline['passed'] and len(baseline['artifacts']) == 7718
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
artifacts = [progress, baseline_path, SOURCE / 'prior-hidden-progress.json']
for name in ['native-json-solo-save-http.json', 'native-json-version-http.json', 'native-json-checkpoint-http.json']:
    path = VERIFY / name
    assert load(path)['passed']
    artifacts.append(path)
for name, token in [('native-json-parser-red-run.txt', 'undefined: decodeStateJSON'), ('native-json-duplicate-limit-red-run.txt', 'state JSON parser differs from strict decoder'), ('native-json-parser-fuzz-run.txt', 'Failing input written'), ('native-json-parser-verified-race-run.txt', 'ok'), ('native-json-parser-final-fuzz-run.txt', 'PASS'), ('native-json-http-run.txt', '--- PASS: TestPALOriginalSoloPrivateSaveRestartHTTP')]:
    path = VERIFY / name
    assert token in path.read_text()
    if 'final-fuzz' in name or 'verified-race' in name or name == 'native-json-http-run.txt':
        assert '\nFAIL' not in path.read_text()
    artifacts.append(path)
assert '--- PASS: TestPALRetainedNativeVersionsRestartHTTP' in (VERIFY / 'native-json-http-run.txt').read_text()
assert '--- PASS: TestPALOriginalManualCheckpointReloadAndOldPageHTTP' in (VERIFY / 'native-json-http-run.txt').read_text()
for name in ['native-json-lint-run.txt', 'native-json-lint-negative-run.txt']:
    path = VERIFY / name
    assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
assert (VERIFY / 'native-json-vet-run.txt').read_bytes() == b''
artifacts.extend(VERIFY / name for name in ['native-json-vet-run.txt', 'native-json-before-browser-run.txt', 'native-json-after-browser-run.txt', 'native-json-before-fixture-run.txt', 'native-json-after-fixture-run.txt', 'native-json-evidence-red-run.txt', 'native-json-cleanup.json'])
for tag in ['native-json-before', 'native-json-after']:
    artifacts.extend(path for path in sorted((PERFORMANCE / tag).rglob('*')) if path.is_file())
for name in ['games/pal/server/module.go', 'games/pal/server/state_decode.go', 'games/pal/server/state_decode_test.go', 'games/pal/server/testdata/fuzz/FuzzStateJSONMatchesStrictDecoder/bce258409d37498c', 'games/pal/tests/state-json-evidence-e2e.py', 'internal/server/pal_solo_save_e2e_test.go', 'internal/server/pal_native_versions_e2e_test.go', 'internal/server/pal_checkpoint_e2e_test.go', 'games/pal/build/lint-rules.json', 'games/pal/LINT.md', 'games/pal/PERFORMANCE.md', 'games/pal/STATE_JSON_TESTING.md', 'games/pal/ACCEPTANCE.md']:
    path = ROOT / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/original/rules.wasm')['sha256'] == after['fixture']['buildID']
assert digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256'] == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': 'compatible outer JSON native-first parsing, strict legacy fallback, bounded isolated parser race/fuzz and actual authenticated solo save/restart/retained-core/manual-rollback regressions; four original inn keyboard/touch performance cases only, not full performance or release', 'fullPerformanceAcceptance': False, 'beforeCasesPassed': before['passed'], 'afterCasesPassed': after['passed'], 'comparison': comparison, 'nativeBuildID': after['fixture']['buildID'], 'nativeEdits': False, 'resourceUploads': 0, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'hiddenProgressUnchanged': True, 'hiddenProgress': {key: load(progress)[key] for key in ['display', 'processed', 'total', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'native-json-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'fullPerformanceAcceptance': False, 'comparison': comparison, 'priorArtifactChecks': len(baseline['artifacts']), 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
