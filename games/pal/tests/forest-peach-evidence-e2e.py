import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'forest-peach-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 381 and prior['total'] == 734
assert progress['passed'] and progress['processed'] == 395 and progress['total'] == 735
assert progress['actuallyClaimed'] == 376 and progress['originalUnreachablePreserved'] == 19 and progress['remaining'] == 340
EXPECTED = {'187': [3150, 3136, 3137, 3147, 3148, 3149, 3146], '193-pre-lift': [3313, 3314, 3316, 3317], '193-post-lift': [3315]}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {3318, 2980}
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'][:len(prior['catalogSupplements'])] == prior['catalogSupplements']
assert len(progress['catalogSupplements']) == len(prior['catalogSupplements'])+1
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'sacred-rewards-stage.tar.gz')['sha256'] == '4e044e3479aef1ac393c05d3a86385f6efc17961a36f23ffc2a55f1888079cfd'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json', VERIFY / 'hidden-touch-candidates.json', SOURCE / 'regional_before_lift_source.go']
cases = []


def source_check(value):
    root = roots['roots'][value['sourceCheckpoint']]
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == root['sha256'] == digest(source_path)['sha256']
    assert root['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    artifacts.extend([source_path, parent_path])


def saved_artifacts(value, prefix):
    proof = value['proof']
    assert proof['Saves'] == len(proof['Saved']) and all(proof['Saved'].values())
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])


