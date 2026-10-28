"""Declared Alien's Lair boundary setups, followed by original E58A execution."""
import gzip,hashlib,json,sys
from enemies_aliens_capture import ROOT,BASE,FIELDS,WORLD,HOOK
from capture_reference import run
from enemies_all_data import inventory

def cases_fetus():
    cases=[]
    def add(routine,**kw):
        c=dict(kind=17,routine=routine,x=180,y=100,sprite=0xac,animationDelay=1,frame=0,attributes=0,
               variable1=6,variable2=0,variable3=1,variable4=18,random=87,frameValue=5,alienAimIndex=0,
               vx=-256,vy=128,frameScroll=0,level=7,scrollType=0,sceneType=0,scrollY=224,scrollX=0,ppuControl=0xb0)
        c.update(kw);cases.append(c)
    for index in range(14):
        for random in [0,1,2,3,127,255]:
            for attribute in [0,1]:add(1,alienAimIndex=index,random=random,attributes=attribute)
    for loop in [0,253,254,255]:
        for over in [0,1,2,3]:add(1,loop=loop,p0over=over&1,p1over=over>>1,frameValue=240,random=255)
    for direction in range(12):
        for target in [(40,40),(220,40),(40,200),(220,200),(180,100)]:
            for delay in [0,1,2]:add(2,variable1=direction,animationDelay=delay,px=target[0],py=target[1])
    for control in range(64):
        for over in [0,1,2,3]:add(2,variable4=control,p0state=2 if over&1 else 1,p1state=2 if over&2 else 1)
    for routine in [3,4,5]:
        for width in [0,0x83,0x8b,255]:
            for sprite in [0,0xac]:add(routine,stateWidth=width,sprite=sprite,frame=0,animationDelay=1)
    for x,y in [(7,100),(8,100),(180,231),(180,232),(255,255)]:add(2,x=x,y=y,frameScroll=2,variable3=2)
    return cases

def materialize(cases):
    props=inventory()['stages'][7]['properties'];rows=[]
    for c in cases:
        writes={str(a+i):0 for _,a in FIELDS for i in range(16)}
        c=dict(c);p=props[c['kind']]['values']
        for n,v in zip(['stateWidth','scoreCollision','hp','variableA'],p):c.setdefault(n,v)
        for name,a in FIELDS:writes[str(a+15)]=c.get(name,0)
        for axis in ['X','Y']:
            velocity=c.get('v'+axis.lower(),0)&65535
            for name,a in FIELDS:
                if name=='velocity'+axis:writes[str(a+15)]=velocity>>8
                elif name=='velocity'+axis+'Fraction':writes[str(a+15)]=velocity&255
        for name,a in WORLD:
            if name in c:writes[str(a)]=c[name]
        writes[str(0x80)]=0x57;writes[str(0x81)]=0xe6;writes[str(0x1a)]=c.get('frameValue',5)
        writes[str(0x21)]=c.get('used',0);writes[str(0x23)]=0
        for player in range(2):
            writes[str(0x90+player)]=c.get(f'p{player}state',1);writes[str(0x38+player)]=c.get(f'p{player}over',0)
            writes[str(0x334+player)]=c.get('px',40+player*170);writes[str(0x31a+player)]=c.get('py',120)
            if f'p{player}weapon'in c:writes[str(0xaa+player)]=c[f'p{player}weapon']
        if c.get('full'):
            for slot in range(15):
                for a,v in [(0x4b8,2),(0x528,17),(0x30a,0xac),(0x33e,180),(0x324,100)]:writes[str(a+slot)]=v
        if 'terrain'in c:
            for a in range(0x680,0x700):writes[str(a)]=c['terrain']
        rows.append({'setup':c,'writes':writes})
    return rows

