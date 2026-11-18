import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'dali-rewards-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 438 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 476 and progress['total'] == 735
assert progress['actuallyClaimed'] == 455 and progress['originalUnreachablePreserved'] == 21 and progress['remaining'] == 259
EXPECTED = {208: list(range(3513, 3536))+[3539, 3536, 3538, 3540, 3541, 3542, 3543, 3544, 3545, 3546], 210: [3576, 3577, 3578, 3579]}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {3354}
assert all(new_points[event] == row for event, row in old_points.items()) and 3537 in progress['pendingEventIDs'] and 3537 not in new_points
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements'] and progress['preservedPoints'] == prior['preservedPoints']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 2279 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'peach-rewards-stage.tar.gz')['sha256'] == '35818fb8d2d991da3b6246700d92ad45e4ab4e7f87b06487a896908c0f2964e7'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', SOURCE / 'regional_before_dali.go', SOURCE / 'regional_before_boot_review.go', SOURCE / 'original-boots-attempt-state.json', SOURCE / 'original-boots-attempt-proof.json', SOURCE / 'original-boots-attempt-run.txt', VERIFY / 'hidden-progress.json', VERIFY / 'hidden-search-candidates.json', VERIFY / 'hidden-conditional-candidates.json', VERIFY / 'hidden-touch-candidates.json', VERIFY / 'dali-rewards-evidence-red-run.txt', VERIFY / 'dali-rewards-evidence-pending-red-run.txt', VERIFY / 'dali-rewards-evidence-pre-aggregate-red-run.txt', VERIFY / 'dali-rewards-progress-run.txt', VERIFY / 'peach-rewards-stage-archive.json', VERIFY / 'peach-rewards-archive-verification.json']
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


