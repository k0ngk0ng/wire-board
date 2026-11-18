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


old = load(VERIFY/'battle-recovery-source/previous-artifacts.json')
previous = VERIFY/'battle-recovery-stage.json'
previous_archive = VERIFY/'battle-recovery-stage.tar.gz'
assert digest(previous_archive)['sha256']=='64b8d09a8fb14edc648c6ad6b103459d57b73a537db5a48453a1bca258ebf9a4'
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
native = PAL/'screen-input/battle-current/rules.wasm'
assert digest(native)['sha256']=='d22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438'
scopes=['desktop-base','desktop-cdn-cold','phone-cdn-default']
runs = {}
for scope in scopes:
    directory = PERFORMANCE/('dense-battle-'+scope)
    report = load(directory/'result.json')
    effects = [row for row in report['wire'] if row.get('effects')]
    assert report['cash']==3 and report['measured'] and not report['passed'] and 'error' not in report
    assert len(effects)==8 and [row['effects'][0][1] for row in effects]==[0,1,2,3,3,4,5,6]
    assert 260<=effects[-1]['at']-effects[0]['at']<=310
    assert report['values']['wireBytesP95']<5000 and report['values']['ruleMSP95']<=20 and report['values']['rafGapP95']<=35
    reference = load(directory/'original.json')
    assert reference['nativeBuild']==digest(native)['sha256']
    assert len([row for row in reference['frames'] if row['phase']==1 and row['waitMS']==40 and row['kind']==4])==69
    native_effects=[row for row in reference['frames'] if row['effects']]
    assert all(row['enemyHP']==30 for row in native_effects[:-1]) and native_effects[-1]['enemyHP']!=30
    assert native_effects[-1]['enemy']==399 and reference['frames'][-1]['cash']==3
    assert all(row['url'].startswith('https://cdn.ichenj.com/bga/') for row in report['effectRequests'])
    wall=report['capture']['wall']
    runs[scope] = {'report':report,'fxFirstAt':effects[0]['at'],'fxLastAt':effects[-1]['at'],'pngStartedAt':wall+report['effectRequests'][1]['start'],
                   'pngEndedAt':wall+report['effectRequests'][1]['end']}
    if scope!='desktop-base':
        assert reference['start']==load(directory/'start.json')
        seed = reference['start']
        assert reference['seed']==int.from_bytes(hashlib.sha256(f"pal/{seed['time']}/{seed['version']}/{seed['players']}".encode()).digest()[:4],'little')
        assert len(report['witnesses'])==7 and report['witnesses'][-1]==[6,12]
    assert not (directory/'profile').exists()
    assert '--- PASS: TestPALBoardBrowserFixture' in (directory/'server.log').read_text()

natural,cold,phone = [runs[s] for s in scopes]
assert natural['pngStartedAt']>natural['fxLastAt']
assert natural['report']['effectRequests'][0]['duration']>=500 and natural['report'].get('assetDelayMS',0)==0
red = cold['report']
assert cold['pngStartedAt']>cold['fxLastAt']
assert red['assetDelayMS']==600 and red['values']['observedEffects']==[] and red['values']['visibleRate']<24
assert red['values']['longestPixelGapMS']>=600
holds=[e for e in red['wire'] if e['type'].startswith('effect-response-')]
assert holds[0]['status']==200 and holds[1]['at']-holds[0]['at']>=599 and holds[0]['url']==red['effectURLs'][0]
mobile=phone['report']
assert mobile['geometry']==[320,200,320,200,0] and 150<=mobile['rtt']['connectionRTTMS']<=300
assert mobile['values']['visibleRate']>=24 and mobile['values']['observedEffects']==[1,2,3,4,5,6]
assert phone['fxFirstAt']<phone['pngEndedAt']<phone['fxLastAt']
assert natural['report']['bundles']==red['bundles']==mobile['bundles']

source=VERIFY/'dense-battle-baseline-source'
stage=VERIFY/'dense-battle-baseline.json'
archive=VERIFY/'dense-battle-baseline.tar.gz'
assert not stage.exists() and not archive.exists()
source.mkdir()
previous_index=source/'previous-artifacts.json'
save(previous_index,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts=[previous_index]
for scope in scopes:
    directory=PERFORMANCE/('dense-battle-'+scope)
    artifacts.extend(p for p in sorted(directory.iterdir()) if p.suffix in {'.json','.png','.log'})
for bundle in red['bundles']:
    input_path=PERFORMANCE/'dense-battle-desktop-cdn-cold/web-dist'/bundle['url'].lstrip('/')
    assert digest(input_path)['sha256']==bundle['sha256']
    output=source/input_path.name
    output.write_bytes(input_path.read_bytes())
    artifacts.append(output)
for name in ['games/pal/DENSE_BATTLE_TESTING.md','games/pal/tests/dense-battle-browser.py','games/pal/tests/dense-battle-original.mjs','games/pal/tests/native-vm.mjs','games/pal/tests/dense-battle-page.js','games/pal/tests/performance-observer.mjs','games/pal/tests/dense-battle-baseline-evidence.py','internal/server/pal_board_browser_test.go']:
    path=source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
measurement={'verifiedBaseline':True,'denseSpellAcceptance':False,'scope':'real solo PAL 98 cooperative spell, controlled CDN response and 320px delayed connection; native unmodified; no whole-canvas parity claim','desktop':red['values'],'phone':mobile['values'],'uploads':0,'nativeBuild':digest(native)['sha256'],'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'artifacts':[digest(p) for p in artifacts]}
save(stage,measurement)
with tarfile.open(archive,'x:gz') as bundle:
    for path in [stage,*artifacts]:
        bundle.add(path,arcname=path.relative_to(ROOT),recursive=False)
with tarfile.open(archive,'r:gz') as bundle:
    expected={r['path']:r['sha256'] for r in [digest(stage),*measurement['artifacts']]}
    assert len(bundle.getmembers())==len(expected)
    for member in bundle:
        assert member.isfile() and hashlib.file_digest(bundle.extractfile(member),'sha256').hexdigest()==expected[member.name]
print(json.dumps({'verifiedBaseline':True,'denseSpellAcceptance':False,'priorArtifactChecks':len(prior),'members':len(expected),'archive':digest(archive)},ensure_ascii=False))
