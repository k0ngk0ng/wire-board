"""Verify and bind the completed source7 eleven-character integration evidence."""
from datetime import datetime, timezone
from pathlib import Path
import gzip
import hashlib
import json
import shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def read(p):return json.loads(p.read_text())
def bound(p):return {'path':str(p.relative_to(ROOT)),'sha256':sha(p)}
def log(name, allow_timeout=False):
    p=LOCAL/name;dst=OUT/name;shutil.copyfile(p,dst)
    rows=[json.loads(l)for l in dst.read_text().splitlines()]
    if allow_timeout:
        assert any('test timed out after 20m' in r.get('Output','') for r in rows),name
        assert not any(r.get('Action') in ('fail','build-fail') and r.get('Test') for r in rows),name
        assert not any(r.get('Action')=='build-fail' for r in rows),name
    else:
        assert not any(r.get('Action')in('fail','build-fail')for r in rows),name
    if not allow_timeout:assert any(r.get('Action')=='pass'and not r.get('Test')for r in rows),name
    return dst,rows

def passed(rows):return [(r['Package'],r['Test'])for r in rows if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
inv=read(OUT/'itsuki-native-shards.json')
expected={(p['package'],n)for p in inv['packages']for n in p['tests']}
assert len(expected)==383
for path,digest in inv['source_sha256'].items():assert sha(ROOT/path)==digest,path
checks=[]
for target in ('native','wasm'):
    names=[f'itsuki-{target}-shard-{i}.jsonl'for i in range(4)]+[f'itsuki-{target}-animation.jsonl']
    names_seen=[];logs=[];continuation=None
    if target=='native':
        cp=OUT/'itsuki-native-continuation.json';continuation=read(cp)
        assert continuation['source_sha256']==inv['source_sha256']
    for name in names:
        timeout=False
        if target=='native' and 'shard-'in name:
            i=int(name.split('shard-')[1].split('.')[0]);timeout=continuation['initial'][i]['timeout']
        p,rows=log(name,allow_timeout=timeout);names_seen+=passed(rows);logs.append(bound(p))
    if target=='native':
        assert len(names_seen)==322
        pending={(inv['packages'][0]['package'],n)for n in continuation['remaining']}
        assert len(pending)==61 and pending==expected-set(names_seen)
        for item in continuation['initial']:assert sha(OUT/f"itsuki-native-shard-{item['shard']}.jsonl")==item['sha256']
        p,rows=log('itsuki-native-remaining.jsonl');assert len(passed(rows))==61 and set(passed(rows))==pending
        names_seen+=passed(rows);logs.append(bound(p));continuation=bound(cp)
    assert len(names_seen)==len(set(names_seen))==len(expected)and set(names_seen)==expected,target
    checks.append({'environment':target,'pass':True,'tests':len(expected),'invocations':logs,'continuation':continuation,'passed_tests':[{'package':p,'test':n}for p,n in sorted(names_seen)]})
assets=[]
for target in ('native','wasm'):
    p,rows=log(f'itsuki-assets-{target}.jsonl');names=passed(rows)
    assert len(names)==len(set(names))==27
    assets.append({'environment':target,'pass':True,'tests':27,'log':bound(p)})

matches=[]
for kind,count in [('rules',25),('render',14)]:
    paths=sorted((OUT/'itsuki-e2e/native'/kind).glob('*.json'));assert len(paths)==count
    for p in paths:
        other=OUT/'itsuki-e2e/wasm'/kind/p.name;a,b=read(p),read(other)
        assert a['pass']and b['pass']and 7 in a['characters']and not a['reference_state_loaded'],p
        assert a.pop('environment')!=b.pop('environment')and a==b,(p,'native/Wasm trace differs')
        if kind=='rules':assert a['completed']and a['human_auto_takeovers']==0
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+p.stem,'native':bound(p),'wasm':bound(other),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'trace_sha256':a['trace_sha256']}if kind=='rules'else{'pixel_trace_sha256':a['pixel_trace_sha256']})})
legacy_path=OUT/'itsuki-legacy-regression.json';legacy=read(legacy_path)
assert legacy['pass']and len(legacy['checks'])==280 and all(r['pass']for r in legacy['checks'])
packed=(ROOT/legacy['actual_reports_path']).read_bytes();raw=gzip.decompress(packed)
assert hashlib.sha256(packed).hexdigest()==legacy['actual_reports_gzip_sha256']and hashlib.sha256(raw).hexdigest()==legacy['actual_reports_sha256']
actuals=json.loads(raw)
for row in legacy['checks']:
    a,b=actuals[row['actual_key']].copy(),read(ROOT/row['expected_path'])
    assert a.pop('rules_sha256')==row['rules_sha256']and b.pop('rules_sha256')==row['previous_rules_sha256']
    assert a==b,row['case']
