import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[3]
PAL = ROOT/'.local/pal'
VERIFY = PAL/'verification'
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


old=load(VERIFY/'dense-battle-assets-source/previous-artifacts.json')
previous=VERIFY/'dense-battle-assets-stage.json'
previous_archive=VERIFY/'dense-battle-assets-stage.tar.gz'
assert digest(previous_archive)['sha256']=='b052c84eb5e45332f24eababee742a2f86fa6a05edcaf9a22e46bb30861bc6e3'
progress=VERIFY/'hidden-progress.json'
assert digest(progress)['sha256']==old['progressSHA256']
assert (load(progress)['processed'],load(progress)['total'])==(722,735)
prior={}
for row in old['artifacts']+load(previous)['artifacts']+[digest(previous),digest(previous_archive)]:
    path=row['path']
    if path==str(progress.relative_to(ROOT)):
        continue
    assert path not in prior or prior[path]['sha256']==row['sha256']
    prior[path]=row
for path,row in prior.items():
    assert digest(ROOT/path)['sha256']==row['sha256'], path

benchmark_paths=[PAL/('dense-frame-memory-'+name+'.json') for name in ('baseline','final','race')]
baseline,final,race=[load(path) for path in benchmark_paths]
for row in (baseline,final,race):
    assert row['passed'] and row['exactEncodedPublicParity'] and row['failedAfterValidStepPreserved']
    assert row['frames']==58 and row['originalElapsedMS']==2320 and row['memoryBytes']==8*1024*1024
    for key in ('buildID','firstMemoryID','lastMemoryID'):
        assert row[key]==baseline[key]
assert final['parallelIndependent'] and race['parallelIndependent']
native=PAL/'dense-battle-candidate/battle/rules.wasm'
assert digest(native)['sha256']==baseline['buildID']
difference=baseline['allocatedBytesPerFrame']-final['allocatedBytesPerFrame']
assert difference>=final['memoryBytes']-1024*1024
red=PAL/'dense-frame-memory-handle-red.log'
assert 'a copy of the same transaction handle detached metadata from native memory' in red.read_text()
race_log=PAL/'dense-frame-memory-engine-race.log'
for name in ('OriginalDenseFrameMemoryAndIsolation','NativeSnapshotBinaryEnvelopeAndLegacyContinuation'):
    assert '--- PASS: Test'+name in race_log.read_text()
assert 'DATA RACE' not in race_log.read_text() and '\nFAIL' not in race_log.read_text()

old_mobile=load(PERFORMANCE/'dense-battle-phone-prepared-repeat/result.json')
assert not old_mobile['passed'] and old_mobile['values']['ruleMSP95']>20
cases=[]
for scope in ('phone-cpu-baseline','phone-memory-reuse'):
    directory=PERFORMANCE/('dense-battle-'+scope)
    row=load(directory/'result.json')
    assert row['passed'] and row['geometry']==[320,200,320,200,0] and row['cash']==3
    assert row['assetDelayMS']==600 and 150<=row['rtt']['connectionRTTMS']<=300
    assert row['bundles']==old_mobile['bundles'] and row['effectURLs']==old_mobile['effectURLs']
    value=row['values']
    assert value['nativeFrames']==69 and value['visibleRate']>=24
    assert value['originalEffects']==value['observedEffects']==list(range(7))
    assert value['originalEffectPackets']==value['observedEffectPackets']
    assert value['ruleMSP95']<=20 and value['pixelGapP95']<=80 and value['wireBytesP95']<=16384
    assert not (directory/'profile').exists()
    assert '--- PASS: TestPALBoardBrowserFixture' in (directory/'server.log').read_text()
    cases.append({'scope':scope,**value})
profile_dir=PERFORMANCE/'dense-battle-phone-cpu-baseline'
profile=profile_dir/'rule-profile.txt'
assert 'server.adapter.timer' in profile.read_text() and 'engine.snapshotIdentity' in profile.read_text()
upgrade_path=VERIFY/'production-application-dense-frame-memory.json'
upgrade=load(upgrade_path)
assert upgrade['passed'] and upgrade['manualCheckpoint'] and upgrade['retainedOriginalStage4']
assert upgrade['upgradedFrom']=='4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3'
production_log=PAL/'dense-frame-memory-production.log'
assert '--- PASS: TestPALProductionCompositionAndProcessRestart' in production_log.read_text()

source=VERIFY/'dense-frame-memory-source'
stage=VERIFY/'dense-frame-memory-stage.json'
archive=VERIFY/'dense-frame-memory-stage.tar.gz'
assert not stage.exists() and not archive.exists()
source.mkdir()
baseline_path=source/'previous-artifacts.json'
save(baseline_path,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts=[baseline_path,upgrade_path,red,race_log,production_log,*benchmark_paths,profile,profile_dir/'rule.cpu',profile_dir/'process-profile.txt']
for name in ('baseline','final'):
    artifacts.append(PAL/('dense-frame-memory-'+name+'.log'))
for case in cases:
    directory=PERFORMANCE/('dense-battle-'+case['scope'])
    artifacts.extend(p for p in sorted(directory.iterdir()) if p.suffix in {'.json','.png','.log'})
for name in ('games/pal/DENSE_FRAME_MEMORY_TESTING.md','games/pal/engine/continuation.go','games/pal/engine/runtime.go','games/pal/engine/runtime_catalog.go','games/pal/engine/dense_frame_memory_e2e_test.go','games/pal/tests/dense-frame-memory-evidence.py'):
    path=source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
report={'passed':True,'scope':'single actual 320px solo original cooperative battle over 200ms RTT with delayed CDN response; same-core memory reuse and package-upgrade save; not other spells, combined-room stress or release',
        'allocBeforeBytes':baseline['allocatedBytesPerFrame'],'allocAfterBytes':final['allocatedBytesPerFrame'],
        'allocReductionPercent':difference*100/baseline['allocatedBytesPerFrame'],'sameNativeState':True,'mobileCasePassed':True,'fullDenseSpellAcceptance':False,
        'browser':cases,'uploads':0,'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'artifacts':[digest(path) for path in artifacts]}
save(stage,report)
with tarfile.open(archive,'x:gz') as bundle:
    for path in [stage,*artifacts]:
        bundle.add(path,arcname=path.relative_to(ROOT),recursive=False)
with tarfile.open(archive,'r:gz') as bundle:
    expected={row['path']:row['sha256'] for row in [digest(stage),*report['artifacts']]}
    assert len(bundle.getmembers())==len(expected)
    for member in bundle:
        assert member.isfile() and hashlib.file_digest(bundle.extractfile(member),'sha256').hexdigest()==expected[member.name]
print(json.dumps({'passed':True,'priorArtifactChecks':len(prior),'members':len(expected),'archive':digest(archive)},ensure_ascii=False))
