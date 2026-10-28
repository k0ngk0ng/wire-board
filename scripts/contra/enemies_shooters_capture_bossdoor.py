"""Record original first-stage Boss door, globals and tunnel collision updates.

Failure inventory before implementation: audio reset ordering; premature level
completion; reset of delay-high when original preserves it; wrong destroy-all
exceptions; queue refusal advancing tunnel; collision before accepted drawing;
full explosion pool; final offset applied twice; and direct platform victory.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters/bossdoor'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
HOOK=r'''
local f=assert(io.open(out..'/bossdoor.jsonl','w'));local fields=FIELDS;local cases=CASES;local index=0;local pending=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function pool()local result={};for slot=0,15 do local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a+slot))end;result[#result+1]='['..table.concat(values,',')..']'end;return '['..table.concat(result,',')..']'end
local function globals()return string.format('{"delayLow":%d,"delayHigh":%d,"boss":%d}',read(0x2a),read(0x2b),read(0x3b))end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending or pending.pc~=pc then return end
  local p=pending;pending=nil
  f:write(string.format('{"case":%d,"before":%s,"after":%s,"globalBefore":%s,"globalAfter":%s,"audio":[%s],"requests":[%s],"collisionBefore":"%s","collisionAfter":"%s","supertiles":"%s","used":%d,"queue":"%s"}\n',p.index,p.before,pool(),p.globalBefore,globals(),table.concat(p.audio,','),table.concat(p.requests,','),p.collision,hex(0x680,128),p.supertiles,read(0x21),hex(0x700,128)))
  for a,v in pairs(p.restore)do memory.writebyte(a,v)end
 end)
end
memory.registerexec(0xf9d4,1,function()if pending then pending.audio[#pending.audio+1]='{"kind":"reset"}'end end)
memory.registerexec(0xf9bc,1,function()if pending then pending.audio[#pending.audio+1]=string.format('{"kind":"play","code":%d}',memory.getregister('a'))end end)
memory.registerexec(0xc3d1,1,function()if pending then pending.requests[#pending.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
memory.registerexec(0xe54f,1,function()
 index=index+1;assert(cases[index],'unexpected extra frame')
 for slot=0,15 do memory.writebyte(0x4b8+slot,0)end
 memory.writebyte(0x528+15,17);memory.writebyte(0x4b8+15,1)
end)
memory.registerexec(0xe58a,1,function()
 if memory.getregister('x')~=15 then return end
 assert(not pending);assert(read(0xfc)==224 and read(0xfd)==0,'initial camera changed')
 local c=cases[index];local p={index=index,pc=ret(),restore={},audio={},requests={},supertiles=hex(0x600,128)}
 local function set(a,v)if p.restore[a]==nil then p.restore[a]=read(a)end;memory.writebyte(a,v)end
 for slot=0,15 do for _,a in ipairs(fields)do set(a+slot,0)end end
 for slot=0,14 do
  if c.full~=0 or c.destroyMatrix~=0 then
   local kind=5;local hp=1;local routine=3;local sprite=0x3b
   if c.destroyMatrix~=0 then
    if slot==1 then kind=2 end;if slot==2 then kind=3 end;if slot==3 then kind=6 end;if slot==4 then kind=1 end
    if slot==5 then hp=0 end;if slot==6 then hp=0xf0 end;if slot==7 then routine=0 end;if slot==8 then sprite=0 end;if slot==9 then routine=6 end
   end
   set(0x528+slot,kind);set(0x4b8+slot,routine);set(0x30a+slot,sprite);set(0x578+slot,hp);set(0x5a8+slot,3)
  end
 end
 for i,a in ipairs(fields)do set(a+15,c.values[i])end
 set(4,c.routine*2);set(0x30,0);set(0x40,0);set(0x41,0);set(0x68,c.scroll);set(0x2a,c.delayLow);set(0x2b,c.delayHigh);set(0x3b,c.boss)
 set(0x21,c.used);for a=0x700,0x77f do set(a,0)end
 for a=0x680,0x6ff do set(a,0xff)end
 p.before=pool();p.globalBefore=globals();p.collision=hex(0x680,128);pending=p;arm(p.pc)
end)
function finish_probe()assert(not pending);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    base=dict(routine=1,x=160,y=176,frame=0,animationDelay=1,index=0,width=0x0c,sprite=1,hp=32,scroll=0,used=0,full=0,destroyMatrix=0,delayLow=17,delayHigh=3,boss=0)
    cases=[]
    for routine in range(1,8):
        for used in (0,63,64):
            for delay in (1,2):
                frame=255 if routine==4 else 0
                cases.append(dict(base,routine=routine,used=used,animationDelay=delay,frame=frame,boss=0 if routine<4 else 1))
    for hp in (0,32):
        for sprite in (0,1):cases.append(dict(base,routine=3,hp=hp,sprite=sprite,destroyMatrix=1))
    for index in range(8):
        for delay in (1,2,8):
            for used in (0,64):
                cases.append(dict(base,routine=7,index=index,animationDelay=delay,used=used,boss=1,x=144+(index//2)*32,y=160+(index%2)*32))
    for full in (0,1):
        for index in (0,3,7):cases.append(dict(base,routine=7,index=index,animationDelay=1,full=full,boss=1))
    cases.extend([dict(base,routine=7,index=8,animationDelay=2,boss=1),dict(base,routine=7,index=8,animationDelay=8,boss=1)])
    encoded=[]
    for c in cases:
        c['values']=[c['sprite'],c['y'],c['x'],0,c['routine'],0,0,0,0,0,0,17,c['animationDelay'],0,0,c['frame'],c['hp'],0x82,c['width'],0,c['index'],0,0,0]
        encoded.append('{'+','.join(k+'={'+','.join(map(str,v))+'}'if isinstance(v,list)else f'{k}={v}'for k,v in c.items())+'}')
    (BASE/'states').mkdir(parents=True,exist_ok=True);origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    spec={'name':'bossdoor-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],
          'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join(encoded)+'}')}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'bossdoor.jsonl').read_text().splitlines()];assert len(rows)==len(cases)
    data={'source_is_original_execution':True,'interventions':'Type17/world/queue/collision and selected other slots staged at E58A; original routine executes and inputs are restored. Not natural boss traversal.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);dest=ROOT/'games/contra/engine/testdata/original-enemy-shooters-bossdoor.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    print(json.dumps({'cases':len(cases),'sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
