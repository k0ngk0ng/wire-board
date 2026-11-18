import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SOURCE = VERIFY / 'toad-hidden-source'
EXPECTED = {102: [1871, 1870, 1873, 1874, 1872, 1875, 1876, 1845, 1869, 1877, 1878, 1879], 103: [1883, 1888, 1889, 1890], 104: [1911, 1919, 1920, 1921, 1922, 1923, 1924, 1925], 105: [1956, 1957, 1958, 1959, 1960, 1961, 1962, 1963, 1964]}
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
AUDIO = str(BASE.relative_to(ROOT))+'/'


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior, progress = load(SOURCE / 'prior-hidden-progress.json'), load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 298
assert progress['passed'] and progress['processed'] == 330 and progress['total'] == 732
assert progress['actuallyClaimed'] == 312 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 402
verified = {row['event'] for row in progress['verifiedPoints']}
preserved = {row['event'] for row in progress['preservedPoints']}
assert verified.isdisjoint(preserved)
assert verified-{row['event'] for row in prior['verifiedPoints']} == set(sum(EXPECTED.values(), []))-{1911}
assert {row['event'] for row in prior['verifiedPoints']} <= verified
assert preserved == {row['event'] for row in prior['preservedPoints']}
current_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
for row in prior['verifiedPoints']+prior['preservedPoints']:
    current = current_points[row['event']]
    if row['event'] != 1911:
        assert current == row
    else:
        assert {k: v for k, v in current.items() if k not in ['groups', 'evidence']} == {k: v for k, v in row.items() if k not in ['groups', 'evidence']}
        assert set(current['groups']) == set(row['groups']) | {'regional-current-104'}
        assert set(current['evidence']) == set(row['evidence']) | {AUDIO+'regional-hidden-v10-104-result-1.json'}
