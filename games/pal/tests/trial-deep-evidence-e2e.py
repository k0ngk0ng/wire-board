import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'trial-deep-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 567 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 646 and progress['total'] == 735 and progress['remaining'] == 89
assert progress['actuallyClaimed'] == 624 and progress['originalUnreachablePreserved'] == 22
plan_path = VERIFY / 'trial-deep-rewards-plan.json'
plan = load(plan_path)
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits'] and plan['separatelyReviewedOriginalEvents'] == [4000]
EXPECTED = {case['scene']: case['events'] for case in plan['cases']}
assert {key: len(value) for key, value in EXPECTED.items()} == {223: 26, 224: 31, 226: 8, 227: 13}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {4000}
assert all(new_points[event] == row for event, row in old_points.items()) and progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 4687 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'yuhang-trial-stage.tar.gz')['sha256'] == 'ae16aca74e193876c7f4ef449f2e17bc10b0fb142d7028b8c1327528ecf758ea'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json', 'regional_hidden_rewards_e2e_test.go', 'trial_rewards_route_e2e_test.go', 'trial-rewards-plan.py', 'regional_before_chest_blocks.go', 'route_before_chest_blocks.go', 'sixth-chest-blocked-run.txt', 'sixth-chest-blocked-state.json', 'sixth-chest-blocked-proof.json']]
artifacts.extend(SOURCE / name for name in ['regional_before_inventory_account.go', 'sixth-repeat-inventory-run.txt', 'sixth-repeat-inventory-state.json', 'sixth-repeat-inventory-proof.json'])
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'trial-deep-evidence-red-run.txt', 'trial-deep-evidence-pre-aggregate-red-run.txt', 'trial-deep-progress-run.txt', 'trial-deep-plan-run.txt', 'trial-deep-plan-including-hidden-worm-red-run.txt', 'trial-deep-priority-plan.json', 'trial-deep-priority-plan-run.txt', 'trial-deep-priority-result.json', 'trial-deep-priority-run.txt', 'yuhang-trial-stage-archive.json', 'yuhang-trial-archive-verification.json'])
artifacts.append(plan_path)
cases = []


def checked_references(value):
    if isinstance(value, dict):
        if 'path' in value and 'sha256' in value:
            path = ROOT / value['path']
            assert path.resolve().is_relative_to(ROOT) and path.is_file() and not path.is_symlink()
            assert digest(path)['sha256'] == value['sha256'], value['path']
            artifacts.append(path)
        for child in value.values():
            checked_references(child)
    elif isinstance(value, list):
        for child in value:
            checked_references(child)


review_path = VERIFY / 'trial-deep-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and review['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert len(review['records']) == 78 and {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
checked_references(review)
artifacts.extend([review_path, VERIFY / 'trial-deep-original-review-run.txt'])
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v18-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 18 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events and value['routePlanSHA256'] == digest(plan_path)['sha256']
    case = next(row for row in plan['cases'] if row['scene'] == scene)
    proof, drops = value['proof'], value['battleItemGrants']
    drop_battles = {row['battle'] for row in drops}
    assert proof['Saves'] == len(proof['Saved']) == len(events)*4+proof['Battles']+len(drop_battles)+case['portalSteps']+2*case['earthReturns'] and all(proof['Saved'].values())
    assert proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0 and value['combatAllowed']
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']] == [(0, 0), (4, 0)]
    assert value['sourceCheckpoint'] == plan['sourceCheckpoint'] and value['sourceSHA256'] == plan['sourceSHA256']
    registration = roots['roots'][value['sourceCheckpoint']]
    root_path, parent_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json'), BASE / 'sacred-medicine-trial-entry-1.json'
    assert registration['sha256'] == value['sourceSHA256'] == digest(root_path)['sha256'] and registration['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed'] and not load(parent_path)['resumedTestRoute']
    artifacts.extend([root_path, parent_path])
    battles = value['battleCashChanges']
    assert len(battles) == proof['Battles'] and all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in battles)
    inventory = dict(value['inventoryInitial'])
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['position'] == [point['X'], point['Y']] and reward['scene'] == point['Scene'] == scene
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert len(reward['repeatPose']) == 3 and 0 <= reward['repeatPose'][2] < 4 and not reward['invisible'] and point['Sprite'] in [10, 389]
        assert not reward['originalHerbDisappearance'] and not reward['disappearsAfterPickup'] and reward['openFrame'] == 1 and reward['closedFrame'] != reward['openFrame'] and reward['searchMode'] == point['TriggerMode'] == 1
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in battles for battle in reward['battleCashDuringPickup']) and reward['cashAfter']-reward['cashBefore'] == sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
        for grant in reward['grants']:
            assert grant['Cash'] == 0
            inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        native = next(row for row in review['records'] if row['event'] == event)
        assert native['result']['sha256'] == digest(path)['sha256'] and [row['label'] for row in native['stages']] == ['unpicked', 'notice', 'collected', 'repeated']
        assert native['finalCaseState']['nativeEvent'][4] == 36509 and native['finalCaseState']['nativeEvent'][6] == 2 and new_points[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    for drop in drops:
        assert drop['endScript'] == 42828 and drop['item'] == 152 and drop['name'] == '傀儡虫' and drop['amount'] > 0 and proof['Saved'][f"battle-{drop['battle']}-worm-drop"]
        inventory['傀儡虫'] = inventory.get('傀儡虫', 0)+drop['amount']
    native_drops = [row for row in review['battleItemGrants'] if row['scene'] == scene]
    assert [{key: row[key] for key in drops[0]} for row in native_drops] == drops and all(row['nativePC'] == 42830 for row in native_drops)
    assert inventory == value['inventory'] and value['inventoryInitial']['傀儡虫'] == 3 and value['inventory']['土灵珠'] == value['inventoryInitial']['土灵珠'] == 1
    assert value['trialPortalSteps'] == case['portalSteps'] and value['trialEarthReturns'] == case['earthReturns'] and len(value['trialRoute']) == case['portalSteps']+case['earthReturns']
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    log = VERIFY / f'trial-deep-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    regional_source = SOURCE / ('regional_before_chest_blocks.go' if scene != 223 else 'regional_final.go')
    route_source = SOURCE / ('route_before_chest_blocks.go' if scene != 223 else 'route_final.go')
    regional_info = digest(regional_source) if scene != 223 else digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go')
    route_info = digest(route_source) if scene != 223 else digest(ROOT / 'games/pal/tests/trial_rewards_route_e2e_test.go')
    regional_info['path'], route_info['path'] = str(regional_source.relative_to(ROOT)), str(route_source.relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'newFreshRuntimeSaves': proof['Saves'], 'newOrdinaryBattles': proof['Battles'], 'normalWorms': sum(row['amount'] for row in drops), 'normalWormDropBattles': len(drop_battles), 'portalActions': case['portalSteps'], 'earthReturns': case['earthReturns'], 'testSource': regional_info, 'routeHelperSource': route_info, 'report': digest(path)})

