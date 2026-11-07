"""Controlled Japanese tank/carrier calls, retaining real queue/palette effects."""
import gzip,hashlib,json,sys,zipfile
from pathlib import Path
from enemies_snow_capture import ROOT,BASE,FIELDS,WORLD,HOOK
from capture_reference import run

def cases():
    out=[]
    def add(kind,routine,**kw):
        c=dict(kind=kind,routine=routine,x=160,y=144 if kind==18 else 80,high=0,delay=1,attack=1,frame=1,scroll=0,tank=0,pause=0,joint=0,fade=0,aim=12 if kind==18 else 0,burst=0,wait=32,hp=71 if kind==18 else 64,width=4,attrs=0,weapon=0,random=0,used=0,full=0,others=0)
        if kind==18 and routine==6:c.update(delay=0,aim=5,hp=0)
        if kind==20:
            c['delay']={1:1,2:1,3:3,4:1,5:17,6:1,7:1,8:3,9:1,10:1,11:0,12:0}[routine]
            c['aim']={4:2,6:1,11:4,12:2}.get(routine,0)
            if routine>=10:c['hp']=0
        c.update(kw);out.append(c)
    for routine in range(1,7):
        for frame in [0,1]:
            for used in [0,64]:add(18,routine,frame=frame,used=used)
    for high,x in [(1,0),(1,48),(0,0),(0,8),(0,159),(0,160),(255,207),(255,208)]:
        add(18,2,high=high,x=x,scroll=1)
        add(18,3,high=high,x=x,scroll=1,hp=0,delay=2)
    for attrs in [0,1]:
        for strength in range(4):add(18,2,x=159,attrs=attrs,weapon=strength)
    for aim in [10,11,12]:
        for x in [20,48,160]:add(18,3,aim=aim,x=x,burst=2,attrs=5)
    for aim in range(6):
        for used in [0,64]:add(18,6,aim=aim,used=used)
    for routine in range(1,13):
        for used in [0,31,64]:add(20,routine,used=used,others=1 if routine==10 else 0)
    for routine in [3,8]:
        for delay in range(4):add(20,routine,delay=delay)
    for delay in [0,1,4,17,33,49,65]:
        add(20,5,delay=delay);add(20,5,delay=delay,full=1)
    for aim in [0,1,2]:add(20,4,aim=aim)
    for aim in [1,2,3]:add(20,6,aim=aim)
    for aim in [0,1,2,3,4,255]:
        for used in [0,64]:add(20,11,aim=aim,used=used)
    for aim in [0,1,2,255]:
        for used in [0,64]:add(20,12,aim=aim,used=used)
    for y in [16,48,80,112]:
        for random in range(4):add(20,2,y=y,random=random)
    return out

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1';items=cases()
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    p=rom[16:131088];ptr=int.from_bytes(p[0x1edf3+8:0x1edf5+8],'little');encoded=[]
    for c in items:
        props=p[0x10000+ptr+c['kind']*4:0x10004+ptr+c['kind']*4]
        values=[1,c['y'],c['x'],0,c['routine'],0,c['high'],0,0,0,0,c['kind'],c['delay'],props[3],c['attack'],0,c['hp'],props[1],c['width'],c['attrs'],c['aim'],0,c['burst'],c['wait']]
        entry='{values={'+','.join(map(str,values))+'},'+','.join(f'{k}={v}'for k,v in c.items())+'}'
        encoded.append(entry)
    control=r'''
local cases={CASES};local selected=nil;local caseIndex=0
memory.registerexec(0xc001,1,function()
 memory.writebyte(0x18,5);memory.writebyte(0x2c,4);memory.writebyte(0x30,4);memory.writebyte(0x3b,0)
 for i=0,1 do memory.writebyte(0x38+i,0);memory.writebyte(0x90+i,1);memory.writebyte(0xae+i,255)end
end)
memory.registerexec(0xe54f,1,function()
 caseIndex=caseIndex+1;local c=cases[caseIndex];assert(c);selected=c
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 if c.full~=0 then for slot=0,14 do memory.writebyte(0x4b8+slot,1);memory.writebyte(0x528+slot,5);memory.writebyte(0x33e+slot,180);memory.writebyte(0x324+slot,100);memory.writebyte(0x598+slot,0x81)end end
 if c.others~=0 then
  for slot,kind in ipairs({21,22,5,2})do local i=slot-1;memory.writebyte(0x4b8+i,1);memory.writebyte(0x528+i,kind);memory.writebyte(0x30a+i,1);memory.writebyte(0x578+i,slot==4 and 0xf0 or 1);memory.writebyte(0x598+i,0x81);memory.writebyte(0x33e+i,180);memory.writebyte(0x324+i,100)end
 end
 for i,a in ipairs(fields)do memory.writebyte(a+15,c.values[i])end
 memory.writebyte(0x1a,c.frame);memory.writebyte(0x68,c.scroll);memory.writebyte(0x77,c.tank);memory.writebyte(0x78,c.pause);memory.writebyte(0x7f,c.joint);memory.writebyte(0x74,c.fade)
 memory.writebyte(0x2f,c.weapon);memory.writebyte(0x34,c.random);memory.writebyte(0x8e,1)
end)
local function controlSetup()
 local restore={[0x21]=memory.readbyte(0x21)}
 for a=0x700,0x77f do restore[a]=memory.readbyte(a)end
 memory.writebyte(0x21,selected.used)
 return restore
end
'''.replace('CASES',','.join(encoded))
    wf=WORLD+[('requestedColors',0x36),('work06',6)]
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CONTROLLED',control)
    name='snow-complex-'+label;spec={'name':name,'load_state':'stage-5','frames':len(items),'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'snow.jsonl').read_text().splitlines()]
    assert sum(r['slot']==15 for r in rows)==len(items)
    data={'source_is_original_execution':True,'scope':'controlled-tank-carrier','interventions':'Per-frame explicit snow entity/global setup; original E58A executes including real graphics/palette/Boss helper calls. Staged graphics cursor/prefix restored only after recorded return. Not natural campaign play.',
       'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':items,'world_fields':[{'name':n,'address':hex(a)}for n,a in wf],'enemy_fields':FIELDS,'records':rows}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);dest=ROOT/'games/contra/engine/testdata/original-enemies-snow-complex-cases.json.gz'
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'cases':len(items),'records':len(rows),'events':sum(len(r['events'])for r in rows),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
