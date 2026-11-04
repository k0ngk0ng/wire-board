"""Verify and bind the completed source0 parent/child integration evidence."""
from datetime import datetime, timezone
from pathlib import Path
import gzip,hashlib,json,shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def bound(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path)}

logs=[]
for target in ('native','wasm'):
    names=[f'genkai-native-shard-{i}.jsonl'for i in range(3)]+['genkai-native-animation.jsonl']if target=='native'else['genkai-integration-wasm.jsonl']
    combined=b'';invocations=[]
    for name in names:
        source=LOCAL/name;raw=source.read_bytes();rows=[json.loads(x)for x in raw.decode().splitlines()]
        assert not any(r.get('Action')in('fail','build-fail')for r in rows),source
        assert any(r.get('Action')=='pass'and not r.get('Test')for r in rows),source
        combined+=raw
        destination=OUT/name;shutil.copyfile(source,destination);invocations.append(bound(destination))
    rows=[json.loads(x)for x in combined.decode().splitlines()]
    tests=[{'package':r['Package'],'test':r['Test']}for r in rows if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
    assert len(tests)==179 and len({(t['package'],t['test'])for t in tests})==179,(target,len(tests))
    destination=OUT/('genkai-engine-'+target+'.jsonl');destination.write_bytes(combined)
    logs.append({**bound(destination),'pass':True,'tests':tests,'invocations':invocations})
assert {(t['package'],t['test'])for t in logs[0]['tests']}=={(t['package'],t['test'])for t in logs[1]['tests']}

matches=[]
for kind,names in [('rules',('zero-zero','zero-chu','chu-zero','zero-ten','ten-zero','three','four','team','human-idle')),('render',('zero-zero','zero-chu','zero-ten','three','four','team'))]:
    for name in names:
        a_path=OUT/'genkai-e2e/native'/kind/(name+'.json');b_path=OUT/'genkai-e2e/wasm'/kind/(name+'.json')
        a,b=load(a_path),load(b_path)
        assert a['pass']and b['pass']and 0 in a['characters']and not a['reference_state_loaded'],(kind,name)
        assert a.pop('environment')!=b.pop('environment')and a==b,(kind,name,'full native/Wasm traces differ')
        if kind=='rules':assert a['completed']and a['human_auto_takeovers']==0
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+name,'native':bound(a_path),'wasm':bound(b_path),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'trace_sha256':a['trace_sha256']}if kind=='rules'else{'pixel_trace_sha256':a['pixel_trace_sha256']})})

legacy_path=OUT/'genkai-legacy-regression.json';legacy=load(legacy_path)
assert legacy['pass']and len(legacy['checks'])==24
assert all(r['pass']and r['only_changed_field']=='rules_sha256'for r in legacy['checks'])
packed=(ROOT/legacy['actual_reports_path']).read_bytes();raw=gzip.decompress(packed)
assert hashlib.sha256(packed).hexdigest()==legacy['actual_reports_gzip_sha256']and hashlib.sha256(raw).hexdigest()==legacy['actual_reports_sha256']
actuals=json.loads(raw)
for row in legacy['checks']:
    a,b=actuals[row['actual_key']].copy(),load(ROOT/row['expected_path'])
    assert a.pop('rules_sha256')==row['new_rules_sha256']and b.pop('rules_sha256')==row['previous_rules_sha256']
    assert a==b,row['case']
browser_path=OUT/'genkai-local-player/result.json';browser=load(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320
assert browser['final_view']['players'][0]['character_id']==0 and browser['source0_forward_action']['Action']==14
assert browser['final_view']['players'][0]['input_channel']==0
for p,h in browser['source_sha256'].items():assert sha(ROOT/p)==h,p
checks=[]
for name in ('genkai-vet.txt','genkai-wasm-vet.txt','genkai-player-tsc.txt'):
    p=LOCAL/name;assert p.is_file()and not p.read_text().strip(),p
    destination=OUT/name;shutil.copyfile(p,destination);checks.append({**bound(destination),'pass':True})
red=[]
for name in ('fighter0-world-red.jsonl','fighter0-rules-red.jsonl','genkai-session-cli-red.txt','genkai-player-ui-red.txt'):
    p=LOCAL/name;assert p.is_file()and p.stat().st_size>0
    destination=OUT/name
    if p.suffix=='.txt':destination.write_text(p.read_text().expandtabs(8).rstrip()+'\n')
    else:shutil.copyfile(p,destination)
    red.append({**bound(destination),'raw_log_sha256':sha(p)})
sources={str(p.relative_to(ROOT)):sha(p)for folder in ('engine','runtimebridge','cmd')for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if p.is_file()and(p.suffix in('.py','.sh','.tsx','.css','.mjs')or p.name=='tsconfig.json')})
evidence=[ROOT/'games/yuyuhakusho'/p for p in ['engine/testdata/fighter0-attacks-validation.json','engine/testdata/fighter0-motion-validation.json','engine/testdata/fighter0-projectile-validation.json','engine/testdata/fighter0-composition-validation.json','engine/testdata/chu-world-fighter0-manifest.json','engine/testdata/chu-world-fighter0-special-manifest.json','data/chu-world-rules-manifest.json','data/fighter0-projectile-rules-manifest.json']]
for name in ('genkai-native-shards.json','genkai-cleanup.json'):
    if(OUT/name).is_file():evidence.append(OUT/name)
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'46cc202b','stage':'Genkai full fighter/projectile rules and three-character independent matches',
    'pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':52,
    'integration_checks':logs,'fresh_matches':matches,'legacy_regression':bound(legacy_path),'browser_e2e':bound(browser_path),'targeted_checks':checks,'test_first_evidence':red,
    'original_world':{'normal_passes':3156,'normal_callbacks':42212,'normal_clear_video_label':4957,'special_passes':1801,'special_callbacks':23362,'special_video_end':3600,'single_initial_state':True,'natural_special_children':{'12c96':34,'1313e':30,'132de':62,'134c2':13,'1361e':13}},
    'supporting_evidence':[bound(p)for p in evidence],'current_source_sha256':sources,
    'limits':['Only source IDs0/9/10 and default arena are playable; eight other characters and seven arenas remain.',
        'Source0 natural AI is not recorded; original private full-input/actor AI and fresh independent AI matches are distinct evidence.',
        'Natural motion15/16/19 remains absent in the two retained inputs; original private and real-animation continuous evidence covers them.',
        'Rule data schema2 adds verified projectile records/short sequences; only the rule-data digest changes in legacy reports, with full state/event traces unchanged.',
        'No platform room/network/prediction/takeover or sound-playback acceptance, CDN upload, release or deployment.']}
(OUT/'genkai-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'tests_per_environment':179,'full_native_wasm_scenarios':15,'legacy_reports':24,'progress_estimate_percent':52}))
