"""Record original enemy firing and aim boundaries under explicit setup.

Failures defined before implementation: field/slot mutation beyond allocation;
attack flag exception lost; speed clamp/rounding/angle quadrant drift; nibble
threshold boundaries; missing original inactive-slot contents; and controlled
function tests mislabeled as natural gameplay.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-shooters/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

FIELDS=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]

HOOK=r'''
local f=assert(io.open(out..'/shooters.jsonl','w'));local fields=FIELDS
local cases=CASES;local index=0;local pending=nil;local aim=nil;local aimReturn=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function pair(a)return read(a)+256*read(a+1)end
local function ret()local s=memory.getregister('s');return pair(0x100+AND(s+1,255))+1 end
local function pool()
 local result={};for slot=0,15 do local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a+slot))end;result[#result+1]='['..table.concat(values,',')..']'end
 return '['..table.concat(result,',')..']'
end
local function savePool()local result={};for slot=0,15 do for _,a in ipairs(fields)do result[a+slot]=read(a+slot)end end;return result end
local function clearPool()for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if aim and aim.pc==pc then
   local a=aim;aim=nil
   f:write(string.format('{"kind":"quadrant","case":%d,"value":%d,"quadrant":%d}\n',a.index,memory.getregister('a'),read(7)))
  end
  if aimReturn and aimReturn.pc==pc then
   f:write(string.format('{"kind":"angle","case":%d,"value":%d}\n',aimReturn.index,read(0x0c)))
   aimReturn=nil
  end
  if pending and pending.pc==pc then
   local p=pending;pending=nil
   f:write(string.format('{"kind":"shot","case":%d,"frame":%d,"success":%s,"before":%s,"after":%s}\n',p.index,emu.framecount(),tostring(memory.getregister('a')==0),p.before,pool()))
   for a,v in pairs(p.restore)do memory.writebyte(a,v)end
  end
 end)
end
memory.registerexec(0xe54f,1,function()
 index=index+1;assert(cases[index],'unexpected extra frame')
 clearPool();local i=15
 memory.writebyte(0x528+i,6);memory.writebyte(0x4b8+i,3);memory.writebyte(0x30a+i,0x43)
 memory.writebyte(0x33e+i,180);memory.writebyte(0x324+i,100);memory.writebyte(0x558+i,1)
 memory.writebyte(0x568+i,3);memory.writebyte(0x5e8+i,1);memory.writebyte(0x598+i,0x80)
 memory.writebyte(0x40,0);memory.writebyte(0x41,0);memory.writebyte(0x68,0)
 for p=0,1 do memory.writebyte(0x90+p,1);memory.writebyte(0xae+p,255);memory.writebyte(0x38+p,0)end
end)
memory.registerexec(0xf3b7,1,function()aimReturn={index=index,pc=ret()};arm(aimReturn.pc)end)
memory.registerexec(0xf4c4,1,function()
 if not aimReturn then return end
 local c=cases[index]
 memory.writebyte(8,c.sy);memory.writebyte(9,c.sx);memory.writebyte(0x0a,c.ty);memory.writebyte(0x0b,c.tx);memory.writebyte(0x0f,c.precision)
 aim={index=index,pc=ret()};arm(aim.pc)
end)
memory.registerexec(0xf225,1,function()
 local c=cases[index];assert(c and not pending)
 local p={index=index,pc=ret(),restore=savePool()}
 p.restore[0x8e]=read(0x8e);p.restore[0x30]=read(0x30)
 clearPool()
 if c.fill~=0 then for slot=0,15 do memory.writebyte(0x4b8+slot,1);memory.writebyte(0x528+slot,5);memory.writebyte(0x30a+slot,0x3b)end end
 if c.hole>=0 then memory.writebyte(0x4b8+c.hole,0)end
 memory.writebyte(8,c.y);memory.writebyte(9,c.x);memory.writebyte(0x8e,c.enabled);memory.writebyte(0x30,0)
 memory.setregister('a',c.angle);memory.setregister('y',c.speed)
 p.before=pool();pending=p;arm(p.pc)
end)
function finish_probe()assert(not pending and not aim and not aimReturn);f:close()end
'''

def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    base=dict(angle=12,speed=3,x=170,y=90,enabled=1,fill=0,hole=-1,sx=180,sy=100,tx=48,ty=100,precision=1)
    cases=[]
    for kind in range(5):
        for speed in range(10):
            for angle in range(24):cases.append(dict(base,angle=kind*32+angle,speed=speed))
    for source in [(0,0),(128,128),(255,255)]:
        for precision in (0,1):
            for x in [0,1,31,32,33,63,64,65,127,128,129,191,192,223,224,255]:
                for y in [0,1,31,32,33,63,64,65,127,128,129,191,192,223,224,255]:
                    cases.append(dict(base,sx=source[0],sy=source[1],tx=x,ty=y,precision=precision))
    for kind in range(5):
        for enabled in (0,1):
            for fill,hole in [(0,-1),(1,-1),(1,0),(1,7),(1,15)]:cases.append(dict(base,angle=kind*32+12,enabled=enabled,fill=fill,hole=hole))
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    origin=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(origin,target)
    assert target.read_bytes()==origin.read_bytes()
    lua='{' + ','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}'
    spec={'name':'shooters-controlled-'+label,'load_state':'coop-play','frames':len(cases),'segments':[], 'captures':[],
          'hooks_lua':HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES',lua),'timeout_seconds':90}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in (out/'shooters.jsonl').read_text().splitlines()]
    kinds={kind:sum(r['kind']==kind for r in rows)for kind in ('shot','angle','quadrant')}
    assert all(n==len(cases)for n in kinds.values()),kinds
    data={'source_is_original_execution':True,'scope':'controlled function boundaries, not natural gameplay',
          'interventions':'One prepared sniper enters original dispatch each frame. At original F4C4/F225 only, inputs and slot availability are staged; original output is recorded, then slots/flags are restored.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'games/contra/engine/testdata/original-enemy-shooters-boundaries.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
    else:dest.write_bytes(packed)
    (out/'summary.json').write_text(json.dumps({'counts':kinds,'fixture_sha256':hashlib.sha256(dest.read_bytes()).hexdigest()},indent=2)+'\n')
    print(json.dumps({'counts':kinds,'fixture_sha256':hashlib.sha256(dest.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
