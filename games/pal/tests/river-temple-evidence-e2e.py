import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SOURCE = VERIFY / 'river-temple-hidden-source'
EXPECTED = {49: [800], 51: [], 53: [887], 54: [911, 912, 913, 914, 915], 59: list(range(974, 984))}
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'

def load(path):
    return json.loads(path.read_text())

def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}

prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 181
assert progress['passed'] and progress['processed'] == 208 and progress['total'] == 732
assert progress['actuallyClaimed'] == 190 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 524
verified = {row['event'] for row in progress['verifiedPoints']}
preserved = {row['event'] for row in progress['preservedPoints']}
assert verified.isdisjoint(preserved)
assert verified-{row['event'] for row in prior['verifiedPoints']} == set(sum(EXPECTED.values(), []))
assert preserved-{row['event'] for row in prior['preservedPoints']} == set(range(861, 871))
assert {row['event'] for row in prior['verifiedPoints']} <= verified
assert {row['event'] for row in prior['preservedPoints']} <= preserved
assert {798, 846, 847} <= set(progress['pendingEventIDs'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
cases, artifacts = [], [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json']
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v6-{scene}'
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['verificationVersion'] == 6 and value['scene'] == scene
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture']
    assert value['auditOnly'] == (scene == 51) and value['auditedEvents'] == (list(range(861, 871)) if scene == 51 else [])
    assert [row['event'] for row in value['claimed']] == events
    assert value['proof']['Saves'] == max(1, len(events)*4) and value['proof']['Battles'] == 0
    assert value['proof']['ScriptedFinishes'] == 0 and value['partyInitial'] == value['party']
    assert value['cashFinal']-value['cashInitial'] == sum(grant['Cash'] for reward in value['claimed'] for grant in reward['grants'])
    root = roots['roots'][value['sourceCheckpoint']]
    assert root['sha256'] == value['sourceSHA256']
    source_path = BASE / (value['sourceCheckpoint'][:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert digest(source_path)['sha256'] == root['sha256'] and digest(parent_path)['sha256'] == root['reportSHA256']
    parent = load(parent_path)
    assert parent['passed'] and not parent.get('resumedTestRoute')
    artifacts.extend([source_path, parent_path])
    for reward in value['claimed']:
        assert reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose'] and len(reward['repeatPose']) == 3
        for stage in ['unpicked', 'notice', 'collected', 'repeated']:
            assert value['proof']['Saved'][f"reward-{reward['event']}-{stage}"]
    log = VERIFY / f'regional-river-{scene}-final-run.txt'
    assert '--- PASS: TestOriginalRegionalHiddenRewardsAndSavedNoDuplication' in log.read_text()
    artifacts.extend(sorted(BASE.glob(f'{prefix}-*.json')))
    artifacts.append(log)
    cases.append({'scene': scene, 'events': events, 'auditOnly': value['auditOnly'], 'auditedEvents': value['auditedEvents'], 'freshRuntimeSaves': value['proof']['Saves'], 'partyRoles': [row['role'] for row in value['party']], 'report': digest(path)})
review_path = VERIFY / 'rice-shop-original-unreachable.json'
review = load(review_path)
assert review['passed'] and review['readyForProgress'] and review['nativeBuildID'] == CORE and not review['nativeEdits']
assert {row['event'] for row in review['records']} == set(range(861, 871))
assert review['graph']['ignoringExitPositions'] == 12414 and review['graph']['sameScenePositions'] == 342
assert review['allSceneTransitions'] == len(review['entrances']) == 1
assert len(review['exit']['laterStageChecks']) == 22
for row in review['records']:
    assert not row['normalPickupVerified'] and row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter']
    assert not row['reachableSearchStances'] and row['ignoringExitSearchStances']
artifacts.extend([review_path, ROOT / review['reference']['path'], ROOT / review['graph']['map']['path']])
artifacts.extend(ROOT / row['state']['path'] for row in review['exit']['laterStageChecks'])
for filename in ['regional-river-review-run.txt', 'regional-river-lint-run.txt', 'regional-river-lint-negative-run.txt', 'regional-river-roots-run.txt']:
    path = VERIFY / filename
    assert path.is_file() and path.stat().st_size > 0
    if filename == 'regional-river-roots-run.txt':
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for filename in ['games/pal/tests/regional_hidden_rewards_e2e_test.go', 'games/pal/tests/rice-shop-review.py', 'games/pal/tests/river-temple-evidence-e2e.py', 'games/pal/tests/hidden-progress.py', 'games/pal/tests/campaign-roots.json', 'games/pal/tests/campaign_roots_test.go', 'games/pal/tests/campaign_roots_e2e_test.go', 'games/pal/build/lint-rules.json', 'games/pal/WHITE_RIVER_REWARDS_TESTING.md', 'games/pal/HIDDEN_ITEMS.md', 'games/pal/ACCEPTANCE.md', 'games/pal/LINT.md']:
    path = ROOT / filename
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
authority_hash = hashlib.sha256((ROOT / '.local/pal/authority-prepared/authority.zip').read_bytes()).hexdigest()
assert authority_hash == 'c63f058aa4c050deafa5d68171027a4f420ce7b58d6bd2276d846f3a7312c508'
summary = {'passed': True, 'scope': '17 actual ordinary pickups in White River and Jade Buddha temple; 10 independently sourced original black-area rice-shop records preserved with active absorbing exit evidence; no merchant/trap completion claim, no native edits or resource upload; not performance or release acceptance', 'nativeBuildID': CORE, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 27, 'newActuallyClaimed': 17, 'newOriginalUnreachablePreserved': 10, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': 0, 'cases': cases, 'preservedEvents': list(range(861, 871)), 'priorStageArtifactsUnchanged': True, 'acceptedCampaignReportsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
assert summary['freshRuntimeSaves'] == 69
(VERIFY / 'river-temple-hidden-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'display': progress['display'], 'freshRuntimeSaves': 69, 'priorArtifactChecks': len(baseline['artifacts']), 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
