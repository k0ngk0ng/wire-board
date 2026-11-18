import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'misc-rewards-source'
GLOBAL = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
BASE = GLOBAL / 'scopes/misc-rewards-v1'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 716 and prior['total'] == 735
assert progress['passed'] and progress['processed'] == 722 and progress['total'] == 735 and progress['remaining'] == 13
assert progress['actuallyClaimed'] == 697 and progress['originalUnreachablePreserved'] == 25
expected = {129, 438, 846, 847, 399, 798}
old = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new)-set(old) == expected and all(new[event] == row for event, row in old.items())
assert progress['catalog'] == prior['catalog'] and progress['catalogSupplements'] == prior['catalogSupplements']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 7382 and digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'tower-remainder-stage.tar.gz')['sha256'] == '4311fdcb824ea7a802b85ba31578bc9d7d50d70cdb1ba329265c7c8b143872e1'
artifacts = [SOURCE / name for name in ['prior-hidden-progress.json', 'prior-artifact-baseline.json']]
artifacts.extend(VERIFY / name for name in ['hidden-progress.json', 'hidden-search-candidates.json', 'hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'misc-rewards-evidence-red-run.txt', 'misc-rewards-evidence-pre-aggregate-red-run.txt', 'misc-rewards-progress-run.txt', 'misc-rewards-original-review-run.txt', 'misc-market-harbor-fixture-run.txt', 'tower-remainder-stage-archive.json', 'tower-remainder-archive-verification.json'])


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


review_path = VERIFY / 'misc-rewards-original-review.json'
review = load(review_path)
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert review['passed'] and not review['nativeEdits'] and review['nativeBuildID'] == core
assert {row['event'] for row in review['merchants']} == {129, 438, 846, 847} and {row['event'] for row in review['pickups']} == {399, 798}
assert all(row['actualSearchTargetVerified'] and not row['insufficientFundsGameplayCovered'] and row['insufficientFundsOriginalBranchPreserved'] for row in review['merchants'])
checked_references(review)
artifacts.append(review_path)
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'


def check_source(value):
    name = value['sourceCheckpoint']
    registration = roots['roots'][name]
    path = BASE / (name[:-5]+'-1.json')
    parent = BASE / (registration['report'][:-5]+'-1.json')
    assert digest(path)['sha256'] == value['sourceSHA256'] == registration['sha256'] and digest(parent)['sha256'] == registration['reportSHA256']
    assert load(parent)['passed'] and not load(parent).get('resumedTestRoute', False)
    artifacts.extend([path, parent])


def save_artifacts(prefix, value):
    proof = value['proof']
    assert proof['Saves'] == len(proof['Saved']) and all(proof['Saved'].values())
    for label in proof['Saved']:
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.extend([BASE / f'{prefix}-result-1.json', BASE / f'{prefix}-state-1.json'])


merchant_cases, pickup_cases = [], []
tracked_saves = 0
for event, mode, price, decline_cost, item in [(129, 'fruit', 25, 0, '水果'), (438, 'sugar', 20, 20, '糖葫芦'), (846, 'rice-left', 100, 0, '糯米'), (847, 'rice-right', 100, 0, '糯米')]:
    prefix = f'misc-market-v1-{event}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and not value['nativeEdits'] and value['kind'] == 'original-paid-repeatable-merchant' and value['event'] == event and value['item'] == item
    assert value['price'] == price and value['declineCost'] == decline_cost and value['chargeBeforeChoice'] == (event == 438) and value['purchases'] == value['declines'] == 2
    assert [row['choice'] for row in value['steps']] == ['否', '是', '是', '否'] and [row['delta'] for row in value['steps']] == [-decline_cost, -price, -price, -decline_cost]
    assert value['inventoryAfter'][item]-value['inventoryBefore'].get(item, 0) == 2 and value['cashFinal']-value['cashInitial'] == -2*price-2*decline_cost
    proof = value['proof']
    assert proof['Battles'] == 0 and proof['Saves'] == (21 if event == 129 else 22 if event == 438 else 23)
    for row in value['steps']:
        for label in ['before', 'search-pose', 'reading', 'choice', 'after']:
            assert proof['Saved'][row['label']+'-'+label]
        assert row['cashAfter']-row['cashBefore'] == row['delta']
    native = next(row for row in review['merchants'] if row['event'] == event)
    assert native['chargePC'] < native['choicePC'] if event == 438 else native['chargePC'] > native['choicePC']
    assert len(native['steps']) == 4 and all(len(row['stages']) == 5 for row in native['steps'])
    assert new[event]['status'] == 'paid-repeatable-transaction-and-saved-no-replay-verified'
    assert new[event]['transactionRules'] == {'repeatable': True, 'price': price, 'item': item, 'declineCost': decline_cost, 'chargeBeforeChoice': event == 438, 'verifiedPurchases': 2, 'verifiedDeclines': 2}
    check_source(value)
    save_artifacts(prefix, value)
    log = VERIFY / f'misc-market-{mode}-final-run.txt'
    assert '--- PASS: TestOriginalEarlyMerchantsPaidRepeatAndSavedChoices' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.append(log)
    tracked_saves += proof['Saves']
    merchant_cases.append({'event': event, 'price': price, 'declineCost': decline_cost, 'actualPurchases': 2, 'actualDeclines': 2, 'trackedFreshRuntimeSaves': proof['Saves'], 'report': digest(path)})
for event, mode, item, item_id in [(399, 'sword', '越女剑', 173), (798, 'trap', '捕兽夹', 285)]:
    prefix = f'misc-pickup-v1-{event}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and not value['nativeEdits'] and value['event'] == event and value['item'] == item and value['itemID'] == item_id and value['notification']
    assert value['amountBefore'] == 0 and value['amountAfter'] == value['amountAfterRepeat'] == 1 and value['repeatSearches'] == 3
    assert value['proof']['Battles'] == 0 and value['cashInitial'] == value['cashFinal'] and value['battleCashChanges'] == [] and value['allRouteRewardCash'] == 0
    assert value['proof']['Saves'] == (6 if event == 399 else 7) and all(value['proof']['Saved'][label] for label in ['source', 'unpicked', 'notice', 'collected', 'repeated'])
    assert new[event]['status'] == 'pickup-and-saved-no-duplication-verified'
    check_source(value)
    save_artifacts(prefix, value)
    log = VERIFY / f'misc-pickup-{mode}-run.txt'
    assert '--- PASS: TestOriginalYueSwordAndDeerTrapPickupWithSavedRepeat' in log.read_text() and '\nFAIL' not in log.read_text()
    artifacts.append(log)
    tracked_saves += value['proof']['Saves']
    pickup_cases.append({'event': event, 'trackedFreshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': 0, 'report': digest(path)})
assert tracked_saves == 102
fixture = load(BASE / 'market-harbor-entry-1.json')
assert fixture['passed'] and fixture['generatedFromNewGame'] and not fixture['resumedTestRoute'] and not fixture['nativeEdits'] and fixture['candidate'] == 'audio' and fixture['developmentProfile'] == 'hp-9999'
assert fixture['proof']['Saves'] == 1 and fixture['stateSHA256'] == digest(BASE / 'market-harbor-entry-state-1.json')['sha256'] and fixture['cash'] == 550 and len(fixture['parents']) == 5
for label in fixture['proof']['Saved']:
    artifacts.extend(BASE / f'market-harbor-entry-{label}-{kind}-1.json' for kind in ['state', 'proof'])
log = VERIFY / 'misc-market-harbor-fixture-run.txt'
assert '--- PASS: TestOriginalMarketHarborFixtureFromNewGame' in log.read_text() and '\nFAIL' not in log.read_text()
for name in ['misc_market_fixture_e2e_test.go', 'misc_market_e2e_test.go', 'misc_pickups_e2e_test.go', 'capital_economy_e2e_test.go']:
    executed = SOURCE / ('executed_'+name)
    assert digest(executed)['sha256'] == digest(ROOT / 'games/pal/tests' / name)['sha256']
    artifacts.append(executed)
for name in ['campaign_inn_e2e_test.go', 'campaign_drunkard_e2e_test.go', 'campaign_village_e2e_test.go']:
    executed = SOURCE / ('harbor_precursor_'+name)
    assert digest(executed)['sha256'] == digest(ROOT / 'games/pal/tests' / name)['sha256']
    artifacts.append(executed)
for filename in ['misc-rewards-lint-run.txt', 'misc-rewards-lint-negative-run.txt', 'misc-rewards-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text() and '\nFAIL' not in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['tests/misc_market_fixture_e2e_test.go', 'tests/misc_market_e2e_test.go', 'tests/misc_pickups_e2e_test.go', 'tests/misc-rewards-original-review.py', 'tests/misc-rewards-evidence-e2e.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'MISC_REWARDS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
target = SOURCE / 'pal.md'
shutil.copyfile(ROOT / 'rules/pal.md', target)
artifacts.append(target)
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == core and digest(ROOT / '.local/pal/authority-prepared/authority.zip')['sha256'] == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': 'four actual original repeatable merchant events, eight purchases and eight declines, exact original sugar pre-choice charge retained, native first-search target verification for all sixteen trades, two original ordinary pickups and three repeated searches each; genuine fresh-new-game harbor fixture separately proved; not all original candidate classifications, performance or release', 'nativeBuildID': core, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 6, 'newActuallyClaimed': 6, 'newOriginalUnreachablePreserved': 0, 'newRewardContinuationSaves': tracked_saves, 'newFixtureContinuationSaves': 1, 'newCountedFreshRuntimeSaves': tracked_saves+1, 'fixturePrecursorSaveCallsIncludedInCount': False, 'fixturePrecursorReports': 5, 'newOrdinaryBattles': 0, 'merchantCases': merchant_cases, 'pickupCases': pickup_cases, 'insufficientFundsGameplayCombinationsCovered': False, 'isolatedEvidenceScope': 'misc-rewards-v1', 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': 7382, 'priorTowerRemainderArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'misc-rewards-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'rewardContinuationSaves': tracked_saves, 'fixtureContinuationSaves': 1, 'newOrdinaryBattles': 0, 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