unreachable_path = VERIFY / 'trial-deep-original-unreachable.json'
unreachable = load(unreachable_path)
assert unreachable['passed'] and unreachable['readyForProgress'] and not unreachable['nativeEdits'] and unreachable['nativeBuildID'] == core and len(unreachable['records']) == 1
record = unreachable['records'][0]
assert record['event'] == 4000 and record['classification'] == new_points[4000]['status'] == 'original-unreachable-preserved' and not record['actuallyObtained'] and not record['normalPickupVerified'] and record['reachableSearchStances'] == []
assert record['nativeRecordPreservedBefore'] and record['nativeRecordPreservedAfter'] and record['allRawIncomingPlacements'] == 28 and record['allActuallyTriggerableIncomingPlacements'] == 26
assert unreachable['graph']['optimisticWalkablePositions'] == 2733 and unreachable['graph']['impossibleShadowedEntranceWalkablePositions'] == 2853 and unreachable['graph']['ignoresAllDynamicCollisions'] and unreachable['graph']['evenAllowsWalkingAcrossAllTeleportTriggerAreas']
assert [(row['firstEvent'], row['shadowedEvent']) for row in unreachable['shadowedPortals']] == [(3949, 3954), (3950, 3955)] and unreachable['explicitStateModeAutoScriptWrites'] == []
checked_references(unreachable)
artifacts.extend([unreachable_path, VERIFY / 'trial-deep-unreachable-review-run.txt'])
priority_path = BASE / 'trial-priority-v1-result-1.json'
priority = load(priority_path)
assert priority['passed'] and priority['firstEvent'] == 3949 and priority['shadowedEvent'] == 3954 and priority['actualDestinationScene'] == 227 and priority['actualDestination'] == [1488, 1272] and priority['proof']['Saves'] == 5 and priority['proof']['Battles'] == 0
registration = roots['roots'][priority['sourceCheckpoint']]
assert registration['sha256'] == priority['sourceSHA256'] and registration['report'] == 'regional-hidden-v18-226-result.json'
for label in priority['proof']['Saved']:
    artifacts.extend(BASE / f'trial-priority-v1-{label}-{kind}-1.json' for kind in ['state', 'proof'])
artifacts.extend([priority_path, BASE / 'trial-priority-v1-state-1.json'])
for filename in ['trial-deep-lint-run.txt', 'trial-deep-lint-negative-run.txt', 'trial-deep-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
snapshot_names = ['tests/trial_deep_priority_e2e_test.go', 'tests/trial-deep-rewards-plan.py', 'tests/trial-deep-priority-plan.py', 'tests/trial-deep-evidence-e2e.py', 'tests/trial-deep-unreachable-review.py', 'tests/trial-deep-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'TRIAL_DEEP_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']
for filename in snapshot_names:
    path = ROOT / 'games/pal' / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
for original_name, frozen_name in [('regional_hidden_rewards_e2e_test.go', 'regional_final.go'), ('trial_rewards_route_e2e_test.go', 'route_final.go')]:
    target = SOURCE / frozen_name
    shutil.copyfile(ROOT / 'games/pal/tests' / original_name, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core
authority_hash = digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256']
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
new_saves, new_battles = sum(row['newFreshRuntimeSaves'] for row in cases)+5, sum(row['newOrdinaryBattles'] for row in cases)
summary = {'passed': True, 'scope': '78 new original sixth/seventh and teleport-layer chests, separately confirmed original inaccessible invisibility worm 4000 with all real entrances and actual first overlapping portal witness, original normal worm drops and full final chest-state retention; no original edits or uploads, not bottom cave, all hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 79, 'newActuallyClaimed': 78, 'newOriginalUnreachablePreserved': 1, 'cases': cases, 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'normalWormDrops': sum(row['normalWorms'] for row in cases), 'normalWormDropBattles': sum(row['normalWormDropBattles'] for row in cases), 'portalActions': sum(row['portalActions'] for row in cases), 'earthReturns': sum(row['earthReturns'] for row in cases), 'priorityWitness': {'freshRuntimeSaves': 5, 'ordinaryBattles': 0, 'report': digest(priority_path)}, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 4687, 'priorYuhangTrialArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'trial-deep-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'normalWormDrops': summary['normalWormDrops'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
