"""Run enemy/graphics checks through the actual package's build constraints.

Only owned enemy/graphics tests execute. All actual package inputs are hashed;
do not compile an artificial file list that can hide GOARCH filename mistakes.
"""
from pathlib import Path
import hashlib,json,os,re,subprocess

ROOT=Path(__file__).resolve().parents[2]

def main():
    base=ROOT/'games/contra/engine';out=ROOT/'.local/contra-enemies/checks';out.mkdir(parents=True,exist_ok=True)
    temp=ROOT/'.local/contra-enemies/tmp';temp.mkdir(parents=True,exist_ok=True)
    tests=sorted(base.glob('enemies*_test.go'))+[base/'graphics_queue_test.go']
    names=[name for p in tests for name in re.findall(r'^func (Test\w+)\(',p.read_text(),re.M)]
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp),GOTOOLCHAIN='local')
    package=json.loads(subprocess.check_output(['go','list','-json','./games/contra/engine'],cwd=ROOT,env=env))
    files=sorted(package['GoFiles']+package.get('TestGoFiles',[]))
    command=['go','test','-race','-count=1','-json','-run','^('+'|'.join(names)+')$','./games/contra/engine']
    source={str((base/p).relative_to(ROOT)):hashlib.sha256((base/p).read_bytes()).hexdigest()for p in files}
    log=out/'subsystem-race.jsonl'
    with log.open('w')as f:result=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    rows=[]
    for line in log.read_text().splitlines():
        try:rows.append(json.loads(line))
        except json.JSONDecodeError:pass
    passed=[row['Test']for row in rows if row.get('Action')=='pass'and row.get('Test')in names]
    after={str((base/p).relative_to(ROOT)):hashlib.sha256((base/p).read_bytes()).hexdigest()for p in files}
    record={'passed':result.returncode==0 and len(passed)==len(names),'scope':'Enemy/graphics rules and integrated original enemy-phase checks only; not a whole-game release acceptance.',
            'command':command,'cwd':'.','tests':names,'passedTests':passed,'actualProductionFiles':package['GoFiles'],'ignoredGoFiles':package.get('IgnoredGoFiles',[]),
            'sourceSHA256':source,'sourceAfterSHA256':after,'changedDuringRun':[p for p in source if source[p]!=after[p]],
            'logSHA256':hashlib.sha256(log.read_bytes()).hexdigest()}
    target=ROOT/'docs/research/nes-contra/evidence/enemies-subsystem-verification.json'
    if record['passed']:
        target.write_text(json.dumps(record,indent=2)+'\n')
        (target.parent/'enemies-subsystem-race.jsonl').write_bytes(log.read_bytes())
    else:(out/'subsystem-failed.json').write_text(json.dumps(record,indent=2)+'\n')
    print(json.dumps({'passed':record['passed'],'tests':len(names),'passedTests':len(passed),'exit':result.returncode}))
    if not record['passed']:raise SystemExit(1)

if __name__=='__main__':main()
