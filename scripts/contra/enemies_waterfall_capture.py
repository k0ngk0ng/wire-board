"""Record Japanese Waterfall dispatches before implementing semantic rules.

Failure inventory: vertical scroll wraps/removes at the wrong time; fractional
horizontal reversal; rock floor/carry and saturated bounce floor; self-slot reuse
after removal; full pool failure; scuba animation/shoot/timeout ordering; Dragon
links, graphics admission and live arm count; false natural-completion claims.
Controlled inputs are restored after the native E54F phase. Outputs come only
from original E58A execution, with full 16-slot, map, graphics and sound state.
"""
import argparse, ast, gzip, hashlib, json, os, shutil
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / '.local/contra-enemies-waterfall/oracle'
os.environ['CONTRA_REFERENCE_ROOT'] = str(BASE)
from capture_reference import run

def literal(path, name):
    for node in ast.parse(path.read_text()).body:
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == name for t in node.targets):
            return ast.literal_eval(node.value)
    raise ValueError(name)

FIELDS = literal(ROOT/'scripts/contra/enemies_capture.py', 'FIELDS')
WORLD = literal(ROOT/'scripts/contra/enemies_capture.py', 'WORLD') + [
    ('paletteTimer', 0x74), ('destroyedArms', 0x85), ('delayLow', 0x2a),
    ('delayHigh', 0x2b), ('weaponStrength', 0x2f), ('gameRoutine', 0x18),
    ('generationBias', 0x06)]

HOOK = r'''
local f=assert(io.open(out..'/waterfall.jsonl','w'))
local ef=ENEMIES;local wf=WORLD;local cases=CASES
local step=nil;local phase=nil;local ordinal=0;local returns={};local hooks={}
local function read(a)return memory.readbyte(a)end
local function hx(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function arr(fields,i)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+i))end;return '['..table.concat(t,',')..']'end
local function state()
 local enemies={};for i=0,15 do enemies[#enemies+1]=arr(ef,i)end
 local players={};for i=0,1 do players[#players+1]=string.format('{"state":%d,"x":%d,"y":%d,"gameOver":%d}',read(0x90+i),read(0x334+i),read(0x31a+i),read(0x38+i))end
 return '{"enemies":['..table.concat(enemies,',')..'],"world":'..arr(wf,0)..',"players":['..table.concat(players,',')..'],"collision":"'..hx(0x680,128)..'","graphics":{"used":'..read(0x21)..',"mode":'..read(0x23)..',"bytes":"'..hx(0x700,128)..'","supertiles":"'..hx(0x600,128)..'"}}'
end
local function returnpoint()
 local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)
end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  local sp=memory.getregister('s')
  if step and step.pc==pc and step.sp==sp then
   local p=step;step=nil
   f:write(string.format('{"id":%d,"frame":%d,"actor":%d,"before":%s,"after":%s,"sounds":[%s],"requests":[%s],"audioResets":%d,"biasWrites":[%s]}\n',p.id,p.frame,p.actor,p.before,state(),table.concat(p.sounds,','),table.concat(p.requests,','),p.resets,table.concat(p.bias,',')))
  end
  if phase and phase.pc==pc and phase.sp==sp then
   local p=phase;phase=nil;for a,v in pairs(p.saved)do memory.writebyte(a,v)end
  end
  if hooks[pc]then hooks[pc]()end
 end)
end
hooks[0xe54f]=function()
 if not cases then return end
 assert(not phase,'nested phase');ordinal=ordinal+1
 local c=assert(cases[ordinal],'too many phases');local pc,sp=returnpoint();local saved={}
 for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=read(a)end end
 for a,v in pairs(c)do memory.writebyte(a,v)end
 phase={pc=pc,sp=sp,saved=saved};arm(pc)
end
hooks[0xe58a]=function()
 local slot=memory.getregister('x');local kind=read(0x528+slot)
 if read(0x8000)~=0x10 or not(kind==12 or kind>=16 and kind<=21)or cases and slot~=15 then return end
 assert(not step,'nested enemy step');local pc,sp=returnpoint()
 if not cases then ordinal=ordinal+1 end
 step={id=ordinal,frame=emu.framecount(),actor=slot,pc=pc,sp=sp,before=state(),sounds={},requests={},resets=0,bias={}};arm(pc)
end
arm(0xe54f);arm(0xe58a)
memory.registerexec(0xf9bc,1,function()if step then step.sounds[#step.sounds+1]=tostring(memory.getregister('a'))end end)
memory.registerexec(0xf9d4,1,function()if step then step.resets=step.resets+1;step.sounds={}end end)
memory.registerexec(0xc3d1,1,function()if step then step.requests[#step.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)
memory.registerwrite(6,1,function(a,n,v)if step then step.bias[#step.bias+1]=string.format('{"pc":%d,"value":%d}',memory.getregister('pc'),v)end end)
function finish_probe()assert(not phase and not step,'unfinished native phase');f:close()end
'''

