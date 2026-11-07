"""Re-observe actual inherited throw inputs for historical isolated Base tests."""
from pathlib import Path
import gzip,hashlib,json,os,re,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-base-throw-inputs'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1';outcomes=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    for stage in [2,4]:
        src=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';dest=BASE/f'states/stage-{stage}.fcs'
        if dest.exists():assert src.read_bytes()==dest.read_bytes()
        else:shutil.copyfile(src,dest)
        for mode in ['room','boss']:
            path=ROOT/f'docs/research/nes-contra/evidence/enemies-base-{stage}-{mode}.json.gz';raw=path.read_bytes();source=json.loads(gzip.decompress(raw));spec=dict(source['scenario']);spec['name']=f'throw-inputs-{stage}-{mode}-{label}';spec['captures']=[]
            hook=spec['hooks_lua'];start=hook.index('local function snapshot()');end=hook.index('\nend\n',start)+5
            hook=hook[:start]+'''local function snapshot()
 return '{"inheritedThrowAttribute":'..read(0x0a)..'}'
end
'''+hook[end:];spec['hooks_lua']=hook
            specpath=BASE/(spec['name']+'.json');specpath.write_text(json.dumps(spec,indent=2)+'\n');observed=run(specpath)
            rows=[json.loads(l)for l in(observed/'base-enemies.jsonl').read_text().splitlines()]
            assert len(rows)==len(source['records'])
            for old,new in zip(source['records'],rows):
                for key in ['id','name','frame','slot','type','audio','requests']:assert old[key]==new[key],(stage,mode,key)
            assert hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()==source['ram_sha256']
            outcomes.append({'fixture':path.name,'fixtureSHA256':hashlib.sha256(raw).hexdigest(),'source':json.loads((observed/'source.json').read_text()),'scenario':spec,
                             'ramSHA256':source['ram_sha256'],'records':[{'id':r['id'],'slot':r['slot'],'before':r['before']['inheritedThrowAttribute']}for r in rows]})
    target=ROOT/'docs/research/nes-contra/evidence/enemies-base-throw-inputs.json.gz';data={'source_is_original_execution':True,'scope':'Passive new input observation on exactly the historical scenarios; record identity, audio, graphics requests and full RAM match. No expected output supplies an input.','fixtures':outcomes}
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['fixtures']==outcomes
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'fixtures':len(outcomes),'records':sum(len(x['records'])for x in outcomes),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
