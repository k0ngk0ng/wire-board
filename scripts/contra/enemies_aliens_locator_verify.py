"""Verify real-package mixed locator regression and retain native/red evidence."""
from pathlib import Path
import gzip,hashlib,json,os,subprocess

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens-locator';EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def events(path):
    rows=[]
    for line in path.read_text().splitlines():
        try:rows.append(json.loads(line))
        except json.JSONDecodeError:pass
    return rows

def main():
    temp=BASE/'tmp';temp.mkdir(parents=True,exist_ok=True)
    engine=ROOT/'games/contra/engine';files=sorted(engine.glob('*.go'));hashes={str(p.relative_to(ROOT)):sha(p)for p in files}
    command=['go','test','-race','./games/contra/engine','-run','^(TestOriginalAlienLocatorProducerEvidence|TestOriginalMixedAlienLocatorHandoff)$','-count=1','-json']
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp),GOTOOLCHAIN='local')
    log=BASE/'green.jsonl'
    with log.open('w')as f:result=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    rows=events(log);passed=[r['Test']for r in rows if r.get('Action')=='pass'and'Test'in r]
    stable=all(sha(ROOT/p)==h for p,h in hashes.items())
    if result.returncode or len(passed)!=76 or not stable or not any(r.get('Action')=='pass'and r.get('Package')=='github.com/k0ngk0ng/wire-board/games/contra/engine'and'Test'not in r for r in rows):
        print(log.read_text()[-6000:]);raise RuntimeError({'exit':result.returncode,'passedEntries':len(passed),'sourceStable':stable})
    red=BASE/'red.jsonl';redrows=events(red);failures=[r['Test']for r in redrows if r.get('Action')=='fail'and str(r.get('Test','')).startswith('TestOriginalMixedAlienLocatorHandoff/')]
    assert len(failures)==74,'missing TDD red matrix'
    errors=[r.get('Output','')for r in redrows if str(r.get('Test','')).startswith('TestOriginalMixedAlienLocatorHandoff/')and'enemies_aliens_locator_test.go:'in r.get('Output','')]
    assert errors and all('final locator got'in e or 'Guardian cleared different collision cells'in e for e in errors),'red contains an unrelated failure'
    fixture=engine/'testdata/original-enemy-collision-tick-aliens-locator.json.gz';data=json.loads(gzip.decompress(fixture.read_bytes()))
    primary=BASE/'oracle/runs/aliens-locator-v1';repeat=BASE/'oracle/runs/aliens-locator-repeat';names=['ram.bin','inputs.csv','enemy-ticks.jsonl'];proof={p:sha(primary/p)for p in names}
    previous_path=EVIDENCE/'enemies-aliens-locator-verification.json';previous=json.loads(previous_path.read_text())if previous_path.exists()else{}
    if repeat.exists():assert all((primary/p).read_bytes()==(repeat/p).read_bytes()for p in names),'native replay differs'
    else:assert previous.get('originalRepeatSHA256')==proof,'missing retained original repeat proof'
    categories={};examples=[]
    for case,row in zip(data['cases'],data['records']):
        category=case['setup']['category'];categories[category]=categories.get(category,0)+1
        if case['setup']['guardianRoutine']==10 and not any(e['category']==category for e in examples):
            examples.append({'category':category,'case':case['name'],'slots':row['slots'],'requests':row['requests'],'locatorReads':row['locatorReads'],'terrainWorkWrites':row['terrainWorkWrites']})
    (EVIDENCE/'enemies-aliens-locator-examples.json').write_text(json.dumps(examples,indent=2)+'\n')
    (EVIDENCE/'enemies-aliens-locator-red.jsonl').write_bytes(red.read_bytes());(EVIDENCE/'enemies-aliens-locator-race.jsonl').write_bytes(log.read_bytes())
    record={'passed':True,'scope':'Controlled native mixed E54F handoff from earlier terrain/graphics producers to Guardian R10/R11. Full before/after state and real queue/collision are compared; write traces are never engine inputs.',
      'phases':len(data['records']),'entityCalls':sum(len(r['slots'])for r in data['records']),'terrainWorkWrites':sum(len(r['terrainWorkWrites'])for r in data['records']),
      'categories':categories,'fixtureSHA256':sha(fixture),'nativeHookParentSHA256':data['parentRecorderSHA256'],'originalRepeatSHA256':proof,
      'tdd':{'redFailedCases':len(failures),'redSourceEvidencePassed':any(r.get('Test')=='TestOriginalAlienLocatorProducerEvidence'and r.get('Action')=='pass'for r in redrows),'redSHA256':sha(red),'greenSHA256':sha(log)},
      'command':command,'cwd':'.','normalPackageBuild':True,'raceTestEntriesPassed':len(passed),'sourceStableDuringCheck':stable,'sourceSHA256':hashes,
      'scriptSHA256':{str(p.relative_to(ROOT)):sha(p)for p in ROOT.glob('scripts/contra/enemies_aliens_locator_*.py')},
      'limitations':['Original setup stages native stage8, explicit enemies, queue budget and map. Not natural gameplay or campaign completion.','Trace PC is the emulator-reported address after each write instruction; bank is the byte read at CPU8000, not a physical PRG bank index.','Recorded addresses live only in research fixtures. Shipping producers publish named terrain query or graphics results.']}
    previous_path.write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({'passed':True,'normalPackageBuild':True,'testEntries':len(passed),'phases':record['phases'],'writes':record['terrainWorkWrites']}))

if __name__=='__main__':main()
