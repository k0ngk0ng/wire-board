import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
VERIFY = LOCAL / 'verification'
SOURCE = VERIFY / 'mixed-room-source'


def read(path):
    return json.loads(path.read_bytes())


def info(path):
    with path.open('rb') as stream:
        sha = hashlib.file_digest(stream, 'sha256').hexdigest()
    return {'path': str(path.relative_to(ROOT)), 'bytes': path.stat().st_size, 'sha256': sha}


def quantile(values, p):
    values = sorted(values)
    return values[int((len(values)-1)*p+.5)]


prior = read(SOURCE/'previous-artifacts.json')
assert len(prior['artifacts']) == prior['count'] == 7986
for row in prior['artifacts']:
    assert info(ROOT/row['path'])['sha256'] == row['sha256'], row['path']
progress = VERIFY/'hidden-progress.json'
assert info(progress)['sha256'] == prior['progressSHA256']
assert read(progress)['processed'] == 722 and read(progress)['total'] == 735

red = (VERIFY/'mixed-room-isolation-red.txt').read_text()
assert 'blocked room stalled the fast socket' in red and 'context deadline exceeded' in red
result = read(VERIFY/'mixed-room-isolation.json')
assert result['passed'] and result['fastPacketsWhileOtherRuleBlocked'] == 5 and result['parallelSlowCalls'] == 1
assert result['staleCommitRejected'] and result['fastLastFrame']-result['fastFirstFrame'] >= 4
race = (VERIFY/'mixed-room-platform-race.txt').read_text()
assert '\nFAIL' not in race
for name in ['DeadlineRoomBlockedRulesDoNotStopOtherRoomOrOverwriteClose','RealtimeSlowDeadlineWebSocketDoesNotBlockActions','RealtimeCursorStreamIsolatesSlowConnection','RealtimeLivePeriodicCheckpoint','ModuleRealtimeAtomicFrameHTTP']:
    assert '--- PASS: Test'+name in race

artifacts = [progress]
bundle = info(SOURCE/'tested-pal.js')
before = read(LOCAL/'performance/continuous-battle-probe-two-battle-rooms/result.json')
assert before['measurementComplete'] and not before['passed']
reports = {}
for name in ['mixed-seed','mixed-phone-rtt','mixed-live-peer','mixed-live-phone']:
    directory = LOCAL/'performance'/('continuous-battle-optimized-'+name)
    data = read(directory/'result.json')
    reports[name] = data
    assert data['passed'] and data['measurementComplete'] and data['mode'] == 'battle'
    assert data['clientBundle']['sha256'] == bundle['sha256'] and data['clientBundle']['bytes'] == bundle['bytes']
    assert data['budgets'] == {'visibleRate':24,'visibleGapP95':80,'rafP95':35,'gameP95Bytes':16384}
    start,end = data['window']
    assert end-start == 5500
    all_raf = [row for row in data['capture']['samples'] if start <= row['at'] <= end]
    assert all(row['phase']=='battle' and row['focused'] for row in all_raf)
    assert len({row['hash'] for row in all_raf}) == 2 and data['originalArrowMaskPixels'] >= 8
    visible = [row['at'] for row in all_raf if row['changed']]
    assert len(visible) >= 132 and data['values']['visibleRate'] == len(visible)/5.5
    game = [row for row in data['network'] if row['type']=='game']
    assert len(game) >= 132 and {frame for row in game for frame in row['arrowFrames']} == {68,69}
    assert quantile([row['bytes'] for row in game],.95) == data['values']['gameP95Bytes'] <= 16384
    assert quantile([b['at']-a['at'] for a,b in zip(all_raf,all_raf[1:])],.95) == data['values']['rafP95'] <= 35
    assert quantile([b-a for a,b in zip(visible,visible[1:])],.95) == data['values']['visibleGapP95'] <= 80
    times = data['timerSamples']
    assert all(row['players']==1 and not row['failed'] and row['afterDueMS'] > row['nowMS'] for row in times)
    if 'phone' in name:
        assert data['phone'] and all(150 <= row['connectionRTTMS'] <= 300 for row in data['rtt'])
        assert 265<=len(times)<=285, (name,len(times))
        offsets=sorted(row['at']-row['nowMS'] for row in times)
        gap, index=max((later-earlier,i) for i,(earlier,later) in enumerate(zip(offsets,offsets[1:])))
        assert gap >= 60 and 130 <= index+1 <= 145 and 130 <= len(times)-index-1 <= 145
    else:
        assert 130<=len(times)<=145
    artifacts.extend(path for path in sorted(directory.iterdir()) if path.is_file())

