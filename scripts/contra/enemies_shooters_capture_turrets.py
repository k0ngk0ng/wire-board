"""Original turret/Boss dispatch plus real graphics queue acceptance evidence.

Failures fixed before state implementation: phase order, queue gate/retry,
temporary Boss coordinate shift, camera-edge removal, aim tie direction,
weapon-strength/loop delays, and red-turret attribute-pointer target alias.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/turrets'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/turrets.jsonl','w'));local fields=FIELDS;local cases=CASES;local index=0;local pending=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
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
  f:write(string.format('{"case":%d,"before":%s,"after":%s,"sounds":[%s],"requests":[%s],"targets":[%s],"supertiles":"%s","used":%d,"queue":"%s"}\n',p.index,p.before,pool(),table.concat(p.sounds,','),table.concat(p.requests,','),table.concat(p.targets,','),p.supertiles,read(0x21),hex(0x700,128)))
  for a,v in pairs(p.restore)do memory.writebyte(a,v)end
 end)
end
memory.registerexec(0xf9bc,1,function()if pending then pending.sounds[#pending.sounds+1]=tostring(memory.getregister('a'))end end)
memory.registerexec(0xc3d1,1,function()
 if pending then pending.requests[#pending.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end
end)
memory.registerexec(0x8605,1,function()
 if pending and read(0x8000)==0x10 then local i=memory.getregister('y');pending.targets[#pending.targets+1]=string.format('{"index":%d,"x":%d,"y":%d}',i,read(0x334+i),read(0x31a+i))end
end)
memory.registerexec(0xe54f,1,function()
 index=index+1;assert(cases[index],'unexpected frame');clearPool()
 memory.writebyte(0x528+15,6);memory.writebyte(0x4b8+15,1);memory.writebyte(0x38,0);memory.writebyte(0x39,0)
end)
memory.registerexec(0xe58a,1,function()
 if memory.getregister('x')~=15 then return end
 assert(not pending);assert(read(0xfc)==224 and read(0xfd)==0,'initial camera changed')
 local c=cases[index];local p={index=index,pc=ret(),restore={},sounds={},requests={},targets={},supertiles=hex(0x600,128)}
 local function set(a,v)if p.restore[a]==nil then p.restore[a]=read(a)end;memory.writebyte(a,v)end
 for slot=0,15 do for _,a in ipairs(fields)do set(a+slot,0)end end
 for i,a in ipairs(fields)do set(a+15,c.values[i])end
 set(4,c.routine*2);set(0x30,0);set(0x40,0);set(0x41,c.vertical);set(0x68,c.scroll);set(0x1a,c.frameCounter);set(0x8e,c.enabled);set(0x2f,c.strength);set(0x31,c.loop);set(0x34,c.random)
 -- Requests starting below 64 use at most 106 bytes. Upper $07xx holds
 -- palette/score/mapper state, so it is not part of this controlled queue.
 set(0x21,c.used);for a=0x700,0x77f do set(a,0)end
 set(0x3a0,c.aliasY);set(0x3ba,c.aliasX)
 for i=0,1 do set(0x90+i,c.players[i+1].state);set(0x334+i,c.players[i+1].x);set(0x31a+i,c.players[i+1].y)end
 p.before=pool();pending=p;arm(p.pc)
end)
function finish_probe()assert(not pending);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    base=dict(kind=4,routine=1,attributes=0,x=180,y=100,frame=0,animationDelay=1,attackDelay=1,data=[6,1,0,0],width=0x8f,sprite=1,vertical=0,scroll=1,frameCounter=8,enabled=1,strength=0,loop=0,random=0,used=0,aliasX=40,aliasY=80,players=[dict(state=1,x=48,y=100),dict(state=1,x=96,y=80)])
    cases=[]
    for kind,maxstate in [(4,10),(7,9),(16,6)]:
        for routine in range(1,maxstate+1):
            for used in (0,63,64):
                for attrs in (0,1):
                    frame=0xff if routine==maxstate-1 else 0
                    if kind==7 and routine==4:frame=3
                    if kind==7 and routine==5:frame=2
                    data=[0 if kind==16 else 6,1,0,0]
                    cases.append(dict(base,kind=kind,routine=routine,used=used,attributes=attrs,frame=frame,data=data))
    for angle in range(12):
        for strength in range(4):
            for routine in (4,5):cases.append(dict(base,data=[angle,1,0,0],routine=routine,strength=strength))
    for angle in (6,7,8):
        for target in [(48,40),(220,200),(48,200)]:
            for alias in [(0,0),(250,250),(40,80)]:
                for used in (0,64):cases.append(dict(base,kind=7,routine=4,data=[angle,1,0,0],frame=3,players=[dict(state=1,x=target[0],y=target[1]),dict(state=1,x=90,y=80)],used=used,aliasX=alias[0],aliasY=alias[1]))
    for kind in (4,7,16):
        for change in [dict(x=7),dict(x=11),dict(x=24),dict(x=48),dict(x=240),dict(y=11),dict(vertical=1,y=199),dict(vertical=1,y=200),dict(enabled=0),dict(loop=1),dict(animationDelay=2,attackDelay=2),dict(random=3)]:
            cases.append(dict(base,kind=kind,routine=2 if kind==16 else 4,data=[2 if kind==16 else 6,1,0,0],frame=3 if kind==7 else 0,**change))
    for phase in (0,2):
        for random in range(4):
            for attrs in (0,1):cases.append(dict(base,kind=16,routine=2,data=[phase,0,0,0],attributes=attrs,random=random))
    encoded=[]
    for c in cases:
        c['values']=[c['sprite'],c['y'],c['x'],0,c['routine'],0,0,0,0,0,0,c['kind'],c['animationDelay'],0,c['attackDelay'],c['frame'],8,0x30,c['width'],c['attributes'],*c['data']]
        terms=[]
        for key,value in c.items():
            if key=='players':terms.append('players={'+','.join('{'+','.join(f'{k}={v}'for k,v in p.items())+'}'for p in value)+'}')
            elif isinstance(value,list):terms.append(key+'={'+','.join(map(str,value))+'}')
            else:terms.append(f'{key}={value}')
        encoded.append('{'+','.join(terms)+'}')
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    spec={'name':'turrets-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join(encoded)+'}')}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'turrets.jsonl').read_text().splitlines()];assert len(rows)==len(cases)
    data={'source_is_original_execution':True,'interventions':'One entity/world/player/graphics queue setup at real dispatcher; original full state runs and inputs are restored before normal continuation. Not natural traversal.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'games/contra/engine/testdata/original-enemy-shooters-turrets.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'cases':len(cases),'requests':sum(len(r['requests'])for r in rows),'targetIndexes':sorted({t['index']for r in rows for t in r['targets']}),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
