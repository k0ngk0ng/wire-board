import hashlib
import json
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SOURCE = VERIFY / 'capital-quests-source'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'


def load(path):
    return json.loads(path.read_text())


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


prior = load(SOURCE / 'prior-hidden-progress.json')
progress = load(VERIFY / 'hidden-progress.json')
assert prior['passed'] and prior['processed'] == 336 and prior['total'] == 732
assert progress['passed'] and progress['processed'] == 338 and progress['total'] == 734
assert progress['actuallyClaimed'] == 320 and progress['originalUnreachablePreserved'] == 18 and progress['remaining'] == 396
old_points = {row['event']: row for row in prior['verifiedPoints']+prior['preservedPoints']}
new_points = {row['event']: row for row in progress['verifiedPoints']+progress['preservedPoints']}
assert set(new_points)-set(old_points) == {2166, 2333}
assert all(new_points[event] == row for event, row in old_points.items())
assert progress['catalog'] == prior['catalog']
evidence = {row['path']: row['sha256'] for row in progress['evidence']}
assert all(evidence[row['path']] == row['sha256'] for row in prior['evidence'])
baseline = load(SOURCE / 'prior-artifact-baseline.json')
assert digest(SOURCE / 'prior-hidden-progress.json')['sha256'] == baseline['mutableProgress']['sha256']
for row in baseline['artifacts']:
    assert digest(ROOT / row['path'])['sha256'] == row['sha256'], row['path']
assert digest(VERIFY / 'capital-side-hidden-stage.tar.gz')['sha256'] == '48c5b45d5f6db4cbce78a2766f9ba0add1015547ab7671d141563be94c7edbe3'
conditional = load(VERIFY / 'hidden-conditional-candidates.json')
assert conditional['passed'] and conditional['directCatalogUnchanged'] and conditional['additionalCandidates'] == 2
assert {row['Event'] for row in conditional['candidates']} == {2166, 2333}
assert conditional['directCatalogSHA256'] == prior['catalog']['sha256'] and conditional['combinedCandidates'] == 734
roots = load(ROOT / 'games/pal/tests/campaign-roots.json')['candidates']['audio']
core = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert roots['coreSHA256'] == core and roots['profile'] == 'hp-9999'
artifacts = [SOURCE / 'prior-hidden-progress.json', SOURCE / 'prior-artifact-baseline.json', VERIFY / 'hidden-progress.json', VERIFY / 'hidden-conditional-candidates.json']
cases = []
for prefix, event, source_name in [('capital-quests-v1-letter-accept', 2166, 'capital-manor-step-15-state.json'), ('capital-quests-v1-letter-decline', 2166, 'capital-manor-step-15-state.json'), ('capital-quests-v1-bundle', 2333, 'toad-mountain-step-18-state.json')]:
    path = BASE / f'{prefix}-result-1.json'
    value = load(path)
    assert value['passed'] and value['event'] == event and value['sourceCheckpoint'] == source_name
    assert value['verificationVersion'] == 1 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert not value['nativeEdits'] and value['resourceUploads'] == 0 and value['repeatTalks'] == 3
    root = roots['roots'][source_name]
    source_path = BASE / (source_name[:-5]+'-1.json')
    parent_path = BASE / (root['report'][:-5]+'-1.json')
    assert value['sourceSHA256'] == root['sha256'] == digest(source_path)['sha256']
    assert root['reportSHA256'] == digest(parent_path)['sha256'] and load(parent_path)['passed'] and not load(parent_path).get('resumedTestRoute')
    assert [(row['role'], row['controller']) for row in value['partyInitial']] == [(row['role'], row['controller']) for row in value['partyFinal']] == [(2, 0), (0, 0)]
    proof, items = value['proof'], value['inventoryChecks']
    assert proof['Saves'] >= 20 and proof['Saves'] == len(proof['Saved']) and proof['ScriptedFinishes'] == 0
    if event == 2166:
        assert value['kind'] == 'original-letter-jade-chain' and value['completeChain'] and proof['Battles'] == proof['Wins'] == 0
        assert value['cashInitial'] == value['cashFinal'] and value['partyInitial'] == value['partyFinal']
        assert value['choice'] == ('否' if prefix.endswith('decline') else '是')
        assert value['reconsideredAfterDeclines'] == prefix.endswith('decline')
        assert items['initial'] == {'情书': 0, '玉佩': 0} and items['letter-received'] == items['letter-wrong-use-retained'] == {'情书': 1, '玉佩': 0}
        assert items['jade-wrong-use-retained'] == items['jade-hui-retained'] == {'情书': 0, '玉佩': 1}
        assert items['jade-returned'] == value['finalInventory'] == {'情书': 0, '玉佩': 0}
        if prefix.endswith('decline'):
            assert items['letter-delivered'] == {'情书': 0, '玉佩': 0}
            assert items['xiang-reconsidered'] == {'情书': 0, '玉佩': 1}
            for n in [1, 2, 3]:
                assert items[f'xiang-declined-repeat-{n}'] == {'情书': 0, '玉佩': 0} and proof['Saved'][f'xiang-declined-repeat-{n}-choice']
        else:
            assert items['letter-delivered'] == {'情书': 0, '玉佩': 1}
        for token in ['letter-read', 'jade-offer', 'jade-not-hui', 'jade-is-de', 'xiang-confession']:
            assert value['readings'][token] and proof['Saved'][token+'-reading']
        for n in [1, 2, 3]:
            for label in ['hui-repeat-', 'de-repeat-', 'xiang-ending-']:
                assert proof['Saved'][label+str(n)+'-after']
    else:
        assert value['kind'] == 'original-corpse-bundle-gourd-spell-chain' and value['sourceCorpseEvent'] == 1845
        assert value['learnedMagic'] == 384 and not value['spellBefore'] and value['spellAfter'] and value['spellAfterRepeat']
        assert items['initial'] == {'布包': 0, '紫金葫芦': 0}
        assert items['corpse-bundle-collected'] == items['bundle-wrong-use-retained'] == {'布包': 1, '紫金葫芦': 0}
        assert items['gourd-received'] == value['finalInventory'] == {'布包': 0, '紫金葫芦': 1}
        expected_travel = [(2085, 113), (2088, 108), (1991, 110), (2037, 105), (1926, 104), (1905, 102), (1844, 104), (1906, 105), (1928, 110), (2036, 108), (1992, 113), (2087, 112)]
        assert [(row['event'], row['scene']) for row in value['travel']] == expected_travel
        battles = value['battleCashChanges']
        assert proof['Battles'] == proof['Wins'] == len(battles)
        assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in battles)
        assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in battles)
        for label in ['corpse-bundle-before', 'corpse-bundle-after', 'bundle-wrong-use-after', 'capital-arrived-with-bundle', 'bundle-delivery-before', 'bundle-delivery-after', 'gourd-grant-reading', 'gourd-spell-reading', 'initial-spell-menu', 'learned-gourd-spell-visible', 'repeated-gourd-spell-visible', 'song-repeat-3-after']:
            assert proof['Saved'][label]
    for label, saved in proof['Saved'].items():
        assert saved
        artifacts.extend(BASE / f'{prefix}-{label}-{kind}-1.json' for kind in ['state', 'proof'])
    artifacts.extend([path, source_path, parent_path, BASE / f'{prefix}-result-state-1.json'])
    cases.append({'event': event, 'variant': value.get('choice', ''), 'freshRuntimeSaves': proof['Saves'], 'ordinaryBattles': proof['Battles'], 'report': digest(path)})
