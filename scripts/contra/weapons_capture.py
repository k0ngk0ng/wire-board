"""Record Japanese weapon calls before implementing their independent rules.

Failure inventory: wrong bank; fire sampled after movement; held vs new B;
water/hidden/electrocution guards; M burst timer carry; occupied slots and P2
partition; R speeds; jump/down direction; stale projectile fields; unchecked
screen wrap; false claims that controlled RAM cases are natural gameplay.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-weapons/oracle'))
from capture_reference import BASE,run
import argparse,gzip,hashlib,json,shutil

COMMON=r'''
local f=assert(io.open(out.."/weapons.jsonl","w"));local fire=nil;local step=nil;local exits={};local caseIndex=0
local function read(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for k=a,a+n-1 do t[#t+1]=string.format('%02x',read(k)) end;return table.concat(t)end
local function puthex(a,s)for i=1,#s,2 do memory.writebyte(a+(i-1)/2,tonumber(string.sub(s,i,i+1),16))end end
local function request(i)
 return string.format('{"actor":%d,"x":%d,"y":%d,"hidden":%s,"aim":%d,"weapon":%d,"jump":%d,"water":%d,"electrocution":%d,"stageType":%d,"machineGunTimer":%d,"recoil":%d,"held":%d,"pressed":%d}',i,read(0x334+i),read(0x31a+i),tostring(read(0xba+i)~=0),read(0xc2+i),read(0xaa+i),read(0xa0+i),read(0xb2+i),read(0xc8+i),read(0x40),read(0xac+i),read(0xce+i),read(0xf1+i),read(0xf5+i))
end
local function returnpc()local s=memory.getregister('s');return read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1 end
local function arm(pc)
 if exits[pc] then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if fire and fire.pc==pc then
   local p=fire;fire=nil
   f:write(string.format('{"kind":"fire","case":%d,"frame":%d,"request":%s,"before":"%s","after":"%s","machineGunTimer":%d,"recoil":%d,"sounds":[%s]}\n',p.case,p.frame,p.request,p.before,hex(0x368,336),read(0xac+p.actor),read(0xce+p.actor),table.concat(p.sounds,',')))
   if p.restore then for a,v in pairs(p.restore)do memory.writebyte(a,v)end;puthex(0x368,p.pool)end
  end
  if step and step.pc==pc then
   local p=step;step=nil
   f:write(string.format('{"kind":"step","frame":%d,"stageType":%d,"vertical":%d,"scroll":%d,"solidCollision":%d,"screenCleared":%d,"scrollX":%d,"scrollY":%d,"ppuControl":%d,"collision":"%s","before":"%s","after":"%s"}\n',p.frame,p.stageType,p.vertical,p.scroll,p.solid,p.cleared,p.scrollX,p.scrollY,p.ppu,p.collision,p.before,hex(0x368,336)))
  end
 end)
end
memory.registerexec(0xf9bc,1,function()if fire then fire.sounds[#fire.sounds+1]=tostring(memory.getregister('a'))end end)
CASES
memory.registerexec(0x8001,1,function()
 if read(0x8000)~=0x1c then return end
 assert(not fire);local i=memory.getregister('x');assert(i<2)
 local p={pc=returnpc(),frame=emu.framecount(),actor=i,sounds={},case=0}
 if cases then
  local c=cases[math.floor(caseIndex/2)+1];caseIndex=caseIndex+1
  if not c then return end
  p.case=math.floor((caseIndex-1)/2)+1;p.restore={};p.pool=hex(0x368,336)
  local values={[0x334+i]=c.x,[0x31a+i]=c.y,[0xba+i]=c.hidden,[0xc2+i]=c.aim,[0xaa+i]=c.weapon,[0xa0+i]=c.jump,[0xb2+i]=c.water,[0xc8+i]=c.electrocution,[0x40]=c.stageType,[0xac+i]=c.timer,[0xce+i]=c.recoil,[0xf1+i]=c.held,[0xf5+i]=c.pressed}
  for a,v in pairs(values)do p.restore[a]=read(a);memory.writebyte(a,v)end
  for k=0,335 do memory.writebyte(0x368+k,0)end
  if c.fill>0 then for k=0,15 do memory.writebyte(0x388+k,1);memory.writebyte(0x368+k,30)end end
  if c.empty>=0 then memory.writebyte(0x388+c.empty,0)end
 end
 p.request=request(i);p.before=hex(0x368,336);fire=p;arm(p.pc)
end)
memory.registerexec(0x8584,1,function()
 if read(0x8000)~=0x1c or cases then return end
 assert(not step);local p={pc=returnpc(),frame=emu.framecount(),stageType=read(0x40),vertical=read(0x41),scroll=read(0x68),solid=read(0x59),cleared=read(0x6a),scrollX=read(0xfd),scrollY=read(0xfc),ppu=read(0xff),collision=hex(0x680,128),before=hex(0x368,336)};step=p;arm(p.pc)
end)
function finish_probe()assert(not fire and not step);f:close()end
'''

def cases():
    defaults=dict(x=100,y=100,hidden=0,aim=2,weapon=0,jump=0,water=0,electrocution=0,stageType=0,timer=7,recoil=0,held=64,pressed=64,fill=0,empty=-1)
    out=[]
    for weapon in range(5):
        for rapid in [0,16]:
            for stage in [0,1,128]:
                for jump in [0,17]:
                    for aim in range(10):out.append(dict(defaults,weapon=weapon|rapid,stageType=stage,jump=jump,aim=aim))
    for weapon in range(5):
        for timer in range(256):
            for held,pressed in [(0,0),(64,0)]:out.append(dict(defaults,weapon=weapon,timer=timer,held=held,pressed=pressed))
        for variant in [dict(hidden=1),dict(electrocution=8),dict(water=1,aim=3),dict(water=1,aim=6),dict(fill=1),dict(fill=1,empty=0),dict(fill=1,empty=3),dict(fill=1,empty=6),dict(fill=1,empty=9),dict(fill=1,empty=10),dict(fill=1,empty=13),dict(fill=1,empty=15)]:out.append(dict(defaults,weapon=weapon,**variant))
    return out

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['natural','isolated'],default='natural');parser.add_argument('--name',required=True);parser.add_argument('--output',type=Path,default=ROOT/'docs/research/nes-contra/evidence');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(source,target)
    assert target.read_bytes()==source.read_bytes()
    segments=[dict(start=n,end=n,player=p,keys=['B']) for p in [1,2] for n in range(1,241,4)]
    segments += [dict(start=10,end=45,player=1,keys=['up']),dict(start=70,end=100,player=2,keys=['left']),dict(start=125,end=140,player=1,keys=['down']),dict(start=150,end=150,player=1,keys=['A']),dict(start=150,end=190,player=1,keys=['down']),dict(start=185,end=220,player=2,keys=['up','left'])]
    count=240;lua='local cases=nil';fixtureCases=[]
    if args.mode=='isolated':
        fixtureCases=cases();count=len(fixtureCases)+2;segments=[]
        lua='local cases={'+','.join('{'+','.join(f'{k}={v}' for k,v in c.items())+'}' for c in fixtureCases)+'}'
    spec={'name':args.name,'load_state':'coop-play','frames':count,'segments':segments,'captures':[1,120,240] if args.mode=='natural' else [],'hooks_lua':COMMON.replace('CASES',lua),'timeout_seconds':90}
    p=BASE/(args.name+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
    records=[json.loads(line) for line in (out/'weapons.jsonl').read_text().splitlines()]
    fire=[r for r in records if r['kind']=='fire'];steps=[r for r in records if r['kind']=='step']
    if args.mode=='isolated':assert len(fire)==len(fixtureCases)*2,(len(fire),len(fixtureCases)*2)
    result={'source':json.loads((out/'source.json').read_text()),'scope':args.mode,'interventions':None if args.mode=='natural' else 'At the weapon-function boundary only, exact request and occupied slots are staged; fields/pool are restored after recording the real original call. Not a natural game traversal.', 'scenario':spec,'fire':fire,'steps':steps}
    args.output=args.output.resolve()
    assert args.output.is_relative_to(ROOT)
    args.output.mkdir(parents=True,exist_ok=True)
    target=args.output/f'weapons-{args.mode}.json.gz'
    temporary=target.with_suffix('.tmp');temporary.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0));temporary.replace(target)
    print(json.dumps({'fixture':str(target.relative_to(ROOT)),'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'fire':len(fire),'steps':len(steps)}))

if __name__=='__main__':main()