first, delayed = reports['mixed-seed'], reports['mixed-phone-rtt']
assert first['roomID'] != delayed['roomID']
peer, phone = reports['mixed-live-peer'], reports['mixed-live-phone']
assert peer['roomID'] != phone['roomID']
peer_connection = peer['peerConnection']
start_wall,end_wall = [value+phone['capture']['wall'] for value in phone['window']]
assert peer_connection['readyAt'] <= start_wall and peer_connection['endedAt'] >= end_wall and peer_connection['openConnections'] == 1
peer_packets = [row for row in peer_connection['events'] if row['type']=='game' and start_wall <= row['at'] <= end_wall]
assert len(peer_packets) >= 132 and {frame for row in peer_packets for frame in row['arrowFrames']} == {68,69}
peer_flips = sum(a['arrowFrames']!=b['arrowFrames'] for a,b in zip(peer_packets,peer_packets[1:]))
assert peer_flips >= 131

cadence = read(VERIFY/'mixed-room-cadence.json')
assert cadence['passed'] and cadence['nativeElapsedMS']==1000 and cadence['resumedRemainingMS']==17 and cadence['offlineHours']==24
save = LOCAL/'performance/mixed-battle-save'
saves = read(save/'surface-result.json')
assert saves['passed'] and all(row['passed'] for row in saves['cases']) and len(saves['cases'])==4
assert saves['cases'][-1]['cash']==3
assert 'private save/continue' in saves['cases'][0]['check'] and 'private save/continue' in saves['cases'][1]['check']
assert info(LOCAL/'screen-input/battle-current/rules.wasm')['sha256']=='d22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438'
for name in ['mixed-room-lint.txt','mixed-room-lint-negative.txt']:
    assert json.loads((VERIFY/name).read_text().splitlines()[-1])['passed']
artifacts.extend(path for path in sorted(save.iterdir()) if path.is_file())
artifacts.extend(path for path in sorted(SOURCE.iterdir()) if path.is_file())
artifacts.extend(path for path in sorted(VERIFY.glob('mixed-room-*')) if path.is_file() and 'stage' not in path.name and 'evidence-run' not in path.name)
output={'passed':True,'fullPerformanceAcceptance':False,'scope':'two original idle command-arrow battles on one service, measured phone canvas and simultaneous peer stream; no dense spell effects, arbitrary room counts or release claim','syntheticIsolation':result,'previousTwoRoomsVisibleRate':before['values']['visibleRate'],'twoRoomsVisibleRate':delayed['values']['visibleRate'],'bothConnectedPhoneVisibleRate':phone['values']['visibleRate'],'bothConnectedPhoneP95MS':phone['values']['visibleGapP95'],'bothConnectedPeerFrames':len(peer_packets),'bothConnectedPeerArrowChanges':peer_flips,'priorArtifactChecks':prior['count'],'progress':'722/735','uploads':0,'artifacts':[info(path) for path in dict.fromkeys(artifacts)]}
(VERIFY/'mixed-room-performance-stage.json').write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({key:output[key] for key in ['passed','fullPerformanceAcceptance','bothConnectedPhoneVisibleRate','bothConnectedPeerFrames','priorArtifactChecks']},ensure_ascii=False))