def build_cases():
    cases=[]
    props={12:[0x81,0x01,0x01,0],16:[0xc0,4,0xf0,0],17:[0x80,2,0xf0,0],18:[0x81,0,0xf0,0],19:[0x8f,0x31,5,0]}
    def add(kind,routine,**kw):
        x=dict(kind=kind,routine=routine,x=160,y=160,sprite=1,animationDelay=1,frame=0,
               stateWidth=props[kind][0],scoreCollision=props[kind][1],hp=props[kind][2],variableA=props[kind][3],
               variable1=100,variable2=80,level=2,scrollType=1,frameScroll=0,attackDelay=1)
        x.update(kw);cases.append(x)
    for kind in (16,17):
        for attribute in range(4):add(kind,1,attributes=attribute,frameScroll=1)
        for frame in (0,8,15,16):
            for vx in (-384,128):
                for x in (79,80,99,100,255):add(kind,2,frameValue=frame,vx=vx,x=x,xFraction=128)
        add(kind,1,y=231,frameScroll=2)
        add(kind,2,y=231,frameScroll=2,vx=-384,x=77)
    for r in (1,2,3):
        for delay in (0,1,2):
            for full in (False,True):add(18,r,animationDelay=delay,full=full)
    add(18,3,y=231,frameScroll=2,full=True)
    for r in range(1,7):
        for delay in (0,1,2):add(19,r,animationDelay=delay,frame=1)
    for frame in range(8):add(19,2,frameValue=frame,animationDelay=1)
    for y in (92,100,230,250):
        for terrain in (0,0x55,0xaa,0xff):
            for delay in (1,2,8):add(19,3,y=y,terrain=terrain,animationDelay=delay,frameScroll=2,vy=17,yFraction=250)
    for r in range(1,7):
        for delay in (0,1,2,16):add(12,r,animationDelay=delay,frame=1)
    for y in (183,184,231):
        for scroll in (0,2):add(12,2,y=y,frameScroll=scroll,animationDelay=1)
    for anim in (0,1,2):
        for attack in (0,1,2):
            for full in (False,True):add(12,3,animationDelay=anim,attackDelay=attack,variable1=0,full=full)
    add(12,3,level=4,scrollType=0,frameScroll=1,variable1=2)
    return materialize(cases)

