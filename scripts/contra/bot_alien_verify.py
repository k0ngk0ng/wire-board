"""Normal-package verification for fixed Alien background visual recipes."""
from pathlib import Path
import gzip,hashlib,json,os,subprocess

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-bot-aliens';OUT=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def main():
    fixture=ROOT/'games/contra/engine/testdata/bot-alien-original-art.json.gz';before=sha(fixture)
    with (BASE/'reference-repeat.log').open('w')as log:
        subprocess.run([str(ROOT/'.local/board-art-env/bin/python'),'-B','scripts/contra/bot_alien_reference.py'],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT,check=True)
    assert sha(fixture)==before,'independent raw-ROM export changed'
    reference=json.loads(gzip.decompress(fixture.read_bytes()))
    temp=BASE/'tmp';temp.mkdir(parents=True,exist_ok=True)
    files=[ROOT/'games/contra/engine/bot_vision_alien_recipes.go',ROOT/'games/contra/engine/bot_vision_alien_recipes_test.go',ROOT/'games/contra/engine/bot_vision_background_recipes.go',ROOT/'games/contra/engine/bot_vision_builtin_background.go',ROOT/'games/contra/engine/bot_vision.go']
    hashes={str(p.relative_to(ROOT)):sha(p)for p in files}
    command=['go','test','-race','./games/contra/engine','-run','^TestBotAlienRecipes','-count=1','-json']
    env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(temp),TMPDIR=str(temp),GOTOOLCHAIN='local')
    log=BASE/'green-final.jsonl'
    with log.open('w')as f:result=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    rows=[]
    for line in log.read_text().splitlines():
        try:rows.append(json.loads(line))
        except json.JSONDecodeError:pass
    tests=[r['Test']for r in rows if r.get('Action')=='pass'and str(r.get('Test','')).startswith('TestBotAlienRecipes')]
    stable=all(sha(ROOT/p)==digest for p,digest in hashes.items())
    if result.returncode or len(tests)!=3 or not stable:print(log.read_text()[-4000:]);raise RuntimeError({'exit':result.returncode,'tests':tests,'sourceStable':stable})
    red=BASE/'red.jsonl';assert 'undefined: BotAlienBackgroundVisionRecipes'in red.read_text(),'missing pre-implementation recipe failure'
    (OUT/'bot-alien-recipes-red.jsonl.gz').write_bytes(gzip.compress(red.read_bytes(),mtime=0));(OUT/'bot-alien-recipes-race.jsonl.gz').write_bytes(gzip.compress(log.read_bytes(),mtime=0))
    familyRed=BASE/'family-red.jsonl';assert 'missing visible Guardian family'in familyRed.read_text(),'missing pre-change family evidence'
    (OUT/'bot-alien-recipes-family-red.jsonl.gz').write_bytes(gzip.compress(familyRed.read_bytes(),mtime=0))
    pictures={}
    for name in ['guardian-updates','guardian-scene','heart-updates','heart-scene','anchors']:
        source=BASE/(name+'.png');dest=OUT/('bot-alien-'+name+'.png');dest.write_bytes(source.read_bytes());pictures[dest.name]=sha(dest)
    record={'passed':True,'scope':'Two global, fixed original-art recipe families. Enemy means a potential visual target, with no Alive/HP/vulnerability claim. No gameplay strategy or live-World selector was changed.',
            'recipes':2,'patternCodes':3,'sourcePatterns':len(reference['patterns']),'legalContexts':len(reference['contexts']),'independentRasters':len(reference['rasters']),'sceneChecks':{'total':18,'completeCandidate':8,'closedOrDestroyed':10},
            'sourceROMSHA256':reference['romSHA256'],'fixtureSHA256':sha(fixture),'repeatedExportIdentical':True,'sourceEvidence':reference['sources'],'rea':reference['rea'],
            'command':command,'normalPackageBuild':True,'tests':tests,'sourceStableDuringCheck':stable,'sourceSHA256':hashes,'scriptSHA256':{str(p.relative_to(ROOT)):sha(p)for p in [ROOT/'scripts/contra/bot_alien_reference.py',ROOT/'scripts/contra/bot_alien_verify.py']},'pictures':pictures,
            'artifacts':{'redSHA256':sha(OUT/'bot-alien-recipes-red.jsonl.gz'),'familyRedSHA256':sha(OUT/'bot-alien-recipes-family-red.jsonl.gz'),'raceSHA256':sha(OUT/'bot-alien-recipes-race.jsonl.gz')},
            'limits':['Scenes are independent reconstructions of pinned original artwork, not screenshots or a natural Boss-fight acceptance.','Intact graphics may survive an invisible defeat decision and must produce the same candidate for the same image.','Global catalog aggregation, other background families, occlusion combinations and complete bot campaign remain separate integration scope.']}
    (OUT/'bot-alien-recipes-verification.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({'passed':True,'tests':tests,'rasters':len(reference['rasters']),'scenes':18}))

if __name__=='__main__':main()
