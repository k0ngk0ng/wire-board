import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'peach-rewards-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 395 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 438 and progress['total'] == 735
assert progress['remaining'] == 297 and progress['actuallyClaimed'] == 417 and progress['originalUnreachablePreserved'] == 21
EXPECTED = {176: [2915], 181: [2974, 2973, 2975, 2976, 2977], 190: [3188, 3189, 3191, 3192, 3193, 3194, 3195], 191: [3211, 3215, 3217, 3214, 3216, 3213], 192: list(range(3223, 3245))}
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(sum(EXPECTED.values(), [])) | {3190, 3212}
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 1784
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'forest-peach-stage.tar.gz')['sha256'] == '3503a7196db2e76e938897eacf514061e9fc3a136695625979d978d3af94c1f4'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json', VERIFY / 'hidden-search-candidates.json', VERIFY / 'hidden-conditional-candidates.json', VERIFY / 'hidden-touch-candidates.json', SOURCE / 'regional_before_house_route.go', SOURCE / 'regional_before_battle_items.go', SOURCE / 'original-gated-village-state.json']
cases = []


def checked_artifact(info):
    path = ROOT / info['path']
    assert path.resolve().is_relative_to(ROOT) and path.is_file() and not path.is_symlink()
    assert digest(path)['sha256'] == info['sha256'], info['path']
    artifacts.append(path)


def checked_references(value):
    if isinstance(value, dict):
        if 'path' in value and 'sha256' in value:
            checked_artifact(value)
        for child in value.values():
            checked_references(child)
    elif isinstance(value, list):
        for child in value:
            checked_references(child)


def all_text(view):
    chars = []
    if view.get('raster'):
        raster, seen = view['raster'], set()
        def visit(index):
            if index <= 0 or index > len(raster['nodes']) or index in seen:
                return
            seen.add(index)
            node = raster['nodes'][index-1]
            visit(node[1])
            visit(node[2])
            if node[0] == 2:
                chars.append(chr(node[5]))
        visit(raster['root'])
    else:
        chars.extend(chr(op['id']) for op in view.get('overlay') or [] if op['kind'] == 0)
    return ''.join(chars)+(view.get('dialogue') or {}).get('text', '')


