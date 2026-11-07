"""Capture Alien's Lair originals; never inject expected output or execute ROM in rules."""
import ast,gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for node in ast.parse(path.read_text()).body:
        if isinstance(node,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in node.targets):return ast.literal_eval(node.value)
    raise ValueError(name)

FIELDS=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
WORLD=literal(ROOT/'scripts/contra/enemies_capture.py','WORLD')+[
 ('paletteTimer',0x74),('bossCount',0x85),('alienAimIndex',0x8b),('delayLow',0x2a),('delayHigh',0x2b),
 ('weaponStrength',0x2f),('gameRoutine',0x18),('generationBias',0x06),('palettePause',0x78)]
HOOK=literal(ROOT/'scripts/contra/enemies_waterfall_capture.py','HOOK').replace("out..'/waterfall.jsonl'","out..'/aliens.jsonl'")
HOOK=HOOK.replace('not(kind==12 or kind>=16 and kind<=21)','not(kind>=16 and kind<=22)').replace("read(0x8000)~=0x10 or", "read(0x8000)~=0x10 or read(0x30)~=7 or")
HOOK=HOOK.replace("end)\nfunction finish_probe()", "end)\nfunction finish_probe()")

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'natural-v1'
    (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/'.local/contra-campaign/states/stage-8-v2.fcs';target=BASE/'states/stage-8.fcs'
    if target.exists():assert source.read_bytes()==target.read_bytes()
    else:shutil.copyfile(source,target)
    frames=3000
    hook=HOOK.replace('ENEMIES','{'+','.join(hex(a)for _,a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('CASES','nil')
    spec={'name':'aliens-'+label,'load_state':'stage-8','frames':frames,'captures':[1,1000,2000,frames],
          'segments':[dict(start=1,end=frames,player=p,keys=['right'])for p in [1,2]]+
          [dict(start=n,end=n,player=p,keys=['B'])for p in [1,2]for n in range(1,frames+1,3)]+
          [dict(start=n,end=n+1,player=p,keys=['A'])for p in [1,2]for n in range(30,frames,70)],'hooks_lua':hook,'timeout_seconds':150}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'aliens.jsonl').read_text().splitlines()];assert rows
    counts={}
    for row in rows:
        e=row['before']['enemies'][row['actor']];key=f'{e[11]:02x}:{e[4]}';counts[key]=counts.get(key,0)+1
    fixture={'source_is_original_execution':True,'interventions':'Controlled stage-index setup in retained stage-8-v2; this capture is controller input only after that setup. Not a natural full campaign.',
             'source':json.loads((out/'source.json').read_text()),'scenario':spec,'fields':FIELDS,'world_fields':WORLD,'records':rows,'counts':counts}
    target=ROOT/'games/contra/engine/testdata/original-enemies-aliens-natural.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'records':len(rows),'counts':counts,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
