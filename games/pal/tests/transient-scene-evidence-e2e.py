import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'transient-scene-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999/scopes/transient-scene-v1'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


review_path = VERIFY / 'transient-scene-original-review.json'
review = load(review_path)
assert review['passed'] and review['diagnosticOnly'] and not review['readyForProgress'] and not review['nativeEdits']
assert review['event'] == 2947 and review['allLiteralSceneEntrances'] == [27947]
assert review['entryAlwaysLeavesBeforeReturning'] and not review['normalPickupVerified']
assert review['independentUnreachableAttribution'] is False
prior = SOURCE / 'prior-hidden-progress.json'
progress_path = VERIFY / 'hidden-progress.json'
assert prior.read_bytes() == progress_path.read_bytes()
progress = load(progress_path)
assert progress['processed'] == 722 and progress['total'] == 735 and progress['remaining'] == 13
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert baseline['passed'] and len(baseline['artifacts']) == 7656
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
artifacts = [prior, SOURCE / 'prior-artifact-baseline.json', progress_path, review_path]


def references(value):
    if isinstance(value, dict):
        if 'path' in value and 'sha256' in value:
            path = ROOT / value['path']
            assert path.resolve().is_relative_to(ROOT) and not path.is_symlink()
            assert digest(path)['sha256'] == value['sha256'], value['path']
            artifacts.append(path)
        for child in value.values():
            references(child)
    elif isinstance(value, list):
        for child in value:
            references(child)


references(review)
result_path = BASE / 'transient-scene-v1-result-1.json'
result = load(result_path)
assert result['passed'] and result['diagnosticOnly'] and not result['readyForProgress'] and not result['fieldExposed']
assert result['event'] == 2947 and result['finalScene'] == 175 and result['bananaFinalAmount'] == 0
assert len(result['directionProbes']) == 4 and {row['direction'] for row in result['directionProbes']} == {0, 1, 2, 3}
assert all(row['inputRejected'] and row['timerCalls'] == 8 and not row['fieldExposed'] for row in result['directionProbes'])
assert result['proof']['Battles'] == 0 and result['proof']['Saves'] == len(result['proof']['Saved'])
for label in result['proof']['Saved']:
    artifacts.extend(BASE / f'transient-scene-v1-{label}-{kind}-1.json' for kind in ['state', 'proof'])
artifacts.extend([result_path, BASE / 'transient-scene-v1-state-1.json'])
run = VERIFY / 'transient-scene-final-run.txt'
assert '--- PASS: TestOriginalSageVisitRemainsScriptedAcrossSavedDirections' in run.read_text() and '\nFAIL' not in run.read_text()
artifacts.extend([run, VERIFY / 'transient-scene-evidence-red-run.txt', VERIFY / 'transient-scene-original-review-run.txt', VERIFY / 'transient-scene-retry-cleanup.json'])
executed = SOURCE / 'executed_transient_scene_e2e_test.go'
assert executed.read_bytes() == (ROOT / 'games/pal/tests/transient_scene_e2e_test.go').read_bytes()
artifacts.append(executed)
for name in ['transient-scene-lint-run.txt', 'transient-scene-lint-negative-run.txt']:
    path = VERIFY / name
    assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
for name in ['tests/transient_scene_e2e_test.go', 'tests/transient-scene-original-review.py', 'tests/transient-scene-evidence-e2e.py', 'build/lint-rules.json', 'TRANSIENT_SCENE_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
    path = ROOT / 'games/pal' / name
    target = SOURCE / path.name
    shutil.copyfile(path, target)
    artifacts.append(target)
for name in ['campaign_caiyi_forest_e2e_test.go', 'campaign_white_river_medicine_e2e_test.go', 'campaign_evidence_scope_test.go', 'campaign_roots_e2e_test.go', 'campaign-roots.json']:
    path = ROOT / 'games/pal/tests' / name
    target = SOURCE / ('reused_'+name)
    shutil.copyfile(path, target)
    artifacts.append(target)
summary = {'passed': True, 'diagnosticOnly': True, 'readyForProgress': False, 'scope': 'original scripted sage visit, four directional waits across fresh-runtime saves, unchanged distinct banana events and unchanged progress; independent classification still pending', 'event': 2947, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 0, 'freshRuntimeSaves': result['proof']['Saves'], 'directionalTimerCalls': 32, 'ordinaryBattles': 0, 'priorArtifactChecks': len(baseline['artifacts']), 'priorImmutableArtifactsUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'transient-scene-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'priorArtifactChecks': summary['priorArtifactChecks'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
