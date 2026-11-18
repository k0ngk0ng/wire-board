import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'yuhang-trial-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 507 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 567 and progress['total'] == 735
assert progress['actuallyClaimed'] == 546 and progress['originalUnreachablePreserved'] == 21 and progress['remaining'] == 168
plan_path = VERIFY / 'trial-rewards-plan.json'
plan = load(plan_path)
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits']
EXPECTED = {case['scene']: case['events'] for case in plan['cases']}
assert set(EXPECTED[220]) == set(range(3749, 3785)) and set(EXPECTED[221]) == set(range(3788, 3810))
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {4663, 4674}
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements'] and progress['preservedPoints'] == prior['preservedPoints']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 3704 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'past-rewards-stage.tar.gz')['sha256'] == '19d8745ee3e648ceecc6c7853d9a02367a29e53eb94952cbf41a607081eb8122'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', SOURCE / 'regional_before_trial.go', plan_path]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'yuhang-trial-evidence-red-run.txt', 'yuhang-trial-evidence-pre-aggregate-red-run.txt', 'yuhang-trial-progress-run.txt', 'trial-rewards-plan-run.txt', 'trial-rewards-plan-before-earth-return-run.txt', 'past-rewards-stage-archive.json', 'past-rewards-archive-verification.json'])
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


def registered_source(name, checksum, parent_name):
    registration = roots['roots'][name]
    path = BASE / (name[:-5]+'-1.json')
    parent = BASE / (parent_name[:-5]+'-1.json')
    assert registration['sha256'] == checksum == digest(path)['sha256'] and registration['report'] == parent_name and registration['reportSHA256'] == digest(parent)['sha256']
    assert load(parent)['passed'] and not load(parent)['resumedTestRoute']
    artifacts.extend([path, parent])


review_path = VERIFY / 'yuhang-trial-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and review['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert len(review['records']) == 58 and {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
checked_references(review)
artifacts.extend([review_path, VERIFY / 'yuhang-trial-original-review-run.txt'])
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v17-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 17 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events and value['routePlanSHA256'] == digest(plan_path)['sha256']
    case = next(row for row in plan['cases'] if row['scene'] == scene)
    proof = value['proof']
    drops = value['battleItemGrants']
    drop_battles = {row['battle'] for row in drops}
    assert proof['Saves'] == len(proof['Saved']) == len(events)*4+proof['Battles']+len(drop_battles)+case['portalSteps']+2*case['earthReturns'] and all(proof['Saved'].values())
    assert proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0 and value['combatAllowed']
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']] == [(0, 0), (4, 0)]
    assert value['sourceCheckpoint'] == plan['sourceCheckpoint'] and value['sourceSHA256'] == plan['sourceSHA256']
    registered_source(value['sourceCheckpoint'], value['sourceSHA256'], 'sacred-medicine-trial-entry.json')
    battles = value['battleCashChanges']
    assert len(battles) == proof['Battles'] and all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in battles)
    expected_inventory = dict(value['inventoryInitial'])
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['position'] == [point['X'], point['Y']] and reward['scene'] == point['Scene'] == scene
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert len(reward['repeatPose']) == 3 and 0 <= reward['repeatPose'][2] < 4 and not reward['invisible'] and point['Sprite'] == 10
        assert not reward['originalHerbDisappearance'] and not reward['disappearsAfterPickup'] and reward['openFrame'] == 1 and reward['closedFrame'] != reward['openFrame'] and reward['searchMode'] == point['TriggerMode'] == 1
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in battles for battle in reward['battleCashDuringPickup']) and reward['cashAfter']-reward['cashBefore'] == sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
        for grant in reward['grants']:
            assert grant['Cash'] == 0
            expected_inventory[grant['Name']] = expected_inventory.get(grant['Name'], 0)+grant['Amount']
        native = next(row for row in review['records'] if row['event'] == event)
        assert native['result']['sha256'] == digest(path)['sha256'] and [row['label'] for row in native['stages']] == ['unpicked', 'notice', 'collected', 'repeated']
        assert native['finalCaseState']['nativeEvent'][4] == 36509 and native['finalCaseState']['nativeEvent'][6] == 2
        assert new_points[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    for drop in drops:
        assert drop['endScript'] == 42828 and drop['item'] == 152 and drop['name'] == '傀儡虫' and drop['amount'] > 0 and 1 <= drop['battle'] <= proof['Battles']
        expected_inventory['傀儡虫'] = expected_inventory.get('傀儡虫', 0)+drop['amount']
        assert proof['Saved'][f"battle-{drop['battle']}-worm-drop"]
    native_drops = [row for row in review['battleItemGrants'] if row['scene'] == scene]
    assert [{key: row[key] for key in drops[0]} for row in native_drops] == drops and all(row['nativePC'] == 42830 for row in native_drops)
    assert expected_inventory == value['inventory'] and value['inventoryInitial']['傀儡虫'] == 3 and value['inventory']['傀儡虫'] == 23 and value['inventory']['土灵珠'] == value['inventoryInitial']['土灵珠'] == 1
    assert value['trialPortalSteps'] == case['portalSteps'] == (14 if scene == 220 else 8) and value['trialEarthReturns'] == case['earthReturns'] == (1 if scene == 220 else 0)
    assert len(value['trialRoute']) == case['portalSteps']+case['earthReturns']
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    log = VERIFY / f'trial-rewards-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    source_info = digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go')
    source_info['path'] = str((SOURCE / 'regional_hidden_rewards_e2e_test.go').relative_to(ROOT))
    helper_info = digest(ROOT / 'games/pal/tests/trial_rewards_route_e2e_test.go')
    helper_info['path'] = str((SOURCE / 'trial_rewards_route_e2e_test.go').relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'newFreshRuntimeSaves': proof['Saves'], 'newOrdinaryBattles': proof['Battles'], 'normalWorms': sum(row['amount'] for row in drops), 'normalWormDropBattles': len(drop_battles), 'portalActions': case['portalSteps'], 'earthReturns': case['earthReturns'], 'testSource': source_info, 'routeHelperSource': helper_info, 'report': digest(path)})