for suffix, events in EXPECTED.items():
    scene = int(suffix.split('-')[0])
    prefix = 'regional-hidden-v13-'+suffix
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 13 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events and value['proof']['Saves'] >= len(events)*4
    assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0
    assert value['combatAllowed'] == (scene == 193)
    assert [row['role'] for row in value['partyInitial']] == [row['role'] for row in value['party']] == [0]
    assert all(row['controller'] == 0 for row in value['party'])
    source_check(value)
    battles = value['battleCashChanges']
    assert len(battles) == value['proof']['Battles']-value['preCollectionBattles'] and all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in battles)
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert reward['invisible'] == (point['Sprite'] == 0) and reward['originalHerbDisappearance'] == (point['Sprite'] in [68, 69, 70])
        assert reward['searchMode'] == point['TriggerMode']
        assert all(value['proof']['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in battles for battle in reward['battleCashDuringPickup'])
        assert reward['cashAfter']-reward['cashBefore'] == sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
    saved_artifacts(value, prefix)
    log = VERIFY / f'forest-peach-{suffix}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    if suffix == '187':
        code_info = digest(SOURCE / 'regional_before_lift_source.go')
    else:
        code_info = digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go')
        code_info['path'] = str((SOURCE / 'regional_hidden_rewards_e2e_test.go').relative_to(ROOT))
    cases.append({'kind': 'forest-pickups', 'branch': suffix, 'events': events, 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'testSource': code_info, 'report': digest(path)})

unreachable_path = VERIFY / 'forest-original-unreachable.json'
unreachable = load(unreachable_path)
assert unreachable['passed'] and unreachable['readyForProgress'] and not unreachable['nativeEdits'] and unreachable['nativeBuildID'] == core
assert len(unreachable['records']) == 1 and unreachable['records'][0]['event'] == 3318
record = unreachable['records'][0]
assert record['classification'] == 'original-unreachable-preserved' and not record['normalPickupVerified'] and not record['actuallyObtained']
assert record['nativeRecordPreservedBefore'] and record['nativeRecordPreservedAfter'] and not record['reachableSearchStances']
assert record['allSceneTransitions'] == 2 and record['independentReferenceQuote'] == '7.灵山仙芝（拿不到）' and record['otherSameItemEventActuallyClaimed'] == 3314
assert len(unreachable['graph']['climbs']) == len(unreachable['persistentClimbNativeChecks']) == 52 and len(unreachable['graph']['lifts']) == 2
assert unreachable['graph']['optimisticWalkablePositions'] == 5785 and unreachable['graph']['retainsOriginalMandatoryClimbTriggers']
assert new_points[3318]['status'] == 'original-unreachable-preserved'
artifacts.extend([unreachable_path, VERIFY / 'forest-unreachable-review-run.txt'])

prefix = 'peach-quest-v1'
path = BASE / f'{prefix}-result-1.json'
value = load(path)
assert value['passed'] and not value['resumedTestRoute'] and not value['nativeEdits'] and value['resourceUploads'] == 0
assert value['kind'] == 'original-peach-village-quest' and value['event'] == 2980 and value['bossBattles'] == 1 and value['forceInputs'] > 0
assert value['returnAttempts'] == 3 and value['temporarySoloSaved'] and value['scene'] == 187
assert [row['role'] for row in value['partyInitial']] == [0, 4] and [row['role'] for row in value['partyFinal']] == [4, 0]
assert all(row['controller'] == 0 for row in value['partyInitial']+value['partyFinal'])
assert value['inventoryChecks']['initial'] == value['inventoryChecks']['wrong-wind-orb-retained'] == value['inventoryChecks']['wind-orb-retained'] == {'风灵珠': 1, '金罡珠': 0, '寿葫芦': 0}
assert value['inventoryChecks']['gifts-received'] == value['finalInventory'] == {'风灵珠': 1, '金罡珠': 1, '寿葫芦': 1}
assert all(value['inventoryChecks'][f'rescue-repeat-{n}'] == value['finalInventory'] for n in [1, 2, 3])
assert all(value['signs'][key] for key in ['fog-cleared', 'mission', 'song-story', 'metal-orb', 'village-gift', 'gourd-accepted', 'tree-reveal'])
assert value['proof']['Battles'] == value['proof']['Wins'] == len(value['battleCashChanges']) and value['proof']['ScriptedFinishes'] == 0
assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in value['battleCashChanges'])
assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges'])
for label in ['wind-orb-selected-in-fog', 'wooden-taoist-before', 'wooden-taoist-battle-1', 'village-solo-in-progress', 'gourd-return-solo', 'anu-rejoined', 'rescue-repeat-1', 'rescue-repeat-2', 'rescue-repeat-3']:
    assert value['proof']['Saved'][label]
source_check(value)
saved_artifacts(value, prefix)
assert len(value['fixtureCheckpoints']) == 4
for name in value['fixtureCheckpoints']:
    root = roots['roots'][name]
    fixture_path = BASE / (name[:-5]+'-1.json')
    assert root['sha256'] == digest(fixture_path)['sha256'] and root['report'] == 'peach-quest-v1-result.json' and root['reportSHA256'] == digest(path)['sha256']
    assert load(fixture_path)['public']['phase'] == 'field'
    artifacts.append(fixture_path)
log = VERIFY / 'peach-quest-final-run.txt'
assert '--- PASS: TestOriginalPeachVillageWindWoodenTaoistLongevityGourdAndSavedReturn' in log.read_text() and '\nFAIL' not in log.read_text()
artifacts.extend([path, BASE / f'{prefix}-result-state-1.json', log])
quest_source = digest(ROOT / 'games/pal/tests/peach_quest_e2e_test.go')
quest_source['path'] = str((SOURCE / 'peach_quest_e2e_test.go').relative_to(ROOT))
cases.append({'kind': 'peach-quest', 'events': [2980], 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'forceInputs': value['forceInputs'], 'testSource': quest_source, 'report': digest(path)})
touch_path = VERIFY / 'hidden-touch-candidates.json'
touch = load(touch_path)
assert touch['passed'] and touch['additionalCandidates'] == 1 and touch['combinedCandidates'] == 735 and touch['conditionalCatalogUnchanged']
assert touch['candidates'][0]['Event'] == 2980 and [row['Item'] for row in touch['candidates'][0]['Grants']] == [261, 269]
artifacts.append(VERIFY / 'forest-peach-touch-catalog-run.txt')
review_path = VERIFY / 'forest-peach-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and len(review['records']) == 13
assert {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), [])) | {2980}
artifacts.extend([review_path, VERIFY / 'forest-peach-original-review-run.txt'])
for filename in ['forest-peach-lint-run.txt', 'forest-peach-lint-negative-run.txt', 'forest-peach-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
artifacts.append(ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md')
for filename in ['tests/peach_quest_e2e_test.go', 'tests/regional_hidden_rewards_e2e_test.go', 'tests/forest-peach-evidence-e2e.py', 'tests/forest-peach-original-review.py', 'tests/forest-unreachable-review.py', 'tests/hidden-touch-catalog-e2e.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'FOREST_PEACH_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
summary = {'passed': True, 'scope': '12 actual original forest pickups across legal pre/post lift roots, separately source/map/native confirmed unreachable fungus, complete original Peach optional touch quest with wrong/correct wind orb, original boss force/mana and victory, temporary solo village and Anu reunion, closed entrance no repeated gifts and normally earned follow-up fixtures; no native edits or uploads, not all Peach hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 14, 'newActuallyClaimed': 13, 'newOriginalUnreachablePreserved': 1, 'additionalTouchCandidates': 1, 'cases': cases, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'priorSacredRewardsArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'forest-peach-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
