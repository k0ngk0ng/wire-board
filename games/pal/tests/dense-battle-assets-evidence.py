import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[3]
PAL = ROOT/'.local/pal'
VERIFY = PAL/'verification'
CANDIDATE = PAL/'dense-battle-candidate'
PERFORMANCE = PAL/'performance'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    with path.open('rb') as stream:
        sha = hashlib.file_digest(stream,'sha256').hexdigest()
    return {'path':str(path.relative_to(ROOT)),'bytes':path.stat().st_size,'sha256':sha}


def save(path, data):
    with path.open('x') as output:
        json.dump(data,output,ensure_ascii=False,indent=2)
        output.write('\n')


old = load(VERIFY/'dense-battle-baseline-source/previous-artifacts.json')
previous = VERIFY/'dense-battle-baseline.json'
previous_archive = VERIFY/'dense-battle-baseline.tar.gz'
assert digest(previous_archive)['sha256']=='059c98866ee5d7209b756dd6c7e3007175a02a8053d661b58b41e05a1cffbc09'
progress = VERIFY/'hidden-progress.json'
assert digest(progress)['sha256']==old['progressSHA256']
assert (load(progress)['processed'],load(progress)['total'])==(722,735)
prior = {}
for row in old['artifacts']+load(previous)['artifacts']+[digest(previous),digest(previous_archive)]:
    path=row['path']
    if path==str(progress.relative_to(ROOT)):
        continue
    assert path not in prior or prior[path]['sha256']==row['sha256']
    prior[path]=row
for path,row in prior.items():
    assert digest(ROOT/path)['sha256']==row['sha256'], path

scope_names=['desktop-prepared','phone-prepared-settled','phone-prepared-repeat']
runs={}
resources=load(PERFORMANCE/'dense-battle-desktop-cdn-cold/result.json')['effectURLs']
for scope in scope_names:
    directory=PERFORMANCE/('dense-battle-'+scope)
    report=load(directory/'result.json')
    values=report['values']
    plans=report['plan']
    assert plans==[[8,id] for id in [0,1,2,4,6,7,10,12,27,28,29,40,50]], plans
    assert report['measured'] and report['cash']==3
    assert report['assetDelayMS']==600 and sorted(report['beforeInputEffects'])==sorted(resources)
    assert values['originalEffects']==values['observedEffects']==list(range(7))
    assert values['originalEffectPackets']==values['observedEffectPackets']
    assert values['visibleRate']>=24 and values['pixelGapP95']<=80
    assert values['rafGapP95']<=35 and values['wireBytesP95']<=16384
    assert sorted(r['url'] for r in report['effectRequests'])==sorted(resources)
    first_input=report['capture']['start']+report['capture']['wall']
    held=[row for row in report['wire'] if row['type'].startswith('effect-response-')]
    assert len(held)==2 and held[0]['status']==200 and held[1]['at']-held[0]['at']>=599
    assert held[1]['at']<first_input
    assert all(row['end']+report['capture']['wall']<first_input for row in report['effectRequests'])
    assert '--- PASS: TestPALBoardBrowserFixture' in (directory/'server.log').read_text()
    assert not (directory/'profile').exists()
    if report['phone']:
        assert report['geometry']==[320,200,320,200,0] and 150<=report['rtt']['connectionRTTMS']<=300
        assert not report['passed'] and values['ruleMSP95']>20
    else:
        assert report['passed'] and values['ruleMSP95']<=20
    runs[scope]=report
