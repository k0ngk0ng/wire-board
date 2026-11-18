import hashlib
import json
from pathlib import Path
import tarfile

ROOT=Path(__file__).resolve().parents[3]
PAL=ROOT/'.local/pal'
VERIFY=PAL/'verification'
PERFORMANCE=PAL/'performance'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    with path.open('rb') as stream:
        sha=hashlib.file_digest(stream,'sha256').hexdigest()
    return {'path':str(path.relative_to(ROOT)),'bytes':path.stat().st_size,'sha256':sha}


def save(path,row):
    with path.open('x') as output:
        json.dump(row,output,ensure_ascii=False,indent=2)
        output.write('\n')


def overlap(first,second):
    return max(0,min(first[1],second[1])-max(first[0],second[0]))


old=load(VERIFY/'summon-wind-source/previous-artifacts.json')
previous=VERIFY/'summon-wind-stage.json'
previous_archive=VERIFY/'summon-wind-stage.tar.gz'
assert digest(previous_archive)['sha256']=='1ed98607cde34c269c61ed999d216887b30ff4d01c48fc2598f28cdb05bbffd4'
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
    assert digest(ROOT/path)['sha256']==row['sha256'],path

failed=PERFORMANCE/'dense-battle-summon-parallel-primary'
verified=PERFORMANCE/'dense-battle-summon-parallel-verified'
assert not load(failed/'parallel.json')['passed']
assert "row['trusted'] and row.get('code')=='Enter'" in (failed/'runner.log').read_text()
assert not load(failed/'desktop/result.json')['capture']['inputs']
report=load(verified/'parallel.json')
assert report['passed'] and report['windowRequirementMS']==2000
assert report['clientExitCodes']=={'desktop':0,'phone':0}
desk,phone=[report['cases'][side] for side in ('desktop','phone')]
assert desk['roomID']!=phone['roomID'] and desk['matchID']!=phone['matchID']
assert desk['instance']==phone['instance'] and not set(desk['epoch'])&set(phone['epoch'])
assert 0<=desk['rttMS']<50 and 150<=phone['rttMS']<=300
assert report['authorityOverlapMS']==overlap(desk['authorityWindowMS'],phone['authorityWindowMS'])>=2000
assert report['visibleOverlapMS']==overlap(desk['pixelWindowMS'],phone['pixelWindowMS'])>=2000
assert report['parallelCallbackPairs']>0
native=digest(PAL/'summon-wind/native/rules.wasm')
assert native['sha256']=='34115e655a56fb5c2ccedf3f55ac0ac70c7072aa079ebe5ab144e67f67a61627'
previous_builds=load(PERFORMANCE/'dense-battle-summon-desktop/result.json')['bundles']
for folder in (failed,verified):
    assert '--- PASS: TestPALBoardBrowserFixture' in (folder/'server.log').read_text()
    for side in ('desktop','phone'):
        directory=folder/side
        run=load(directory/'result.json')
        original=load(directory/'original.json')
        assert run['passed'] and run['reward']==3 and run['bundles']==previous_builds
        assert original['nativeBuild']==native['sha256']
        assert run['observed']['publicFire']==run['observed']['visibleFire']==list(range(16))*3
        assert run['observed']['visibleSprites']==[0,1]
        assert all(row['roomID']==run['roomID'] for row in run['wire'] if row['type']=='game')
        assert all(row['partyMP']==[758] for row in run['wire'] if row.get('cash')==3)
        assert len(run['preloaded'])==4
        assert not (directory/'profile').exists()
        if folder==verified:
            case=report['cases'][side]
            assert case['visibleFireChanges']==48 and case['ruleSamples']==case['wireFrames']==60
            assert case['wireBytesP95']<=16384 and case['rAFP95MS']<=35
            assert any(row['trusted'] and (row.get('pointer')=='touch' if side=='phone' else row.get('code')=='Enter') for row in run['capture']['inputs'])

source=VERIFY/'parallel-summon-source'
stage=VERIFY/'parallel-summon-stage.json'
archive=VERIFY/'parallel-summon-stage.tar.gz'
assert not stage.exists() and not archive.exists()
source.mkdir()
baseline=source/'previous-artifacts.json'
save(baseline,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts=[baseline]
for folder in (failed,verified):
    artifacts.extend(path for path in sorted(folder.glob('*')) if path.is_file() and path.suffix in {'.json','.log'})
    for side in ('desktop','phone'):
        artifacts.extend(path for path in sorted((folder/side).glob('*')) if path.is_file() and path.suffix in {'.json','.png'})
for name in ('games/pal/PARALLEL_SUMMON_TESTING.md','games/pal/tests/parallel-summon-browser.py','games/pal/tests/parallel-summon-evidence.py','games/pal/tests/summon-wind-browser.py','games/pal/tests/summon-wind-original.mjs','games/pal/tests/summon-battle-page.js','games/pal/tests/performance-observer.mjs','internal/server/pal_board_browser_test.go','internal/server/pal_summon_browser_trace_test.go'):
    path=source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
result={'passed':True,'scope':'two verified independent solo PAL 98 wind summons on one server with delayed phone and unshaped desktop; original rules, no loss injection or full release acceptance',
        'authorityOverlapMS':report['authorityOverlapMS'],'visibleOverlapMS':report['visibleOverlapMS'],'parallelCallbackPairs':report['parallelCallbackPairs'],
        'clients':[desk,phone],'uploads':0,'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'artifacts':[digest(path) for path in artifacts]}
save(stage,result)
with tarfile.open(archive,'x:gz') as bundle:
    for path in [stage,*artifacts]:
        bundle.add(path,arcname=path.relative_to(ROOT),recursive=False)
with tarfile.open(archive,'r:gz') as bundle:
    expected={row['path']:row['sha256'] for row in [digest(stage),*result['artifacts']]}
    assert len(bundle.getmembers())==len(expected)
    for member in bundle:
        assert member.isfile() and hashlib.file_digest(bundle.extractfile(member),'sha256').hexdigest()==expected[member.name]
print(json.dumps({'passed':True,'priorArtifactChecks':len(prior),'members':len(expected),'archive':digest(archive)},ensure_ascii=False))
