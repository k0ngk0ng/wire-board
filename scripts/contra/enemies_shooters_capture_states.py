"""Record original shooter state-machine calls before extending implementation.

Controlled setup is restricted to entry at the real enemy dispatcher, then the
original full state routine executes. Input pool/world/player fields are restored
after its return. This is boundary coverage, never a natural game completion.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/states'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/states.jsonl','w'));local fields=FIELDS;local cases=CASES;local index=0;local pending=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function pool()
 local result={};for slot=0,15 do local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a+slot))end;result[#result+1]='['..table.concat(values,',')..']'end
 return '['..table.concat(result,',')..']'
end
local function clearPool()for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending or pending.pc~=pc then return end
  local p=pending;pending=nil
  f:write(string.format('{"case":%d,"before":%s,"after":%s,"sounds":[%s]}\n',p.index,p.before,pool(),table.concat(p.sounds,',')))
  for a,v in pairs(p.restore)do memory.writebyte(a,v)end
 end)
end
memory.registerexec(0xf9bc,1,function()if pending then pending.sounds[#pending.sounds+1]=tostring(memory.getregister('a'))end end)
memory.registerexec(0xe54f,1,function()
 index=index+1;assert(cases[index],'unexpected frame')
 clearPool();memory.writebyte(0x528+15,6);memory.writebyte(0x4b8+15,1)
 memory.writebyte(0x38,0);memory.writebyte(0x39,0)
end)
memory.registerexec(0xe58a,1,function()
 if memory.getregister('x')~=15 then return end
 assert(read(0xfc)==224 and read(0xfd)==0,'controlled initial camera changed')
 assert(not pending);local c=cases[index];local p={index=index,pc=ret(),restore={},sounds={}}
 local function set(a,v)if p.restore[a]==nil then p.restore[a]=read(a)end;memory.writebyte(a,v)end
 for slot=0,15 do for _,a in ipairs(fields)do set(a+slot,0)end end
 for i,a in ipairs(fields)do set(a+15,c.values[i])end
 set(4,c.routine*2);set(0x30,0);set(0x40,c.sceneType);set(0x41,c.vertical);set(0x68,c.scroll);set(0x1a,c.frameCounter);set(0x8e,c.enabled)
 for i=0,1 do set(0x90+i,c.players[i+1].state);set(0x334+i,c.players[i+1].x);set(0x31a+i,c.players[i+1].y)end
 p.before=pool();pending=p;arm(p.pc)
end)
function finish_probe()assert(not pending);f:close()end
'''

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    default=dict(kind=6,routine=1,attributes=0,x=180,y=100,frame=3,animationDelay=1,attackDelay=1,data=[0,0,0,1],width=0x83,sprite=0x43,velocityX=0,velocityY=0,sceneType=0,vertical=0,scroll=1,frameCounter=5,enabled=1,players=[dict(state=1,x=48,y=100),dict(state=1,x=96,y=120)])
    cases=[]
    for attr in range(3):
        for routine in range(1,10):
            for delay in [1,2]:
                frame=0xff if routine==8 else 3
                if routine==4:frame=2
                cases.append(dict(default,attributes=attr,routine=routine,frame=frame,animationDelay=delay))
    for attr in range(3):
        for scene in [0,1,128]:
            for states in [(1,1),(0,1),(1,0),(0,0)]:
                for target in [(48,40),(220,180)]:
                    cases.append(dict(default,attributes=attr,routine=3,sceneType=scene,players=[dict(state=states[0],x=target[0],y=target[1]),dict(state=states[1],x=90,y=80)]))
    for change in [dict(routine=3,data=[0,0,0,0]),dict(routine=3,enabled=0),dict(routine=3,x=8,scroll=2),dict(routine=5,x=8),dict(routine=5,x=250),dict(routine=5,data=[0,1,0,0]),dict(routine=6,y=4),dict(routine=6,velocityY=-1152),dict(routine=7,sprite=0),dict(routine=8,frame=2),dict(routine=8,width=0x8b,frame=3),dict(routine=9,vertical=1,y=231,scroll=2)]:cases.append(dict(default,**change))
    for attributes in [4,8]:
        for direction in [0,1]:
            for shots in [0,1,3]:
                for x in [7,16,128,240,250]:
                    cases.append(dict(default,kind=5,routine=4,attributes=attributes,x=x,data=[0,direction,shots,0],frame=0,width=0x02))
    encoded=[]
    for c in cases:
        c['values']=[c['sprite'],c['y'],c['x'],0,c['routine'],0,0,(c['velocityY']>>8)&255,c['velocityY']&255,(c['velocityX']>>8)&255,c['velocityX']&255,c['kind'],c['animationDelay'],0,c['attackDelay'],c['frame'],1,0x30,c['width'],c['attributes'],*c['data']]
        terms=[]
        for key,value in c.items():
            if key=='players':terms.append('players={'+','.join('{'+','.join(f'{k}={v}'for k,v in p.items())+'}'for p in value)+'}')
            elif isinstance(value,list):terms.append(key+'={'+','.join(map(str,value))+'}')
            else:terms.append(f'{key}={value}')
        encoded.append('{'+','.join(terms)+'}')
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    spec={'name':'shooters-states-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],
          'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join(encoded)+'}')}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'states.jsonl').read_text().splitlines()];assert len(rows)==len(cases)
    data={'source_is_original_execution':True,'interventions':'Enemy slot/world/player fields staged at real E58A; original state routine executes, then input fields/pool restored. Not natural traversal.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    target=ROOT/'games/contra/engine/testdata/original-enemy-shooters-states.json.gz'
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
    else:target.write_bytes(packed)
    print(json.dumps({'cases':len(cases),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
