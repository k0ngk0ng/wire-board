import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SOURCE = VERIFY / 'capital-side-hidden-source'
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
EXPECTED = {101: 1818, 109: 2035, 138: 2336}
ECONOMY = {2003: 'capital-funding-v1', 2086: 'capital-merchant-v1-sugar', 2225: 'capital-merchant-v1-wine'}


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 330
assert progress['passed'] and progress['processed'] == 336 and progress['total'] == 732
assert progress['actuallyClaimed'] == 318 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 396
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == set(EXPECTED.values()) | set(ECONOMY)
assert all(new_points[event] == row for event, row in old_points.items())
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
archive_path = VERIFY / 'toad-hidden-stage.tar.gz'
assert digest(archive_path)['sha256'] == '175228a1b20ab56d811278415fb03bb1922511b4e84cd600d78fe23e51af9d92'
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json']
cases = []


def accepted_report(prefix):
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert value['proof']['Battles'] == value['proof']['Wins'] == value['proof']['ScriptedFinishes'] == 0
    root = roots['roots'][value['sourceCheckpoint']]
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == root['sha256'] == digest(source_path)['sha256']
    assert root['reportSHA256'] == digest(parent_path)['sha256']
    assert load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    artifacts.extend([path, source_path, parent_path])
    for label, saved in value['proof']['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    return value, path


for scene, event in EXPECTED.items():
    prefix = f'regional-hidden-v11-{scene}'
    value, path = accepted_report(prefix)
    assert value['verificationVersion'] == 11 and value['scene'] == scene and value['authorityFixture']
    assert not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert value['partyInitial'] == value['party'] and value['cashInitial'] == value['cashFinal']
    assert not value['combatAllowed'] and value['battleCashChanges'] == [] and len(value['claimed']) == 1
    reward = value['claimed'][0]
    assert reward['event'] == event and reward['grants'] == catalog[event]['Grants']
    assert reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
    assert reward['searchMode'] == catalog[event]['TriggerMode'] and reward['cashBefore'] == reward['cashAfter']
    assert value['proof']['Saves'] >= 4 and all(value['proof']['Saved'][f'reward-{event}-{label}'] for label in ['unpicked', 'notice', 'collected', 'repeated'])
    if event == 1818:
        assert reward['originalLetterRequest'] and reward['decisions'] == ['是'] and not reward['disappearsAfterPickup']
    elif event == 2035:
        assert reward['originalGroundVialDisappearance'] and reward['disappearsAfterPickup']
    else:
        assert reward['invisible'] and not reward['disappearsAfterPickup']
    log = VERIFY / f'regional-capital-side-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text()
    artifacts.extend([BASE / f'{prefix}-state-1.json', log])
    cases.append({'event': event, 'kind': 'once-only-pickup', 'freshRuntimeSaves': value['proof']['Saves'], 'report': digest(path)})

for event, prefix in ECONOMY.items():
    value, path = accepted_report(prefix)
    assert value['verificationVersion'] == 1 and value['event'] == event and not value['nativeEdits'] and value['resourceUploads'] == 0
    if event == 2003:
        assert value['kind'] == 'original-capped-funding' and value['grants'] == 3 and value['grantAmount'] == 30000
        assert value['shopItem'] == 183 and value['shopPrice'] == 12000 and value['shopQuantity'] == 7
        assert value['cashInitial'] == 14499 and value['cashFinal'] == 20499
        assert value['inventoryFinal']['金童剑']-value['inventoryInitial'].get('金童剑', 0) == 7
        assert len(value['steps']) == 14
        for grant in [1, 2, 3]:
            step = next(row for row in value['steps'] if row['label'] == f'grant-{grant}')
            assert step['delta'] == step['cashAfter']-step['cashBefore'] == 30000 and step['receipt'] == '得三万文钱'
            high = next(row for row in value['steps'] if row['label'] == f'grant-{grant}-high-balance')
            assert high['cashBefore'] >= 30000 and high['cashAfter'] == high['cashBefore'] and high['delta'] == 0
            spending = next(row for row in value['steps'] if row['label'] == f'spend-{grant}')
            assert len(spending['purchases']) == (3 if grant == 3 else 2)
            assert spending['cashAfter'] < 30000 and spending['cashBefore']-spending['cashAfter'] == len(spending['purchases'])*12000
            for buy in spending['purchases']:
                assert buy['item'] == 183 and buy['cashBefore']-buy['cashAfter'] == buy['price'] == 12000
                for label in ['item-selected', 'choice', 'after']:
                    assert value['proof']['Saved'][buy['label']+'-'+label]
        for label in ['grant-2-declined', 'grant-3-declined']+[f'funding-exhausted-{n}' for n in [1, 2, 3]]:
            step = next(row for row in value['steps'] if row['label'] == label)
            assert step['cashBefore'] < 30000 and step['cashBefore'] == step['cashAfter'] and step['delta'] == 0
    else:
        price, item = (30, '糖葫芦') if event == 2086 else (100, '酒')
        assert value['kind'] == 'original-paid-repeatable-purchase' and value['purchases'] == 2
        assert value['price'] == price and value['item'] == item and len(value['steps']) == 4
        assert value['cashInitial']-value['cashFinal'] == 2*price and value['inventoryFinal'][item]-value['inventoryInitial'].get(item, 0) == 2
        assert [row['choice'] for row in value['steps']] == ['否', '是', '是', '否']
        assert [row['delta'] for row in value['steps']] == [0, -price, -price, 0]
        assert new_points[event]['status'] == 'paid-repeatable-transaction-and-saved-no-replay-verified'
    for step in value['steps']:
        if step['label'].startswith('spend-'):
            continue
        assert step['cashAfter']-step['cashBefore'] == step['delta']
        for label in ['before', 'after']+(['choice'] if step['choice'] else [])+(['reading'] if step['receipt'] else []):
            assert value['proof']['Saved'][step['label']+'-'+label]
    artifacts.append(BASE / f'{prefix}-result-state-1.json')
    cases.append({'event': event, 'kind': value['kind'], 'freshRuntimeSaves': value['proof']['Saves'], 'report': digest(path)})

log = VERIFY / 'capital-economy-final-run.txt'
assert '--- PASS: TestOriginalCapitalThreeFundingGrantsWithRealPurchasesAndSave' in log.read_text()
assert '--- PASS: TestOriginalSugarAndWineRepeatPurchasesCancelAndSave' in log.read_text()
artifacts.append(log)
review_path = VERIFY / 'capital-side-original-review.json'
review = load(review_path)
assert review['passed'] and review['nativeBuildID'] == CORE and not review['nativeEdits'] and len(review['records']) == 6
artifacts.extend([review_path, VERIFY / 'capital-side-original-review-run.txt'])
for filename in ['capital-side-lint-run.txt', 'capital-side-lint-negative-run.txt', 'capital-side-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['capital-text-guide.md', 'yangzhou-text-guide.md']:
    artifacts.append(ROOT / '.local/pal/hidden-items-research' / filename)
for filename in ['tests/capital_economy_e2e_test.go', 'tests/regional_hidden_rewards_e2e_test.go', 'tests/capital-side-evidence-e2e.py', 'tests/capital-side-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'CAPITAL_SIDE_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == CORE
authority_hash = digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256']
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': 'three original one-time pickups and three original conditional funding/paid merchant events, wallet and inventory through ordinary inputs, fresh-runner restoration and no restore replay; no native edits or uploads, not full quest chains, performance or release acceptance', 'nativeBuildID': CORE, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 6, 'newActuallyClaimed': 6, 'newOriginalUnreachablePreserved': 0, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'cases': cases, 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'priorToadArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'capital-side-hidden-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
