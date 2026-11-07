"""Capture original contact/hit effects before implementing collision rules.

Failure cases: pose/variable hitboxes, byte-wrap edges and the original borrow
in SBC tests; P2-first order/early return; underwater Down and both invincibility
types; platforms changing position/inertia; HP sentinel values; laser chain
selection; pickup/R preservation; score overflow, extra-life thresholds and
duplicate death credit. Natural and controlled sources remain distinct.
"""
import argparse
import ast
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-collision/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal_assignment(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

PLAYER=literal_assignment(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
ENEMY=literal_assignment(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
WORLD=[('demo',0x1c),('level',0x30),('sceneType',0x40),('flash',0x7d)]

HOOK=r'''
local f=assert(io.open(out..'/collision.jsonl','w'))
local playerFields=PLAYERS;local enemyFields=ENEMIES;local worldFields=WORLD
local active=nil;local exits={};local hooks={};local sequence=0
local function read(a)return memory.readbyte(a)end
local function array(fields,i)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+i))end;return '['..table.concat(t,',')..']'end
local function hex(first,count)local t={};for a=first,first+count-1 do t[#t+1]=string.format('%02x',read(a))end;return table.concat(t)end
local function state()
 local players={array(playerFields,0),array(playerFields,1)}
 local enemies={};for i=0,15 do enemies[#enemies+1]=array(enemyFields,i)end
 return '{"players":['..table.concat(players,',')..'],"enemies":['..table.concat(enemies,',')..'],"weapons":"'..hex(0x368,336)..'","score":"'..hex(0x7e0,6)..hex(0x3c,4)..'","world":'..array(worldFields,0)..'}'
end
local function saved_state()
 local saved={}
 for _,a in ipairs(playerFields)do for i=0,1 do saved[a+i]=read(a+i)end end
 for _,a in ipairs(enemyFields)do for i=0,15 do saved[a+i]=read(a+i)end end
 for a=0x368,0x4b7 do saved[a]=read(a)end
 for a=0x7e0,0x7e5 do saved[a]=read(a)end
 for a=0x3c,0x3f do saved[a]=read(a)end
 for _,a in ipairs(worldFields)do saved[a]=read(a)end
 return saved
end
local function regs()return string.format('[%d,%d,%d,%d]',memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.getregister('p'))end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)end
local function arm(pc)
 if exits[pc]then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if active and active.pc==pc and active.sp==memory.getregister('s')then
   local p=active;active=nil
   f:write(string.format('{"id":%d,"kind":"%s","frame":%d,"slot":%d,"case":%d,"input":%s,"output":%s,"before":%s,"after":%s,"sounds":[%s]}\n',p.id,p.kind,p.frame,p.slot,p.case,p.input,regs(),p.before,state(),table.concat(p.sounds,',')))
   if p.saved then for a,v in pairs(p.saved)do memory.writebyte(a,v)end end
  end
  if hooks[pc]then hooks[pc]()end
 end)
end
local function hook(pc,name)
 hooks[pc]=function()
  assert(not active,'nested contact routines')
  local target,stack=ret();sequence=sequence+1
  local p={id=sequence,kind=name,pc=target,sp=stack,slot=read(0x83),frame=emu.framecount(),case=0,sounds={}}
  if configure_case and not configure_case(p)then return end
  p.input=regs();p.before=state();active=p;arm(target)
 end
 arm(pc)
end
memory.registerexec(0xf9bc,1,function()if active then active.sounds[#active.sounds+1]=tostring(memory.getregister('a'))end end)
CASE_SETUP
hook(0xe10a,'players');hook(0xe276,'bullets')
function finish_probe()assert(not active,'unfinished collision call');f:close()end
'''

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);parser.add_argument('--cases',type=Path);parser.add_argument('--route',choices=['machine','hurt'],default='machine');parser.add_argument('--scope',choices=['controlled','controlled-special'],default='controlled');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    src=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(src,target)
    assert target.read_bytes()==src.read_bytes()
    hook=HOOK.replace('PLAYERS','{'+','.join(hex(a)for _,a in PLAYER)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}')
    setup='';cases=None
    frames=700
    segments=([{'start':1,'end':500,'player':p,'keys':['right']}for p in (1,2)]+
             [{'start':n,'end':n,'player':p,'keys':['A']}for p in (1,2)for n in (15,85,155)]+
             [{'start':220,'end':270,'player':p,'keys':['down','right']}for p in (1,2)]+
             [{'start':n,'end':n,'player':p,'keys':['B']}for p in (1,2)for n in range(1,351,4)]+
             [{'start':351,'end':700,'player':p,'keys':['B']}for p in (1,2)])
    if args.route=='hurt' and not args.cases:
        frames=1200
        segments=([{'start':1,'end':1200,'player':p,'keys':['right','B']}for p in (1,2)]+
                  [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in (1,2)for n in range(60,1200,90)])
    if args.cases:
        cases=json.loads(args.cases.read_text());frames=len(cases);segments=[]
        staged=','.join('{kind="'+case['kind']+'",writes={'+','.join(f'[{a}]={v}'for a,v in case['writes'].items())+'}}'for case in cases)
        setup=r'''
local cases={STAGED};local caseIndex=0
memory.registerexec(0xe54f,1,function()
 caseIndex=caseIndex+1
 for i=0,15 do for _,a in ipairs(enemyFields)do memory.writebyte(a+i,0)end end
 memory.writebyte(0x4b8+15,1);memory.writebyte(0x528+15,1)
 memory.writebyte(0x30a+15,1);memory.writebyte(0x33e+15,240);memory.writebyte(0x324+15,8)
end)
function configure_case(p)
 local c=cases[caseIndex]
 if not c or p.kind~=c.kind then return false end
 p.case=caseIndex;p.saved=saved_state()
 for a,v in pairs(c.writes)do memory.writebyte(a,v)end
 return true
end
'''.replace('STAGED',staged)
    spec={'name':args.name,'load_state':'coop-play','frames':frames,'segments':segments,'captures':[1,292,600]if not cases else [],'hooks_lua':hook.replace('CASE_SETUP',setup),'timeout_seconds':120}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'collision.jsonl').read_text().splitlines()]
    if cases:assert len(records)==len(cases),(len(records),len(cases))
    suffix=args.scope if cases else ('natural-hurt'if args.route=='hurt'else'natural')
    fixture={'source_is_original_execution':True,'scope':suffix,'source':json.loads((out/'source.json').read_text()),
      'interventions':'Each named input is staged at the original collision function boundary and restored immediately afterward; not natural gameplay.'if cases else 'none; ordinary controller input only',
      'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],
      'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],'cases':cases,'scenario':spec,'records':records}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    path=ROOT/f'games/contra/engine/testdata/original-enemy-collision-{suffix}.json.gz'
    if path.exists():
        previous=json.loads(gzip.decompress(path.read_bytes()));assert previous['records']==records,'new capture differs from retained oracle'
    else:path.write_bytes(packed)
    counts={kind:sum(r['kind']==kind for r in records)for kind in ('players','bullets')}
    result={'passed':True,'scope':suffix,'counts':counts,'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
