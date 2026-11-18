import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'sacred-rewards-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 338 and prior['total'] == 734
assert progress['passed'] and progress['processed'] == 381 and progress['total'] == 734
assert progress['actuallyClaimed'] == 363 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 353
EXPECTED = {175: [2880, 2881, 2882, 2883, 2884, 2885, 2886, 2887], 177: [2918, 2919, 2920, 2921, 2922, 2923, 2924, 2932, 2925, 2926, 2927, 2928, 2929, 2930], 180: [2958, 2959], 185: [3036, 3037, 3038, 3039, 3040, 3041, 3042, 3043, 3031, 3030, 3028, 3029, 3032, 3033, 3034, 3035], 189: [3158, 3159, 3160]}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), []))
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'capital-quests-stage.tar.gz')['sha256'] == '29ec442a2204f82aa816ccab7704505f2ad4964b84c204eff3c43aaa0b2ccce1'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json', SOURCE / 'regional_before_vessel_followup.go', SOURCE / 'regional_before_herb_dialogue.go']
cases = []
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v12-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 12 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == [] and value['scholarOffer'] == ''
    assert [row['event'] for row in value['claimed']] == events
    assert value['proof']['Saves'] >= len(events)*4 and value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0
    assert value['combatAllowed'] == (scene in [180, 185, 189])
    assert [(row['role'], row['controller']) for row in value['partyInitial']] == [(row['role'], row['controller']) for row in value['party']]
    if scene in [175, 177]:
        assert [row['role'] for row in value['party']] == [0] and value['partyInitial'] == value['party'] and value['proof']['Battles'] == 0
    if scene == 185:
        assert [row['role'] for row in value['party']] == [0, 4]
    if scene == 189:
        assert [row['role'] for row in value['party']] == [4, 0]
    root = roots['roots'][value['sourceCheckpoint']]
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == root['sha256'] == digest(source_path)['sha256']
    assert root['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    battle_cash = sum(row['rewardCash'] for row in value['battleCashChanges'])
    reward_cash = sum(g['Cash'] for row in value['claimed'] for g in row['grants'])
    assert value['cashFinal']-value['cashInitial'] == reward_cash+battle_cash
    assert len(value['battleCashChanges']) == value['proof']['Battles']-value['preCollectionBattles']
    assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in value['battleCashChanges'])
    for row in value['claimed']:
        event, point = row['event'], catalog[row['event']]
        assert row['grants'] == point['Grants'] and row['notification'] and row['repeatSearches'] >= 3 and row['repeatedFromSavedPose']
        assert row['invisible'] == (point['Sprite'] == 0) and row['originalHerbDisappearance'] == (point['Sprite'] in [68, 69, 70])
        assert row['searchMode'] == point['TriggerMode'] and all(value['proof']['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in value['battleCashChanges'] for battle in row['battleCashDuringPickup'])
        assert row['cashAfter']-row['cashBefore'] == sum(g['Cash'] for g in row['grants'])+sum(b['rewardCash'] for b in row['battleCashDuringPickup'])
        if point['Sprite'] in [68, 69, 70]:
            assert row['disappearsAfterPickup']
        if point['Sprite'] == 10:
            assert row['closedFrame'] != row['openFrame'] == 1 and not row['disappearsAfterPickup']
        if event == 2886:
            follow = row['followup']
            assert (follow['PC'], follow['Item'], follow['Amount'], follow['afterRepeat']) == (28168, 268, 1, 1)
            assert follow['after']-follow['before'] == 1 and follow['noticeSeen'] and follow['noDuplicateSearches'] >= 3
            assert value['proof']['Saved']['reward-2886-vessel-notice'] and value['proof']['Saved']['reward-2886-vessel-collected']
            assert new_points[event]['followupRewards'] == follow
        if event == 2922:
            assert row['originalRenewableHarvest'] and row['inventoryAfter']-row['inventoryBefore'] == 4 and len(row['freshSearches']) == 3
            assert all(step['after']-step['before'] == 1 for step in row['freshSearches'])
            assert all(value['proof']['Saved'][f'reward-2922-renewable-search-{n}'] for n in [1, 2, 3])
            assert new_points[event]['status'] == 'renewable-harvest-and-saved-no-replay-verified'
        if event == 2932:
            assert row['originalBananaTreeFrame'] == 1 and not row['disappearsAfterPickup']
    if scene == 175:
        assert value['inventory']['炼蛊皿'] >= 2
    if scene == 180:
        assert value['proof']['Saved']['mountain-climb-in-progress']
    if scene == 185:
        assert reward_cash == 20000 and len([row for row in value['claimed'] if row['event'] in [3041, 3042]]) == 2
    for label, saved in value['proof']['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    log = VERIFY / f'sacred-rewards-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, source_path, parent_path, BASE / f'{prefix}-state-1.json', log])
    source_code = SOURCE / ('regional_before_vessel_followup.go' if scene in [185, 189] else 'regional_before_herb_dialogue.go')
    if scene == 177:
        source_code = ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go'
    source_info = digest(source_code)
    if scene == 177:
        source_info['path'] = str((SOURCE / source_code.name).relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'hiddenCash': reward_cash, 'battleCash': battle_cash, 'partyRoles': [row['role'] for row in value['party']], 'testSource': source_info, 'report': digest(path)})
review_path = VERIFY / 'sacred-rewards-original-review.json'
review = load(review_path)
assert review['passed'] and review['nativeBuildID'] == core and not review['nativeEdits'] and len(review['records']) == 43
assert {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
artifacts.extend([review_path, VERIFY / 'sacred-rewards-original-review-run.txt'])
for filename in ['sacred-rewards-lint-run.txt', 'sacred-rewards-lint-negative-run.txt', 'sacred-rewards-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
artifacts.append(ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md')
for filename in ['tests/regional_hidden_rewards_e2e_test.go', 'tests/sacred-rewards-evidence-e2e.py', 'tests/sacred-rewards-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'SACRED_TREE_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
summary = {'passed': True, 'scope': '43 original event points in sacred house, surrounding herbs and banana tree, mountain top and treasure tree cave; staged extra vessel, original renewable fruit, actual stairs, independent money chests, fresh-runner saves, per-pickup/battle wallets and normal party ownership; no native edits or uploads, not all climbing regions, Peach quest, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 43, 'newActuallyClaimed': 43, 'newOriginalUnreachablePreserved': 0, 'cases': cases, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'priorCapitalQuestsArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'sacred-rewards-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
