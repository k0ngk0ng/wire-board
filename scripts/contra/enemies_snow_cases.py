"""Controlled original snow enemy states; setup is declared, results are recorded."""
import gzip,hashlib,json,sys,zipfile
from pathlib import Path
from enemies_snow_capture import ROOT,BASE,FIELDS,WORLD,HOOK
from capture_reference import run

def make_cases():
    cases=[]
    for kind,states in [(16,3),(17,5),(19,1),(21,7),(22,4)]:
        for routine in range(1,states+1):
            for delay in [0,1,4]:
                cases.append(dict(kind=kind,routine=routine,delay=delay,x=180,y=120,frame=8,frameScroll=1,joint=0,map=0))
    cases += [dict(kind=16,routine=1,delay=9,x=x,y=100,frame=1,frameScroll=1,joint=0,map=0)for x in [0,7,8,199,200,201]]
    for x in [0,7,8,255]:
        for joint in [0,63]:cases.append(dict(kind=19,routine=1,delay=0,x=x,y=120,frame=1,frameScroll=3,joint=joint,map=0))
    for y in [168,169,175,176,231,232,255]:
        for kind,routine in [(17,2),(21,3),(22,1)]:cases.append(dict(kind=kind,routine=routine,delay=1,x=180,y=y,frame=7,frameScroll=0,joint=0,map=0x55,vy=256))
    for x in [7,8,31,32,223,224,255]:cases.append(dict(kind=21,routine=2,delay=1,x=x,y=120,frame=1,frameScroll=0,joint=0,map=0,vx=0))
    cases += [dict(kind=16,routine=2,delay=1,x=180,y=100,frame=1,frameScroll=0,joint=0,map=0,full=1)]
    return cases

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1';cases=make_cases()
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    p=rom[16:131088];properties=int.from_bytes(p[0x1edf3+8:0x1edf5+8],'little')
    encoded=[]
    for c in cases:
        kind=c['kind'];props=p[0x10000+properties+kind*4:0x10000+properties+kind*4+4]
        vx=c.get('vx',384 if kind==21 else 128);vy=c.get('vy',-256 if kind==17 else 128)
        values=[0x7c if kind==21 else 0x22 if kind==22 else 1,c['y'],c['x'],1,c['routine'],240,250,(vy>>8)&255,vy&255,(vx>>8)&255,vx&255,kind,c['delay'],props[3],7,0,props[2],props[1],props[0],0,0,0,0,0]
        encoded.append('{values={'+','.join(map(str,values))+'},frame=%d,scroll=%d,joint=%d,map=%d,full=%d}'%(c['frame'],c['frameScroll'],c['joint'],c['map'],c.get('full',0)))
    controlled=r'''
local cases={CASES};local selected=nil;local index=0
local controlSetup=nil
memory.registerexec(0xc001,1,function()
 memory.writebyte(0x18,5);memory.writebyte(0x2c,4);memory.writebyte(0x30,4);memory.writebyte(0x3b,0)
 for i=0,1 do memory.writebyte(0x38+i,0);memory.writebyte(0x90+i,1);memory.writebyte(0xae+i,255)end
end)
memory.registerexec(0xe54f,1,function()
 index=index+1;local c=cases[index];assert(c,'too many controlled frames');selected=c
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 if c.full~=0 then for slot=0,14 do memory.writebyte(0x4b8+slot,1);memory.writebyte(0x528+slot,5);memory.writebyte(0x33e+slot,180);memory.writebyte(0x324+slot,100);memory.writebyte(0x598+slot,0x81)end end
 for i,a in ipairs(fields)do memory.writebyte(a+15,c.values[i])end
 memory.writebyte(0x1a,c.frame);memory.writebyte(0x68,c.scroll);memory.writebyte(0x7f,c.joint)
 for a=0x680,0x6ff do memory.writebyte(a,c.map)end
end)
'''.replace('CASES',','.join(encoded))
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('CONTROLLED',controlled)
    spec={'name':'snow-small-cases-'+label,'load_state':'stage-5','frames':len(cases),'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':90}
    BASE.mkdir(parents=True,exist_ok=True);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'snow.jsonl').read_text().splitlines()]
    assert sum(row['slot']==15 for row in records)==len(cases)
    fixture={'source_is_original_execution':True,'scope':'controlled-small-types','interventions':'Per frame: one explicit entity/pool/frame/collision setup, then original dispatch. Source stage entry has its declared level initialization override. Not natural play.',
      'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],'enemy_fields':FIELDS,'records':records}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0);target=ROOT/'games/contra/engine/testdata/original-enemies-snow-small-cases.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==records,'repeat differs'
    else:target.write_bytes(packed)
    print(json.dumps({'cases':len(cases),'records':len(records),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
