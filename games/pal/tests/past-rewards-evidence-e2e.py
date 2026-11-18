import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'past-rewards-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 476 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 507 and progress['total'] == 735
assert progress['actuallyClaimed'] == 486 and progress['originalUnreachablePreserved'] == 21 and progress['remaining'] == 228
EXPECTED = {263: list(range(4580, 4587)), 264: list(range(4601, 4607)), 265: list(range(4643, 4647)), 253: [4445, 4444, 4446, 4447, 4448], 251: list(range(4415, 4423))}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {4649}
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements'] and progress['preservedPoints'] == prior['preservedPoints']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 2890 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'dali-rewards-stage.tar.gz')['sha256'] == 'f41cc2c5a63b27b053edd5e8f43ba044c2baef6ce0449b590266bc165b532ad5'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json', 'regional_before_past.go', 'regional_before_spell_cost.go', 'regional_before_maze_order.go', 'water-spell-unaffordable-run.txt', 'water-spell-unaffordable-state.json', 'water-spell-unaffordable-proof.json', 'water-front-spell-unaffordable-run.txt', 'water-front-spell-unaffordable-state.json', 'water-front-spell-unaffordable-proof.json', 'water-interrupted-maze-run.txt', 'water-interrupted-battle-12-state.json', 'water-interrupted-battle-12-proof.json']]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'past-rewards-evidence-red-run.txt', 'past-rewards-evidence-pre-aggregate-red-run.txt', 'past-rewards-native-closed-frame-red-run.txt', 'past-rewards-native-chase-cooldown-red-run.txt', 'past-rewards-progress-run.txt', 'dali-rewards-stage-archive.json', 'dali-rewards-archive-verification.json'])
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


review_path = VERIFY / 'past-rewards-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and review['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert len(review['records']) == 30 and {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
checked_references(review)
artifacts.extend([review_path, VERIFY / 'past-rewards-original-review-run.txt'])
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v16-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 16 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events
    proof = value['proof']
    assert proof['Saves'] == len(proof['Saved']) == len(events)*4+proof['Battles']+(7 if scene == 265 else 0) and all(proof['Saved'].values())
    assert proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0
    assert value['combatAllowed'] == (scene in [251, 253]) and value['battleItemGrants'] == []
    roles = [3, 0] if scene in [251, 253] else [0]
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']] == [(role, 0) for role in roles]
    if not value['combatAllowed']:
        assert value['partyInitial'] == value['party'] and proof['Battles'] == 0
    root = roots['roots'][value['sourceCheckpoint']]
    root_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / 'campaign-temple-past-1.json'
    assert value['sourceSHA256'] == root['sha256'] == digest(root_path)['sha256']
    assert root['report'] == 'campaign-temple-past.json' and root['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed'] and not load(parent_path)['resumedTestRoute']
    assert load(root_path)['public']['scene'] == (262 if scene == 265 else scene)
    artifacts.extend([root_path, parent_path])
    battles = value['battleCashChanges']
    assert len(battles) == proof['Battles'] and all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
    cash = sum(grant['Cash'] for row in value['claimed'] for grant in row['grants'])
    assert cash == (13600 if scene == 263 else 0) and value['cashFinal']-value['cashInitial'] == cash+sum(row['rewardCash'] for row in battles)
    expected_inventory = dict(value['inventoryInitial'])
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['position'] == [point['X'], point['Y']] and reward['scene'] == point['Scene'] == scene
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert len(reward['repeatPose']) == 3 and 0 <= reward['repeatPose'][2] < 4 and not reward['invisible'] and point['Sprite'] == 10
        assert not reward['originalHerbDisappearance'] and not reward['disappearsAfterPickup'] and reward['closedFrame'] == (2 if event == 4645 else 0) and reward['openFrame'] == 1 and reward['searchMode'] == point['TriggerMode'] == 1
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in battles for battle in reward['battleCashDuringPickup'])
        assert reward['cashAfter']-reward['cashBefore'] == sum(g['Cash'] for g in reward['grants'])+sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
        for grant in reward['grants']:
            if grant['Item'] > 0:
                expected_inventory[grant['Name']] = expected_inventory.get(grant['Name'], 0)+grant['Amount']
        native = next(row for row in review['records'] if row['event'] == event)
        assert native['result']['sha256'] == digest(path)['sha256'] and [row['label'] for row in native['stages']] == ['unpicked', 'notice', 'collected', 'repeated']
        assert new_points[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    assert expected_inventory == value['inventory']
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    if scene == 265:
        assert proof['Saved']['past-door-4564-arrived'] and all(proof['Saved'][f'past-monster-{event}-{stage}'] for event in [4628, 4630, 4632] for stage in ['before', 'opened'])
    log = VERIFY / f'past-rewards-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    source_path = SOURCE / ('regional_before_spell_cost.go' if scene in [263, 264, 265] else 'regional_before_maze_order.go' if scene == 253 else 'regional_hidden_rewards_e2e_test.go')
    source_info = digest(source_path) if scene != 251 else digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go')
    source_info['path'] = str(source_path.relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'newFreshRuntimeSaves': proof['Saves'], 'newOrdinaryBattles': proof['Battles'], 'testSource': source_info, 'report': digest(path)})

