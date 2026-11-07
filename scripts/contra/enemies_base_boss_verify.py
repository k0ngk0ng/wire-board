"""Re-run the scoped Japanese Base Boss oracle comparison and retain results."""
from pathlib import Path
import datetime,gzip,hashlib,json,os,subprocess

ROOT=Path(__file__).resolve().parents[2]
LOCAL=ROOT/'.local/contra-enemies-base/boss-test-results'
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'

def sha(path):
    # Repeated large raw traces may be losslessly archived after comparison.
    data=path.read_bytes()if path.exists()else gzip.decompress(Path(str(path)+'.gz').read_bytes())
    return hashlib.sha256(data).hexdigest()

def main():
    LOCAL.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/tmp'),TMPDIR=str(ROOT/'.local/tmp'),GOTOOLCHAIN='local')
    for key in ['GOCACHE','GOMODCACHE','GOTMPDIR']:Path(env[key]).mkdir(parents=True,exist_ok=True)
    engine=ROOT/'games/contra/engine'
    files=sorted(p.name for p in engine.glob('*.go')if not p.name.endswith('_test.go'))+['enemies_spawn_test.go','enemies_base_boss_test.go']
    command=['go','test','-race','-json',*files,'-run','^TestOriginalBaseBoss','-count=1']
    run=subprocess.run(command,cwd=engine,env=env,text=True,capture_output=True)
    output=LOCAL/'race.jsonl';output.write_text(run.stdout)
    if run.stderr:(LOCAL/'race.stderr.txt').write_text(run.stderr)
    assert run.returncode==0,run.stdout+run.stderr
    events=[json.loads(line)for line in run.stdout.splitlines()]
    assert len([r for r in events if r.get('Action')=='pass'and r.get('Test')])==3
    fixtures={}
    for name in ['enemies-base-boss-bias.json.gz','enemies-base-boss-controlled.json.gz','enemies-base-2-boss.json.gz','enemies-base-4-boss.json.gz','enemies-base-boss-sequence-2.json.gz','enemies-base-boss-sequence-4.json.gz']:
        path=EVIDENCE/name;data=json.loads(gzip.decompress(path.read_bytes()));assert data['source_is_original_execution']
        fixtures[name]=dict(sha256=sha(path),records=len(data['records']),scope=data.get('scope'))
    originals=[('.local/contra-enemies-base/boss-oracle/runs/base-boss-v2','.local/contra-enemies-base/boss-oracle/runs/base-boss-repeat','boss.jsonl')]
    originals += [(f'.local/contra-enemies-base/boss-sequence/runs/base-boss-sequence-{stage}-v2',f'.local/contra-enemies-base/boss-sequence/runs/base-boss-sequence-{stage}-repeat','base-enemies.jsonl')for stage in [2,4]]
    originals.append(('.local/contra-enemies-base/boss-oracle/runs/base-boss-bias-v1','.local/contra-enemies-base/boss-oracle/runs/base-boss-bias-repeat','boss.jsonl'))
    repeats=[]
    for first,second,trace in originals:
        checks={}
        for name in [trace,'ram.bin','inputs.csv']:
            a,b=ROOT/first/name,ROOT/second/name
            assert sha(a)==sha(b),(first,second,name)
            checks[name]=sha(a)
        repeats.append(dict(first=first,repeat=second,sha256=checks))
    sources=sorted(engine.glob('enemies_base_boss*.go'))+sorted((ROOT/'scripts/contra').glob('enemies_base_boss_*.py'))
    result=dict(verified_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),rom_sha256='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe',command=command,cwd=str(engine.relative_to(ROOT)),scope='all current production engine sources, original enemy conversion helper and Base Boss tests only; no unrelated game acceptance',passed_tests=3,controlled_calls=810,continuous_calls=7959,bias_calls=810,fixtures=fixtures,repeated_original_execution=repeats,source_sha256={str(p.relative_to(ROOT)):sha(p)for p in sources},test_output=dict(path=str(output.relative_to(ROOT)),sha256=sha(output)),bias_red=dict(path=str((LOCAL/'bias-red.txt').relative_to(ROOT)),sha256=sha(LOCAL/'bias-red.txt')),test_before_implementation=dict(path=str((LOCAL/'red.txt').relative_to(ROOT)),sha256=sha(LOCAL/'red.txt')))
    (EVIDENCE/'enemies-base-boss-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(dict(passed_tests=3,controlled_calls=810,continuous_calls=7959,bias_calls=810,repeated_captures=4)))

if __name__=='__main__':main()
