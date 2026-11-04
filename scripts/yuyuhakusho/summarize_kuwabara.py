"""Verify and bind the completed source2 five-character integration evidence."""
from datetime import datetime, timezone
from pathlib import Path
import gzip,hashlib,json,shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def bound(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path)}

inventory=load(OUT/'kuwabara-native-shards.json')
expected_tests={(p['package'],name)for p in inventory['packages']for name in p['tests']}
for path,digest in inventory['source_sha256'].items():assert sha(ROOT/path)==digest,path
logs=[]
for target in ('native','wasm'):
    names=[f'kuwabara-native-shard-{i}.jsonl'for i in range(3)]+['kuwabara-native-animation.jsonl']if target=='native'else['kuwabara-integration-wasm.jsonl']
    combined=b'';invocations=[]
    for name in names:
        source=LOCAL/name;raw=source.read_bytes();rows=[json.loads(x)for x in raw.decode().splitlines()]
        assert not any(r.get('Action')in('fail','build-fail')for r in rows),source
        assert any(r.get('Action')=='pass'and not r.get('Test')for r in rows),source
        combined+=raw
        destination=OUT/name;shutil.copyfile(source,destination);invocations.append(bound(destination))
    rows=[json.loads(x)for x in combined.decode().splitlines()]
    tests=[{'package':r['Package'],'test':r['Test']}for r in rows if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
    assert len(tests)==len(expected_tests) and {(t['package'],t['test'])for t in tests}==expected_tests,(target,len(tests))
    destination=OUT/('kuwabara-engine-'+target+'.jsonl');destination.write_bytes(combined)
    logs.append({**bound(destination),'pass':True,'tests':tests,'invocations':invocations})
assert {(t['package'],t['test'])for t in logs[0]['tests']}=={(t['package'],t['test'])for t in logs[1]['tests']}

matches=[]
for kind,names in [('rules',('kuwabara-kuwabara','kuwabara-genkai','genkai-kuwabara','kuwabara-yusuke','yusuke-kuwabara','kuwabara-chu','chu-kuwabara','kuwabara-elder','elder-kuwabara','three','four','team','human-idle')),('render',('kuwabara-kuwabara','kuwabara-genkai','kuwabara-chu','kuwabara-elder','three','four','team'))]:
    for name in names:
        a_path=OUT/'kuwabara-e2e/native'/kind/(name+'.json');b_path=OUT/'kuwabara-e2e/wasm'/kind/(name+'.json')
        a,b=load(a_path),load(b_path)
        assert a['pass']and b['pass']and 2 in a['characters']and not a['reference_state_loaded'],(kind,name)
        assert a.pop('environment')!=b.pop('environment')and a==b,(kind,name,'full native/Wasm traces differ')
        if kind=='rules':assert a['completed']and a['human_auto_takeovers']==0
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+name,'native':bound(a_path),'wasm':bound(b_path),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'trace_sha256':a['trace_sha256']}if kind=='rules'else{'pixel_trace_sha256':a['pixel_trace_sha256']})})

legacy_path=OUT/'kuwabara-legacy-regression.json';legacy=load(legacy_path)
assert legacy['pass']and len(legacy['checks'])==64
assert all(r['pass']for r in legacy['checks'])
packed=(ROOT/legacy['actual_reports_path']).read_bytes();raw=gzip.decompress(packed)
assert hashlib.sha256(packed).hexdigest()==legacy['actual_reports_gzip_sha256']and hashlib.sha256(raw).hexdigest()==legacy['actual_reports_sha256']
actuals=json.loads(raw)
for row in legacy['checks']:
    a,b=actuals[row['actual_key']].copy(),load(ROOT/row['expected_path'])
    assert a.pop('rules_sha256')==row['rules_sha256']and b.pop('rules_sha256')==row['previous_rules_sha256']
    assert a==b,row['case']
browser_path=OUT/'kuwabara-local-player/result.json';browser=load(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320
assert browser['final_view']['players'][0]['character_id']==2 and browser['source2_forward_attack']['Action']==3 and browser['source2_forward_attack']['Pose']==131
assert browser['final_view']['players'][0]['input_channel']==0
for p,h in browser['source_sha256'].items():assert sha(ROOT/p)==h,p
checks=[]
for name in ('kuwabara-vet.txt','kuwabara-wasm-vet.txt','kuwabara-player-tsc.txt'):
    p=LOCAL/name;assert p.is_file()and not p.read_text().strip(),p
    destination=OUT/name;shutil.copyfile(p,destination);checks.append({**bound(destination),'pass':True})
red=[]
for name in ('fighter2-world-red.jsonl','fighter2-world-special-red.jsonl','fighter2-world-data-red.jsonl','fighter2-world-bundle-red.jsonl','kuwabara-session-cli-red.txt','kuwabara-player-ui-red.txt'):
    p=LOCAL/name;assert p.is_file()and p.stat().st_size>0
    destination=OUT/name
    if p.suffix=='.txt':destination.write_text(p.read_text().expandtabs(8).rstrip()+'\n')
    else:shutil.copyfile(p,destination)
    red.append({**bound(destination),'raw_log_sha256':sha(p)})
sources={str(p.relative_to(ROOT)):sha(p)for folder in ('engine','runtimebridge','cmd')for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if p.is_file()and(p.suffix in('.py','.sh','.tsx','.css','.mjs')or p.name=='tsconfig.json')})
evidence=[ROOT/'games/yuyuhakusho'/p for p in ['engine/testdata/fighter2-attacks-validation.json','engine/testdata/fighter2-motion-validation.json','engine/testdata/fighter2-projectile-validation.json','engine/testdata/fighter2-composition-validation.json','engine/testdata/chu-world-fighter2-manifest.json','engine/testdata/chu-world-fighter2-special-manifest.json','data/chu-world-rules-manifest.json']]
for name in ('kuwabara-native-shards.json','kuwabara-cleanup.json'):
    if(OUT/name).is_file():evidence.append(OUT/name)
evidence.extend(OUT/name for name in ('kuwabara-test-first.json','kuwabara-graphic-keys.json'))
for path in evidence[:4]:assert load(path)['pass'],path
for path,digest in load(OUT/'kuwabara-test-first.json')['files'].items():assert sha(ROOT/path)==digest,path
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'9133ceae','stage':'Kuwabara complete shared rules and five-character independent matches',
    'pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':59,
    'integration_checks':logs,'fresh_matches':matches,'legacy_regression':bound(legacy_path),'browser_e2e':bound(browser_path),'targeted_checks':checks,'test_first_evidence':red,
    'original_world':{'normal_passes':3165,'normal_callbacks':42007,'normal_clear_video_label':4965,'special_passes':1801,'special_callbacks':23630,'special_video_end':3600,'single_initial_state':True,'natural_special_children':{'12dd4':73,'12c96':51}},
    'supporting_evidence':[bound(p)for p in evidence],'current_source_sha256':sources,
    'limits':['Only source IDs0/2/3/9/10 and default arena are playable; six other characters and seven arenas remain.',
        'Natural source2 AI is not recorded; private complete-input/actor AI and fresh independent AI matches remain distinct evidence.',
        'Source2 request1 direction indices11..15 are separately diagnosed; API error bounds must not be described as original no-op.',
        'Rule schema2 gains23 original animation records; previous rule digests change, while legacy world/event/lifecycle fields remain exact.',
        'No platform room/network/prediction/takeover or audio-playback acceptance, CDN upload, release or deployment.']}
(OUT/'kuwabara-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'tests_per_environment':len(expected_tests),'full_native_wasm_scenarios':20,'legacy_reports':64,'progress_estimate_percent':59}))
