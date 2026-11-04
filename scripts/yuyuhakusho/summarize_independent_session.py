"""Bind the current independent-session evidence; this does not run a game."""
from datetime import datetime, timezone
from pathlib import Path
import hashlib, json, shutil

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification'
LOCAL=ROOT/'.local/md-yuyuhakusho'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def load(path):return json.loads(path.read_text())
def bound(path):return {'path':str(path.relative_to(ROOT)),'sha256':sha(path)}
def write(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
def go_log(local_name,canonical):
    source=LOCAL/local_name
    rows=[json.loads(line)for line in source.read_text().splitlines()]
    assert not any(row.get('Action')in ('fail','build-fail')for row in rows),source
    packages={row['Package']:row['Elapsed']for row in rows if row.get('Action')=='pass'and not row.get('Test')}
    tests=[{'package':row['Package'],'test':row['Test']}for row in rows if row.get('Action')=='pass'and row.get('Test')and'/'not in row['Test']]
    assert packages and tests,source
    destination=OUT/canonical
    shutil.copyfile(source,destination)
    return {**bound(destination),'pass':True,'package_seconds':packages,'top_level_tests':tests}

logs=[go_log('world-session-integration-'+kind+'.jsonl','independent-engine-'+kind+'.jsonl')for kind in ('native','wasm')]
logs += [go_log('session-presentation-integration-'+kind+'.jsonl','independent-presentation-'+kind+'.jsonl')for kind in ('native','wasm')]
assert len(logs[0]['top_level_tests'])==len(logs[1]['top_level_tests'])>=142
assert len(logs[2]['top_level_tests'])==len(logs[3]['top_level_tests'])>=27

matches=[]
for directory,names in [('session-e2e',('two-ai','three-ai','four-ai','team-ai','mixed-idle')),('runtime-e2e',('two-ai','team-ai'))]:
    for name in names:
        native=OUT/directory/'native'/(name+'.json')
        wasm=OUT/directory/'wasm'/(name+'.json')
        a,b=load(native),load(wasm)
        assert a['pass']and b['pass']and not a['reference_state_loaded']and not b['reference_state_loaded'],name
        assert a.pop('environment')!=b.pop('environment'),name
        assert a==b,(name,'complete native/Wasm trace or state differs')
        if directory=='runtime-e2e':assert a['hud_window_complete']and a['restored_pixel_frames']==30
        matches.append({'case':directory+'/'+name,'native':bound(native),'wasm':bound(wasm),'ticks':a['ticks'],'final_state_sha256':a['final_state_sha256'],**({'pixel_trace_sha256':a['pixel_trace_sha256']}if'pixel_trace_sha256'in a else {'trace_sha256':a['trace_sha256']})})

browser_path=OUT/'local-player/result.json'
browser=load(browser_path)
assert browser['pass']and browser['input_trusted']and browser['viewport']==320
assert browser['final_view']['window_complete']and browser['final_view']['players'][0]['input_channel']==0
for path,digest in browser['source_sha256'].items():assert sha(ROOT/path)==digest,path
checks=[]
for name in ('independent-vet.txt','independent-wasm-vet.txt','local-player-tsc.txt'):
    source=LOCAL/name
    assert source.exists()and not source.read_text().strip(),source
    destination=OUT/name
    shutil.copyfile(source,destination)
    checks.append({**bound(destination),'pass':True})

red=[]
for source_name,destination in [
    ('world-frame-red.jsonl','engine/testdata/world-frame-red.jsonl'),
    ('chu-session-red.jsonl','engine/testdata/chu-session-red.jsonl'),
    ('world-draw-red.jsonl','engine/testdata/world-draw-red.jsonl'),
    ('chu-session-cli-red.txt','verification/chu-session-cli-red.txt'),
    ('chu-runtime-red.txt','verification/chu-runtime-red.txt'),
    ('chu-runtime-hud-red.txt','verification/chu-runtime-hud-red.txt'),
    ('local-player-red.txt','verification/local-player-red.txt'),
    ('local-player-wasm-red.txt','verification/local-player-wasm-red.txt'),
    ('local-player-channel-red.txt','verification/local-player-channel-red.txt'),
]:
    source=LOCAL/source_name
    target=ROOT/'games/yuyuhakusho'/destination
    assert source.is_file()and source.stat().st_size>0,source
    shutil.copyfile(source,target)
    red.append(bound(target))

sources={str(path.relative_to(ROOT)):sha(path)for folder in ('engine','presentation','runtimebridge','cmd')for path in sorted((ROOT/'games/yuyuhakusho'/folder).rglob('*.go'))}
sources.update({str(path.relative_to(ROOT)):sha(path)for path in sorted((ROOT/'scripts/yuyuhakusho').iterdir())if path.is_file()and(path.suffix in ('.py','.sh','.mjs','.tsx','.css')or path.name=='tsconfig.json')})
data_paths=['data/chu-world-rules.json.gz','data/chu-world-rules-manifest.json','data/world-sprites.json.gz','data/arena-assets-default.json.gz','data/hud-assets.json.gz',
    'engine/testdata/chu-world-validation.json','engine/testdata/chu-world-start-cells-validation.json','engine/testdata/world-frame-irq-validation.json',
    'engine/testdata/match-start-validation.json','engine/testdata/match-overlays-validation.json','engine/testdata/default-arena-validation.json',
    'engine/testdata/world-objects-validation.json','engine/testdata/fighter-ai-validation.json','data/world-sprites-validation.json','data/arena-presentation-validation.json','data/hud-validation.json','data/audio-validation.json']
evidence=[bound(ROOT/'games/yuyuhakusho'/path)for path in data_paths]
for path in [ROOT/'games/yuyuhakusho/data/audio-loop-validation.json',ROOT/'docs/research/md-yuyuhakusho/static/fighter-action-coverage.json',OUT/'independent-session-cleanup.json']:
    if path.is_file():evidence.append(bound(path))

report={'verified_at_utc':datetime.now(timezone.utc).isoformat(),'stage':'Fresh independent Chu sessions, complete object scheduling, indexed world rendering and local browser input',
    'pass':True,'goal_complete':False,'release_ready':False,'overall_progress_estimate_percent':45,
    'counts':{'native_top_level_tests':len(logs[0]['top_level_tests'])+len(logs[2]['top_level_tests']),'wasm_top_level_tests':len(logs[1]['top_level_tests'])+len(logs[3]['top_level_tests']),
        'fresh_rule_scenarios':5,'fresh_rendered_scenarios':2,'original_world_passes':4758,'original_world_callbacks':62333},
    'integration_checks':logs,'fresh_native_wasm_matches':matches,'browser_e2e':bound(browser_path),'targeted_checks':checks,'test_first_evidence':red,
    'supporting_evidence':evidence,'current_source_sha256':sources,
    'limits':['Only source character9 and the default arena are available in fresh playable sessions; all-character AI/catalog coverage does not establish other-character attacks.',
        'Original-call and selected original-frame pixel evidence remain separate from complete fresh native/Wasm state and pixel equality.',
        'No platform registration, room/ready/takeover integration, prediction, authority synchronization or network interference/latency acceptance yet.',
        'The local research player is silent; acquired audio is not a validated original mixing/playback scheduler.',
        'No CDN upload, release, deployment or main-worktree changes.']}
write(OUT/'independent-session.json',report)
print(json.dumps({'pass':True,'tests_per_environment':report['counts']['native_top_level_tests'],'fresh_scenarios':len(matches),'progress_estimate_percent':45},ensure_ascii=False))
