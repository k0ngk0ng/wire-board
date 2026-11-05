"""Verify and bind the completed source5 ten-character integration evidence."""
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
    assert any(r.get('Action')=='pass'and not r.get('Test')for r in rows),name
    return dst,rows

def passed(rows):return [(r['Package'],r['Test'])for r in rows if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
inv=read(OUT/'sensui-native-shards.json')
expected={(p['package'],n)for p in inv['packages']for n in p['tests']}
assert len(expected)==354
for path,digest in inv['source_sha256'].items():assert sha(ROOT/path)==digest,path
checks=[]
for target in ('native','wasm'):
    names=[f'sensui-native-shard-{i}.jsonl'for i in range(4)]+['sensui-native-animation.jsonl']if target=='native'else['sensui-integration-wasm.jsonl']
    names_seen=[];logs=[]
    recovery=None
    for name in names:
        p,rows=log(name,allow_timeout=target=='wasm');names_seen+=passed(rows);logs.append(bound(p))
    if target=='wasm':
        recovery_path=OUT/'sensui-wasm-continuation.json';recovery=read(recovery_path)
        assert recovery['source_sha256']==inv['source_sha256']
        assert sha(OUT/'sensui-integration-wasm.jsonl')==recovery['initial_log_sha256']
        assert len(names_seen)==recovery['initial_top_passes']==279
        pending={tuple(row) for row in recovery['remaining']}
        assert pending==expected-set(names_seen) and len(pending)==75
        p,rows=log('sensui-integration-wasm-remaining.jsonl')
        assert set(passed(rows))==pending and len(passed(rows))==75
        names_seen+=passed(rows);logs.append(bound(p));recovery=bound(recovery_path)
    assert len(names_seen)==len(set(names_seen))==len(expected)and set(names_seen)==expected,target
    checks.append({'environment':target,'pass':True,'tests':len(expected),'invocations':logs,'timeout_continuation':recovery,'passed_tests':[{'package':p,'test':n}for p,n in sorted(names_seen)]})
assets=[]
for target in ('native','wasm'):
    p,rows=log(f'sensui-assets-{target}.jsonl');names=passed(rows)
    assert len(names)==len(set(names))==27
    assets.append({'environment':target,'pass':True,'tests':27,'log':bound(p)})

matches=[]
for kind,count in [('rules',23),('render',13)]:
    paths=sorted((OUT/'sensui-e2e/native'/kind).glob('*.json'));assert len(paths)==count
    for p in paths:
        other=OUT/'sensui-e2e/wasm'/kind/p.name;a,b=read(p),read(other)
        assert a['pass']and b['pass']and 5 in a['characters']and not a['reference_state_loaded'],p
        assert a.pop('environment')!=b.pop('environment')and a==b,(p,'native/Wasm trace differs')
        if kind=='rules':assert a['completed']and a['human_auto_takeovers']==0
        else:assert a['complete']and a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':kind+'/'+p.stem,'native':bound(p),'wasm':bound(other),'ticks':a['ticks'],'characters':a['characters'],'final_state_sha256':a['final_state_sha256'],**({'trace_sha256':a['trace_sha256']}if kind=='rules'else{'pixel_trace_sha256':a['pixel_trace_sha256']})})
legacy_path=OUT/'sensui-legacy-regression.json';legacy=read(legacy_path)
assert legacy['pass']and len(legacy['checks'])==234 and all(r['pass']for r in legacy['checks'])
packed=(ROOT/legacy['actual_reports_path']).read_bytes();raw=gzip.decompress(packed)
assert hashlib.sha256(packed).hexdigest()==legacy['actual_reports_gzip_sha256']and hashlib.sha256(raw).hexdigest()==legacy['actual_reports_sha256']
actuals=json.loads(raw)
for row in legacy['checks']:
    a,b=actuals[row['actual_key']].copy(),read(ROOT/row['expected_path'])
    assert a.pop('rules_sha256')==row['rules_sha256']and b.pop('rules_sha256')==row['previous_rules_sha256']
    assert a==b,row['case']
browser_path=OUT/'sensui-local-player/result.json';browser=read(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320 and browser['human_idle_no_takeover']
assert browser['final_view']['players'][0]['character_id']==5 and browser['final_view']['players'][0]['input_channel']==0
assert browser['source5_forward_attack']['Action']==3 and browser['source5_forward_attack']['Pose']==131
for p,h in browser['source_sha256'].items():assert sha(ROOT/p)==h,p
extra=[]
for name in ('sensui-vet.txt','sensui-wasm-vet.txt','sensui-player-tsc.txt'):
    p=LOCAL/name;assert p.is_file()and not p.read_text().strip();dst=OUT/name;shutil.copyfile(p,dst);extra.append({**bound(dst),'pass':True})
shutil.copyfile(LOCAL/'sensui-player.txt',OUT/'sensui-player.txt')
worlds=[]
for name,counts in [('chu-world-fighter5',(3116,41188,None)),('chu-world-fighter5-special',(1801,24007,12058))]:
    p=ROOT/f'games/yuyuhakusho/engine/testdata/{name}-manifest.json';m=read(p)
    assert m['pass']and m['frames_match_original_core']and not m['errors']and m['incomplete_calls']==0
    assert (m['records'],m['object_calls'])==counts[:2]
    if counts[2]is not None:assert m['draws']==counts[2]
    f=p.with_name(name+'.json.gz');data=f.read_bytes()
    assert hashlib.sha256(data).hexdigest()==m['gzip_sha256']and hashlib.sha256(gzip.decompress(data)).hexdigest()==m['fixture_sha256']
    worlds.append({'manifest':bound(p),'fixture':bound(f),'passes':counts[0],'callbacks':counts[1],'ordered_draws':counts[2],'first_video_label':m['first_frame'],'last_video_label':m['last_frame'],'single_initial_state':True,'control_frames':m['frames_checked']})
evidence=[ROOT/'games/yuyuhakusho/engine/testdata'/name for name in ['fighter5-attacks-validation.json','fighter5-motion-validation.json','fighter5-projectile-validation.json','fighter5-composition-validation.json','fighter5-storage-test-first.json','fighter5-world-test-first.json']]
for p in evidence[:4]:assert read(p)['pass'],p
asset_path=OUT/'fighter5-projectile-assets.json';assert read(asset_path)['pass'];evidence.append(asset_path)
evidence += [OUT/'sensui-test-first.json',OUT/'sensui-native-shards.json',ROOT/'games/yuyuhakusho/data/chu-world-rules-manifest.json',ROOT/'games/yuyuhakusho/engine/FIGHTER5_WORLD_VERIFICATION.md']
evidence += [ROOT/'docs/research/md-yuyuhakusho'/name for name in ['source5-next-selection.json','source5-special-discovery.json','SOURCE5_SPECIAL_DISCOVERY.md']]
for p,h in read(OUT/'sensui-test-first.json')['files'].items():assert sha(ROOT/p)==h,p
if (OUT/'sensui-cleanup.json').is_file():evidence.append(OUT/'sensui-cleanup.json')
if (ROOT/'games/yuyuhakusho/engine/testdata/fighter5-world-reference-frames.json.gz').is_file():evidence.append(ROOT/'games/yuyuhakusho/engine/testdata/fighter5-world-reference-frames.json.gz')
sources={str(p.relative_to(ROOT)):sha(p)for folder in ('engine','runtimebridge','cmd')for p in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if p.is_file()and(p.suffix in('.py','.sh','.tsx','.css','.mjs')or p.name=='tsconfig.json')})
report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'baseline_commit':'b832d149','stage':'Sensui complete shared rules and ten-character independent matches','pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':74,'integration_checks':checks,'asset_checks':assets,'fresh_matches':matches,'legacy_regression':bound(legacy_path),'browser_e2e':bound(browser_path),'targeted_checks':extra,'original_world':worlds,'supporting_evidence':[bound(p)for p in evidence],'current_source_sha256':sources,
'limits':['Only source IDs0/1/2/3/4/5/6/8/9/10 and default arena are playable; source7 and seven arenas remain.','Special source5 world is active at3600; only the ordinary world claims complete match clear at4916.','Ten selector roots succeeded naturally; only final-C19444 has natural complete followup. Other four followups and other31→32 variants use original private/clock evidence.','Adjacent142b4/1458a slave semantics have isolated evidence but no natural/caller evidence; world does not invent dispatch from HitStopTicks.','Natural source5 original world uses human control; AI uses existing isolated/full-input evidence and independent complete computer matches.','Schema3 adds16 deduplicated AN frames; all indexed media and previous world/event/lifecycle traces remain unchanged.','Raw hardware sprite cache remains outside the rule boundary; ordered callback-time Draw data is compared.','The initial full Wasm invocation hit its20m limit after279 passing tests without assertion failures. The remaining75 tests completed separately with the same frozen sources; both logs and startup-limit diagnostics are retained.','No platform room/network/prediction/takeover/audio playback acceptance, CDN upload, release or deployment.']}
(OUT/'sensui-session.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'rule_tests_per_environment':354,'asset_tests_per_environment':27,'full_native_wasm_scenarios':len(matches),'legacy_reports':234,'progress_estimate_percent':74}))