review_path = VERIFY / 'peach-rewards-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core
assert review['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert len(review['records']) == 41 and {row['event'] for row in review['records']} == set(sum(EXPECTED.values(), []))
checked_references(review)
artifacts.extend([review_path, VERIFY / 'peach-rewards-original-review-run.txt'])
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v14-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 14 and value['scene'] == scene and value['authorityFixture']
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert [row['event'] for row in value['claimed']] == events
    proof = value['proof']
    assert proof['Saves'] == len(proof['Saved']) >= len(events)*4 and all(proof['Saved'].values())
    assert proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == 0
    assert value['combatAllowed'] == (scene in [181, 190, 191]) and value['preCollectionBattles'] == 0
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']] == [(0, 0), (4, 0)]
    if not value['combatAllowed']:
        assert value['partyInitial'] == value['party'] and proof['Battles'] == 0
    root = roots['roots'][value['sourceCheckpoint']]
    root_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceCheckpoint'] == f'peach-quest-v1-field-{191 if scene == 192 else scene}-state.json'
    assert value['sourceSHA256'] == root['sha256'] == digest(root_path)['sha256']
    assert root['report'] == 'peach-quest-v1-result.json' and root['reportSHA256'] == digest(parent_path)['sha256']
    assert load(parent_path)['passed'] and not load(parent_path)['resumedTestRoute'] and load(root_path)['public']['phase'] == 'field'
    artifacts.extend([root_path, parent_path])
    battles = value['battleCashChanges']
    assert len(battles) == proof['Battles'] and all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
    cash = sum(grant['Cash'] for row in value['claimed'] for grant in row['grants'])
    assert value['cashFinal']-value['cashInitial'] == cash+sum(row['rewardCash'] for row in battles)
    for reward in value['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['position'] == [point['X'], point['Y']] and reward['scene'] == point['Scene'] == scene
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        assert len(reward['repeatPose']) == 3 and 0 <= reward['repeatPose'][2] < 4
        assert reward['invisible'] == (point['Sprite'] == 0) and not reward['originalHerbDisappearance'] and not reward['disappearsAfterPickup']
        assert reward['searchMode'] == point['TriggerMode']
        assert (reward['closedFrame'], reward['openFrame']) == ((0, 1) if point['Sprite'] == 10 else (-1, -1))
        assert all(proof['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        assert all(battle in battles for battle in reward['battleCashDuringPickup'])
        assert reward['cashAfter']-reward['cashBefore'] == sum(grant['Cash'] for grant in reward['grants'])+sum(row['rewardCash'] for row in reward['battleCashDuringPickup'])
        native = next(row for row in review['records'] if row['event'] == event)
        assert native['result']['sha256'] == digest(path)['sha256']
        assert [stage['label'] for stage in native['stages']] == ['unpicked', 'notice', 'collected', 'repeated']
        assert new_points[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    if scene == 181:
        grants = value['battleItemGrants']
        assert grants == [{"amount": 1, "battle": battle, "endScript": 41008, "enemy": 402, "item": 104, "name": "鼠儿果"} for battle in [10, 16, 17]]
        expected_inventory = dict(value['inventoryInitial'])
        for reward in value['claimed']:
            for grant in reward['grants']:
                if grant['Item'] > 0:
                    expected_inventory[grant['Name']] = expected_inventory.get(grant['Name'], 0)+grant['Amount']
        for grant in grants:
            expected_inventory[grant['name']] = expected_inventory.get(grant['name'], 0)+grant['amount']
            battle = grant['battle']
            assert proof['Saved'][f'battle-{battle}'] and proof['Saved'][f'battle-{battle}-fruit-drop']
            start = load(BASE / f'{prefix}-battle-{battle}-state-1.json')['public']
            notice = load(BASE / f'{prefix}-battle-{battle}-fruit-drop-state-1.json')['public']
            assert sum(enemy['object'] == 402 for enemy in start['battle']['enemies']) == grant['amount']
            assert notice['phase'] == 'battle' and notice['battle']['phase'] == 1 and '捡到一颗鼠儿果' in all_text(notice)
        assert expected_inventory == value['inventory'] and value['inventory']['鼠儿果'] == 5
        assert [{key: row[key] for key in grants[0]} for row in review['battleItemGrants']] == grants
        assert all(row['nativePC'] == 41010 for row in review['battleItemGrants'])
        assert all(proof['Saved'][f'peach-door-{event}-arrived'] for event in [2971, 3162])
    if scene == 192:
        assert len(review['originalHouseDoorSequence']) == 10
        assert [row['event'] for row in review['originalHouseDoorSequence']] == [3197, 2890, 3220, 2892, 3222, 2889, 3219, 2891, 3221, 2888]
        assert all(proof['Saved'][f"peach-door-{row['event']}-arrived"] for row in review['originalHouseDoorSequence'])
    log = VERIFY / f'peach-rewards-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    code_path = SOURCE / ('regional_before_house_route.go' if scene in [176, 190, 191] else 'regional_before_battle_items.go' if scene == 192 else 'regional_hidden_rewards_e2e_test.go')
    code_info = digest(ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go') if scene == 181 else digest(code_path)
    code_info['path'] = str(code_path.relative_to(ROOT))
    cases.append({'scene': scene, 'events': events, 'freshRuntimeSaves': proof['Saves'], 'ordinaryBattles': proof['Battles'], 'testSource': code_info, 'report': digest(path)})

assert review['current98VersionContrast']['eventsActuallyObtained'] == [3211, 3215]
assert review['current98VersionContrast']['referenceQuote'] == '傀儡虫（右边的拿不到）'
assert [row['activationPC'] for row in review['originalBlockedHouseDoor']] == [26840, 26841]
assert all(row['nativeEvent'][6] == 2 and row['nativeEvent'][8] == 523 for row in review['originalBlockedHouseDoor'])
assert digest(SOURCE / 'original-gated-village-state.json')['sha256'] == '6b3a8be9b2449c47606de3f7c12eb719d4c38ca80e4a87e169ffc9ae66b36788'
unreachable_path = VERIFY / 'peach-original-unreachable.json'
unreachable = load(unreachable_path)
assert unreachable['passed'] and unreachable['readyForProgress'] and not unreachable['nativeEdits'] and unreachable['nativeBuildID'] == core
assert unreachable['sssSHA256'] == review['sssSHA256'] and unreachable['reference'] == review['reference']
assert len(unreachable['records']) == 2 and [row['event'] for row in unreachable['records']] == [3190, 3212]
assert [(row['scene'], row['optimisticWalkablePositions']) for row in unreachable['graphs']] == [(190, 3571), (191, 2395)]
for row in unreachable['records']:
    event = row['event']
    assert row['classification'] == new_points[event]['status'] == 'original-unreachable-preserved'
    assert not row['normalPickupVerified'] and not row['actuallyObtained'] and not row['reachableSearchStances']
    assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and row['nativeEventBefore'][1:13] == row['nativeEventAfter'][1:13]
    assert row['allSceneTransitions'] == 2 and row['referenceQuote'] == ('2.女娲石（拿不到）' if event == 3190 else '2.蜂王蜜（拿不到）')
for graph in unreachable['graphs']:
    assert graph['ignoresAllDynamicCollisions'] and graph['evenAllowsWalkingAcrossSceneExitAreas'] and len(graph['entrances']) == len(graph['exits']) == 2
checked_references(unreachable)
artifacts.extend([unreachable_path, VERIFY / 'peach-unreachable-review-run.txt', VERIFY / 'peach-rewards-evidence-red-run.txt', VERIFY / 'peach-rewards-evidence-pre-aggregate-red-run.txt', VERIFY / 'peach-rewards-progress-run.txt', VERIFY / 'forest-peach-stage-archive.json', VERIFY / 'forest-peach-archive-verification.json'])
for filename in ['peach-rewards-lint-run.txt', 'peach-rewards-lint-negative-run.txt', 'peach-rewards-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['tests/regional_hidden_rewards_e2e_test.go', 'tests/peach-rewards-evidence-e2e.py', 'tests/peach-rewards-original-review.py', 'tests/peach-unreachable-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'PEACH_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
assert sum(row['freshRuntimeSaves'] for row in cases) == 224 and sum(row['ordinaryBattles'] for row in cases) == 45
assert len({row['testSource']['sha256'] for row in cases}) == 3
summary = {'passed': True, 'scope': '41 actual original Peach forest/village pickups, 2 separately source/map/native confirmed original unreachable records, five original room doors after normal story barrier closure, two independent current-98 worm chests despite guide claim and three normal enemy-end-script mouse-fruit grants reconciled separately; no original edits or uploads, not all hidden rewards, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 43, 'newActuallyClaimed': 41, 'newOriginalUnreachablePreserved': 2, 'cases': cases, 'freshRuntimeSaves': 224, 'ordinaryBattles': 45, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'priorForestPeachArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'peach-rewards-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': 224, 'ordinaryBattles': 45, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
