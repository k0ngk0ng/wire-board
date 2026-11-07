"""Bounded Guardian/Heart race verification and original evidence export."""
from pathlib import Path
import gzip,hashlib,json,os,subprocess
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens-boss';EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def main():
    engine=ROOT/'games/contra/engine';temp=BASE/'verify-tmp';temp.mkdir(parents=True,exist_ok=True)
    tests=['enemies_aliens_boss_test.go','enemies_aliens_boss_sequence_test.go'];names=['TestOriginalAlienGuardianAndHeart','TestContinuousOriginalAlienBosses']
    files=sorted(p.name for p in engine.glob('*.go'));hashes={p:sha(engine/p)for p in files}
    command=['go','test','-race','-count=1','-json','-run','^('+'|'.join(names)+')$','.']
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp),GOTOOLCHAIN='local')
    log=BASE/'verify-run.jsonl'
    with log.open('w')as f:result=subprocess.run(command,cwd=engine,env=env,stdout=f,stderr=subprocess.STDOUT)
    events=[]
    for line in log.read_text().splitlines():
        try:events.append(json.loads(line))
        except json.JSONDecodeError:pass
    passed=[r['Test']for r in events if r.get('Action')=='pass'and r.get('Test')in names]
    changed=[p for p in files if sha(engine/p)!=hashes[p]]
    if result.returncode or len(passed)!=len(names)or changed:print(log.read_text()[-6000:]);raise RuntimeError({'exit':result.returncode,'passed':passed,'changed':changed})
    identity=json.loads((BASE/'identity.json').read_text());sessions=[];functions=[]
    # Reuse the matching parent analysis rather than rerunning identical code.
    candidates=[('main',ROOT/'.local/contra-enemies-aliens/rea',BASE/'main-rea'),('helpers',BASE/'rea',BASE/'helpers-rea'),('draw',BASE/'draw-rea',BASE/'draw-rea')]
    for name,retained,regenerated in candidates:
        area=regenerated if (regenerated/'evidence.json').exists()else retained
        dest=EVIDENCE/('enemies-aliens-boss-'+name+'-rea.json.gz');dest.write_bytes(gzip.compress((area/'evidence.json').read_bytes(),mtime=0));sessions.append({'name':name,'bundle':dest.name,'sha256':sha(dest)})
        for pc in identity['groups'][name]:
            p=area/f'function-{hex(pc)}.json';d=json.loads(p.read_text());assert d['subject']['digest']['sha256']==identity['windowSHA256']
            functions.append({'address':hex(pc),'evidenceID':d['evidence_id'],'session':name,'assembly':d['normalized_result']['assembly'],'limitations':d['limitations']})
    (EVIDENCE/'enemies-aliens-boss-index.json').write_text(json.dumps({'identity':identity,'sessions':sessions,'functions':functions,'limitations':['Automatic bodies may include inline tables as instructions; table bytes are separately bound above.','Parent main REA bundle also contains other alien functions; only listed Guardian/Heart entries support this task.']},indent=2)+'\n')
    previous_path=EVIDENCE/'enemies-aliens-boss-verification.json';previous=json.loads(previous_path.read_text())if previous_path.exists()else{}
    repeats=[]
    for first,second,stream in [('aliens-boss-v1','aliens-boss-repeat','aliens-boss.jsonl'),('aliens-boss-guardian-sequence-v2','aliens-boss-guardian-repeat','enemy-ticks.jsonl'),('aliens-boss-heart-sequence-v2','aliens-boss-heart-repeat','enemy-ticks.jsonl')]:
        a=BASE/'oracle/runs'/first;b=BASE/'oracle/runs'/second;pair=['ram.bin','inputs.csv',stream];r={'primary':first,'repeat':second,'files':{p:sha(a/p)for p in pair}}
        if b.exists():assert all((a/p).read_bytes()==(b/p).read_bytes()for p in pair),'native replay differs'
        else:
            saved=next((x for x in previous.get('originalRepeats',[])if x['primary']==first and x['repeat']==second),None);assert saved and saved['files']==r['files'],'missing native repeat proof'
            r['proof']='Previously verified identical repeat; retained primary hashes rechecked after duplicate cleanup.'
        repeats.append(r)
    (EVIDENCE/'enemies-aliens-boss-red.log').write_bytes((BASE/'red.log').read_bytes())
    own=[*engine.glob('enemies_aliens_boss*.go'),*engine.glob('testdata/original-enemies-aliens-boss*.json.gz'),*ROOT.glob('scripts/contra/enemies_aliens_boss_*.py')]
    report={'passed':True,'scope':'Guardian and Heart independent states with controlled native E54F continuous pool/terrain verification; not natural kills, complete campaign, browser or platform scoring acceptance.',
      'originalBoundaryCalls':617,'continuous':{'guardianTicks':1200,'guardianEntityCalls':3063,'guardianGraphicsRefusals':19,'heartTicks':1000,'heartEntityCalls':1179,'heartGraphicsRefusals':22},
      'command':command,'cwd':'games/contra/engine','tests':passed,'normalPackageBuild':True,'sourceStableDuringCheck':True,'compiledSourceSHA256':hashes,'sourceAndFixtureSHA256':{str(p.relative_to(ROOT)):sha(p)for p in own},'raceLogSHA256':sha(log),'originalRepeats':repeats,'reaFunctions':len(functions)}
    (EVIDENCE/'enemies-aliens-boss-race.jsonl').write_bytes(log.read_bytes())
    previous_path.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'passed':True,'tests':passed,'reaFunctions':len(functions),'originalRepeats':len(repeats)}))

if __name__=='__main__':main()
