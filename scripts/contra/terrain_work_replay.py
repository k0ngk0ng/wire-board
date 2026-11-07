"""Repeat canonical observer scenarios without overwriting any primary record."""
from pathlib import Path
import copy,gzip,hashlib,json,os,re,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-terrain-work'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

NAMES=['stage8-v2','stage6-v2','guardian-cases-v5','vertical-cases-v1','indoor-cases-v2','guardian-world-v1']

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1';records=[]
    for name in NAMES:
        fixture=ROOT/f'docs/research/nes-contra/evidence/terrain-work-{name}.json.gz'
        data=json.loads(gzip.decompress(fixture.read_bytes()));spec=copy.deepcopy(data['scenario']);spec['name']='replay-'+name+'-'+label
        # A saved state is an observation artifact, not an input to this run.
        # Redirect that output only; never replace a retained live-Guardian state.
        destination=BASE/'states'/f'guardian-{spec["name"]}.fcs';assert not destination.exists()
        spec['hooks_lua']=re.sub(r'"'+re.escape(str(BASE/'states'))+r'/guardian-[^"\n]+\.fcs"',json.dumps(str(destination)),spec['hooks_lua'])
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path);primary=BASE/'runs'/name
        files=[]
        for filename in ['ram.bin','inputs.csv','terrain-work.jsonl']:
            a=(primary/filename).read_bytes();b=(out/filename).read_bytes();assert a==b,(name,filename)
            files.append({'name':filename,'bytes':len(a),'sha256':hashlib.sha256(a).hexdigest()})
        records.append({'name':name,'primary':str(primary.relative_to(ROOT)),'repeat':str(out.relative_to(ROOT)),'equal':True,'files':files,'fixtureSHA256':hashlib.sha256(fixture.read_bytes()).hexdigest()})
    target=ROOT/'docs/research/nes-contra/evidence/terrain-work-replay.json';target.write_text(json.dumps({'passed':True,'scope':'Six independent original-emulator repeats; controller inputs, full RAM and operation/write records match. Controlled contexts retain their declared scope.','pairs':records},indent=2)+'\n')
    print(json.dumps({'passed':True,'pairs':len(records)}))

if __name__=='__main__':main()