assert [row['event'] for row in review['originalBreedingSwitches']] == [4628, 4630, 4632] and review['breedingDoor']['event'] == 4564
assert len(review['originalWaterChasePreserved']) == 4 and {row['scene'] for row in review['originalWaterChasePreserved']} == {251, 253}
assert all(row['nativeEvent'][6] > 0 and row['nativeEvent'][7] in [0, 7] and row['reactivation'] == {'PC': 33667, 'instruction': [64, 65535, 7, 0]} for row in review['originalWaterChasePreserved'])
cooldown = next(row for row in review['originalWaterChasePreserved'] if row['scene'] == 251 and row['stage'] == 'after')
assert cooldown['originalPostBattleCooldown'] and cooldown['nativeEvent'][5] == 33667 and cooldown['nativeEvent'][7] == 0
assert review['originalWaterBattleSnapshots'] and {row['scene'] for row in review['originalWaterBattleSnapshots']} == {251, 253}
assert [(row['object'], row['mpCost']) for row in review['originalWaterSpellCosts']] == [(348, 44), (320, 68)]
staff = review['staff']
assert staff['event'] == 4649 and staff['reusedExistingStoryEvidence'] and staff['newGameplayRuns'] == 0
assert staff['existingFreshRuntimeSaves'] == 164 and staff['existingOrdinaryBattles'] == 22 and (staff['originalEntry'], staff['grantPC'], staff['item']) == (34080, 34082, 195)
assert new_points[4649]['status'] == 'original-staff-pickup-delivery-and-saved-no-duplicate-verified' and new_points[4649]['grants'] == catalog[4649]['Grants']
main_path = ROOT / staff['report']['path']
main = load(main_path)
assert main['passed'] and not main['resumedTestRoute'] and main['checks']['StaffRepeatInputs'] == 3 and main['checks']['InventoryWithStaff']['天蛇杖'] == 1 and main['checks']['InventoryAfter'].get('天蛇杖', 0) == 0
assert main['proof']['Battles'] == main['proof']['Wins'] == 22 and main['proof']['ScriptedFinishes'] == 0
assert main['proof']['Saves'] == len(main['proof']['Saved']) == 164 and all(main['proof']['Saved'].values())
for label in main['proof']['Saved']:
    artifacts.extend(BASE / f'temple-past-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    check_path = BASE / f'temple-past-{label}-checks-1.json'
    if check_path.is_file():
        artifacts.append(check_path)
for filename in ['past-rewards-lint-run.txt', 'past-rewards-lint-negative-run.txt', 'past-rewards-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['tests/regional_hidden_rewards_e2e_test.go', 'tests/past-rewards-evidence-e2e.py', 'tests/past-rewards-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'PAST_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
new_saves, new_battles = sum(row['newFreshRuntimeSaves'] for row in cases), sum(row['newOrdinaryBattles'] for row in cases)
assert new_saves == 127+new_battles and len({row['testSource']['sha256'] for row in cases}) == 3
summary = {'passed': True, 'scope': '30 new original underground/water chest pickups with original breeding switches, original water chase and combat, separately revalidated existing staff pickup/no-duplication/delivery story; no original edits or uploads, not all hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 31, 'newActuallyClaimed': 31, 'newOriginalUnreachablePreserved': 0, 'cases': cases, 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'revalidatedExistingStoryEvidence': {'event': 4649, 'freshRuntimeSaves': 164, 'ordinaryBattles': 22, 'report': digest(main_path)}, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 2890, 'priorDaliRewardsArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'past-rewards-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': new_saves, 'newOrdinaryBattles': new_battles, 'revalidatedExistingStorySaves': 164, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
