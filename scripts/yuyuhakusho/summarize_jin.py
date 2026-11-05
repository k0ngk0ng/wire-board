"""Verify and bind the completed source6 eight-character integration evidence."""
from datetime import datetime, timezone
from pathlib import Path
import gzip,hashlib,json,shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def bound(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path)}

inventory=load(OUT/'jin-native-shards.json')
expected_tests={(p['package'],name)for p in inventory['packages']for name in p['tests']}
for path,digest in inventory['source_sha256'].items():assert sha(ROOT/path)==digest,path
logs=[]
for target in ('native','wasm'):
    names=[f'jin-native-shard-{i}.jsonl'for i in range(4)]+['jin-native-animation.jsonl']if target=='native'else['jin-integration-wasm.jsonl']
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
    destination=OUT/('jin-engine-'+target+'.jsonl');destination.write_bytes(combined)
    logs.append({**bound(destination),'pass':True,'tests':tests,'invocations':invocations})
assert {(t['package'],t['test'])for t in logs[0]['tests']}=={(t['package'],t['test'])for t in logs[1]['tests']}

matches=[]
for kind,names in [('rules',('jin-jin','jin-genkai','genkai-jin','jin-kurama','kurama-jin','jin-kuwabara','kuwabara-jin','jin-yusuke','yusuke-jin','jin-elder','elder-jin','jin-chu','chu-jin','jin-younger','younger-jin','three','four','team','human-idle')),('render',('jin-jin','jin-genkai','jin-kurama','jin-kuwabara','jin-yusuke','jin-elder','jin-chu','jin-younger','three','four','team'))]:
    for name in names:
        a_path=OUT/'jin-e2e/native'/kind/(name+'.json');b_path=OUT/'jin-e2e/wasm'/kind/(name+'.json')
        a,b=load(a_path),load(b_path)
        assert a['pass']and b['pass']and 6 in a['characters']and not a['reference_state_loaded'],(kind,name)
        assert a.pop('environment')!=b.pop('environment')and a==b,(kind,name,'full native/Wasm traces differ')
        if kind=='rules':assert a['completed']and a['human_auto_takeovers']==0
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+name,'native':bound(a_path),'wasm':bound(b_path),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'trace_sha256':a['trace_sha256']}if kind=='rules'else{'pixel_trace_sha256':a['pixel_trace_sha256']})})

legacy_path=OUT/'jin-legacy-regression.json';legacy=load(legacy_path)
assert legacy['pass']and len(legacy['checks'])==154
assert all(r['pass']for r in legacy['checks'])
packed=(ROOT/legacy['actual_reports_path']).read_bytes();raw=gzip.decompress(packed)
assert hashlib.sha256(packed).hexdigest()==legacy['actual_reports_gzip_sha256']and hashlib.sha256(raw).hexdigest()==legacy['actual_reports_sha256']
actuals=json.loads(raw)
for row in legacy['checks']:
    a,b=actuals[row['actual_key']].copy(),load(ROOT/row['expected_path'])
    assert a.pop('rules_sha256')==row['rules_sha256']and b.pop('rules_sha256')==row['previous_rules_sha256']
    assert a==b,row['case']
