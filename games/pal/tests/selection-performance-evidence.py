import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
VERIFY = LOCAL / 'verification'
SOURCE = VERIFY / 'selection-source'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    with path.open('rb') as stream:
        sha = hashlib.file_digest(stream, 'sha256').hexdigest()
    return {'path': str(path.relative_to(ROOT)), 'bytes': path.stat().st_size, 'sha256': sha}


def quantile(values, fraction):
    values = sorted(values)
    return values[int((len(values)-1)*fraction+.5)]


comparison = {}
artifacts = []
bundles = {}
for mode in ['menu', 'battle']:
    pair = {}
    for run in ['baseline', 'optimized']:
        folder = LOCAL / 'performance' / f'selection-{mode}-{run}'
        path = folder / 'result.json'
        result = load(path)
        assert result['mode'] == mode and result['run'] == run
        partial = mode == 'menu' and run == 'optimized'
        assert result['measurementComplete'] == (not partial)
        assert result['passed'] == (mode == 'battle' and run == 'optimized')
        assert result['budgets'] == {'p50': 33, 'p95': 50, 'max': 100}
        asset = result['clientBundle']
        assert asset['prediction'] == (run == 'optimized')
        manifest = load(SOURCE / (run+'-game-load-manifest.json'))
        assert manifest['games']['pal'][0] == {'url': asset['name'], 'bytes': asset['bytes']}
        bundle = SOURCE / (run+'-pal.js')
        check = digest(bundle)
        assert check['sha256'] == asset['sha256'] and check['bytes'] == asset['bytes']
        assert (b':main-menu' in bundle.read_bytes()) == asset['prediction']
        if run in bundles:
            assert bundles[run] == asset
        bundles[run] = asset
        artifacts.extend([path, bundle])
        artifacts.extend(VERIFY / f'selection-{mode}-{run}{suffix}.txt' for suffix in ['', '-fixture'])
        if partial:
            failure = result['failedSelection']
            sample, trace = failure['sample'], failure['network']
            sent = next(row for row in trace if row['type'] == 'sent' and row['action']['keys'] == 4)
            ack = next(row for row in trace if row.get('nonce') == sent['nonce'] and row['type'] == 'ack')
            discard = next(row for row in trace if row.get('discardedSeq') == sent['seq'])
            assert sample['trusted'] and sample['responseMS'] < 50 and sample['target'] == 2 and failure['current'] == 1
            assert ack['status'] == 200 and sample['pixelEpochAt'] < discard['at']
            assert discard['menuSelected'] == failure['current'] and discard['seq'] == sent['seq']
            pair['discardedOptimizedSelection'] = {'sample': sample, 'sent': sent, 'ack': ack, 'discard': discard}
            continue
        assert len(result['cases']) == 4 and [case['name'] for case in result['cases']] == ['desktop', 'desktop-rtt', 'phone-rtt', 'phone']
        for case in result['cases']:
            samples = case['samples']
            label = case['name']
            assert len(samples) == 12 and all(row['trusted'] for row in samples)
            assert all(row['kind'] == ('touch' if 'phone' in label else 'key') for row in samples)
            assert all(row['network']['ack']['status'] == 200 for row in samples)
            assert all(row['frames'][-1]['restored'] >= 12 and row['frames'][-1]['targetPixels'] >= 12 for row in samples)
            assert all(row['beforeAuthority'] == (row['pixelEpochAt'] < row['network']['authority']['at']) for row in samples)
            values = [row['responseMS'] for row in samples]
            assert case['responseMS'] == {'p50': quantile(values, .50), 'p95': quantile(values, .95), 'max': max(values)}
            rtt = [row['connectionRTTMS'] for row in case['rtt']]
            assert rtt and all((150 <= value <= 300 if 'rtt' in label else value < 50) for value in rtt)
            if run == 'optimized':
                assert case['passed'] and all(case['responseMS'][key] <= limit for key, limit in result['budgets'].items())
                if 'rtt' in label:
                    assert all(row['beforeAuthority'] and row['pixelEpochAt'] < row['network']['ack']['at'] for row in samples)
            elif 'rtt' in label:
                assert not any(row['beforeAuthority'] for row in samples)
            pair.setdefault(label, {})[run] = {'responseMS': case['responseMS'], 'rttMS': rtt, 'beforeAuthority': sum(row['beforeAuthority'] for row in samples)}
            screenshot = folder / (label + '.png')
            assert screenshot.read_bytes().startswith(bytes.fromhex('89504e470d0a1a0a'))
            artifacts.append(screenshot)
        if run == 'optimized':
            assert result['confirmation'] == {'passed': True, 'onlyExplicitConfirmChangedBoundary': True, 'cashAndPartyUnchanged': True, 'mode': mode}
        assert result['projection']['raster'] == (mode == 'battle')
    comparison[mode] = pair

