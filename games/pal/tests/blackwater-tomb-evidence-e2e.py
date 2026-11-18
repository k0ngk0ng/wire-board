import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SOURCE = VERIFY / 'blackwater-tomb-hidden-source'
EXPECTED = {60: [1048, 1073, 1074, 1075, 1076, 1077, 1078, 1079], 61: [1168, 1170, 1169, 1171, 1173, 1174], 62: [1190], 63: [1197, 1198, 1195, 1199, 1200, 1201, 1194, 1202, 1203, 1204], 65: list(range(1229, 1238)), 66: [1333, 1334, 1335, 1336, 1323, 1324, 1325, 1326, 1327, 1329, 1330, 1331, 1332, 1328]}
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'

def load(path):
    return json.loads(path.read_text())

def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}

prior, progress = load(SOURCE / 'prior-hidden-progress.json'), load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 208
assert progress['passed'] and progress['processed'] == 256 and progress['total'] == 732
assert progress['actuallyClaimed'] == 238 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 476
verified = {row['event'] for row in progress['verifiedPoints']}
preserved = {row['event'] for row in progress['preservedPoints']}
assert verified.isdisjoint(preserved)
assert verified-{row['event'] for row in prior['verifiedPoints']} == set(sum(EXPECTED.values(), []))
assert {row['event'] for row in prior['verifiedPoints']} <= verified
assert preserved == {row['event'] for row in prior['preservedPoints']}
assert {798, 846, 847, 1172, 1196} <= set(progress['pendingEventIDs'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
cases, artifacts = [], [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', SOURCE / 'regional_hidden_rewards_before_left_plate.go', VERIFY / 'hidden-progress.json', VERIFY / 'tomb-optional-1328-plan.json', VERIFY / 'regional-blackwater-research-cleanup.json']
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v7-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 7 and value['scene'] == scene
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
    if scene == 60:
        body = next(row for row in value['claimed'] if row['event'] == 1048)
        assert body['persistentBodyAfterSavedSearches'] and not body['disappearsAfterPickup']
        assert [(g['Item'], g['Amount']) for g in body['grants']] == [(152, 2)]
        assert all(value['proof']['Saved'][f'blood-pool-optional-switch-{event}'] for event in [1042, 1047])
    if scene == 61:
        assert value['proof']['Saved']['tomb-optional-switch-1146']
    if scene == 66:
        assert value['proof']['Saved']['tomb-optional-switch-1147']
    log = VERIFY / f'regional-blackwater-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json'])
    for label, saved in value['proof']['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.append(log)
    source_path = SOURCE / 'regional_hidden_rewards_before_left_plate.go' if scene != 66 else ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go'
    source_snapshot = SOURCE / ('regional_hidden_rewards_before_left_plate.go' if scene != 66 else 'regional_hidden_rewards_e2e_test.go')
    cases.append({'scene': scene, 'testSource': {'path': str(source_snapshot.relative_to(ROOT)), 'sha256': digest(source_path)['sha256']}, 'events': events, 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'preCollectionBattles': value['preCollectionBattles'], 'pickupCash': pickup_cash, 'checkedCollectionBattleCash': battle_cash, 'partyRoles': [row['role'] for row in value['party']], 'report': digest(path)})
for filename in ['regional-blackwater-lint-run.txt', 'regional-blackwater-lint-negative-run.txt', 'regional-blackwater-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
artifacts.append(ROOT / '.local/pal/hidden-items-research/white-river-text-guide.md')
pending_path = VERIFY / 'blackwater-extra-records-pending.json'
pending = load(pending_path)
assert pending['passed'] and pending['diagnosticOnly'] and not pending['readyForProgress'] and not pending['nativeEdits']
assert {row['event'] for row in pending['records']} == {1172, 1196}
for row in pending['records']:
    assert not row['readyForProgress'] and not row['normalPickupVerified'] and row['originalRecordUnchanged']
    assert not row['sameSceneSearchStances']
    artifacts.append(ROOT / row['mapFile']['path'])
artifacts.append(pending_path)
for filename in ['games/pal/tests/regional_hidden_rewards_e2e_test.go', 'games/pal/tests/blackwater-tomb-evidence-e2e.py', 'games/pal/tests/blackwater-pending-review.py', 'games/pal/tests/hidden-progress.py', 'games/pal/tests/campaign-roots.json', 'games/pal/tests/campaign_roots_test.go', 'games/pal/tests/campaign_roots_e2e_test.go', 'games/pal/build/lint-rules.json', 'games/pal/BLACKWATER_REWARDS_TESTING.md', 'games/pal/HIDDEN_ITEMS.md', 'games/pal/ACCEPTANCE.md', 'games/pal/LINT.md']:
    path = ROOT / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
authority_hash = hashlib.sha256((ROOT / '.local/pal/authority-prepared/authority.zip').read_bytes()).hexdigest()
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': '48 actual original pickups in Blackwater, burial grounds, General tomb and blood pool; real doors, stairs and optional switches, original two-worm persistent body, fresh runtime saved fixed-pose repeat searches; two extra original candidates still pending, no native edits or uploads, not performance or release acceptance', 'nativeBuildID': CORE, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 48, 'newActuallyClaimed': 48, 'newOriginalUnreachablePreserved': 0, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'cases': cases, 'pendingRegionalEvents': [1172, 1196], 'priorImmutableArtifactsUnchanged': True, 'acceptedCampaignReportsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'blackwater-tomb-hidden-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