def cases_small():
    cases=[]
    def add(kind,routine,**kw):
        c=dict(kind=kind,routine=routine,x=180,y=100,sprite=1,animationDelay=0x81,frame=0,attributes=0,
               variable1=0,variable2=0,variable3=0,variable4=0,random=87,frameValue=5,vx=-256,vy=128,
               frameScroll=0,level=7,scrollType=0,sceneType=0,scrollY=224,scrollX=0,ppuControl=0xb0,
               soldierAllowed=1,weaponStrength=2,p0weapon=0,p1weapon=3,terrain=0)
        if kind==18:c.update(variable3=1,variable4=0,animationDelay=1)
        if kind==19:c.update(variable1=6,variable2=1,variable3=0,variable4=0)
        if kind==20:c.update(variable1=0,variable2=2,variable3=0,variable4=0)
        if kind==21:c.update(variable1=0xa1,variable2=0,variable3=0,variable4=0,animationDelay=1)
        c.update(kw);cases.append(c)
    for kind,count in [(18,6),(19,6),(20,8),(21,6)]:
        for routine in range(1,count+1):
            for delay in [0,1,2,0x11,0x81]:
                for used in [0,63,64]:add(kind,routine,animationDelay=delay,used=used)
    for kind in [18,21]:
        for x,y in [(7,100),(8,100),(11,11),(12,12),(31,100),(32,100),(180,231)]:
            for full in [0,1]:
                for enabled in [0,1]:add(kind,2,x=x,y=y,full=full,soldierAllowed=enabled,frameScroll=1)
        for loop in [0,1,127,255]:
            for strength in [0,4,127,255]:add(kind,1,loop=loop,weaponStrength=strength,random=255,frameValue=255)
    for routine in [1,2]:
        for over in [0,1,2,3]:
            for random in [0,1,2,3,255]:add(19,routine,p0over=over&1,p1over=over>>1,random=random,frameValue=255)
    for angle in range(24):
        for target in [(40,40),(220,40),(40,200),(220,200)]:
            for routine in [2,3]:add(19,routine,variable1=angle,variable2=1,variable4=1 if routine==2 else 8,px=target[0],py=target[1])
        add(19,2,variable1=angle,variable2=2,variable4=0,animationDelay=0x90)
    for routine in [1,2,3,4,5]:
        for y in [20,47,48,55,56,100,127,128,184,192,193,232]:
            for terrain in [0,0x55,0xff]:add(20,routine,y=y,terrain=terrain,variable3=0xfc if routine==3 else 0,variable4=0,px=160,py=120)
    for x in [159,160,179,191,192,207,208]:
        for y in [40,180]:
            for strength in [0,1,7,255]:add(20,4,x=x,y=y,px=160,py=120,weaponStrength=strength)
    for delay in [1,0x11,0x31]:
        for counter in [0,1,2,255]:
            for weapons in [(0,0),(1,4),(6,3)]:add(21,2,animationDelay=delay,variable4=counter,p0weapon=weapons[0],p1weapon=weapons[1])
    return cases

def main():
    mode=sys.argv[1]if len(sys.argv)>1 else'fetus';label=sys.argv[2]if len(sys.argv)>2 else'v1'
    assert mode in ['fetus','small'];cases=materialize(cases_fetus()if mode=='fetus'else cases_small())
    world=WORLD if mode=='fetus'else WORLD+[('p0weapon',0xaa),('p1weapon',0xab)]
    hook=HOOK.replace('ENEMIES','{'+','.join(hex(a)for _,a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in world)+'}')
    hook=hook.replace('CASES','{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in cases)+'}')
    spec={'name':f'aliens-{mode}-{label}','load_state':'stage-8','frames':len(cases),'captures':[],'segments':[],'hooks_lua':hook,'timeout_seconds':150}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'aliens.jsonl').read_text().splitlines()];assert len(rows)==len(cases),(len(rows),len(cases))
    fixture={'source_is_original_execution':True,'interventions':'Explicit setup before original E54F and restored at its return, each E58A result recorded. Not natural gameplay.',
             'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'fields':FIELDS,'world_fields':world,'records':rows}
    target=ROOT/f'games/contra/engine/testdata/original-enemies-aliens-{mode}.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'records':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
