import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[3]
PAL = ROOT/'.local/pal'
VERIFY = PAL/'verification'
PERFORMANCE = PAL/'performance'
NATIVE = PAL/'screen-input/battle-current/rules.wasm'


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


def result(scope):
    return load(PERFORMANCE/('battle-recovery-'+scope)/'result.json')


progress = VERIFY/'hidden-progress.json'
old = load(VERIFY/'mixed-room-source/previous-artifacts.json')
last = VERIFY/'mixed-room-performance-stage.json'
last_archive = VERIFY/'mixed-room-performance-stage.tar.gz'
assert digest(last_archive)['sha256'] == 'd94c0be6e319bcc275bdfc5b062ecc10ef5935cdd411f02354680554a43b494d'
assert digest(progress)['sha256'] == old['progressSHA256']
assert (load(progress)['processed'],load(progress)['total']) == (722,735)
prior = {}
for row in old['artifacts'] + load(last)['artifacts']:
    path = row['path']
    if path == str(progress.relative_to(ROOT)):
        continue
    assert path not in prior or prior[path]['sha256'] == row['sha256'], path
    prior[path] = row
for path in (last,last_archive):
    row = digest(path)
    prior[row['path']] = row
assert all(digest(ROOT/path)['sha256'] == row['sha256'] for path,row in prior.items())
assert digest(NATIVE)['sha256'] == 'd22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438'

red, green, audio = [result(scope) for scope in ('oracle-red','verified','audio-cdn')]
red_events = red['debugSurface']['interruptions']
assert not red['passed'] and red_events and {e['kind'] for e in red_events} == {'board-not-ready'}
assert green['passed'] and green['browserBundles'] != red['browserBundles']
drop, background, committed, won = green['cases']
assert all(not row['state']['interruptions'] and row['state']['sameCanvas'] and not row['state']['loading'] and row['state']['ready']=='true' for row in green['cases'])
assert len({row['state']['page'] for row in green['cases']}) == 1
assert drop['network']['stats']['uploaded']==0 and drop['network']['stats']['maxQueueBytes']>0
assert background['hidden']=={'hidden':True,'focused':False} and background['foreground']=={'hidden':False,'focused':True}
assert committed['geometry']==[320,200,True,0]
assert committed['network']['stats']['downloaded']==0 and committed['network']['stats']['uploaded']>0
assert [committed['command'][k] for k in ('state','action','target','object')]==[2,3,1,296]
nonces = set()
for row in (drop,committed):
    assert [e['kind'] for e in row['reconnect'] if e['kind'] in ('closed','connected','state')]==['closed','connected','state']
    sent = [e for e in row['reconnect'] if e['kind']=='sent' and e['action'].get('keys')]
    assert len(sent)==1
    nonces.add(sent[0]['nonce'])
assert len(nonces)==2 and all(e.get('nonce') not in nonces for e in green['trace'] if e['kind']=='ack')
assert won['spell']['maxRecoveryHP']==75 and won['spell']['costMP']==6 and won['cash']==3
assert won['before'][0]['mp']-won['healed'][0]['mp']==6 and 28 < won['healed'][1]['hp'] <= 103
saved = PERFORMANCE/'battle-recovery-save/battle-save'
saves = load(saved/'surface-result.json')
assert saves['passed'] and len(saves['cases'])==4 and all(case['passed'] for case in saves['cases']) and saves['cases'][-1]['cash']==3
assert audio['passed'] and [r['viewport'][0] for r in audio['cases']]==[1280,320]
assert all(r['staleSoundNotReplayed'] and r['globalMuteAndCurrentMusicRestore'] and r['first']['source'].startswith('https://cdn.ichenj.com/bga/') for r in audio['cases'])

source = VERIFY/'battle-recovery-source'
stage = VERIFY/'battle-recovery-stage.json'
archive = VERIFY/'battle-recovery-stage.tar.gz'
assert not source.exists() and not stage.exists() and not archive.exists()
source.mkdir()
baseline = source/'previous-artifacts.json'
save(baseline,{'count':len(prior),'progressSHA256':digest(progress)['sha256'],'artifacts':list(prior.values())})
artifacts = [baseline]
for scope,data in [('oracle-red',red),('verified',green),('audio-cdn',audio)]:
    directory = PERFORMANCE/('battle-recovery-'+scope)
    artifacts.extend(path for path in sorted(directory.iterdir()) if path.suffix in {'.json','.png','.log'})
    for bundle in data.get('browserBundles',[]):
        built = directory/'web-dist'/bundle['path'].lstrip('/')
        content = built.read_bytes()
        assert bundle['sha256']==hashlib.sha256(content).hexdigest() and bundle['bytes']==len(content)
        path = source/(scope+'-'+built.name)
        path.write_bytes(content)
        artifacts.append(path)
artifacts.extend(path for path in sorted(saved.iterdir()) if path.suffix in {'.json','.png'})
for name in ['BATTLE_RECOVERY_TESTING.md','tests/battle-recovery-browser.py','tests/lifecycle-driver.mjs','tests/battle-recovery-evidence.py','web/Board.tsx','web/input.ts','web/audio.ts','web/module.tsx']:
    path = source/name.replace('/','__')
    path.write_bytes((ROOT/'games/pal'/name).read_bytes())
    artifacts.append(path)
for name in ['web/src/main.tsx','web/src/game-api/module.ts','internal/server/pal_board_browser_test.go','internal/server/pal_audio_browser_test.go']:
    path = source/name.replace('/','__')
    path.write_bytes((ROOT/name).read_bytes())
    artifacts.append(path)
report = {'passed':True,'scope':green['scope'],'noLoadingDuringRecovery':True,'committedHealingOnce':True,'cash':won['cash'],'originalCoreSHA256':digest(NATIVE)['sha256'],'priorArtifactChecks':len(prior),'progress':load(progress)['display'],'uploads':0,'cdnAudioBrowserCases':len(audio['cases']),'artifacts':[digest(path) for path in artifacts]}
save(stage,report)
with tarfile.open(archive,'x:gz') as bundle:
    for path in [stage,*artifacts]:
        bundle.add(path,arcname=path.relative_to(ROOT),recursive=False)
with tarfile.open(archive,'r:gz') as bundle:
    expected = {r['path']:r['sha256'] for r in [digest(stage),*report['artifacts']]}
    assert len(bundle.getmembers())==len(expected)
    for member in bundle:
        assert member.isfile() and hashlib.file_digest(bundle.extractfile(member),'sha256').hexdigest()==expected[member.name]
print(json.dumps({'passed':True,'priorArtifactChecks':len(prior),'archive':digest(archive),'members':len(expected)},ensure_ascii=False))
