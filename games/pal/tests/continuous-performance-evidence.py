import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
VERIFY = LOCAL / 'verification'
SOURCE = VERIFY / 'continuous-performance-source'


def read(path):
    return json.loads(path.read_bytes())


def info(path):
    with path.open('rb') as stream:
        sha = hashlib.file_digest(stream, 'sha256').hexdigest()
    return {'path': str(path.relative_to(ROOT)), 'bytes': path.stat().st_size, 'sha256': sha}


def p95(values):
    values = sorted(values)
    return values[int((len(values)-1)*.95+.5)]


old = read(SOURCE / 'previous-artifacts.json')
assert old['count'] == len(old['artifacts']) == 7902
for row in old['artifacts']:
    assert info(ROOT/row['path'])['sha256'] == row['sha256'], row['path']
progress = VERIFY / 'hidden-progress.json'
assert info(progress)['sha256'] == old['progressSHA256']
assert read(progress)['processed'] == 722 and read(progress)['total'] == 735

artifacts = [progress]
raw = read(VERIFY / 'continuous-battle-rule.json')
assert raw['passed'] and raw['nativeElapsedMS'] == 1000 and raw['offlineHours'] == 24 and raw['resumedRemainingMS'] == 17
assert raw['newEpoch'] > raw['oldEpoch']
frames = raw['originalFrames']
assert len(frames) == 25 and {row['arrow'] for row in frames} == {68,69}
assert all(row['nextMS']-row['dueMS'] == 40 and 0 <= row['atMS']-row['dueMS'] <= 6 for row in frames)
assert all(a['nextMS'] == b['dueMS'] and a['arrow'] != b['arrow'] for a,b in zip(frames,frames[1:]))
assert 'original arrow cadence changed' in (VERIFY / 'continuous-battle-rule-red.txt').read_text()
assert '--- PASS: TestPALBattleCadenceRetainsOriginalFramesAndSavedDelay' in (VERIFY / 'continuous-battle-rule-green.txt').read_text()

scopes = ['field-baseline','field-optimized','battle-baseline','battle-optimized','battle-optimized-phone-rtt','battle-probe-throttled','battle-probe-foreground-gaps','battle-probe-two-battle-rooms']
expect_pass = {'field-baseline','field-optimized','battle-optimized','battle-optimized-phone-rtt'}
summary = []
bundles = {stage: info(SOURCE / (stage+'-pal.js')) for stage in ['baseline','tested']}
normalize = lambda data: re.sub(rb'-[A-Za-z0-9_\-]{8}(?=\.js[\x22\x27\x60])',b'-bundleid',data)
assert normalize((SOURCE/'baseline-pal.js').read_bytes()) == normalize((SOURCE/'tested-pal.js').read_bytes())
for scope in scopes:
    folder = LOCAL / 'performance' / ('continuous-'+scope)
    result = read(folder / 'result.json')
    bundle = bundles['baseline' if 'baseline' in scope else 'tested']
    assert result['clientBundle']['sha256'] == bundle['sha256'] and result['clientBundle']['bytes'] == bundle['bytes']
    assert result['measurementComplete'] and result['passed'] == (scope in expect_pass), scope
    start, end = result['window']
    all_raf = [row for row in result['capture']['samples'] if start <= row['at'] <= end]
    visible = [row['at'] for row in all_raf if row['changed']]
    game = [row for row in result['network'] if row['type'] == 'game']
    values = result['values']
    assert values['visibleRate'] == len(visible)*1000/(end-start)
    assert values['visibleGapP95'] == p95([b-a for a,b in zip(visible,visible[1:])])
    assert values['rafP95'] == p95([b['at']-a['at'] for a,b in zip(all_raf,all_raf[1:])])
    assert values['gameP95Bytes'] == p95([row['bytes'] for row in game])
    if result['mode'] == 'battle':
        assert end-start == 5500 and result['originalArrowMaskPixels'] >= 8
        assert {frame for row in game for frame in row['arrowFrames']} == {68,69}
        assert len({row['hash'] for row in all_raf}) == 2
        assert result['budgets'] == {'visibleRate':24,'visibleGapP95':80,'rafP95':35,'gameP95Bytes':16384}
    else:
        assert result['budgets'] == {'visibleRate':9,'visibleGapP95':150,'rafP95':35,'gameP95Bytes':4096}
        assert result['route']['authoritySteps'] == 7 and result['route']['releasedAndStopped']
        assert all(row['stance'] >= 128 and row['match'] >= .85 for row in all_raf if row['changed'])
    samples = result.get('timerSamples', [])
    metrics = {'scope':scope,'passed':result['passed'],'wireSamples':len(game),'visibleSamples':len(visible),'rafSamples':len(all_raf),'ruleSamples':len(samples),**values}
    if samples:
        assert all(row['players'] == 1 and not row['failed'] and row['afterDueMS'] > row['nowMS'] for row in samples)
        metrics['ruleP95MS'] = p95([row['elapsedMS'] for row in samples])
    if result['passed']:
        assert all(row['focused'] for row in all_raf)
        assert values['visibleRate'] >= result['budgets']['visibleRate']
        assert all(values[key] <= result['budgets'][key] for key in ['visibleGapP95','rafP95','gameP95Bytes'])
        if samples:
            assert metrics['ruleP95MS'] <= 20
    if scope=='battle-optimized-phone-rtt':
        assert result['phone'] and all(150 <= row['connectionRTTMS'] <= 300 for row in result['rtt']) and 130 <= len(samples) <= 150
    if scope=='battle-probe-two-battle-rooms':
        assert len(samples)>250 and max(row['at']-row['nowMS'] for row in samples)-min(row['at']-row['nowMS'] for row in samples)>100
    artifacts.extend(path for path in sorted(folder.iterdir()) if path.is_file())
    summary.append(metrics)

