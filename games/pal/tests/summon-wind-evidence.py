import hashlib
import json
from pathlib import Path
import tarfile

ROOT=Path(__file__).resolve().parents[3]
PAL=ROOT/'.local/pal'
VERIFY=PAL/'verification'
PERFORMANCE=PAL/'performance'
NATIVE=PAL/'summon-wind/native'


def load(path):
    return json.loads(path.read_bytes())


def digest(path):
    with path.open('rb') as stream:
        sha=hashlib.file_digest(stream,'sha256').hexdigest()
    return {'path':str(path.relative_to(ROOT)),'bytes':path.stat().st_size,'sha256':sha}


def save(path,values):
    with path.open('x') as output:
        json.dump(values,output,ensure_ascii=False,indent=2)
        output.write('\n')


old=load(VERIFY/'dense-frame-memory-source/previous-artifacts.json')
previous=VERIFY/'dense-frame-memory-stage.json'
previous_archive=VERIFY/'dense-frame-memory-stage.tar.gz'
assert digest(previous_archive)['sha256']=='2d8809fadf1b06b88c691fd1d16da5b93eaaeda13eed56b3ccb729a09baa6f74'
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

native=digest(NATIVE/'rules.wasm')
assert native['sha256']=='34115e655a56fb5c2ccedf3f55ac0ac70c7072aa079ebe5ab144e67f67a61627'
build=load(NATIVE/'build.json')
assert build['runtimeSHA256']==native['sha256'] and build['testBattleHP'] is None
assert build['battleFixture']==1 and build['fixtureBattleField']==18 and build['fixtureRoles']==[3]
scopes=['summon-desktop','summon-phone-save']
runs=[]
for scope in scopes:
    directory=PERFORMANCE/('dense-battle-'+scope)
    run=load(directory/'result.json')
    reference=load(directory/'original.json')
    assert run['passed'] and run['reward']==3 and reference['nativeBuild']==native['sha256']
    assert reference['start']==load(directory/'start.json')
    assert [6,14] in run['plan'] and [8,44] in run['plan']
    assert len(run['preloaded'])==4 and run['preloaded']==run['publishedEffects']
    assert all(url.startswith('https://cdn.ichenj.com/bga/') for url in run['publishedEffects'])
    evidence=run['observed']
    assert evidence['nativeFireFrames']==60 and evidence['nativeFireSpanMS']==2360
    assert evidence['publicFire']==evidence['visibleFire']==list(range(16))*3
    assert evidence['publicSprites']==evidence['visibleSprites']==[0,1]
    assert evidence['wireBytesP95']<=16384
    frames=reference['frames']
    fire=[f for f in frames if f['effects']]
    assert all(f['mp']==726 and f['enemyHP']==30 for f in fire[:-1])
    assert fire[-1]['enemyHP']!=30 and frames[-1]['cash']==3 and frames[-1]['mp']==758
    assert all(row['partyMP']==[758] for row in run['wire'] if row.get('cash')==3)
    assert any(row.get('cash')==3 for row in run['wire'])
    started=run['capture']['wall']+run['capture']['start']
    confirms=[row for row in run['wire'] if row['type']=='sent' and row['at']>=started and row['action'].get('keys')==2]
    assert len(confirms)==1
    assert any(row['type']=='ack' and row['status']==200 and row['nonce']==confirms[0]['nonce'] for row in run['wire'])
    assert not (directory/'profile').exists()
    assert '--- PASS: TestPALBoardBrowserFixture' in (directory/'server.log').read_text()
    runs.append(run)
desktop,phone=runs
assert not desktop['phoneSave'] and phone['phoneSave']
assert desktop['publishedEffects']==phone['publishedEffects'] and desktop['bundles']==phone['bundles']
assert abs(desktop['observed']['fireSpanMS']-2360)<=200
assert phone['geometry']==[320,200,320,200,0] and 150<=phone['rtt']['connectionRTTMS']<=300
save_info=phone['save']
assert save_info['exactRaster'] and save_info['offlineMS']>=3000 and 6<=save_info['frame']<=15
folder=PERFORMANCE/'dense-battle-summon-phone-save'
saved,resumed=[load(folder/('summon-'+name+'.json')) for name in ('saved','resumed')]
assert saved['paused'] and not resumed['paused'] and saved['deadline']==0
assert resumed['deadline']-resumed['now']==saved['tickLeftMS']==save_info['tickLeftMS']
assert saved['remainingMS']==resumed['remainingMS'] and 0<saved['remainingMS']<=40
a,b=saved['public'],resumed['public']
assert all(a[key]==b[key] for key in ('raster','battle','party','cash','paletteEffect','battleAssets'))
assert a['epoch']<b['epoch']
assert a['party'][0]['mp']==726 and a['cash']==0
assert any(n[0]==1 and n[5:8]==[8,44,save_info['frame']] for n in a['raster']['nodes'])
assert any(r['save'] and r['trusted'] and r['pointer']=='touch' for r in phone['capture']['inputs'])

source=VERIFY/'summon-wind-source'
stage=VERIFY/'summon-wind-stage.json'
archive=VERIFY/'summon-wind-stage.tar.gz'
assert not stage.exists() and not archive.exists()
source.mkdir()
baseline=source/'previous-artifacts.json'
save(baseline,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts=[baseline,NATIVE/'rules.wasm',NATIVE/'build.json',NATIVE.parent/'native.log']
for scope in scopes:
    directory=PERFORMANCE/('dense-battle-'+scope)
    artifacts.extend(p for p in sorted(directory.iterdir()) if p.suffix in {'.json','.png','.log'})
for name in ('games/pal/SUMMON_WIND_TESTING.md','games/pal/tests/summon-wind-browser.py','games/pal/tests/summon-wind-original.mjs','games/pal/tests/summon-battle-page.js','games/pal/tests/performance-observer.mjs','games/pal/tests/summon-wind-evidence.py','internal/server/pal_summon_browser_trace_test.go','internal/server/pal_board_browser_test.go'):
    path=source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
report={'passed':True,'scope':'authentic PAL 98 wind summoning on desktop and delayed 320px phone; save/continue from the middle of the first pass; no parallel-room or full game acceptance','windSummon':315,'desktopFireChanges':48,'phoneFireChanges':48,'savedFrame':save_info,'settlementCash':3,'manaSpent':64,'nativeUnmodifiedRules':True,'uploads':0,'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'artifacts':[digest(path) for path in artifacts]}
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