def build_boss_cases():
    cases=[]
    def add(kind,routine,**kw):
        x=dict(kind=kind,routine=routine,x=128,y=64,sprite=1,animationDelay=1,frame=1,
               stateWidth=0x8d if kind==20 else 0x0e,scoreCollision=0x83 if kind==20 else 0x52,hp=0xf1,variableA=2 if kind==20 else 2,
               variable1=32,variable2=0,variable3=1,variable4=255,level=2,scrollType=1,frameScroll=0,attackDelay=1,
               bossScrollComplete=1,paletteTimer=0,destroyedArms=0,soldierAllowed=1)
        x.update(kw);cases.append(x)
    for routine in range(1,10):
        for delay in (0,1,2):
            for used in (0,63,64):add(20,routine,animationDelay=delay,queueUsed=used)
    for frame in (0,1,2):
        for routine in (3,5):
            for used in (0,40,64):add(20,routine,frame=frame,queueUsed=used)
    for arms in (0,1,2,255):
        for attack in (0,1,2):
            for allowed in (0,1):add(20,4,destroyedArms=arms,attackDelay=attack,soldierAllowed=allowed)
    for index in range(14):
        for used in (0,64):add(20,9,variable2=index,variable3=1,queueUsed=used,full=index%2==0)
    for flag in (0,1):
        for timer in (0,1):add(20,2,bossScrollComplete=flag,paletteTimer=timer)
    for x,y in ((5,32),(128,5),(128,244),(128,231)):
        for routine in (2,3,4,5):add(20,routine,x=x,y=y,frameScroll=2)
    for side in (0,1):
        add(21,1,attributes=side)
        for count in range(4):
            add(21,2,attributes=side,chain=list(range(15,14-count,-1)),frame=4-count)
        add(21,2,attributes=side,chain=[15],frame=4,full=True)
        for flag in (0,1):
            for timer in (0,1):add(21,2,attributes=side,chain=[15],frame=4,bossScrollComplete=flag,paletteTimer=timer)
        for order in ([15,14,13,12,11],[11,15,12,13,14],[11,12,15,13,14],[11,12,13,14,15]):
            for delay in (0,1,2):
                for value in ((0,)if order[0]==15 else(0,1,14,15,255)):
                    for frame in (0,1):add(21,3,attributes=side,chain=order,animationDelay=delay,variable2=value,frameValue=frame)
        for pattern in range(5):
            for angle in (0,8,12,20,44,52,56,63):
                for delay in (0,1,3):add(21,4,attributes=side,chain=[15,14,13,12,11],frame=pattern,variable1=angle,animationDelay=delay,attackDelay=3,variableA=2)
        for pattern in range(5):
            for allowed in (0,1):add(21,4,attributes=side,chain=[15,14,13,12,11],frame=pattern,variableA=1,attackDelay=1,soldierAllowed=allowed)
        for routine in (4,5,6,7,8):
            for order in ([15,14,13,12,11],[11,12,15,13,14],[11,12,13,14,15]):add(21,routine,attributes=side,chain=order)
    return materialize(cases)

