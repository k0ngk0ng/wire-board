import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
VERIFY = LOCAL / 'verification'
SOURCE = VERIFY / 'menu-input-arrival-source'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    with path.open('rb') as stream:
        sha = hashlib.file_digest(stream, 'sha256').hexdigest()
    return {'path': str(path.relative_to(ROOT)), 'bytes': path.stat().st_size, 'sha256': sha}


result = load(VERIFY / 'menu-input-arrival-http.json')
assert result['passed'] and result['buildID'] == '4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3'
cases = result['cases']
assert len(cases) == 9
by_name = {row['case']: row for row in cases}
for name in ['ws-live-before-host-wait', 'http-live-before-host-wait', 'ws-arrival-before-host-lock']:
    row = by_name[name]
    assert 0 <= row['receivedMS'] - row['sourceMS'] <= 500 < row['ruleTimeMS'] - row['sourceMS']
    assert 0 <= row['ruleTimeMS'] - row['receivedMS'] <= 500
    assert row['selected'] != row['selectionBefore'] and row['seq'] == row['appliedSeq'] and row['seq'] != row['discardedSeq']
for name, check in [
    ('expired-before-arrival', lambda row: row['receivedMS'] - row['sourceMS'] > 500),
    ('expired-in-host-queue', lambda row: row['ruleTimeMS'] - row['receivedMS'] > 500),
    ('future-input', lambda row: row['sourceMS'] > row['receivedMS']),
]:
    row = by_name[name]
    assert check(row) and row['discardedSeq'] == row['seq'] and row['selected'] == row['selectionBefore']
assert by_name['old-neutral-release']['seq'] == by_name['old-neutral-release']['appliedSeq'] != by_name['old-neutral-release']['discardedSeq']
assert by_name['fresh-after-discard']['selected'] != by_name['fresh-after-discard']['selectionBefore']
boundary = by_name['waiting-input-cannot-enter-resumed-menu']
assert boundary['ackStatus'] in [400, 409] and boundary['resumedEpoch'] != boundary['oldEpoch'] and boundary['selected'] == boundary['selectionBefore']

folder = LOCAL / 'performance/selection-menu-optimized-input-arrival'
browser = load(folder / 'result.json')
assert browser['passed'] and browser['measurementComplete'] and browser['confirmation']['passed']
assert browser['projection']['menu']['kind'] == 1
assert browser['budgets'] == {'p50': 33, 'p95': 50, 'max': 100}
asset = browser['clientBundle']
assert asset['prediction']
assert load(VERIFY / 'menu-input-arrival-web.json')['assets'][asset['name'].lstrip('/')] == {'sha256': asset['sha256'], 'bytes': asset['bytes']}
assert digest(SOURCE / 'tested-pal.js')['sha256'] == asset['sha256']
client_builds = load(SOURCE / 'client-builds.json')
assert client_builds['passed'] and [row['stage'] for row in client_builds['builds']] == ['tested', 'rebuilt']
for row in client_builds['builds']:
    stage = row['stage']
    assert load(SOURCE / (stage+'-manifest.json'))['games']['pal'][0] == {'url': row['asset'], 'bytes': row['size']}
    assert digest(SOURCE / (stage+'-pal.js'))['sha256'] == row['sha256']
assert (VERIFY / 'menu-input-arrival-typecheck.txt').read_bytes() == b''
assert 'built in' in (VERIFY / 'menu-input-arrival-build.txt').read_text()
assert len(browser['cases']) == 4
measurements = []
for case in browser['cases']:
    assert case['passed'] and len(case['samples']) == 12
    for sample in case['samples']:
        network = sample['network']
        assert sample['trusted'] and sample['beforeAuthority'] and sample['responseMS'] <= 100
        assert sample['pixelEpochAt'] < network['authority']['at']
        if 'rtt' in case['name']:
            assert sample['pixelEpochAt'] < network['ack']['at']
        assert network['ack']['status'] == 200 and network['authority']['menuSelected'] == sample['target']
        assert network['authority']['appliedSeq'] >= network['sent']['seq'] > network['authority'].get('discardedSeq', 0)
    for key, value in browser['budgets'].items():
        assert case['responseMS'][key] <= value
    measurements.append({'case': case['name'], 'responseMS': case['responseMS'], 'samples': 12, 'rttMS': [row['connectionRTTMS'] for row in case['rtt']]})

previous = load(SOURCE / 'previous-artifacts.json')
assert previous['count'] == len(previous['artifacts']) == 7846
for row in previous['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
progress = VERIFY / 'hidden-progress.json'
assert digest(progress)['sha256'] == previous['progressSHA256']
assert load(progress)['processed'] == 722 and load(progress)['total'] == 735
for name in ['menu-input-arrival-lint.txt', 'menu-input-arrival-lint-negative.txt']:
    assert json.loads((VERIFY / name).read_text().splitlines()[-1])['passed']
assert 'ws-live-before-host-wait: native selection=1 discarded=1' in (VERIFY / 'menu-input-arrival-red.txt').read_text()
assert '--- PASS: TestPALOriginalMenuInputTimeThroughHTTPAndWebSocket' in (VERIFY / 'menu-input-arrival-final.txt').read_text()
platform = (VERIFY / 'menu-input-arrival-platform-race.txt').read_text()
for name in ['RealtimeBatchAtomicDurableInputs', 'RealtimeBatchMissingSnapshotRejectsAcknowledgement', 'RealtimeStreamInputAcknowledgedExactlyOnce', 'RealtimeSlowWebSocketDoesNotBlockActions', 'ModuleTimerClockAfterHostLockWaitHTTP']:
    assert '--- PASS: Test'+name in platform
assert '\nFAIL' not in platform
artifacts = [progress]
artifacts.extend(path for path in sorted(folder.iterdir()) if path.is_file())
artifacts.extend(path for path in sorted(SOURCE.iterdir()) if path.is_file())
artifacts.extend(path for path in sorted(VERIFY.glob('menu-input-arrival-*')) if path.is_file() and 'stage' not in path.name and 'evidence-run' not in path.name)
report = {'passed': True, 'fullPerformanceAcceptance': False, 'menuSelectionPassed': True, 'scope': 'actual main menu selection, bounded receipt and execution time, real WebSocket/HTTP transport and save generation protection; no full campaign, network lifecycle or release claim', 'realRoomInputChecks': len(cases), 'originalCanvasSamples': 48, 'measurements': measurements, 'priorArtifactChecks': previous['count'], 'buildID': result['buildID'], 'testedClient': asset, 'resourceUploads': 0, 'progress': '722/735', 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'menu-input-arrival-stage.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({key: report[key] for key in ['passed', 'originalCanvasSamples', 'realRoomInputChecks', 'priorArtifactChecks']}, ensure_ascii=False))
