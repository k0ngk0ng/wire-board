import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'tower-remainder-source'
GLOBAL = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
BASE = GLOBAL / 'scopes/tower-remainder-v1'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 703 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 716 and progress['total'] == 735 and progress['remaining'] == 19
assert progress['actuallyClaimed'] == 691 and progress['originalUnreachablePreserved'] == 25
plan_path = VERIFY / 'tower-remainder-plan.json'
plan = load(plan_path)
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits']
case = plan['case']
expected = set(case['events']) | {2677, 2408}
assert len(case['events']) == 11 and len(expected) == 13 and case['unplannedEvents'] == [2763]
old = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new)-set(old) == expected and all(new[event] == row for event, row in old.items())
assert 2763 not in new and 2763 in progress['pendingEventIDs'] and progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 7172 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'final-maze-stage.tar.gz')['sha256'] == '03124c00ace93d1c2f79f72f293373866d53367bc9402650e06fab4de53c681c'
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json', 'regional_hidden_rewards_e2e_test.go', 'trial_rewards_route_e2e_test.go', 'seven-star-ascii-alias.json', 'plan-all-twelve-red-run.txt']]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'tower-remainder-evidence-red-run.txt', 'tower-remainder-evidence-corrected-red-run.txt', 'tower-remainder-evidence-pre-aggregate-red-run.txt', 'tower-remainder-progress-run.txt', 'tower-remainder-plan-run.txt', 'tower-remainder-original-review-run.txt', 'final-maze-stage-archive.json', 'final-maze-archive-verification.json'])
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


review_path = VERIFY / 'tower-remainder-original-review.json'
review = load(review_path)
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and len(review['records']) == 11 and {row['event'] for row in review['records']} == set(case['events'])
assert {row['event'] for row in review['storyRewards']} == {2677, 2408} and not review['pendingOriginalEvent']['readyForProgress'] and not review['pendingOriginalEvent']['actuallyObtained']
assert review['pendingOriginalEvent']['event'] == 2763 and review['pendingOriginalEvent']['reachableSearchStances'] == [] and review['pendingOriginalEvent']['optimisticWalkablePositions'] == 5726
checked_references(review)
artifacts.append(review_path)
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'


def root_checked(name, source_hash):
    registration = roots['roots'][name]
    path = BASE / (name[:-5]+'-1.json')
    parent = BASE / (registration['report'][:-5]+'-1.json')
    assert registration['sha256'] == source_hash == digest(path)['sha256'] and registration['reportSHA256'] == digest(parent)['sha256']
    assert load(parent)['passed'] and not load(parent)['resumedTestRoute']
    artifacts.extend([path, parent])


def saved_artifacts(prefix, value):
    for label in value['proof']['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.extend([BASE / f'{prefix}-result-1.json', BASE / f'{prefix}-state-1.json'])


prefix = 'regional-hidden-v20-167'
value = load(BASE / f'{prefix}-result-1.json')
proof = value['proof']
assert value['passed'] and value['verificationVersion'] == 20 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture'] and value['scene'] == 167 and value['combatAllowed']
assert [row['event'] for row in value['claimed']] == case['events'] and value['routePlanSHA256'] == digest(plan_path)['sha256'] and value['trialPortalSteps'] == value['trialEarthReturns'] == 0 and value['trialRoute'] == []
assert proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0
assert proof['Saves'] == len(proof['Saved']) == len(case['events'])*4+proof['Battles'] and value['wormAppearances'] == value['battleItemGrants'] == []
assert [(row['role'], row['controller']) for row in value['partyInitial']] == [(row['role'], row['controller']) for row in value['party']] == [(0, 0), (2, 0)]
inventory = dict(value['inventoryInitial'])
for row in value['claimed']:
    point = catalog[row['event']]
    assert row['grants'] == point['Grants'] and row['position'] == [point['X'], point['Y']] and row['notification'] and row['repeatSearches'] == 3 and row['repeatedFromSavedPose']
    for label in ['unpicked', 'notice', 'collected', 'repeated']:
        assert proof['Saved'][f"reward-{row['event']}-{label}"]
    for grant in row['grants']:
        assert grant['Cash'] == 0
        inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
assert inventory == value['inventory'] and value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges']) and len(value['battleCashChanges']) == proof['Battles']
assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in value['battleCashChanges'])
root_checked(value['sourceCheckpoint'], value['sourceSHA256'])
saved_artifacts(prefix, value)
log = VERIFY / 'tower-remainder-167-run.txt'
assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
artifacts.append(log)
new_saves, new_battles = proof['Saves'], proof['Battles']
reused = []
for event in [2677, 2408]:
    prefix = f'shu-reward-repeat-v1-{event}'
    repeat = load(BASE / f'{prefix}-result-1.json')
    assert repeat['passed'] and not repeat['nativeEdits'] and repeat['existingEarnedBranch'] and repeat['repeatSearches'] == 3 and repeat['proof']['Saves'] == len(repeat['proof']['Saved']) == 4 and repeat['proof']['Battles'] == 0
    assert all(row == repeat['inventoryBefore'] for row in repeat['repeatInventories']) and repeat['inventoryBefore'] == repeat['inventoryAfter'] and repeat['inventoryAfter'][repeat['item']] == 1
    root_checked(repeat['sourceCheckpoint'], repeat['sourceSHA256'])
    saved_artifacts(prefix, repeat)
    source = next(row for row in review['storyRewards'] if row['event'] == event)
    parent = load(ROOT / source['reusedCompleteStory']['path'])
    assert parent['passed'] and not parent['resumedTestRoute']
    if event == 2408:
        assert repeat['inventoryAfter']['天师道袍'] == 1 and source['originalBattleFollowup']['enemy'] == 494 and source['originalBattleFollowup']['endScript'] == 42420 and new[event]['followupRewards'] == [{'PC': 42420, 'Item': 216, 'Amount': 1, 'Cash': 0, 'Name': '天师道袍'}]
    reused.append({'event': event, 'existingStorySaves': parent['proof']['Saves'], 'existingStoryBattles': parent['proof']['Battles'], 'newRepeatSaves': repeat['proof']['Saves'], 'newBattles': 0, 'originalReport': source['reusedCompleteStory']})
    new_saves += repeat['proof']['Saves']
