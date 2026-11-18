import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'trial-bottom-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 646 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 652 and progress['total'] == 735 and progress['remaining'] == 83
assert progress['actuallyClaimed'] == 630 and progress['originalUnreachablePreserved'] == 22
expected = {3607, 3641, 3642, 3643, 3644, 3645}
old = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new)-set(old) == expected and all(new[event] == value for event, value in old.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 5894 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'trial-deep-stage.tar.gz')['sha256'] == 'a6ae9ab0969bb941eb9478ef9d226eab0c187ab3c4be87feb639fcc0cb7f3fe7'
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
review_path = VERIFY / 'trial-bottom-original-review.json'
review = load(review_path)
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core and len(review['records']) == 12
assert {row['event'] for row in review['records']} == expected and [row['branch'] for row in review['branches']] == ['two', 'three']
plan_path = VERIFY / 'trial-bottom-plan.json'
plan = load(plan_path)
assert plan['passed'] and plan['diagnosticOnly'] and len(plan['branches']) == 2
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json', 'regional_hidden_rewards_e2e_test.go', 'trial_rewards_route_e2e_test.go']]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'trial-bottom-evidence-red-run.txt', 'trial-bottom-evidence-pre-aggregate-red-run.txt', 'trial-bottom-progress-run.txt', 'trial-bottom-plan-run.txt', 'trial-bottom-original-review-run.txt', 'trial-deep-stage-archive.json', 'trial-deep-archive-verification.json'])
artifacts.extend([plan_path, review_path])


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


checked_references(review)
cases = []
for branch, roles, fire_role in [('two', [0, 4], 4), ('three', [1, 0, 4], 1)]:
    prefix = f'trial-bottom-v1-{branch}'
    path = BASE / f'{prefix}-result-1.json'
    result = load(path)
    proof = result['proof']
    assert result['passed'] and not result['nativeEdits'] and result['nativeBuildID'] == core and result['branch'] == branch and result['roles'] == roles and result['fireRole'] == fire_role
    assert proof['Saves'] == len(proof['Saved']) and all(proof['Saved'].values()) and proof['Battles'] == proof['Wins'] and proof['ScriptedFinishes'] == 0
    registration = roots['roots'][result['sourceCheckpoint']]
    source_path = BASE / (result['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (registration['report'][:-5]+'-1.json')
    assert registration['sha256'] == result['sourceSHA256'] == digest(source_path)['sha256'] and registration['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed']
    route = next(row for row in plan['branches'] if row['branch'] == branch)
    assert route['sourceCheckpoint'] == result['sourceCheckpoint'] and route['sourceSHA256'] == result['sourceSHA256'] and len(result['route']) == len(route['legs'])
    assert [row['event'] for row in result['boatTrips']] == [3607, 3614, 3630, 3631, 3632, 3634, 3635, 3635] and result['boatTrips'][-1]['destination'] == [1104, 1176]
    assert result['boatSaves'] == len(result['boatTrips']) == 8
    assert {row['event'] for row in result['claimed']} == expected
    inventory = dict(result['inventoryInitial'])
    for row in result['claimed']:
        point = catalog[row['event']]
        assert row['scene'] == point['Scene'] == 214 and row['position'] == [point['X'], point['Y']] and row['grants'] == point['Grants'] and row['notification'] and row['repeatSearches'] == 3
        assert row['before'].get(point['Grants'][0]['Name'], 0)+1 == row['after'][point['Grants'][0]['Name']]
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            assert proof['Saved'][f"reward-{row['event']}-{label}"]
        for grant in row['grants']:
            inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
    inventory.pop('芦苇漂', None)
    assert result['inventory'].get('芦苇漂', 0) == 0
    for row in result['battleItemGrants']:
        assert row['endScript'] == 42828 and row['item'] == 152 and row['name'] == '傀儡虫' and row['amount'] > 0 and proof['Saved'][f"battle-{row['battle']}-worm-drop"]
        inventory['傀儡虫'] = inventory.get('傀儡虫', 0)+row['amount']
    assert inventory == result['inventory'] and result['inventory']['土灵珠'] == result['inventoryInitial']['土灵珠'] == 1
    assert len(result['battleCashChanges']) == proof['Battles'] and result['cashFinal']-result['cashInitial'] == sum(row['rewardCash'] for row in result['battleCashChanges'])
    assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in result['battleCashChanges'])
    native = next(row for row in review['branches'] if row['branch'] == branch)
    assert native['roles'] == roles and native['fireRole'] == fire_role and len(native['gods']) == 5 and len(native['transits']) == 8
    assert len(native['priorClaimedChestStatesRetained']) == (17 if branch == 'two' else 4)
    for god in native['gods']:
        assert god['nativeLearnedExactlyOnce'] and new.get(god['event']) is None
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    log = VERIFY / f'trial-bottom-{branch}-run.txt'
    assert '--- PASS: TestOriginalTrialBottomReedBoatsEquipmentFiveGodsAndSave' in log.read_text() and '\nFAIL' not in log.read_text()
    execution_source = SOURCE / f'{branch}_execution.go'
    assert digest(execution_source)['sha256'] == digest(ROOT / 'games/pal/tests/trial_bottom_e2e_test.go')['sha256']
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', source_path, parent_path, log, execution_source])
    cases.append({'branch': branch, 'roles': roles, 'fireRole': fire_role, 'newFreshRuntimeSaves': proof['Saves'], 'newOrdinaryBattles': proof['Battles'], 'normalWormDrops': sum(row['amount'] for row in result['battleItemGrants']), 'boatRides': len(result['boatTrips']), 'report': digest(path), 'executionSource': digest(execution_source)})
for filename in ['trial-bottom-lint-run.txt', 'trial-bottom-lint-negative-run.txt', 'trial-bottom-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['tests/trial_bottom_e2e_test.go', 'tests/trial-bottom-plan.py', 'tests/trial-bottom-original-review.py', 'tests/trial-bottom-evidence-e2e.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'TRIAL_BOTTOM_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
authority = digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256']
assert authority == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508' and digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core
summary = {'passed': True, 'scope': 'six new original bottom item points, actual blocked and successful reed use, sixteen original boat rides, five god statues in two genuine party branches with distinct fire recipients; independent native events and role magic tables, saves and repeat searches; not all hidden rewards, performance or release', 'nativeBuildID': core, 'authorityArchiveSHA256': authority, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 6, 'newActuallyClaimed': 6, 'newOriginalUnreachablePreserved': 0, 'cases': cases, 'newFreshRuntimeSaves': sum(row['newFreshRuntimeSaves'] for row in cases), 'newOrdinaryBattles': sum(row['newOrdinaryBattles'] for row in cases), 'normalWormDrops': sum(row['normalWormDrops'] for row in cases), 'originalBoatRides': 16, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 5894, 'priorDeepTrialArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'trial-bottom-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'newFreshRuntimeSaves': summary['newFreshRuntimeSaves'], 'newOrdinaryBattles': summary['newOrdinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
