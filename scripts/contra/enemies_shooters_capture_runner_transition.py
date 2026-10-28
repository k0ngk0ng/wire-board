"""Controlled original running-soldier firing transition, before its Go change."""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/runner-transition'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_shooters_capture_states import HOOK,FIELDS

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    cases=[]
    for attributes in (4,8):
        for delay in (0,1,2):
            for enabled in (0,1):
                for strength in (0,2):
                    for random in (0,3):
                        values=[0x3b,100,180,0x40,3,0,0,0,0,0xff,0,5,delay,0,0,0,1,0x10,2,attributes,0,0,0,0]
                        cases.append(dict(routine=3,values=values,sceneType=0,vertical=0,scroll=0,frameCounter=8,enabled=enabled,strength=strength,random=random,players=[dict(state=1,x=48,y=100),dict(state=1,x=96,y=100)]))
    lua=[]
    for c in cases:
        parts=[]
        for k,v in c.items():
            if k=='players':parts.append('players={'+','.join('{'+','.join(f'{key}={value}'for key,value in p.items())+'}'for p in v)+'}')
            elif isinstance(v,list):parts.append(k+'={'+','.join(map(str,v))+'}')
            else:parts.append(f'{k}={v}')
        lua.append('{'+','.join(parts)+'}')
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join(lua)+'}')
    hook=hook.replace('set(0x8e,c.enabled)','set(0x8e,c.enabled);set(0x2f,c.strength);set(0x34,c.random)')
    hook=hook.replace("local f=assert(io.open", "local stableMap=memory.readbyterange(0x680,128)\nlocal f=assert(io.open",1)
    hook=hook.replace("assert(not pending);local c=cases[index]", "assert(memory.readbyterange(0x680,128)==stableMap,'collision map changed');assert(not pending);local c=cases[index]")
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    spec={'name':'runner-fire-transition-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':hook}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'states.jsonl').read_text().splitlines()];assert len(rows)==len(cases)
    ram=(out/'initial-ram.bin').read_bytes()
    data={'source_is_original_execution':True,'interventions':'Original state3 input/world fields staged at E58A and restored after return; no expected results synthesized.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows,'terrain':ram[0x680:0x700].hex(),'scrollY':ram[0xfc],'scrollX':ram[0xfd],'control':ram[0xff]}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);dest=ROOT/'games/contra/engine/testdata/original-enemy-shooters-runner-transition.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'cases':len(cases),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
