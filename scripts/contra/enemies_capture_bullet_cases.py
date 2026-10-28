"""Controlled original enemy-bullet states, distinct from natural gameplay.

Each original dispatch receives an explicitly documented slot/world setup before
the original $E54F loop. Expected results are recorded at its actual $E58A return;
no result is predicted by this script and no campaign completion is claimed.
"""
import gzip
import hashlib
import json
from pathlib import Path
import shutil
import sys

import enemies_capture as capture
from capture_reference import run

ROOT,BASE=capture.ROOT,capture.BASE

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    cases=[]
    for variant in range(6):
        for routine in range(1,5):
            for vertical in (0,1):
                cases.append(dict(variant=variant,routine=routine,vertical=vertical,x=180,y=100,vx=-385,vy=273,fx=240,fy=250,timer=1,animation=1,frame=12,scroll=1,level=0,solid=0))
    for extra in [dict(x=8,vx=-1,fx=0),dict(x=7,vx=0),dict(y=232,vy=0),dict(y=231,vy=256),dict(x=255,vx=512),dict(y=255,vy=512),dict(vx=32767,vy=-32768),dict(level=4),dict(solid=128),dict(vertical=1,y=231,scroll=2),dict(timer=0,routine=3),dict(routine=3,animation=2),dict(routine=2,variant=4,frame=0),dict(routine=2,variant=4,frame=4),dict(routine=2,variant=4,frame=8)]:
        c=dict(variant=0,routine=2,vertical=0,x=180,y=100,vx=0,vy=0,fx=0,fy=0,timer=1,animation=0,frame=12,scroll=0,level=0,solid=0);c.update(extra);cases.append(c)
    fields=capture.FIELDS
    world=capture.WORLD+[('solidFlags',0x59)]
    setup=[]
    for case in cases:
        values=[0x1e,case['y'],case['x'],1,case['routine'],case['fy'],case['fx'],
                (case['vy']>>8)&255,case['vy']&255,(case['vx']>>8)&255,case['vx']&255,
                1,case['timer'],0,0,case['animation'],1,1,0x80,0,case['variant'],0,0,0]
        setup.append('{values={'+','.join(map(str,values))+'},vertical=%d,scroll=%d,frame=%d,level=%d,solid=%d}'%(case['vertical'],case['scroll'],case['frame'],case['level'],case['solid']))
    inject=r'''
local setups={SETUPS};local caseIndex=0
local entityFields={ENTITY_FIELDS}
memory.registerexec(0xe54f,1,function()
 caseIndex=caseIndex+1;local c=setups[caseIndex];assert(c,'too many controlled dispatches')
 for slot=0,15 do for _,address in ipairs(entityFields)do memory.writebyte(address+slot,0)end end
 for i,address in ipairs(entityFields)do memory.writebyte(address+15,c.values[i])end
 memory.writebyte(0x41,c.vertical);memory.writebyte(0x68,c.scroll);memory.writebyte(0x1a,c.frame)
 memory.writebyte(0x30,c.level);memory.writebyte(0x59,c.solid)
 for a=0x680,0x6ff do memory.writebyte(a,c.solid~=0 and 0xff or 0)end
end)
'''.replace('SETUPS',','.join(setup)).replace('ENTITY_FIELDS',','.join(hex(a)for _,a in fields))
    hook=capture.HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in fields)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in world)+'}')
    target=BASE/'states/coop-play.fcs';target.parent.mkdir(parents=True,exist_ok=True)
    if not target.exists():shutil.copyfile(ROOT/'.local/contra/states/coop-play.fcs',target)
    name='controlled-enemy-bullets-'+label
    spec={'name':name,'load_state':'coop-play','frames':len(cases),'captures':[], 'hooks_lua':hook+'\n'+inject}
    path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'enemies.jsonl').read_text().splitlines()]
    rows=[r for r in rows if r['kind']=='enemyStep'];assert len(rows)==len(cases),(len(rows),len(cases))
    fixture={'source_is_original_execution':True,'interventions':'Each frame sets one enemy bullet, world scroll/frame/level flags, and collision map before original dispatch. Not natural play.',
       'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,
       'fields':[{'name':n,'address':hex(a)}for n,a in fields],'world_fields':[{'name':n,'address':hex(a)}for n,a in world],
       'maps':[json.loads(l)for l in (out/'enemy-maps.jsonl').read_text().splitlines()],'records':rows}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    path=ROOT/'games/contra/engine/testdata/original-enemy-bullet-cases.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()));assert old['records']==rows and old['maps']==fixture['maps'], 'controlled bullet replay differs'
    else:path.write_bytes(packed)
    print(json.dumps({'cases':len(rows),'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
