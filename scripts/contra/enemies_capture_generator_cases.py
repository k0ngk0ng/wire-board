"""Explicit original AA85 boundary matrix; no claim of natural campaign play."""
import gzip,hashlib,json,sys
import enemies_capture as original
from capture_reference import run
ROOT,BASE=original.ROOT,original.BASE

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    default=dict(level=0,loop=0,strength=0,frame=4,random=3,screen=1,scroll=0,routine=2,timer=1,x=252,y=112,lastScreen=1,generated=0,attack=1,seed=0xa5,occupied=0,map=0x55,p0x=128,p1x=144,p0y=100,p1y=100,p0over=0,p1over=0)
    cases=[]
    def add(**options):c=default.copy();c.update(options);cases.append(c)
    for level in range(8):
        for loop in [0,1,4]:
            for strength in [0,2]:add(level=level,loop=loop,strength=strength,routine=0,timer=7)
    for level in [0,2,4,6]:
        for timer in [0,1,2,3]:
            for frame in [0,1,254,255]:
                for scroll in [0,1]:
                    for strength in [0,2]:
                        for terrain in [0,0x55]:add(level=level,routine=1,timer=timer,frame=frame,scroll=scroll,strength=strength,map=terrain,random=255,p0y=0,p1y=144)
    for seed in [0,1,2,3,254,255]:
        for occupied in range(8):
            for generated in [0,30]:add(seed=seed,occupied=occupied,generated=generated,scroll=1,random=0)
    for level in [0,2,4,5,6,7]:
        for screen in [1,7]:
            for x in [10,250]:
                for y in [0,63,64,223,224]:
                    for attack in [0,1]:
                        for random in [0,3]:add(level=level,screen=screen,x=x,y=y,attack=attack,random=random)
    for side in [10,252]:
        for p0x,p1x in [(0,128),(128,255),(0,255)]:
            for p0over,p1over in [(0,0),(0,1),(1,0),(1,1)]:add(x=side,p0x=p0x,p1x=p1x,p0over=p0over,p1over=p1over)
    for routine in [0,1,2]:add(routine=routine,lastScreen=0,generated=99,timer=1)
    setups=','.join('{'+','.join(k+'='+str(v)for k,v in c.items())+'}'for c in cases)
    world=original.WORLD+[('weaponStrength',0x2f),('burstSeed',6)]
    hook=original.HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in original.FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in world)+'}')
    hook=hook.replace('table.remove(pending,i)',"table.remove(pending,i);if p.kind=='soldierSpawn' then restore_generator_environment()end")
    inject=r'''
local cases={CASES};local ordinal=0;local prior={}
local map=memory.readbyterange(0x680,128)
local protected={0x18,0x1a,0x2c,0x30,0x31,0x2f,0x38,0x39,0x64,0x65,0x68,0x90,0x91,0x334,0x335,0x31a,0x31b}
function restore_generator_environment()
 for _,a in ipairs(protected)do memory.writebyte(a,prior[a])end
 for slot=0,15 do memory.writebyte(0x4b8+slot,0)end
 for i=1,128 do memory.writebyte(0x67f+i,string.byte(map,i))end
end
memory.registerexec(0xfa32,1,function()
 ordinal=ordinal+1;local c=cases[ordinal];assert(c)
 for _,a in ipairs(protected)do prior[a]=memory.readbyte(a)end
 local values={{0x30,c.level},{0x31,c.loop},{0x2f,c.strength},{0x1a,c.frame},{0x34,c.random},{0x64,c.screen},{0x68,c.scroll},
 {0x79,c.routine},{0x7a,c.timer},{0x7b,c.x},{0x7c,c.y},{0x195,c.lastScreen},{0x196,c.generated},{0x8e,c.attack},{6,c.seed},
 {0x334,c.p0x},{0x335,c.p1x},{0x31a,c.p0y},{0x31b,c.p1y},{0x38,c.p0over},{0x39,c.p1over}}
 for _,v in ipairs(values)do memory.writebyte(v[1],v[2])end
 for slot=0,15 do memory.writebyte(0x4b8+slot,slot<c.occupied and 1 or 0);memory.writebyte(0x528+slot,9)end
 for a=0x680,0x6ff do memory.writebyte(a,c.map)end
end)
'''.replace('CASES',setups)
    name='controlled-generators-'+label
    spec={'name':name,'load_state':'coop-play','frames':len(cases),'captures':[],'hooks_lua':hook+'\n'+inject,'timeout_seconds':120}
    p=BASE/(name+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
    rows=[json.loads(l)for l in (out/'enemies.jsonl').read_text().splitlines()];rows=[r for r in rows if r['kind']=='soldierSpawn'];assert len(rows)==len(cases),(len(rows),len(cases))
    fixture={'source_is_original_execution':True,'interventions':'Explicit generator/world/pool/map setup before AA85; upstream gameplay safety fields restored only after actual return was recorded. Not natural play.',
      'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows,'maps':[json.loads(l)for l in (out/'enemy-maps.jsonl').read_text().splitlines()]}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0);target=ROOT/'games/contra/engine/testdata/original-enemy-generator-cases.json.gz'
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['records']==rows and old['maps']==fixture['maps'], 'controlled generator replay differs'
    else:target.write_bytes(packed)
    print(json.dumps({'cases':len(rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
