"""Bind the two-character integration checks after they have actually passed."""
from datetime import datetime, timezone
from pathlib import Path
import hashlib, json, shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def bound(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path)}

logs=[]
for target in ('native','wasm'):
    names=['fighter-native-shard-0.jsonl','fighter-native-shard-1.jsonl','fighter-native-animation.jsonl']if target=='native'else['fighter-integration-wasm.jsonl']
    raw=b'';invocations=[]
    for name in names:
        source=LOCAL/name;data=source.read_bytes();parts=[json.loads(line)for line in data.decode().splitlines()]
        assert not any(row.get('Action')in('fail','build-fail')for row in parts),source
        assert any(row.get('Action')=='pass'and not row.get('Test')for row in parts),source
        raw+=data
        if target=='native':
            destination=OUT/name.replace('fighter-native','fighter10-native');shutil.copyfile(source,destination);invocations.append(bound(destination))
    rows=[json.loads(line)for line in raw.decode().splitlines()]
    packages={}
    for row in rows:
        if row.get('Action')=='pass'and not row.get('Test'):packages[row['Package']]=packages.get(row['Package'],0)+row['Elapsed']
    assert len(packages)==2 and any(path.endswith('/engine')for path in packages),names
    tests=[{'package':row['Package'],'test':row['Test']}for row in rows if row.get('Action')=='pass'and row.get('Test')and'/'not in row['Test']]
    assert len(tests)>=158 and len({(test['package'],test['test'])for test in tests})==len(tests),(names,len(tests))
    destination=OUT/('fighter10-engine-'+target+'.jsonl');destination.write_bytes(raw)
    logs.append({**bound(destination),'pass':True,'top_level_tests':tests,'package_seconds_sum_of_invocations':packages,'invocations':invocations})
assert len(logs[0]['top_level_tests'])==len(logs[1]['top_level_tests'])
assert {(t['package'],t['test'])for t in logs[0]['top_level_tests']}=={(t['package'],t['test'])for t in logs[1]['top_level_tests']}

matches=[]
for kind,names in [('rules',('ten-ten','ten-chu','chu-ten','three','four','team','human-idle')),('render',('ten-ten','ten-chu','four','team'))]:
    for name in names:
        native=OUT/'fighter10-e2e/native'/kind/(name+'.json')
        wasm=OUT/'fighter10-e2e/wasm'/kind/(name+'.json')
        a,b=load(native),load(wasm)
        assert a['pass']and b['pass']and 10 in a['characters']and not a['reference_state_loaded'],name
        assert a.pop('environment')!=b.pop('environment')
        assert a==b,(kind,name,'complete state/pixel trace differs')
        if kind=='rules':
            assert a['completed']and a['human_auto_takeovers']==0
            assert 0<a['first_damage_tick']<=a['first_ko_tick']<=a['retirement_tick']<=a['result_tick']<=a['clear_tick']
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+name,'native':bound(native),'wasm':bound(wasm),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'pixel_trace_sha256':a['pixel_trace_sha256']}if kind=='render'else{'trace_sha256':a['trace_sha256']})})

legacy=OUT/'fighter10-legacy-regression.json';regression=load(legacy)
assert regression['pass']and len(regression['checks'])==10
assert all(case['pass']and case['identical_json_bytes']for case in regression['checks'])
browser_path=OUT/'fighter10-local-player/result.json';browser=load(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320
assert browser['final_view']['players'][0]['character_id']==10 and browser['source10_retreat_action']['Action']==17
assert browser['final_view']['players'][0]['input_channel']==0
for name,digest in browser['source_sha256'].items():assert sha(ROOT/name)==digest,name

checks=[]
for name in ('fighter-vet.txt','fighter-wasm-vet.txt','fighter-player-tsc.txt'):
    source=LOCAL/name;assert source.is_file()and not source.read_text().strip(),source
    destination=OUT/name;shutil.copyfile(source,destination)
    checks.append({**bound(destination),'pass':True})
red=[]
for name in ('fighter-world-red.jsonl','fighter-session-cli-red.txt','fighter-player-ui-red.txt'):
    source=LOCAL/name;assert source.is_file()and source.stat().st_size>0
    destination=OUT/name
    if source.suffix=='.txt':
        destination.write_text(source.read_text().expandtabs(8).rstrip()+'\n')
        red.append({**bound(destination),'raw_log_sha256':sha(source),'display_formatting':'Expanded tabs and removed surplus trailing blank lines; diagnostic text unchanged'})
    else:
        shutil.copyfile(source,destination);red.append(bound(destination))

sources={str(path.relative_to(ROOT)):sha(path)for folder in ('engine','runtimebridge','cmd')for path in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(path.relative_to(ROOT)):sha(path)for path in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if path.is_file()and(path.suffix in ('.py','.sh','.tsx','.css','.mjs')or path.name=='tsconfig.json')})
evidence=[ROOT/'games/yuyuhakusho'/path for path in ['engine/testdata/fighter10-attacks-validation.json','engine/testdata/fighter10-motion-validation.json','engine/testdata/fighter10-composition-validation.json','engine/testdata/fighter10-composition-input-view-correction.json','engine/testdata/chu-world-fighter10-manifest.json','engine/testdata/fighter10-inputs.json']]
for name in ('fighter10-cleanup.json','fighter10-native-shards.json','fighter10-native-timeout.json'):
    path=OUT/name
    if path.is_file():evidence.append(path)
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'149cc02d','stage':'Source10 complete independent fighter, shared rules and mixed-character sessions',
    'pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':48,
    'integration_checks':logs,'fresh_matches':matches,'legacy_regression':bound(legacy),'browser_e2e':bound(browser_path),'targeted_checks':checks,'test_first_evidence':red,
    'original_world':{'passes':2789,'callbacks':36871,'first_video_label':1800,'clear_video_label':4590,'single_initial_state':True},
    'supporting_evidence':[bound(path)for path in evidence],'current_source_sha256':sources,
    'limits':['Playable source IDs9/10 and the default arena only; the remaining nine characters and seven arenas are unfinished.',
        'Natural source10 evidence is human10 versus original AI9; source10 AI has original private input/actor evidence and fresh independent matches, not a natural source10 AI recording.',
        'Private full-action streams use original animation submission and run all five added slots to recovery; this does not turn isolated runs into natural gameplay.',
        'No platform room/network/prediction/takeover integration or audio playback acceptance yet; no publishing or deployment.']}
(OUT/'fighter10-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'rule_tests_per_environment':len(logs[0]['top_level_tests']),'complete_native_wasm_scenarios':len(matches),'legacy_byte_identical_reports':10,'progress_estimate_percent':48}))
