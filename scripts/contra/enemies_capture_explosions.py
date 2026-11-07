"""Controlled original shared destruction phases, including native sound calls."""
import gzip,hashlib,json,shutil,sys
from pathlib import Path
import enemies_capture as capture
from capture_reference import run
ROOT,BASE=capture.ROOT,capture.BASE

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    runner=len(sys.argv)>2 and sys.argv[2]=='runner'
    cases=[]
    for kind in (5,6):
        for routine in (7,8,9):
            for width in (0x83,0x8b,0x81):
                for sprite in (0,5):cases.append(dict(kind=kind,routine=routine,width=width,sprite=sprite,frame=0,timer=1,x=180,y=100,vertical=0,scroll=1))
    for extra in [dict(frame=255),dict(frame=2),dict(frame=3),dict(frame=4),dict(timer=0),dict(vertical=1,y=231,scroll=2),dict(x=8,scroll=2)]:
        c=dict(kind=5,routine=8,width=0x83,sprite=5,frame=0,timer=1,x=180,y=100,vertical=0,scroll=1);c.update(extra);cases.append(c)
    if runner:
        cases=[dict(kind=5,routine=r,width=0x82,sprite=0x3b,frame=11 if r<7 else 0,timer=1,x=x,y=y,vertical=v,scroll=2,facing=f)
               for r in [5,6,7,8,9] for v in [0,1] for x,y in [(8,100),(16,7),(180,100),(240,231)] for f in [0,1]]
        cases.append(dict(kind=5,routine=8,width=0x83,sprite=1,frame=255,timer=1,x=180,y=100,vertical=0,scroll=1,facing=0))
    setups=[]
    for c in cases:
        b=[c['sprite'],c['y'],c['x'],0x49,c['routine'],12,13,1,24,255,60,c['kind'],c['timer'],7,9,c['frame'],0,0x10,c['width'],2,3,c.get('facing',0),4,1]
        setups.append('{values={'+','.join(map(str,b))+'},vertical=%d,scroll=%d}'%(c['vertical'],c['scroll']))
    inject=r'''
local setups={SETUPS};local ordinal=0
local fields={FIELDS}
local sound=assert(io.open(out..'/explosion-sounds.jsonl','w'))
memory.registerexec(0xe54f,1,function()
 ordinal=ordinal+1;local c=setups[ordinal];assert(c)
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 for i,a in ipairs(fields)do memory.writebyte(a+15,c.values[i])end
 memory.writebyte(0x41,c.vertical);memory.writebyte(0x68,c.scroll)
end)
memory.registerexec(0xf9bc,1,function()
 local sp=memory.getregister('s');local ret=memory.readbyte(0x100+AND(sp+1,255))+256*memory.readbyte(0x100+AND(sp+2,255))+1
 if ret==0xe69b then sound:write(string.format('{"frame":%d,"code":%d}\n',emu.framecount(),memory.getregister('a')))end
end)
local original_finish=finish_probe
function finish_probe()original_finish();sound:close()end
'''.replace('SETUPS',','.join(setups)).replace('FIELDS',','.join(hex(a)for _,a in capture.FIELDS))
    hook=capture.HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in capture.FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in capture.WORLD)+'}')
    name=('controlled-enemy-runner-death-' if runner else 'controlled-enemy-explosions-')+label;spec={'name':name,'load_state':'coop-play','frames':len(cases),'captures':[],'hooks_lua':hook+'\n'+inject}
    p=BASE/(name+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
    rows=[json.loads(x)for x in (out/'enemies.jsonl').read_text().splitlines()];rows=[r for r in rows if r['kind']=='enemyStep'];assert len(rows)==len(cases)
    sounds=[json.loads(x)for x in (out/'explosion-sounds.jsonl').read_text().splitlines()]
    for r in rows:r['sounds']=[s['code']for s in sounds if s['frame']==r['frame']]
    data={'source_is_original_execution':True,'interventions':'Enemy entity setup before original E54F; recorded E58A output and E69B sound calls. Not natural gameplay.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows,
          'maps':[json.loads(l)for l in (out/'enemy-maps.jsonl').read_text().splitlines()]}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    target=ROOT/'games/contra/engine/testdata'/('original-enemy-runner-death.json.gz' if runner else 'original-enemy-explosions.json.gz')
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['records']==rows and old['maps']==data['maps'], 'destruction replay differs'
    else:target.write_bytes(packed)
    print(json.dumps({'cases':len(rows),'sounds':len(sounds),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