current_evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(current_evidence[row['path']] == row['sha256'] for row in prior['evidence'])
assert {798, 846, 847, 1172, 1196} <= set(progress['pendingEventIDs'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
catalog = {row['Event']: row for row in load(VERIFY / 'hidden-search-candidates.json')['candidates']}
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', SOURCE / 'regional_before_orb_dialogue.go', VERIFY / 'hidden-progress.json']
cases = []


def accepted_case(scene, events, suffix=''):
    prefix = f'regional-hidden-v10-{scene}{suffix}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 10 and value['scene'] == scene
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture']
    assert not value['auditOnly'] and value['auditedEvents'] == value['emptyChestAudits'] == []
    assert value['scholarOffer'] == ('renegotiated' if suffix else '')
    assert [row['event'] for row in value['claimed']] == events
    assert value['proof']['Saves'] >= len(events)*4 and value['proof']['Battles'] == value['proof']['Wins']
    assert value['proof']['ScriptedFinishes'] == 0 and value['combatAllowed']
    assert [(p['role'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['controller']) for p in value['party']]
    battle_cash = sum(row['rewardCash'] for row in value['battleCashChanges'])
    pickup_cash = sum(grant['Cash'] for reward in value['claimed'] for grant in reward['grants'])
    assert value['cashFinal']-value['cashInitial'] == battle_cash+pickup_cash
    assert len(value['battleCashChanges']) == value['proof']['Battles']-value['preCollectionBattles']
    root = roots['roots'][value['sourceCheckpoint']]
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == digest(source_path)['sha256'] == root['sha256']
    assert digest(parent_path)['sha256'] == root['reportSHA256']
    assert load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    artifacts.extend([source_path, parent_path])
    pickup_battles = 0
    for reward in value['claimed']:
        assert reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose'] and len(reward['repeatPose']) == 3
        assert reward['notification'] and reward['searchMode'] == catalog[reward['event']]['TriggerMode']
        if not suffix:
            assert reward['grants'] == catalog[reward['event']]['Grants']
        during = reward['battleCashDuringPickup']
        pickup_battles += len(during)
        assert all(row in value['battleCashChanges'] and row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in during)
        assert reward['cashAfter']-reward['cashBefore'] == sum(g['Cash'] for g in reward['grants'])+sum(row['rewardCash'] for row in during)
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            assert value['proof']['Saved'][f"reward-{reward['event']}-{label}"]
        if reward['event'] == 1845:
            assert reward['originalMerchantCorpsePreserved'] and not reward['disappearsAfterPickup']
        if reward['event'] == 1911:
            assert reward['originalPoisonCorpseFrame'] == 2
        if reward['event'] == 1883:
            assert reward['originalEscortContract'] and reward['disappearsAfterPickup']
            assert reward['escortFollowerSprite'] == (83 if suffix else 82)
            assert reward['decisions'] == (['否'] if suffix else ['是'])
            assert reward['searchesBeforeReward'] == (3 if suffix else 1)
            expected_grants = [(21261, 1500)] if suffix else [(21216, 1000)]
            assert [(g['PC'], g['Cash']) for g in reward['grants']] == expected_grants
            assert value['proof']['Saved']['reward-1883-decision-1']
            if suffix:
                assert all(value['proof']['Saved'][f'reward-1883-declined-field-{index}'] for index in [1, 2])
    log = VERIFY / f'regional-toad-{scene}{suffix}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text()
    artifacts.extend([path, BASE / f'{prefix}-state-1.json', log])
    for label, saved in value['proof']['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    source_code = ROOT / 'games/pal/tests/regional_hidden_rewards_e2e_test.go' if scene == 104 else SOURCE / 'regional_before_orb_dialogue.go'
    snapshot_code = SOURCE / ('regional_hidden_rewards_e2e_test.go' if scene == 104 else 'regional_before_orb_dialogue.go')
    return {'scene': scene, 'variant': 'renegotiated' if suffix else 'direct', 'events': events, 'testSource': {'path': str(snapshot_code.relative_to(ROOT)), 'sha256': digest(source_code)['sha256']}, 'freshRuntimeSaves': value['proof']['Saves'], 'ordinaryBattles': value['proof']['Battles'], 'pickupInterveningBattles': pickup_battles, 'pickupCash': pickup_cash, 'checkedCollectionBattleCash': battle_cash, 'partyRoles': [row['role'] for row in value['party']], 'report': digest(path)}


for scene, events in EXPECTED.items():
    cases.append(accepted_case(scene, events))
cases.append(accepted_case(103, [1883], '-renegotiated'))
assert current_points[1883]['conditionalRewards'] == [{'condition': 'accept-first-offer', 'cash': 1000, 'followerSprite': 82, 'evidence': AUDIO+'regional-hidden-v10-103-result-1.json'}, {'condition': 'decline-then-talk-twice', 'cash': 1500, 'followerSprite': 83, 'evidence': AUDIO+'regional-hidden-v10-103-renegotiated-result-1.json'}]
assert sum(row['pickupInterveningBattles'] for row in cases) > 0
for filename in ['regional-toad-lint-run.txt', 'regional-toad-lint-negative-run.txt', 'regional-toad-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
review_path = VERIFY / 'toad-original-rewards-review.json'
review = load(review_path)
assert review['passed'] and review['nativeBuildID'] == CORE and not review['nativeEdits'] and len(review['records']) == 4
assert [r['details']['cash'] for r in review['records'] if r['event'] == 1883] == [1000, 1500]
artifacts.extend([review_path, VERIFY / 'regional-toad-original-review-run.txt', ROOT / '.local/pal/hidden-items-research/yangzhou-text-guide.md'])
for filename in ['games/pal/tests/regional_hidden_rewards_e2e_test.go', 'games/pal/tests/campaign_caiyi_forest_e2e_test.go', 'games/pal/tests/toad-evidence-e2e.py', 'games/pal/tests/toad-original-review.py', 'games/pal/tests/hidden-progress.py', 'games/pal/tests/campaign-roots.json', 'games/pal/tests/campaign_roots_test.go', 'games/pal/tests/campaign_roots_e2e_test.go', 'games/pal/build/lint-rules.json', 'games/pal/TOAD_REWARDS_TESTING.md', 'games/pal/HIDDEN_ITEMS.md', 'games/pal/ACCEPTANCE.md', 'games/pal/LINT.md']:
    path = ROOT / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
authority_hash = hashlib.sha256((ROOT / '.local/pal/authority-prepared/authority.zip').read_bytes()).hexdigest()
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': '32 new actual Toad mountain/cave/back-cave event pickups, original scholar 1000/1500 dialogue and follower branches, persistent merchant corpse, reverified original orb/body frame; actual per-pickup wallet plus intervening ordinary battle ledgers, fresh runner saves and fixed-pose repeat searches; no native edits or uploads, not performance or release acceptance', 'nativeBuildID': CORE, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 32, 'newActuallyClaimed': 32, 'newOriginalUnreachablePreserved': 0, 'reverifiedPoints': 1, 'conditionalAdditionalExecutions': 1, 'totalActualPickupExecutionsVerified': 34, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'cases': cases, 'priorImmutableArtifactsUnchanged': True, 'acceptedCampaignReportsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'toad-hidden-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