save = LOCAL / 'performance/continuous-battle-save'
report = read(save / 'surface-result.json')
assert report['passed'] and len(report['cases']) == 4 and all(case['passed'] for case in report['cases'])
assert report['cases'][-1]['cash']==3
artifacts.extend(path for path in sorted(save.iterdir()) if path.is_file())
race = (VERIFY / 'continuous-platform-race.txt').read_text()
for name in ['RealtimeSlowDeadlineWebSocketDoesNotBlockActions','RealtimeCursorStreamIsolatesSlowConnection','ClientLoadingAllHumansBeforeRealtimeClock','ModuleTimerClockAfterHostLockWaitHTTP','RealtimeLivePeriodicCheckpoint']:
    assert '--- PASS: Test'+name in race
assert '\nFAIL' not in race
assert read(SOURCE/'deadline-slow-reader.json')['acceptedActions']==12
assert info(LOCAL / 'screen-input/battle-current/rules.wasm')['sha256'] == 'd22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438'
assert read(SOURCE/'field-identity.json')['buildID'] == '4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3'
for name in ['continuous-lint.txt','continuous-lint-negative.txt']:
    assert json.loads((VERIFY / name).read_text().splitlines()[-1])['passed']
artifacts.extend(path for path in sorted(SOURCE.iterdir()) if path.is_file())
artifacts.extend(path for path in sorted(VERIFY.glob('continuous-*')) if path.is_file() and 'stage' not in path.name and 'evidence-run' not in path.name)
result = {'passed':True,'fullPerformanceAcceptance':False,'singleRoomBattleCadencePassed':True,'mixedBattleRoomCadencePassed':False,'scope':'continuous original active-role arrow and one noncollision original inn walking segment; mixed battle load and browser outliers retained, no full combat or release claim','priorArtifactChecks':old['count'],'progress':'722/735','resourceUploads':0,'summary':summary,'artifacts':[info(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'continuous-performance-stage.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({key:result[key] for key in ['passed','fullPerformanceAcceptance','singleRoomBattleCadencePassed','mixedBattleRoomCadencePassed','priorArtifactChecks']},ensure_ascii=False))
