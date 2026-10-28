"""Original Japanese Guardian/Heart oracle, established before implementation.

Failures: wrong region sound/HP scaling; omitted scroll correction; partial
two-tile drawing and retry flags; stale collision address on queue refusal;
same-slot explosion allocation; inactive-type cleanup and excluded slot zero;
premature clear/score; heart palette edits/audio order. These are controlled
routine inputs with native outputs, not natural completion or a gameplay bot.
"""
from pathlib import Path
import argparse,ast,gzip,hashlib,json,os,shutil

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens-boss/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

FIELDS=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
WORLD=literal(ROOT/'scripts/contra/enemies_capture.py','WORLD')+[
 ('weaponStrength',0x2f),('generationBias',6),('addressLow',0x12),('addressHigh',0x13),
 ('delayLow',0x2a),('delayHigh',0x2b),('cycle1',0x4d),('cycle3',0x4f),('presentationPace',0x5d)]

def cases():
    all=[]
    def add(kind,routine,**extra):
        c=dict(kind=kind,routine=routine,x=176,y=96,sprite=1,stateWidth=0x8d,hp=100,scoreCollision=0x83,animationDelay=1,
               variable1=0x90 if kind==16 and routine<4 else 0,variable2=1,variable3=1,variable4=1,
               level=7,scrollType=0,frameScroll=0,soldierAllowed=1,loop=0,weaponStrength=0,used=0)
        c.update(extra);all.append(c)
    for kind,maximum in [(16,13),(22,8)]:
        for routine in range(1,maximum+1):
            for delay in (0,1,2):
                for used in (0,39,40,63,64):add(kind,routine,animationDelay=delay,used=used)
        for loop in (0,1,4,15,16,255):
            for strength in (0,1,3,7,15,16,255):add(kind,1,loop=loop,weaponStrength=strength)
        for index in range(12):
            for full in (False,True):add(kind,5,variable1=index,animationDelay=1,full=full)
        for x,y in ((7,96),(8,96),(79,96),(80,96),(255,240),(176,0)):
            for routine in (2,3,4,5):add(kind,routine,x=x,y=y,frameScroll=2,full=True)
    for code in (0x90,0x92):
        for flag1 in (0,1):
            for flag2 in (0,1):
                for timer in (0,1,2):add(16,2,variable1=code,variable2=flag1,variable3=flag2,variable4=timer,animationDelay=1)
    for delay in (0,1,2,3,4):
        for strength in (0,2,3,4):
            for allowed in (0,1):add(16,3,animationDelay=delay,weaponStrength=strength,soldierAllowed=allowed)
    for routine in (7,8,10,11):
        for flag in (0,1):
            for used in (0,40,64):add(16,routine,variable2=flag,used=used,address=0x21c0)
    for hp in (0,1,2,3,127,255):add(22,2,hp=hp)
    for flag in (0,1):
        for frame in (0,1,2,255):
            for used in (0,40,64):add(22,3,variable2=flag,variable4=frame,used=used)
    for kind,routine in ((16,12),(22,8)):
        for inactive in (False,True):add(kind,routine,cleanup=True,inactive=inactive)
    out=[]
    for c in all:
        writes={str(a+i):0 for _,a in FIELDS for i in range(16)}
        if c.get('full'):
            for i in range(15):
                for a,v in [(0x4b8,1),(0x528,1),(0x30a,1),(0x33e,160),(0x324,160)]:writes[str(a+i)]=v
        if c.get('cleanup'):
            for i in range(15):
                for a,v in [(0x4b8,0 if c['inactive']else 2),(0x528,17+i%5),(0x30a,1),(0x33e,160),(0x324,160)]:writes[str(a+i)]=v
        for name,a in FIELDS:writes[str(a+15)]=c.get(name,0)
        for name,a in WORLD:
            if name in c:writes[str(a)]=c[name]
        writes[str(0x21)]=c['used'];writes[str(0x23)]=0
        writes[str(0x12)]=c.get('address',0x21a0)&255;writes[str(0x13)]=c.get('address',0x21a0)>>8
        writes[str(0x2a)]=7;writes[str(0x2b)]=3
        out.append({'setup':c,'writes':writes})
    return out

def main():
    p=argparse.ArgumentParser();p.add_argument('--name',required=True);a=p.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra-campaign/states/stage-8-v2.fcs';dest=BASE/'states/stage-8.fcs'
    if dest.exists():assert source.read_bytes()==dest.read_bytes()
    else:shutil.copyfile(source,dest)
    rows=cases()
    hook=literal(ROOT/'scripts/contra/enemies_waterfall_capture.py','HOOK').replace("out..'/waterfall.jsonl'","out..'/aliens-boss.jsonl'")
    hook=hook.replace('not(kind==12 or kind>=16 and kind<=21)','not(kind==16 or kind==22)').replace("read(0x8000)~=0x10 or","read(0x8000)~=0x10 or read(0x30)~=7 or")
    hook=hook.replace('ENEMIES','{'+','.join(hex(a)for _,a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}')
    hook=hook.replace('CASES','{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in rows)+'}')
    spec={'name':a.name,'load_state':'stage-8','frames':len(rows),'captures':[1,len(rows)],'hooks_lua':hook,'timeout_seconds':180}
    path=BASE/(a.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in(out/'aliens-boss.jsonl').read_text().splitlines()];assert len(records)==len(rows)
    data={'source_is_original_execution':True,'scope':'Controlled Japanese Guardian/Heart dispatch boundaries after controlled stage-8 entry; not a natural campaign.',
          'source':json.loads((out/'source.json').read_text()),'cases':rows,'scenario':spec,'fields':FIELDS,'world_fields':WORLD,'records':records}
    fixture=ROOT/'games/contra/engine/testdata/original-enemies-aliens-boss.json.gz'
    if fixture.exists():assert json.loads(gzip.decompress(fixture.read_bytes()))['records']==records,'original replay differs'
    else:fixture.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    result={'records':len(records),'fixtureSHA256':hashlib.sha256(fixture.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()};(out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