review_path = VERIFY / 'capital-quests-original-review.json'
review = load(review_path)
assert review['passed'] and review['nativeBuildID'] == core and not review['nativeEdits'] and len(review['records']) == 3
assert {row['event'] for row in review['records']} == {2166, 2333}
artifacts.extend([review_path, VERIFY / 'capital-quests-original-review-run.txt', VERIFY / 'capital-quests-conditional-catalog-run.txt'])
for suffix, name in [('letter', 'TestOriginalCapitalLetterJadeCompleteChainAndSavedChoices'), ('bundle', 'TestOriginalCapitalCorpseBundleReturnGourdSpellAndSave')]:
    path = VERIFY / f'capital-quests-{suffix}-final-run.txt'
    assert f'--- PASS: {name}' in path.read_text() and '\nFAIL' not in path.read_text()
    artifacts.append(path)
for filename in ['capital-quests-lint-run.txt', 'capital-quests-lint-negative-run.txt', 'capital-quests-roots-run.txt']:
    path = VERIFY / filename
    if filename.endswith('roots-run.txt'):
        assert '--- PASS: TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport' in path.read_text()
    else:
        assert json.loads(path.read_text().splitlines()[-1])['passed']
    artifacts.append(path)
artifacts.append(ROOT / '.local/pal/hidden-items-research/capital-text-guide.md')
for filename in ['tests/capital_quests_e2e_test.go', 'tests/capital_economy_e2e_test.go', 'tests/hidden-conditional-catalog-e2e.py', 'tests/capital-quests-evidence-e2e.py', 'tests/capital-quests-original-review.py', 'tests/hidden-progress.py', 'tests/campaign-roots.json', 'build/lint-rules.json', 'CAPITAL_QUESTS_TESTING.md', 'HIDDEN_ITEMS.md', 'ACCEPTANCE.md', 'LINT.md']:
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
summary = {'passed': True, 'scope': 'complete original letter/jade chain for direct acceptance and repeated rejection then reconsideration, original corpse bundle collected by backtracking and delivered after normal boat arrival, gourd gift and actual learned spell menu; fresh-runner saves, wrong-use retention and no repeated gifts, no native edits or uploads, not full conditional catalog, performance or release acceptance', 'nativeBuildID': core, 'authorityArchiveSHA256': authority_hash, 'nativeEdits': False, 'resourceUploads': 0, 'newProcessed': 2, 'newActuallyClaimed': 2, 'additionalConditionalCandidates': 2, 'directCandidateCatalogUnchanged': True, 'cases': cases, 'freshRuntimeSaves': sum(row['freshRuntimeSaves'] for row in cases), 'ordinaryBattles': sum(row['ordinaryBattles'] for row in cases), 'priorImmutableArtifactsUnchanged': True, 'priorArtifactChecks': len(baseline['artifacts']), 'priorCapitalSideArchiveUnchanged': True, 'progress': {key: progress[key] for key in ['display', 'processed', 'total', 'actuallyClaimed', 'originalUnreachablePreserved', 'remaining']}, 'artifacts': [digest(path) for path in dict.fromkeys(artifacts)]}
(VERIFY / 'capital-quests-stage.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, **summary['progress'], 'freshRuntimeSaves': summary['freshRuntimeSaves'], 'ordinaryBattles': summary['ordinaryBattles'], 'artifacts': len(summary['artifacts'])}, ensure_ascii=False))
