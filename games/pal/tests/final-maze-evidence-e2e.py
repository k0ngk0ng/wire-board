import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'final-maze-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 652 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 703 and progress['total'] == 735 and progress['remaining'] == 32
assert progress['actuallyClaimed'] == 678 and progress['originalUnreachablePreserved'] == 25
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
plan_path = VERIFY / 'final-maze-rewards-plan.json'
plan = load(plan_path)
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits']
assert {row['scene']: len(row['events']) for row in plan['cases']} == {280: 5, 287: 3, 288: 2, 289: 11, 290: 6, 291: 3, 292: 4, 293: 9, 294: 5}
actual_events = {event for case in plan['cases'] for event in case['events']}
preserved_events, still_pending = {4846, 4847, 4917}, {5071, 5072, 5073, 5074, 5077}
assert len(actual_events) == 48 and len(actual_events | preserved_events | still_pending) == 56
assert {event for case in plan['cases'] for event in case['unplannedEvents']} == still_pending
old = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new)-set(old) == actual_events | preserved_events and all(new[event] == value for event, value in old.items())
assert all(event in progress['pendingEventIDs'] and event not in new for event in still_pending)
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 6341 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'trial-bottom-stage.tar.gz')['sha256'] == '127ffd0390e8d9c2615ff402380dfa1e93e3310eebbd007f71aecf6badf3ea34'
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json', 'regional_hidden_rewards_e2e_test.go', 'trial_rewards_route_e2e_test.go', 'regional_first.go', 'route_first.go', 'final_route_first.go', 'regional_summons_physical.go', 'regional_force.go']]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'final-maze-evidence-red-run.txt', 'final-maze-evidence-corrected-scope-red-run.txt', 'final-maze-evidence-pre-aggregate-red-run.txt', 'final-maze-progress-run.txt', 'final-maze-plan-run.txt', 'final-maze-original-review-run.txt', 'final-maze-unreachable-review-run.txt', 'trial-bottom-stage-archive.json', 'trial-bottom-archive-verification.json'])
artifacts.append(plan_path)


def checked_references(value):
    if isinstance(value, dict):
        if 'path' in value and 'sha256' in value:
            path = ROOT / value['path']
            assert path.resolve().is_relative_to(ROOT) and path.is_file() and not path.is_symlink()
            assert digest(path)['sha256'] == value['sha256']
            artifacts.append(path)
        for child in value.values():
            checked_references(child)
    elif isinstance(value, list):
        for child in value:
            checked_references(child)