browser_path=OUT/'jin-local-player/result.json';browser=load(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320
assert browser['final_view']['players'][0]['character_id']==6 and browser['source6_forward_attack']['Action']==19 and browser['source6_forward_attack']['Pose']==131
assert browser['final_view']['players'][0]['input_channel']==0
for p,h in browser['source_sha256'].items():assert sha(ROOT/p)==h,p
checks=[]
for name in ('jin-vet.txt','jin-wasm-vet.txt','jin-player-tsc.txt'):
    p=LOCAL/name;assert p.is_file()and not p.read_text().strip(),p
    destination=OUT/name;shutil.copyfile(p,destination);checks.append({**bound(destination),'pass':True})
red=[]
for name in ('fighter6-world-red.jsonl','jin-session-cli-red.txt','jin-player-ui-red.txt'):
    p=LOCAL/name;assert p.is_file()and p.stat().st_size>0
    destination=OUT/name
    if p.suffix=='.txt':destination.write_text(p.read_text().expandtabs(8).rstrip()+'\n')
    else:shutil.copyfile(p,destination)
    red.append({**bound(destination),'raw_log_sha256':sha(p)})
sources={str(p.relative_to(ROOT)):sha(p)for folder in ('engine','runtimebridge','cmd')for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if p.is_file()and(p.suffix in('.py','.sh','.tsx','.css','.mjs')or p.name=='tsconfig.json')})
evidence=[ROOT/'games/yuyuhakusho'/p for p in ['engine/testdata/fighter6-attacks-validation.json','engine/testdata/fighter6-motion-validation.json','engine/testdata/fighter6-support-motion-validation.json','engine/testdata/fighter6-composition-validation.json','engine/testdata/chu-world-fighter6-manifest.json','engine/testdata/chu-world-fighter6-special-manifest.json','engine/testdata/chu-world-fighter6-ai-manifest.json','data/chu-world-rules-manifest.json','engine/testdata/fighter6-ai-natural-validation.json','engine/testdata/fighter6-ai-r4-validation.json']]
for name in ('jin-native-shards.json','jin-cleanup.json'):
    if(OUT/name).is_file():evidence.append(OUT/name)
evidence.extend(OUT/name for name in ('jin-test-first.json','jin-resource-reuse.json'))
for path in evidence[:4]:assert load(path)['pass'],path
evidence.extend(ROOT/'docs/research/md-yuyuhakusho'/name for name in ('fighter6-main-wait-evidence.json','SOURCE6_CPU_MENU.md','source6-ai-selection.json','source6-ai-world-boundary.json','source6-special-discovery.json'))
for path,digest in load(OUT/'jin-test-first.json')['files'].items():assert sha(ROOT/path)==digest,path
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'8c2ed877','stage':'Jin complete shared rules and eight-character independent matches',
    'pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':68,
    'integration_checks':logs,'fresh_matches':matches,'legacy_regression':bound(legacy_path),'browser_e2e':bound(browser_path),'targeted_checks':checks,'test_first_evidence':red,
    'original_world':{'normal_passes':2685,'normal_callbacks':35242,'normal_clear_video_label':4485,'special_passes':2401,'special_callbacks':31617,'special_video_end':4200,'single_initial_state':True,'ai_passes':2280,'ai_callbacks':26366,'ai_first_video_label':2533,'ai_clear_video_label':4813,'actual_original_ai6_and_independent_go_world_match':True,'main38_exit_video_label':3761},
    'supporting_evidence':[bound(p)for p in evidence],'current_source_sha256':sources,
    'limits':['Only source IDs0/1/2/3/6/8/9/10 and default arena are playable; three other characters and seven arenas remain.',
        'Source6 human DD request4 is shadowed by later request6. No successful natural34 is claimed, including the bounded originalAI6 search; private originalCPU and actual-clock flows cover it.',
        'Human component special entry limits3600 leave38 active; the separate original4200-world corpus and single-initial-state Go replay prove its complete exit and recovery.',
        'Original AI6 natural calls and the complete practice AI6 Go world are distinct evidence. The passive AI sampler itself does not claim Go acceptance.',
        'Only practice maina01e/stage32 ready-match wait is supported; later practice menu/survival progression lies outside the retained match boundary.',
        'Rule package/schema/indexed media are unchanged. Historical older rule digests are separated from exact prior world/event/lifecycle regression.',
        'No platform room/network/prediction/takeover or audio-playback acceptance, CDN upload, release or deployment.']}
(OUT/'jin-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'tests_per_environment':len(expected_tests),'full_native_wasm_scenarios':30,'legacy_reports':154,'progress_estimate_percent':68}))
