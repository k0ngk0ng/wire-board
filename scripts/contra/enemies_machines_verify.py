"""Persist scoped engine checks and complete REA evidence for machines.

This reports only the recovered actors and their boundary/original-call
comparisons. It cannot certify natural stage-six traversal, networking,
campaign completion, bots, presentation, or platform ratings.
"""
import gzip,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-machines'
OUT=ROOT/'docs/research/nes-contra/evidence'

def main():
    tmp=ROOT/'dist/contra-enemies-machines-tmp';tmp.mkdir(parents=True,exist_ok=True)
    cache=BASE/'go-cache';cache.mkdir(parents=True,exist_ok=True)
    env=dict(os.environ,GOCACHE=str(cache),GOTMPDIR=str(tmp))
    command=['go','test','./games/contra/engine','-run','TestOriginalMachine|TestControlledMachine|TestMachineTile','-count=1','-v']
    test=subprocess.run(command,cwd=ROOT,env=env,capture_output=True,text=True)
    log=OUT/'enemies-machines-go-test.txt';log.write_text(test.stdout+test.stderr)
    fixtures=[]
    for p in sorted((ROOT/'games/contra/engine/testdata').glob('original-enemy-machines*.json.gz')):
        d=json.loads(gzip.decompress(p.read_bytes()));assert d['source_is_original_execution']
        fixtures.append({'path':str(p.relative_to(ROOT)),'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'records':len(d['records'])})
    records=[]
    for directory in ['dispatch-rea','helpers-rea','graphics-common-rea','mechanics-helpers-rea','mechanics-extra-rea','tile-pair-rea']:
        for p in sorted((BASE/directory).glob('function-*.json')):
            d=json.loads(p.read_text());assert d['authority']=='shipped-artifact'and d['operation']=='analyze_function'
            records.append({'file':directory+'/'+p.name,'evidence':d})
    archive=OUT/'enemies-machines-rea.json.gz'
    data={'romSHA256':'6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe','dispatch':json.loads((BASE/'dispatch.json').read_text()),'records':records}
    archive.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    report={'passed':test.returncode==0,'command':command,'testLog':str(log.relative_to(ROOT)),'testLogSHA256':hashlib.sha256(log.read_bytes()).hexdigest(),
            'fixtures':fixtures,'records':sum(f['records']for f in fixtures),'rea':{'functions':len(records),'path':str(archive.relative_to(ROOT)),'sha256':hashlib.sha256(archive.read_bytes()).hexdigest()},
            'replay':'docs/research/nes-contra/evidence/enemies-machines-replay.json',
            'limitations':['The controller-only continuation starts from previously declared stage-entry overrides.','Stage-six continuation yielded no stage-specific machinery calls before player death; machinery there is covered by declared function-boundary inputs.','Per-call native agreement does not replace a continuous independent world/campaign E2E, networking, bots, audiovisual or platform-rating acceptance.']}
    (OUT/'enemies-machines-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':report['passed'],'records':report['records'],'reaFunctions':len(records),'archiveBytes':archive.stat().st_size}))
    raise SystemExit(test.returncode)

if __name__=='__main__':main()
