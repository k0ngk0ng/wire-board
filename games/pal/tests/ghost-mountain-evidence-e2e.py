import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SOURCE = VERIFY / 'ghost-mountain-hidden-source'
EXPECTED = {70: [1376, 1377, 1378, 1379], 71: [1387, 1388], 72: [1396, 1397], 73: [1405, 1406], 74: [1413], 75: [1416], 76: [1421, 1422, 1423, 1424, 1425], 77: [1479, 1485, 1480, 1481, 1478, 1482, 1483, 1484, 1486]}
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'

def load(path):
    return json.loads(path.read_text())

def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}

prior, progress = load(SOURCE / 'prior-hidden-progress.json'), load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 256
assert progress['passed'] and progress['processed'] == 282 and progress['total'] == 732
assert progress['actuallyClaimed'] == 264 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 450
verified = {row['event'] for row in progress['verifiedPoints']}
preserved = {row['event'] for row in progress['preservedPoints']}
assert verified.isdisjoint(preserved)
assert verified-{row['event'] for row in prior['verifiedPoints']} == set(sum(EXPECTED.values(), []))
assert {row['event'] for row in prior['verifiedPoints']} <= verified
assert preserved == {row['event'] for row in prior['preservedPoints']}
current_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
for row in prior['verifiedPoints']+prior['preservedPoints']:
    assert current_points[row['event']] == row
current_evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(current_evidence[row['path']] == row['sha256'] for row in prior['evidence'])
assert {798, 846, 847, 1172, 1196} <= set(progress['pendingEventIDs'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
selector_path = VERIFY / 'regional-ghost-mountain-selector-equivalence.json'
selector = load(selector_path)
assert selector['passed'] and selector['sha256'] == digest(SOURCE / 'prior-hidden-progress.json')['sha256']
cases, artifacts = [], [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json', selector_path]
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v8-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 8 and value['scene'] == scene
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture']
    assert not value['auditOnly'] and value['auditedEvents'] == []
    assert [row['event'] for row in value['claimed']] == events
    assert value['proof']['Saves'] >= len(events)*4 and value['proof']['Battles'] == value['proof']['Wins']
    assert value['proof']['ScriptedFinishes'] == 0 and value['combatAllowed']
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']]
    battle_cash = sum(row['rewardCash'] for row in value['battleCashChanges'])
    pickup_cash = sum(grant['Cash'] for reward in value['claimed'] for grant in reward['grants'])
    assert value['cashFinal']-value['cashInitial'] == battle_cash+pickup_cash
    assert len(value['battleCashChanges']) == value['proof']['Battles']-value['preCollectionBattles']
    root = roots['roots'][value['sourceCheckpoint']]
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == digest(source_path)['sha256'] == root['sha256']
    assert digest(parent_path)['sha256'] == root['reportSHA256']
    assert load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    artifacts.extend([source_path, parent_path])
    for reward in value['claimed']:
        assert reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose'] and len(reward['repeatPose']) == 3
        for stage in ['unpicked', 'notice', 'collected', 'repeated']:
            assert value['proof']['Saved'][f"reward-{reward['event']}-{stage}"]
    if scene == 77:
        weapon = next(row for row in value['claimed'] if row['event'] == 1486)
        assert weapon['originalGroundWeaponDisappearance'] and weapon['disappearsAfterPickup']
        assert weapon['openFrame'] == weapon['closedFrame'] == -1 and weapon['searchMode'] == 3
        assert [(g['Item'], g['Amount']) for g in weapon['grants']] == [(174, 1)]
    if scene == 70:
        assert len(value['claimed']) == 4 and all(row['originalHerbDisappearance'] and row['disappearsAfterPickup'] for row in value['claimed'])
    log = VERIFY / f'regional-ghost-mountain-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json'])
    for label, saved in value['proof']['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.append(log)
    cases.append({'scene': scene, 'events': events, 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'preCollectionBattles': value['preCollectionBattles'], 'pickupCash': pickup_cash, 'checkedCollectionBattleCash': battle_cash, 'partyRoles': [row['role'] for row in value['party']], 'report': digest(path)})
for filename in ['regional-ghost-mountain-lint-run.txt', 'regional-ghost-mountain-lint-negative-run.txt', 'regional-ghost-mountain-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
artifacts.append(ROOT / '.local/pal/hidden-items-research/white-river-text-guide.md')
for filename in ['games/pal/tests/regional_hidden_rewards_e2e_test.go', 'games/pal/tests/ghost-mountain-evidence-e2e.py', 'games/pal/tests/hidden-progress.py', 'games/pal/tests/campaign-roots.json', 'games/pal/tests/campaign_roots_test.go', 'games/pal/tests/campaign_roots_e2e_test.go', 'games/pal/build/lint-rules.json', 'games/pal/GHOST_MOUNTAIN_REWARDS_TESTING.md', 'games/pal/HIDDEN_ITEMS.md', 'games/pal/ACCEPTANCE.md', 'games/pal/LINT.md']:
    path = ROOT / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
authority_hash = hashlib.sha256((ROOT / '.local/pal/authority-prepared/authority.zip').read_bytes()).hexdigest()
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': '26 actual original pickups in Ghost mountain optional caves, herbs and temple; ordinary entrances, original disappearing ground weapon, original chest frames, fresh runtime saved fixed-pose repeat searches; no native edits or uploads, not performance or release acceptance', 'nativeBuildID': CORE, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 26, 'newActuallyClaimed': 26, 'newOriginalUnreachablePreserved': 0, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'cases': cases, 'priorImmutableArtifactsUnchanged': True, 'acceptedCampaignReportsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'ghost-mountain-hidden-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