browser_path=OUT/'itsuki-local-player/result.json';browser=read(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320 and browser['human_idle_no_takeover']
assert browser['final_view']['players'][0]['character_id']==7 and browser['final_view']['players'][0]['input_channel']==0
assert browser['source7_forward_attack']['Action']==3 and browser['source7_forward_attack']['Pose']==131
for p,h in browser['source_sha256'].items():assert sha(ROOT/p)==h,p
extra=[]
for name in ('itsuki-vet.txt','itsuki-wasm-vet.txt','itsuki-player-tsc.txt'):
    p=LOCAL/name;assert p.is_file()and not p.read_text().strip();dst=OUT/name;shutil.copyfile(p,dst);extra.append({**bound(dst),'pass':True})
shutil.copyfile(LOCAL/'itsuki-player-final.txt',OUT/'itsuki-player.txt')
worlds=[]
for name,counts in [('chu-world-fighter7',(3144,41797,None)),('chu-world-fighter7-special',(3700,50006,24968))]:
    p=ROOT/f'games/yuyuhakusho/engine/testdata/{name}-manifest.json';m=read(p)
    assert m['pass']and m['frames_match_original_core']and not m['errors']and m['incomplete_calls']==0
    assert (m['records'],m['object_calls'])==counts[:2]
    if counts[2]is not None:assert m['draws']==counts[2]
    f=p.with_name(name+'.json.gz');data=f.read_bytes()
    assert hashlib.sha256(data).hexdigest()==m['gzip_sha256']and hashlib.sha256(gzip.decompress(data)).hexdigest()==m['fixture_sha256']
    worlds.append({'manifest':bound(p),'fixture':bound(f),'passes':counts[0],'callbacks':counts[1],'ordered_draws':counts[2],'first_video_label':m['first_frame'],'last_video_label':m['last_frame'],'single_initial_state':True,'control_frames':m['frames_checked']})
evidence=[ROOT/'games/yuyuhakusho/engine/testdata'/name for name in ['fighter7-attacks-validation.json','fighter7-motion-validation.json','fighter7-children-validation.json','fighter7-composition-validation.json','fighter7-motion-special-oracle-manifest.json','fighter7-storage-test-first.json','fighter7-world-test-first.json','raster-input-validation.json']]
for p in evidence[:4]:assert read(p)['pass'],p
assert read(ROOT/'games/yuyuhakusho/engine/testdata/raster-input-validation.json')['pass']
asset_path=OUT/'fighter7-children-assets.json';assert read(asset_path)['pass'];evidence.append(asset_path)
evidence += [OUT/'itsuki-test-first.json',OUT/'itsuki-native-shards.json',OUT/'itsuki-wasm-shards.json',OUT/'itsuki-pre-raster-matrix-manifest.json',ROOT/'games/yuyuhakusho/data/chu-world-rules-manifest.json',ROOT/'games/yuyuhakusho/data/fighter7-children-rules-manifest.json',ROOT/'games/yuyuhakusho/engine/FIGHTER7_WORLD_VERIFICATION.md',ROOT/'games/yuyuhakusho/engine/RASTER_INPUT_STORAGE_VERIFICATION.md']
evidence += [ROOT/'docs/research/md-yuyuhakusho'/name for name in ['source7-next-selection.json','source7-special-discovery.json']]
for p,h in read(OUT/'itsuki-test-first.json')['files'].items():assert sha(ROOT/p)==h,p
if (OUT/'itsuki-cleanup.json').is_file():evidence.append(OUT/'itsuki-cleanup.json')
if (ROOT/'games/yuyuhakusho/engine/testdata/fighter7-world-reference-frames.json.gz').is_file():evidence.append(ROOT/'games/yuyuhakusho/engine/testdata/fighter7-world-reference-frames.json.gz')
sources={str(p.relative_to(ROOT)):sha(p)for folder in ('engine','runtimebridge','cmd')for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if p.is_file()and(p.suffix in('.py','.sh','.tsx','.css','.mjs')or p.name=='tsconfig.json')})
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'4b1a5c15','stage':'Itsuki complete shared rules and eleven-character independent matches','pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':77,'integration_checks':checks,'asset_checks':assets,'fresh_matches':matches,'legacy_regression':bound(legacy_path),'browser_e2e':bound(browser_path),'targeted_checks':extra,'original_world':worlds,'supporting_evidence':[bound(p)for p in evidence],'current_source_sha256':sources,
'limits':['All11 source characters are playable only in the default arena; seven other arenas remain.','Special source7 retained boundary is1800..5500 and does not claim match end; ordinary source7 clears at4944.','Fourteen player-reachable roots succeeded naturally. R4 has no player command binding and is covered only by declared private evidence.','Natural healing helper was reached twice without parent Health increments; actual healing deltas/caps use original private and world-callback boundary proof.','Exact audio projection v2 retains v1 source/fixtures/failures and all observed request/write/drop/clear ordering; no whole-queue masking or business-code change.','Shared raster/input storage now has a single Stream owner; legacy PanA/PanB are serialization-only derived views. Old full JSON bytes and previous world/event/lifecycle traces remain compatible.','Natural source7 original world uses human control; original AI isolation/full-input and independent complete computer matches are distinct proof.','94 new AN records expand schema3 to4907; indexed catalog bytes are unchanged, and dirty/preload context boundaries remain explicit.','Wasm completed4 exact-set shards. Three native shards reached20m with319 engine tests passed; the remaining61 completed from the same frozen inventory. Both final unions cover all383 engine/catalog tests with no skipped tests or assertion failures.','No platform room/network/prediction/takeover/audio playback acceptance, CDN upload, release or deployment.']}
(OUT/'itsuki-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'rule_tests_per_environment':383,'asset_tests_per_environment':27,'full_native_wasm_scenarios':len(matches),'legacy_reports':280,'progress_estimate_percent':77}))