log = VERIFY / 'tower-remainder-story-repeat-run.txt'
assert '--- PASS: TestOriginalShuJadeAndSevenStarSavedRepeat' in log.read_text() and '\nFAIL' not in log.read_text()
artifacts.append(log)
prefix = 'tower-remainder-empty-v1'
empty = load(BASE / f'{prefix}-result-1.json')
assert empty['passed'] and empty['event'] == 2764 and empty['event'] not in catalog and empty['originalItemID'] == 0 and empty['originalGrantPC'] == 37482 and empty['itemBefore'] == empty['itemAfter'] and empty['repeatSearches'] == 3
assert empty['proof']['Saves'] == len(empty['proof']['Saved']) == empty['proof']['Battles']+4 and empty['proof']['Battles'] == empty['proof']['Wins']
root_checked(empty['sourceCheckpoint'], empty['sourceSHA256'])
saved_artifacts(prefix, empty)
new_saves += empty['proof']['Saves']
new_battles += empty['proof']['Battles']
log = VERIFY / 'tower-remainder-empty-final-run.txt'
assert '--- PASS: TestOriginalTowerFalseJadeChestAndSavedNoGrant' in log.read_text() and '\nFAIL' not in log.read_text()
artifacts.append(log)
for name in ['regional_hidden_rewards_e2e_test.go', 'trial_rewards_route_e2e_test.go', 'tower_remainder_route_e2e_test.go', 'shu_story_rewards_repeat_e2e_test.go', 'final_maze_empty_chest_e2e_test.go']:
    executed = SOURCE / ('executed_'+name)
    assert digest(executed)['sha256'] == digest(ROOT / 'games/pal/tests' / name)['sha256']
    artifacts.append(executed)
for filename in ['tower-remainder-lint-run.txt', 'tower-remainder-lint-negative-run.txt', 'tower-remainder-roots-run.txt', 'tower-remainder-scope-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    elif filename.endswith('scope-run.txt'):
        assert '--- PASS: TestCampaignEvidenceScopesPreserveUnscopedParents' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['tests/tower_remainder_route_e2e_test.go', 'tests/shu_story_rewards_repeat_e2e_test.go', 'tests/final_maze_empty_chest_e2e_test.go', 'tests/tower-remainder-plan.py', 'tests/tower-remainder-original-review.py', 'tests/tower-remainder-evidence-e2e.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'TOWER_REMAINDER_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core and digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256'] == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': '11 actual remaining tower-floor chests, existing same-core complete jade bead and seven-star/robe rewards revalidated with new actual repeat searches, original zero-item chest actual opening; 2763 stays pending for independent classification; not all hidden rewards, performance or release', 'nativeBuildID': core, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 13, 'newActuallyClaimed': 13, 'newOriginalUnreachablePreserved': 0, 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'newChestEvents': case['events'], 'reusedStoryRewards': reused, 'pendingEvent': 2763, 'falseChestEvent': 2764, 'isolatedEvidenceScope': 'tower-remainder-v1', 'unscopedParentsUnchanged': True, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 7172, 'priorFinalMazeArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'tower-remainder-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