review_path = VERIFY / 'final-maze-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and len(review['records']) == 48 and {row['event'] for row in review['records']} == actual_events
checked_references(review)
artifacts.append(review_path)
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
cases = []
for case in plan['cases']:
    scene = case['scene']
    prefix = f'regional-hidden-v19-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    proof, grants, appearances = value['proof'], value['battleItemGrants'], value.get('wormAppearances', [])
    assert value['passed'] and value['verificationVersion'] == 19 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture'] and not value['auditOnly']
    assert value['auditedEvents'] == value['emptyChestAudits'] == [] and value['sourceCheckpoint'] == case['sourceCheckpoint'] and value['sourceSHA256'] == case['sourceSHA256']
    assert [row['event'] for row in value['claimed']] == case['events'] and proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0
    assert [(row['role'], row['controller']) for row in value['partyInitial']] == [(row['role'], row['controller']) for row in value['party']] == [(1, 0), (0, 0), (4, 0)]
    assert value['routePlanSHA256'] == digest(plan_path)['sha256'] and value['trialPortalSteps'] == case['portalSteps'] and value['trialEarthReturns'] == 0
    drop_battles = {row['battle'] for row in grants}
    assert proof['Saves'] == len(proof['Saved']) == len(case['events'])*4+proof['Battles']+len(drop_battles)+len(appearances)+case['portalSteps'] and all(proof['Saved'].values())
    inventory = dict(value['inventoryInitial'])
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['scene'] == scene and reward['position'] == [point['X'], point['Y']] and reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] == 3 and reward['repeatedFromSavedPose']
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        for grant in reward['grants']:
            assert grant['Cash'] == 0
            inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        assert reward['cashAfter']-reward['cashBefore'] == sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
    for row in grants:
        assert row['endScript'] == 42828 and row['item'] == 152 and row['name'] == '傀儡虫' and row['amount'] > 0 and proof['Saved'][f"battle-{row['battle']}-worm-drop"]
        inventory['傀儡虫'] = inventory.get('傀儡虫', 0)+row['amount']
    for row in appearances:
        assert proof['Saved'][row['label']] and row['slot'] in range(5) and any(native['scene'] == scene and all(native[key] == row[key] for key in row) for native in review['wormAppearances'])
    assert inventory == value['inventory'] and value['inventory']['土灵珠'] == value['inventoryInitial']['土灵珠'] == 1
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges']) and len(value['battleCashChanges']) == proof['Battles']
    registration = roots['roots'][case['sourceCheckpoint']]
    source_path = BASE / (case['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / 'campaign-finale-1.json'
    assert registration['sha256'] == digest(source_path)['sha256'] == value['sourceSHA256'] and registration['report'] == 'campaign-finale.json' and registration['reportSHA256'] == digest(parent_path)['sha256']
    assert load(parent_path)['passed'] and not load(parent_path)['resumedTestRoute']
    source_name = 'regional_summons_physical.go' if scene == 280 else 'regional_first.go' if scene == 287 else 'regional_force.go'
    source = SOURCE / source_name
    log = VERIFY / (f'final-maze-{scene}-final-run.txt' if scene == 280 else f'final-maze-{scene}-run.txt' if scene == 287 else f'final-maze-{scene}-force-run.txt')
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', source_path, parent_path, log])
    cases.append({'scene': scene, 'events': case['events'], 'freshRuntimeSaves': proof['Saves'], 'ordinaryBattles': proof['Battles'], 'normalWormDrops': sum(row['amount'] for row in grants), 'wormAppearances': len(appearances), 'portalActions': case['portalSteps'], 'forceInputs': value.get('forceInputs', 0), 'executionSource': digest(source), 'routeHelperSource': digest(SOURCE / 'route_first.go'), 'finalRouteHelperSource': digest(SOURCE / 'final_route_first.go'), 'report': digest(path)})
unreachable_path = VERIFY / 'final-maze-original-unreachable.json'
unreachable = load(unreachable_path)
assert unreachable['passed'] and unreachable['readyForProgress'] and not unreachable['nativeEdits'] and {row['event'] for row in unreachable['records']} == preserved_events
assert {row['event'] for row in unreachable['unverifiedRecords']} == still_pending and all(not row['readyForProgress'] for row in unreachable['unverifiedRecords'])
assert unreachable['explicitStateModePositionOrScriptWrites'] == [] and unreachable['graph']['optimisticWalkablePositions'] == 1019 and len(unreachable['graph']['allRawIncomingPlacements']) == 5
assert unreachable['unverifiedBackAbyssGraph']['optimisticWalkablePositions'] == 2103 and len(unreachable['unverifiedBackAbyssGraph']['allRawIncomingPlacements']) == 2
for row in unreachable['records']:
    assert row['classification'] == new[row['event']]['status'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['normalPickupVerified'] and row['reachableSearchStances'] == []
checked_references(unreachable)
artifacts.append(unreachable_path)
empty_path = BASE / 'final-maze-empty-v1-result-1.json'
empty = load(empty_path)
assert empty['passed'] and empty['event'] == 5049 and empty['originalItemID'] == 0 and empty['itemBefore'] == empty['itemAfter'] and empty['repeatSearches'] == 3 and empty['proof']['Battles'] == empty['proof']['Wins']
assert empty['proof']['Saves'] == len(empty['proof']['Saved']) == empty['proof']['Battles']+4 and 5049 not in catalog and 5049 not in new
for label in empty['proof']['Saved']:
    artifacts.extend(BASE / f'final-maze-empty-v1-{label}-{kind}-1.json' for kind in ['state', 'proof'])
empty_log = VERIFY / 'final-maze-empty-run.txt'
assert '--- PASS: TestOriginalFinalMazeFalseJadeChestAndSavedNoGrant' in empty_log.read_text() and '\nFAIL' not in empty_log.read_text()
artifacts.extend([empty_path, BASE / 'final-maze-empty-v1-state-1.json', empty_log])
for filename in ['final-maze-lint-run.txt', 'final-maze-lint-negative-run.txt', 'final-maze-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['tests/final_maze_rewards_route_e2e_test.go', 'tests/final_maze_empty_chest_e2e_test.go', 'tests/final-maze-rewards-plan.py', 'tests/final-maze-original-review.py', 'tests/final-maze-unreachable-review.py', 'tests/final-maze-evidence-e2e.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'FINAL_MAZE_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
authority = digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256']
assert authority == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508' and digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core
summary = {'passed': True, 'scope': '48 actual final maze rewards and 3 independently confirmed original preserved records; actual false jade chest separately checked; 5 additional back-abyss records stay pending for independent classification evidence; not all hidden rewards, performance or release', 'nativeBuildID': core, 'authorityArchiveSHA256': authority, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 51, 'newActuallyClaimed': 48, 'newOriginalUnreachablePreserved': 3, 'stillUnverified': sorted(still_pending), 'cases': cases, 'newFreshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases)+empty['proof']['Saves'], 'newOrdinaryBattles': sum(row['ordinaryBattles'] for row in cases)+empty['proof']['Battles'], 'normalWormDrops': sum(row['normalWormDrops'] for row in cases), 'portalActions': sum(row['portalActions'] for row in cases), 'emptyChest': {'event': 5049, 'freshRuntimeSaves': empty['proof']['Saves'], 'ordinaryBattles': empty['proof']['Battles'], 'report': digest(empty_path)}, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 6341, 'priorBottomTrialArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'final-maze-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': summary['newFreshRuntimeSaves'], 'newOrdinaryBattles': summary['newOrdinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
