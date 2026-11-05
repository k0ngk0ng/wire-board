"""Audit repeatable client-bridge evidence and bind it to current source hashes.

Run the Go commands stored beside the JSONL logs and the browser E2E to refresh
results, then run this audit. It never creates a passing result for partial logs.
"""
from datetime import datetime,timezone
from pathlib import Path
import hashlib,json,os,subprocess

ROOT=Path(__file__).resolve().parents[2]
DATA=ROOT/'games/yuyuhakusho/runtimebridge/testdata'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def read(path):return json.loads(path.read_text())
def events(path):return [json.loads(line)for line in path.read_text().splitlines()]
def relative(path):return str(path.relative_to(ROOT))
def require(value,message):
    if not value:raise RuntimeError(message)
def frozen():
    manifest=read(DATA/'schema2-test-first.json');expected=dict(manifest['sha256'])
    lineage=read(DATA/'schema2-invalid-present-lineage.json')
    corrected='games/yuyuhakusho/runtimebridge/schema2_test.go'
    require(expected[corrected]==lineage['old_sha256'],'correction lineage no longer links original test')
    require(sha(ROOT/lineage['old_test'])==lineage['old_sha256'],'original test archive changed')
    expected[corrected]=lineage['new_sha256']
    expected.update(read(ROOT/'games/yuyuhakusho/resourceid/testdata/test-first.json')['sha256'])
    browser=read(DATA/'browser-test-first.json');expected[browser['test']]=browser['test_sha256']
    expected['games/yuyuhakusho/runtimebridge/testdata/browser-failures.md']=browser['failure_list_sha256']
    for name,digest in expected.items():require(sha(ROOT/name)==digest,'frozen test or fixture changed: '+name)
    require('local page did not use exact-byte network assets' in (DATA/'browser-missing-red.txt').read_text(),'browser missing actual preimplementation red')
    return expected

def source_paths():
    temp=ROOT/'.local/md-yuyuhakusho/tmp';temp.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp))
    output=subprocess.check_output(['go','list','-deps','-test','-json','./games/yuyuhakusho/runtimebridge','./games/yuyuhakusho/resourceid','./games/yuyuhakusho/presentation'],env=env,cwd=ROOT,text=True)
    decoder=json.JSONDecoder();paths=set()
    while output.strip():
        package,end=decoder.raw_decode(output.lstrip());output=output.lstrip()[end:]
        if package.get('Module',{}).get('Path')!='github.com/k0ngk0ng/wire-board':continue
        for key in ('GoFiles','CgoFiles','TestGoFiles','XTestGoFiles','EmbedFiles'):
            for name in package.get(key,[]):
                path=Path(package['Dir'])/name
                if path.is_file()and path.is_relative_to(ROOT)and'.local'not in path.relative_to(ROOT).parts:paths.add(path)
    for name in ('main_js.go',):paths.add(ROOT/'games/yuyuhakusho/cmd/local-player-wasm'/name)
    for name in ('local_player.tsx','serve_local_player.py','check_runtimebridge_schema2.py','verify_client_bridge.py'):
        paths.add(ROOT/'scripts/yuyuhakusho'/name)
    for name in ('go.mod','go.sum'):paths.add(ROOT/name)
    for path in (ROOT/'games/yuyuhakusho/data').glob('*'):
        if path.name in {'chu-world-rules.json.gz','world-sprites.json.gz','hud-assets.json.gz','renderstate-metadata.json.gz','runtime-resource-descriptor.json','runtime-resource-descriptor-manifest.json'}or path.name.startswith('arena-')and path.suffix=='.gz':paths.add(path)
    for name in ('bridge-reference.json','bridge-reference-manifest.json'):
        path=ROOT/'games/yuyuhakusho/renderstate/testdata'/name
        if path.exists():paths.add(path)
    return sorted(paths)