assert runs[scope_names[0]]['bundles']==runs[scope_names[1]]['bundles']==runs[scope_names[2]]['bundles']
assert '--- PASS: TestOriginalBattleAssetPlanAndFreshSavedContinuation' in (CANDIDATE/'engine-alternate.log').read_text()
assert '--- PASS: TestPALProductionCompositionAndProcessRestart' in (CANDIDATE/'production-upgrade.log').read_text()
upgrade_path=VERIFY/'production-application-battle-assets.json'
upgrade=load(upgrade_path)
assert upgrade['passed'] and upgrade['manualCheckpoint'] and upgrade['retainedOriginalStage4']
assert upgrade['upgradedFrom']=='4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3'
package=load(CANDIDATE/'server-package.json')
preflight=load(CANDIDATE/'server-preflight.json')
assert package['passed'] and package['supportsSave'] and package['supportsCheckpoint'] and package['singlePlayer']
assert preflight['passed'] and preflight['uploads']==0 and preflight['pin']['reusedObjects']==3 and preflight['pin']['newObjects']==2
assert preflight['sourceReproduced']==package['identity']['buildID']==digest(CANDIDATE/'campaign/rules.wasm')['sha256']
assert upgrade['upgradedFrom'] in preflight['pin']['retainedCores']

source=VERIFY/'dense-battle-assets-source'
stage=VERIFY/'dense-battle-assets-stage.json'
archive=VERIFY/'dense-battle-assets-stage.tar.gz'
assert not stage.exists() and not archive.exists()
source.mkdir()
baseline=source/'previous-artifacts.json'
save(baseline,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts=[baseline,upgrade_path]
for scope in scope_names:
    directory=PERFORMANCE/('dense-battle-'+scope)
    artifacts.extend(p for p in sorted(directory.iterdir()) if p.suffix in {'.json','.png','.log'})
for bundle in runs[scope_names[0]]['bundles']:
    input_path=PERFORMANCE/'dense-battle-desktop-prepared/web-dist'/bundle['url'].lstrip('/')
    assert digest(input_path)['sha256']==bundle['sha256']
    output=source/input_path.name
    output.write_bytes(input_path.read_bytes())
    artifacts.append(output)
for name in ('engine-alternate.log','native-battle-final.log','native-alternate.log','native-campaign-final.log','native-reproduction.log','production-upgrade.log','server-package.json','server-preflight.json','web-build-final.log'):
    artifacts.append(CANDIDATE/name)
for fixture in ('battle','alternate','campaign'):
    artifacts.extend(CANDIDATE/fixture/name for name in ('build.json','rules.wasm'))
for name in ('games/pal/DENSE_BATTLE_TESTING.md','games/pal/tests/dense-battle-browser.py','games/pal/tests/dense-battle-assets-evidence.py','games/pal/native/battle_assets.c','games/pal/build/native.py','games/pal/engine/battle_assets.go','games/pal/engine/battle_assets_e2e_test.go','games/pal/engine/runtime.go','games/pal/server/module.go','games/pal/web/Board.tsx','games/pal/web/scene-renderer.ts','internal/server/pal_board_browser_test.go','internal/production/pal_e2e_test.go','rules/pal.md'):
    path=source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
result={'passed':True,'observedCooperativeFramesRestored':True,'denseSpellAcceptance':False,'mobileRuleBudgetPassed':False,'olderPersonalSaveUpgradePassed':True,'uploads':0,'campaignCore':preflight['sourceReproduced'],'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'measurements':{k:r['values'] for k,r in runs.items()},'artifacts':[digest(p) for p in artifacts]}
save(stage,result)
with tarfile.open(archive,'x:gz') as bundle:
    for path in [stage,*artifacts]:
        bundle.add(path,arcname=path.relative_to(ROOT),recursive=False)
with tarfile.open(archive,'r:gz') as bundle:
    expected={r['path']:r['sha256'] for r in [digest(stage),*result['artifacts']]}
    assert len(bundle.getmembers())==len(expected)
    for member in bundle:
        assert member.isfile() and hashlib.file_digest(bundle.extractfile(member),'sha256').hexdigest()==expected[member.name]
print(json.dumps({'passed':True,'denseSpellAcceptance':False,'priorArtifactChecks':len(prior),'members':len(expected),'archive':digest(archive)},ensure_ascii=False))
