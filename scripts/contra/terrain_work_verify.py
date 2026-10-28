"""Verify named spatial projections and their bounded consumer integration."""
from pathlib import Path
import gzip,hashlib,json,os,subprocess
from terrain_work_analyze import analyze
ROOT=Path(__file__).resolve().parents[2]
NAMES=['stage8-v2','stage6-v2','guardian-cases-v5','vertical-cases-v1','indoor-cases-v2','guardian-world-v1']
TESTS=['TestTerrainWorkC001UpstreamContract','TestTerrainWorkGraphicAndOverflowContracts','TestTerrainWorkGuardianConsumptionContract','TestOriginalWeaponTerrainSupportContract',
       'TestOriginalAlienGuardianAndHeart','TestContinuousOriginalAlienBosses','TestOriginalMixedAlienLocatorHandoff','TestOriginalAlienLocatorProducerEvidence','TestOriginalAllStageEnemyPhaseDispatch',
       'TestGraphicsPrefixTerminationPreservesBudget','TestGraphicsPrefixDecodeAndFailure']

def main():
    out=ROOT/'.local/contra-terrain-work/checks';tmp=out/'tmp';tmp.mkdir(parents=True,exist_ok=True)
    ev=ROOT/'docs/research/nes-contra/evidence';reports=[]
    digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
    for name in NAMES:
        path=ev/f'terrain-work-{name}.json.gz';data=json.loads(gzip.decompress(path.read_bytes()));r=analyze(data);assert r['passed'],name
        reports.append({'name':name,'scope':data['scope'],'sha256':digest(path),'writes':sum(len(f['writes'])for f in data['frames']),'prefixWrites':sum(len(f.get('prefixWrites',[]))for f in data['frames']),'projection':r})
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(tmp),TMPDIR=str(tmp),GOTOOLCHAIN='local')
    package=json.loads(subprocess.check_output(['go','list','-json','./games/contra/engine'],cwd=ROOT,env=env));base=ROOT/'games/contra/engine'
    files=[base/n for n in package['GoFiles']+package.get('TestGoFiles',[])];before={str(p.relative_to(ROOT)):digest(p)for p in files}
    command=['go','test','-race','-count=1','-json','-run','^('+'|'.join(TESTS)+')$','./games/contra/engine'];log=out/'terrain-work-race.jsonl'
    with log.open('w')as f:completed=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    rows=[json.loads(s)for s in log.read_text().splitlines()if s.startswith('{')];passed=[x['Test']for x in rows if x.get('Action')=='pass'and x.get('Test')in TESTS]
    after={str(p.relative_to(ROOT)):digest(p)for p in files}
    record={'passed':completed.returncode==0 and len(passed)==len(TESTS),'scope':'Named TerrainWork effects and full-C001 original producer evidence, plus actual Guardian/graphics consumer and weapon support regression. This is not full eight-stage World integration acceptance.',
            'worldIntegrationAccepted':False,'command':command,'tests':TESTS,'passedTests':passed,'sourceSHA256':before,'sourceAfterSHA256':after,'changedDuringRun':[p for p in before if before[p]!=after[p]],'logSHA256':digest(log),
            'scenarios':reports,'counts':{'frames':sum(x['projection']['frames']for x in reports),'workWrites':sum(x['writes']for x in reports),'semanticOperations':sum(x['projection']['projectedOperations']for x in reports),'checkpoints':sum(x['projection']['checkpoints']for x in reports),'prefixWrites':sum(x['prefixWrites']for x in reports)},
            'redEvidence':{str(p.relative_to(ROOT)):digest(p)for p in [out/'owner-contract-red.jsonl',out/'owner-contract-package-red.jsonl',out/'weapon-support-red.jsonl']}}
    if record['passed']:
        (ev/'terrain-work-verification.json').write_text(json.dumps(record,indent=2)+'\n');(ev/'terrain-work-race.jsonl').write_bytes(log.read_bytes())
    else:(out/'verification-failed.json').write_text(json.dumps(record,indent=2)+'\n')
    print(json.dumps({'passed':record['passed'],'tests':len(passed),'changed':record['changedDuringRun'],'counts':record['counts']}))
    if not record['passed']:raise SystemExit(1)

if __name__=='__main__':main()
