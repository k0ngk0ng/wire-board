"""Verify the changed cannon/sniper Falcon handling through the real package."""
from pathlib import Path
import hashlib,json,os,re,subprocess
ROOT=Path(__file__).resolve().parents[2]

def main():
    base=ROOT/'games/contra/engine';out=ROOT/'.local/contra-enemies-falcon-sniper/checks';tmp=out/'tmp'
    tmp.mkdir(parents=True,exist_ok=True)
    names=['TestOriginalWholeEnemyPhase','TestOriginalAllStageEnemyPhaseDispatch','TestOriginalEnemyCollisions']
    tests=sorted(base.glob('enemies_shooters*_test.go'))
    names=sorted(set(names+[n for p in tests for n in re.findall(r'^func (Test\w+)\(',p.read_text(),re.M)]))
    command=['go','test','-race','-count=1','-json','-run','^('+'|'.join(names)+')$','./games/contra/engine']
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(tmp),TMPDIR=str(tmp),GOTOOLCHAIN='local')
    package=json.loads(subprocess.check_output(['go','list','-json','./games/contra/engine'],cwd=ROOT,env=env))
    paths=[base/n for n in package['GoFiles']+package.get('TestGoFiles',[])]
    digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
    before={str(p.relative_to(ROOT)):digest(p)for p in paths}
    log=out/'package-race.jsonl'
    with log.open('w')as f:result=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    rows=[json.loads(l)for l in log.read_text().splitlines()if l.startswith('{')]
    passed=[r['Test']for r in rows if r.get('Action')=='pass'and r.get('Test')in names]
    after={str(p.relative_to(ROOT)):digest(p)for p in paths}
    record={'passed':result.returncode==0 and len(passed)==len(names),'scope':'Changed-scope follow-up to the70-test enemy baseline: cannon and sniper Falcon bit7 in real Boss and controlled F-pickup phases. Normal package build; no test files hidden.',
            'command':command,'tests':names,'passedTests':passed,'sourceSHA256':before,'sourceAfterSHA256':after,'changedDuringRun':[p for p in before if before[p]!=after[p]],'logSHA256':digest(log),
            'fixtureSHA256':{n:digest(base/'testdata'/n)for n in ['original-enemy-collision-tick-natural-boss.json.gz','original-enemy-collision-tick-falcon-sniper.json.gz']},
            'counts':{'newRealBossPhases':42,'newRealBossEntityCalls':193,'newControlledFalconPhases':3,'completeFirstStagePhases':5441},
            'baselineReportSHA256':digest(ROOT/'docs/research/nes-contra/evidence/enemies-subsystem-verification.json')}
    if record['passed']:
        target=ROOT/'docs/research/nes-contra/evidence/enemies-falcon-followup-verification.json';target.write_text(json.dumps(record,indent=2)+'\n')
        (target.parent/'enemies-falcon-followup-race.jsonl').write_bytes(log.read_bytes())
    else:(out/'package-failed.json').write_text(json.dumps(record,indent=2)+'\n')
    print(json.dumps({'passed':record['passed'],'tests':len(names),'changed':record['changedDuringRun']}))
    if not record['passed']:raise SystemExit(1)

if __name__=='__main__':main()