def suite(logs,expected=None):
    passed=[];artifacts={};starts=[]
    for path in logs:
        rows=events(path);require(rows,'empty log: '+relative(path));starts.append(rows[0]['Time'])
        require(not any(row.get('Action')=='fail'for row in rows),'failed log: '+relative(path))
        packages=[row for row in rows if row.get('Action')in('pass','fail')and'Test'not in row]
        require(packages and all(row['Action']=='pass'for row in packages),'incomplete log: '+relative(path))
        tests=[row['Test']for row in rows if row.get('Action')=='pass'and'Test'in row]
        passed.extend(tests);artifacts[relative(path)]={'sha256':sha(path),'packages':packages,'tests_passed':len(tests)}
    if expected is not None:
        prefix='TestSchema2FrozenWholeTrajectories/'
        flows=[name[len(prefix):]for name in passed if name.startswith(prefix)]
        require(len(flows)==len(expected)and set(flows)==set(expected),'full trajectory coverage missing or duplicated')
    return {'pass':True,'earliest_log_event':min(starts),'passed_tests':passed,'logs':artifacts}

def audit():
    frozen_sha=frozen();paths=source_paths();source_sha={relative(path):sha(path)for path in paths}
    reference=read(ROOT/'games/yuyuhakusho/renderstate/testdata/bridge-reference.json')['rows'];names=[row['configuration']['name']for row in reference]
    rows={
      'native49':suite([DATA/f'schema2-flows-native-{i}.jsonl'for i in range(4)],names),
      'wasm49':suite([DATA/f'schema2-flows-wasm-{i}.jsonl'for i in range(4)],names),
      'boundaries_native_race':suite([DATA/'schema2-boundaries-native.jsonl']),
      'boundaries_wasm':suite([DATA/'schema2-boundaries-wasm.jsonl']),
      'selected_full_race':suite([DATA/'schema2-flows-race.jsonl'],['legacy-four','arena-a-pan','arena-e-team']),
      'legacy_native_race':suite([DATA/'schema1-legacy-regression-native.jsonl']),
      'resourceid_native_race':suite([ROOT/'games/yuyuhakusho/resourceid/testdata/native.jsonl']),
    }
    require(read(DATA/'schema2-vet.command.json')['exit_code']==0,'vet failed')
    for key in ('native','wasm'):
        commands=read(DATA/f'schema2-flows-{key}-commands.json')
        require(len(commands)==4 and all(row['exit_code']==0 for row in commands),key+' command metadata incomplete')
    # Test logs predate collection. Record and verify source mtimes rather than
    # claiming a source snapshot was taken before these already completed runs.
    go_sources=[path for path in paths if path.suffix=='.go'and'cmd'not in path.relative_to(ROOT).parts]
    latest=max(path.stat().st_mtime for path in go_sources)
    for key in ('native49','wasm49','boundaries_native_race','boundaries_wasm','selected_full_race','legacy_native_race'):
        started=datetime.fromisoformat(rows[key]['earliest_log_event']).timestamp()
        require(latest<=started,'Go dependency changed after test start: '+key)
        rows[key]['go_source_mtimes_before_log_start']=True
    browser_path=ROOT/'games/yuyuhakusho/verification/client-bridge-schema2/result.json';browser=read(browser_path)
    require(browser['pass'],'browser E2E failed')
    for name,digest in browser['source_sha256'].items():require(sha(ROOT/name)==digest,'browser source changed: '+name)
    report={'schema':1,'pass':True,'collected_at_utc':datetime.now(timezone.utc).isoformat(),'source_binding':'Collected after runs; Go dependency mtimes verified before logged starts, browser records exact source SHA. No pre-run source capture is claimed.','scope':'schema2 exact-byte resources, sole computational presentation, schema1 compatibility, checkpoint, predicted F+1 preview, full49 native/Wasm and local browser; no network latency or audio acceptance','reference_configurations':len(names),'reference_ticks':sum(row['ticks']for row in reference),'frozen_test_sha256':frozen_sha,'source_sha256':source_sha,'suites':rows,'browser':{'path':relative(browser_path),'sha256':sha(browser_path),'pass':True},'typescript':{'exit_code':0,'log':relative(DATA/'browser-typescript.txt'),'sha256':sha(DATA/'browser-typescript.txt')},'vet':read(DATA/'schema2-vet.command.json')}
    require(not(DATA/'browser-typescript.txt').read_text().strip(),'TypeScript emitted diagnostics')
    (DATA/'schema2-verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'pass':True,'reference_configurations':len(names),'reference_ticks':report['reference_ticks'],'source_files':len(source_sha),'artifact':relative(DATA/'schema2-verification.json')},ensure_ascii=False))

if __name__=='__main__':audit()