def materialize(cases):
    out=[]
    for case in cases:
        writes={str(a+i):0 for _,a in FIELDS for i in range(16)}
        if case.get('full'):
            for i in range(15):
                for a,v in [(0x4b8,1),(0x528,1),(0x30a,1),(0x33e,160),(0x324,160)]:writes[str(a+i)]=v
        for name,a in FIELDS:writes[str(a+15)]=case.get(name,0)
        if 'chain'in case:
            order=case['chain']
            for index,slot in enumerate(order):
                node=dict(sprite=0x7b if index==4 else 0x7a,x=128+(index*8),y=64+(index*8),kind=21,routine=case['routine'],
                          stateWidth=0x0e if index==0 else 0x0c if index==4 else 0x8c,scoreCollision=0x52,hp=0x10 if index==4 else 0xf1,
                          attributes=case.get('attributes',0),variable1=0x38 if index==0 else 0,variable2=0,variable3=order[index+1]if index+1<len(order)else 255,
                          variable4=order[index-1]if index else 255,variableA=2,animationDelay=0,frame=0,attackDelay=1)
                if case['routine']==2:
                    node['variable3']=order[index+1]if index+1<len(order)else 0
                    if index==0:node['variable2']=order[-1]
                if slot==15:
                    for name,_ in FIELDS:
                        if name in case and name not in ['variable3','variable4']:node[name]=case[name]
                    if case['routine']==2:node['variable2']=order[-1]
                for name,a in FIELDS:writes[str(a+slot)]=node.get(name,0)
        for axis in ('X','Y'):
            velocity=case.get('v'+axis.lower(),0)&65535
            for name,a in FIELDS:
                if name=='velocity'+axis:writes[str(a+15)]=velocity>>8
                if name=='velocity'+axis+'Fraction':writes[str(a+15)]=velocity&255
        if 'chain'in case:
            writes[str(0x518+case['chain'][0])]=case['chain'][-1]
        for name,a in WORLD:
            if name in case:writes[str(a)]=case[name]
        writes['26']=case.get('frameValue',12)
        writes[str(0x90)]=writes[str(0x91)]=1
        writes[str(0x38)]=writes[str(0x39)]=0
        writes[str(0x84)]=case.get('bossScrollComplete',1);writes[str(0x74)]=case.get('paletteTimer',0);writes[str(0x85)]=case.get('destroyedArms',0)
        writes[str(0x21)]=case.get('queueUsed',0);writes[str(0x23)]=0
        if case['kind']>=20:writes[str(0x2a)]=7;writes[str(0x2b)]=3
        if 'terrain'in case:
            for a in range(0x680,0x700):writes[str(a)]=case['terrain']
        out.append({'setup':case,'writes':writes})
    return out

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);mode=parser.add_mutually_exclusive_group();mode.add_argument('--natural',action='store_true');mode.add_argument('--boss',action='store_true');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra-campaign/states/stage-3-v2.fcs';target=BASE/'states/stage-3.fcs'
    if target.exists():assert target.read_bytes()==source.read_bytes()
    else:shutil.copyfile(source,target)
    cases=None if args.natural else build_boss_cases()if args.boss else build_cases()
    hook=HOOK.replace('ENEMIES','{'+','.join(hex(a)for _,a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}')
    hook=hook.replace('CASES','nil'if cases is None else'{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in cases)+'}')
    frames=1800 if args.natural else len(cases)
    segments=[]
    if args.natural:
        # A controller policy may read visible character/map observations but
        # never writes native state. Record its exact resulting input timeline.
        hook+='''
local targets={168,168}
local function platform(x,y)
 local sy=y+memory.readbyte(0xfc);local row=sy%256;if sy>=240 then row=(row+16)%256 end
 local sx=x+memory.readbyte(0xfd);local page=AND(memory.readbyte(0xff),1);if sx>=256 then page=1-page end
 local column=math.floor((sx%256)/16);local index=OR(math.floor(row/16)*4,math.floor(column/4),page*64)
 return AND(math.floor(memory.readbyte(0x680+index)/2^(6-2*(column%4))),3)==1
end
function control_frame(n,pads)
 for p=1,2 do
  local i=p-1;local x=memory.readbyte(0x334+i);local y=memory.readbyte(0x31a+i)
  pads[p].B=true
  if memory.readbyte(0xa0+i)==0 and memory.readbyte(0xa4+i)==0 then
   local found=false
   for dy=16,64,16 do
    local best=nil;local distance=999
    for tx=8,248,16 do
     if y+12-dy>=0 and platform(tx,y+12-dy) and math.abs(x-tx)<distance then best=tx;distance=math.abs(x-tx)end
    end
    if best and distance<=70 then targets[p]=best;found=true;break end
   end
   if found and math.abs(x-targets[p])<60 and n%3~=0 then pads[p].A=true end
  end
  if x<targets[p]-2 then pads[p].right=true elseif x>targets[p]+2 then pads[p].left=true end
 end
end
'''
    spec={'name':args.name,'load_state':'stage-3','frames':frames,'captures':[1,frames],'segments':segments,'hooks_lua':hook,'timeout_seconds':180}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
    records=[json.loads(line)for line in(observed/'waterfall.jsonl').read_text().splitlines()]
    if cases:assert len(records)==len(cases),(len(records),len(cases))
    assert records,'controller route did not reach any owned enemies'
    fixture={'source_is_original_execution':True,'interventions':'Controlled stage entry at CC5F from retained stage-3-v2. '+('Controller-only traversal after that entry.'if args.natural else'Explicit per-phase setup, restored at original E54F return. No after state injected.'),
             'source':json.loads((observed/'source.json').read_text()),'cases':cases,'scenario':spec,'fields':FIELDS,'world_fields':WORLD,'records':records}
    suffix='natural'if args.natural else'boss'if args.boss else'hazards'
    dest=ROOT/f'games/contra/engine/testdata/original-enemies-waterfall-{suffix}.json.gz'
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==records,'original replay differs'
    else:dest.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    result={'records':len(records),'fixtureSHA256':hashlib.sha256(dest.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()}
    (observed/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
