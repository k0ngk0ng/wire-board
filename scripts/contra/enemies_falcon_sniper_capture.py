"""Original F pickup to sniper destruction, staged before complete E54F only."""
from pathlib import Path
import ast,copy,gzip,hashlib,json,os,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-falcon-sniper'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1';path=ROOT/'scripts/contra/enemies_collision_tick_capture.py'
    pf=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS');ef=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS');wf=literal(path,'WORLD')+[('generationBias',6)]
    source=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-controlled-bias.json.gz').read_bytes()))
    template=next(c for c in source['cases']if c['name']=='falcon-changes-future-slot');cases=[]
    for attr in range(3):
        c=copy.deepcopy(template);c['name']=f'falcon-sniper-attribute-{attr}'
        c['writes'].update({str(0x528+14):6,str(0x4b8+14):2,str(0x5a8+14):attr})
        cases.append(c)
    hook=literal(path,'HOOK').replace('PLAYERS','{'+','.join(hex(a)for _,a in pf)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ef)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CAPTURE_BIAS','true')
    hook+='\nlocal cases={'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in cases)+'}\n'
    hook+='''function configure_phase(index)
 local c=assert(cases[index]);local saved={}
 for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 for a,v in pairs(c)do memory.writebyte(a,v)end
 return saved
end
'''
    (BASE/'states').mkdir(parents=True,exist_ok=True);start=ROOT/'.local/contra/states/coop-play.fcs';dest=BASE/'states/coop-play.fcs'
    if dest.exists():assert start.read_bytes()==dest.read_bytes()
    else:shutil.copyfile(start,dest)
    spec={'name':'falcon-sniper-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':hook}
    p=BASE/(spec['name']+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
    rows=[json.loads(l)for l in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert len(rows)==3
    data={'source_is_original_execution':True,'scope':'falcon-sniper','interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,
          'player_fields':[{'name':n,'address':hex(a)}for n,a in pf],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ef],'world_fields':[{'name':n,'address':hex(a)}for n,a in wf],'records':rows}
    target=ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-falcon-sniper.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'phases':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