review_path = VERIFY / 'dali-rewards-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and review['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert len(review['records']) == 37 and {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
checked_references(review)
artifacts.extend([review_path, VERIFY / 'dali-rewards-original-review-run.txt'])
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v15-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 15 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events
    proof = value['proof']
    assert proof['Saves'] == len(proof['Saved']) == (145 if scene == 208 else 18) and all(proof['Saved'].values())
    assert proof['Battles'] == proof['Wins'] == proof['ScriptedFinishes'] == value['preCollectionBattles'] == 0 and not value['combatAllowed'] and value['battleCashChanges'] == value['battleItemGrants'] == []
    assert value['partyInitial'] == value['party'] and [(p['role'], p['controller']) for p in value['party']] == [(4, 0), (0, 0)]
    assert value['sourceCheckpoint'] == 'campaign-dali-kirin-state.json'
    root = roots['roots'][value['sourceCheckpoint']]
    root_path = BASE / 'campaign-dali-kirin-state-1.json'
    parent_path = BASE / 'campaign-dali-kirin-1.json'
    assert value['sourceSHA256'] == root['sha256'] == digest(root_path)['sha256']
    assert root['report'] == 'campaign-dali-kirin.json' and root['reportSHA256'] == digest(parent_path)['sha256']
    assert load(parent_path)['passed'] and not load(parent_path)['resumedTestRoute'] and load(root_path)['public']['scene'] == 207
    artifacts.extend([root_path, parent_path])
    cash = sum(grant['Cash'] for row in value['claimed'] for grant in row['grants'])
    assert cash == (280 if scene == 208 else 300) and value['cashFinal']-value['cashInitial'] == cash and value['cashInitial'] == 67125
    expected_inventory = dict(value['inventoryInitial'])
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['position'] == [point['X'], point['Y']] and reward['scene'] == point['Scene'] == scene
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert len(reward['repeatPose']) == 3 and 0 <= reward['repeatPose'][2] < 4 and reward['invisible'] and point['Sprite'] == 0
        assert not reward['originalHerbDisappearance'] and not reward['disappearsAfterPickup'] and reward['closedFrame'] == reward['openFrame'] == -1 and reward['searchMode'] == point['TriggerMode'] == 1
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert reward['battleCashDuringPickup'] == [] and reward['cashAfter']-reward['cashBefore'] == sum(g['Cash'] for g in reward['grants'])
        for grant in reward['grants']:
            if grant['Item'] > 0:
                expected_inventory[grant['Name']] = expected_inventory.get(grant['Name'], 0)+grant['Amount']
        native = next(row for row in review['records'] if row['event'] == event)
        assert native['result']['sha256'] == digest(path)['sha256'] and [row['label'] for row in native['stages']] == ['unpicked', 'notice', 'collected', 'repeated']
        assert new_points[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    assert expected_inventory == value['inventory']
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    doors = [row for row in review['originalDoorRoutes'] if row['collectionScene'] == scene]
    assert [row['event'] for row in doors] == ([3466, 3499, 3463, 3496, 3464, 3497, 3465, 3498, 3470, 3503, 3469, 3502, 3468] if scene == 208 else [3460, 3551])
    assert all(proof['Saved'][f"dali-door-{row['event']}-arrived"] for row in doors)
    if scene == 208:
        assert value['pendingOriginalEvents'] == [3537] and value['inventory']['酒']-value['inventoryInitial'].get('酒', 0) == 4
    log = VERIFY / f'dali-rewards-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    source_path = SOURCE / ('regional_before_boot_review.go' if scene == 210 else 'regional_hidden_rewards_e2e_test.go')
    source_info = digest(source_path) if scene == 210 else digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go')
    source_info['path'] = str(source_path.relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'newFreshRuntimeSaves': proof['Saves'], 'newOrdinaryBattles': 0, 'testSource': source_info, 'report': digest(path)})

pending = review['pendingBoots']
assert pending['event'] == 3537 and not pending['readyForProgress'] and not pending['actuallyObtained'] and pending['independentClassificationReferenceMissing']
assert [row['label'] for row in pending['nativeStages']] == ['before', 'attempt', 'after'] and all(row['nativeEvent'][6] == 1 for row in pending['nativeStages'])
kirin = review['kirin']
assert kirin['event'] == 3354 and kirin['reusedExistingStoryEvidence'] and kirin['newGameplayRuns'] == 0
assert kirin['existingFreshRuntimeSaves'] == 103 and kirin['existingOrdinaryBattles'] == 43
assert (kirin['battlePC'], kirin['battleFormation'], kirin['fireOrbGrantPC'], kirin['hornGrantPC']) == (29972, 224, 29976, 30093)
assert new_points[3354]['status'] == 'original-kirin-reward-chain-and-saved-no-duplicate-verified' and new_points[3354]['grants'] == catalog[3354]['Grants'] and new_points[3354]['followupRewards'] == [{'PC': 30093, 'Item': 276, 'Amount': 1, 'Cash': 0, 'Name': '火眼麒麟角'}]
main_path = ROOT / kirin['report']['path']
main = load(main_path)
assert main['passed'] and not main['resumedTestRoute'] and main['checks']['RepeatInputs'] == 3 and main['checks']['RewardOrder'] == ['获得火灵珠', '获得火眼麒麟角']
assert main['proof']['Battles'] == main['proof']['Wins'] == 43 and main['proof']['ScriptedFinishes'] == 0
assert main['proof']['Saves'] == len(main['proof']['Saved']) == 103 and all(main['proof']['Saved'].values())
assert main['checks']['InventoryBefore'] == {'风灵珠': 1} and main['checks']['InventoryAfter'] == {'火灵珠': 1, '火眼麒麟角': 1, '风灵珠': 1}
for label in main['proof']['Saved']:
    artifacts.extend(BASE / f'dali-kirin-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    check_path = BASE / f'dali-kirin-{label}-checks-1.json'
    if check_path.is_file():
        artifacts.append(check_path)
stages = {row['label']: row for row in kirin['nativeStages']}
assert stages['sacred-reward-2']['roles'] == [0] and all(stages[f'ended-kirin-confirm-{n}']['roles'] == [4, 0] and stages[f'ended-kirin-confirm-{n}']['nativeEvent'][6] == 0 for n in [1, 2, 3])
for filename in ['dali-rewards-lint-run.txt', 'dali-rewards-lint-negative-run.txt', 'dali-rewards-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['tests/regional_hidden_rewards_e2e_test.go', 'tests/dali-rewards-evidence-e2e.py', 'tests/dali-rewards-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'DALI_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
assert sum(row['newFreshRuntimeSaves'] for row in cases) == 163 and len({row['testSource']['sha256'] for row in cases}) == 2
summary = {'passed': True, 'scope': '37 new actual Dali hidden pickups through 15 original door transitions, separately revalidated existing complete Kirin sacred reward chain, boots 3537 kept pending unchanged; no original edits, uploads or new Kirin gameplay, not all hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 38, 'newActuallyClaimed': 38, 'newOriginalUnreachablePreserved': 0, 'pendingEventsReviewed': [3537], 'cases': cases, 'newFreshRuntimeSaves': 163, 'newOrdinaryBattles': 0, 'revalidatedExistingStoryEvidence': {'event': 3354, 'freshRuntimeSaves': 103, 'ordinaryBattles': 43, 'report': digest(main_path)}, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 2279, 'priorPeachRewardsArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'dali-rewards-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': 163, 'revalidatedExistingStorySaves': 103, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