previous_path = SOURCE / 'previous-artifacts.json'
previous = load(previous_path)
assert previous['count'] == len(previous['artifacts']) == 7779
for row in previous['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
progress = VERIFY / 'hidden-progress.json'
assert digest(progress)['sha256'] == previous['progressSHA256']
assert load(progress)['processed'] == 722 and load(progress)['total'] == 735
menu_core = load(SOURCE / 'menu-core-identity.json')
assert menu_core['buildID'] == '4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3' and menu_core['ruleOnlyBundle']
battle_core = digest(LOCAL / 'screen-input/battle-current/rules.wasm')
assert battle_core['sha256'] == 'd22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438'
save_folder = LOCAL / 'performance/selection-battle-save'
save = load(save_folder / 'surface-result.json')
assert save['passed'] and len(save['cases']) == 4 and all(row['passed'] for row in save['cases'])
assert save['cases'][2]['afterHP'] > save['cases'][2]['beforeHP']
assert save['cases'][3]['cash'] == 3 and len(save['cases'][3]['party']) == 3
artifacts.extend(path for path in sorted(save_folder.iterdir()) if path.is_file())
artifacts.append(VERIFY / 'selection-battle-save.txt')
outlier_path = LOCAL / 'performance/selection-menu-probe-parallel-outlier/result.json'
outlier = load(outlier_path)
assert not outlier['passed'] and outlier['cases'][0]['responseMS']['max'] > 100
assert outlier['measurementComplete'] and outlier['confirmation']['passed']
assert len(outlier['cases']) == 4 and all(case['passed'] for case in outlier['cases'][1:])
assert outlier['clientBundle'] == bundles['optimized']
comparison['menu']['attemptWithDisplayStall'] = [{'name': case['name'], 'responseMS': case['responseMS'], 'passed': case['passed']} for case in outlier['cases']]
artifacts.extend([outlier_path, VERIFY / 'selection-menu-probe-parallel-outlier.txt'])
for name in ['selection-lint.txt', 'selection-lint-negative.txt']:
    path = VERIFY / name
    assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['selection-build.txt', 'selection-baseline-build.txt']:
    path = VERIFY / name
    assert 'built in' in path.read_text()
    artifacts.append(path)
artifacts.extend([progress, previous_path])
artifacts.extend(path for path in sorted(SOURCE.iterdir()) if path.is_file())
report = {'passed': False, 'evidenceVerified': True, 'battleRootSelectionPassed': True, 'mainMenuSelectionPassed': False, 'fullPerformanceAcceptance': False, 'scope': 'main field menu diagnostics and completed root battle icon navigation; native keyboard and 320px touch with bounded delay; menu authority discard recorded and not accepted', 'battleVerifiedInputSamples': 48, 'priorArtifactChecks': previous['count'], 'resourceUploads': 0, 'progress': '722/735', 'comparison': comparison, 'clientBundles': bundles, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'selection-performance-stage.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'evidenceVerified': True, 'battleRootSelectionPassed': True, 'mainMenuSelectionPassed': False, 'artifacts': len(report['artifacts'])}, ensure_ascii=False))