old = review['oldYuhang']
assert old['events'] == [4674, 4663] and old['reusedExistingStoryEvidence'] and old['newMainStoryRuns'] == 0 and old['existingFreshRuntimeSaves'] == 88 and old['existingOrdinaryBattles'] == 8
assert new_points[4674]['status'] == 'original-cloth-use-and-saved-no-replay-verified' and new_points[4663]['status'] == 'original-wooden-sword-exchange-and-saved-no-duplicate-verified'
assert new_points[4663]['grants'] == catalog[4663]['Grants'] and new_points[4674]['grants'] == catalog[4674]['Grants']
assert new_points[4663]['consumedItems'] == [{'PC': 34918, 'Item': 166, 'Amount': 1, 'Name': '木剑'}] and new_points[4663]['followupRewards'] == [{'PC': 30511, 'Item': 275, 'Amount': 1, 'Cash': 0, 'Name': '金凤凰蛋壳'}]
main_path = ROOT / old['report']['path']
main = load(main_path)
assert main['passed'] and not main['resumedTestRoute'] and main['checks']['WrongUseChecked'] and main['checks']['ClothUsed'] and main['checks']['RepeatInputs'] == 3
assert main['proof']['Battles'] == main['proof']['Wins'] == 8 and main['proof']['ScriptedFinishes'] == 0 and main['proof']['Saves'] == len(main['proof']['Saved']) == 88 and all(main['proof']['Saved'].values())
assert all(main['proof']['Seen'][label] for label in ['cloth-granted', 'cloth-recognized', 'xiulan-search', 'toy-chest', 'wooden-sword-request', 'exchange', 'water-orb', 'egg-shell', 'anu-reunion'])
for label in main['proof']['Saved']:
    artifacts.extend(BASE / f'old-yuhang-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    checks_path = BASE / f'old-yuhang-{label}-checks-1.json'
    if checks_path.is_file():
        artifacts.append(checks_path)
repeat_path = ROOT / review['newClothRepeat']['report']['path']
repeat = load(repeat_path)
assert repeat['passed'] and repeat['repeatConversations'] == 3 and repeat['inventoryBefore'] == repeat['inventoryAfter'] == {'凤纹手绢': 1, '木剑': 1} and repeat['cashBefore'] == repeat['cashAfter']
assert repeat['proof']['Saves'] == 4 and repeat['proof']['Battles'] == 0 and all(repeat['proof']['Saved'].values()) and all(n > 0 for n in repeat['readingsPerRepeat'])
registered_source(repeat['sourceCheckpoint'], repeat['sourceSHA256'], 'campaign-old-yuhang.json')
for label in repeat['proof']['Saved']:
    artifacts.extend(BASE / f'old-yuhang-rewards-v1-{label}-{kind}-1.json' for kind in ['state', 'proof'])
log = VERIFY / 'old-yuhang-rewards-repeat-run.txt'
assert '--- PASS: TestOriginalOldYuhangClothRepeatedDialogueAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
artifacts.extend([log, BASE / 'old-yuhang-rewards-v1-state-1.json'])
repeat_source = digest(ROOT / 'games/pal/tests/old_yuhang_rewards_e2e_test.go')
repeat_source['path'] = str((SOURCE / 'old_yuhang_rewards_e2e_test.go').relative_to(ROOT))
for filename in ['yuhang-trial-lint-run.txt', 'yuhang-trial-lint-negative-run.txt', 'yuhang-trial-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['tests/regional_hidden_rewards_e2e_test.go', 'tests/trial_rewards_route_e2e_test.go', 'tests/old_yuhang_rewards_e2e_test.go', 'tests/trial-rewards-plan.py', 'tests/yuhang-trial-evidence-e2e.py', 'tests/yuhang-trial-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'YUHANG_TRIAL_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core
authority_hash = digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256']
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
assert sum(row['newFreshRuntimeSaves'] for row in cases)+repeat['proof']['Saves'] == 355 and sum(row['newOrdinaryBattles'] for row in cases) == 57 and sum(row['normalWorms'] for row in cases) == 40
summary = {'passed': True, 'scope': '58 new actual original trial fourth/fifth chests through original doors/ropes/chained falls and real earth-orb return, separate 40 original normal worm drops; new three grandmother repeat conversations, separately revalidated existing cloth/use and wooden-sword/orb/egg story chains; no original edits or uploads, not all trial floors, hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 60, 'newActuallyClaimed': 60, 'newOriginalUnreachablePreserved': 0, 'cases': cases, 'newClothRepeat': {'event': 4674, 'newFreshRuntimeSaves': 4, 'repeatConversations': 3, 'testSource': repeat_source, 'report': digest(repeat_path)}, 'newFreshRuntimeSaves': 355, 'newOrdinaryBattles': 57, 'normalWormDrops': 40, 'normalWormDropBattles': sum(row['normalWormDropBattles'] for row in cases), 'portalActions': 22, 'earthReturns': 1, 'revalidatedExistingStoryEvidence': {'events': [4674, 4663], 'freshRuntimeSaves': 88, 'ordinaryBattles': 8, 'report': digest(main_path)}, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 3704, 'priorPastRewardsArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'yuhang-trial-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': 355, 'newOrdinaryBattles': 57, 'normalWormDrops': 40, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
