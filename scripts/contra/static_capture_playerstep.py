#!/usr/bin/env python3
"""Observe complete outdoor normal-player calls and their actual subcall order.

Failure cases fixed before capture: first-frame jump displacement, stale Y
fraction, ground/air direction conflict confusion, missed ledge/water phases,
sprite-stage physics omitted, camera changing a peer, wrong nested return,
and source-state or RAM mutation. This script only observes original execution.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
BASE=Path(os.environ.get('CONTRA_REFERENCE_ROOT',ROOT/'.local/contra-static/player-oracle')).resolve()
if not BASE.is_relative_to(ROOT):raise ValueError('player oracle must remain inside workspace')
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

PLAYER_FIELDS=[
 ('state',0x90),('x',0x334),('xFraction',0x92),('y',0x31a),('yFraction',0x94),('yHigh',0xba),
 ('vx',0x98),('vxFraction',0x96),('vyLo',0xc4),('vyHi',0xc6),('indoorVyLo',0x9a),('indoorVyHi',0x9c),
 ('jump',0xa0),('scrolling',0xa2),('fall',0xa4),('animFrame',0xa6),('animTimer',0x9e),
 ('weapon',0xaa),('machineGunTimer',0xac),('spawnInvincible',0xae),('invincible',0xb0),
 ('water',0xb2),('death',0xb4),('onEnemy',0xb6),('fallLockY',0xb8),('sequence',0xbc),
 ('previousAim',0xc0),('aim',0xc2),('shock',0xc8),('forceJump',0xca),('waterTimer',0xcc),
 ('recoil',0xce),('indoorAdvance',0xd0),('specialSpriteTimer',0xd2),('boostX',0xd4),
 ('sprite',0xd6),('spriteFlip',0xd8),('skipLedge',0xda),('held',0xf1),('pressed',0xf5),
 ('lives',0x32),('gameOver',0x38),('displaySprite',0x300),('displayAttribute',0x34e)]
WORLD_FIELDS=[('gameRoutine',0x18),('frameCounter',0x1a),('playerMode',0x22),('levelRoutine',0x2c),
 ('level',0x30),('bossDefeated',0x3b),('sceneType',0x40),('scrollType',0x41),('stopScroll',0x58),
 ('screen',0x64),('screenScroll',0x65),('frameScroll',0x68),('autoScrollA',0x75),('autoScrollB',0x76),
 ('bossScrollComplete',0x84),('activePlayers',0xdf),('scrollY',0xfc),('scrollX',0xfd),('ppuControl',0xff)]
ENTRIES={
 0xd495:'normalMotion',0xd848:'aim',0xd8b0:'ledge',0xfa12:'fire',0xd518:'stateAndX',
 0xd525:'applyX',0xd595:'action',0xd5fd:'beginJump',0xd626:'groundDpad',0xd690:'setXWaterGate',
 0xd6dd:'jump',0xd6ad:'fall',0xd735:'airDirection',0xd752:'airDpad',0xd825:'landingSnap',
 0xd79d:'land',0xd941:'camera',0xd8d9:'feet',0xda5e:'side',0xfa4a:'sprite',0xd4a1:'autoScroll'}

HOOK=r'''
local f=assert(io.open(out.."/player-steps.jsonl","w"))
local maps=assert(io.open(out.."/player-maps.jsonl","w"))
local fields=PLAYER_FIELDS
local worldFields=WORLD_FIELDS
local pending={};local exits={};local entryHooks={};local active=nil;local serial=0;local knownMaps={};local mapCount=0
local function read(a) return memory.readbyte(a) end
local function array_fields(fields,actor)
 local out={};for _,a in ipairs(fields) do out[#out+1]=tostring(read(a+actor)) end
 return '['..table.concat(out,',')..']'
end
local function map_id()
 local t={};for i=0x680,0x6ff do t[#t+1]=string.format('%02x',read(i)) end
 local data=table.concat(t);local id=knownMaps[data]
 if not id then mapCount=mapCount+1;id=mapCount;knownMaps[data]=id;maps:write(string.format('{"id":%d,"bytes":"%s"}\n',id,data)) end
 return id
end
local function state(actor)
 return string.format('{"player":%s,"peer":%s,"world":%s,"map":%d}',array_fields(fields,actor),array_fields(fields,1-actor),array_fields(worldFields,0),map_id())
end
local function registers()
 return string.format('[%d,%d,%d,%d]',memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.getregister('p'))
end
local function pool()
 local t={};for i=0x368,0x4b7 do t[#t+1]=string.format('%02x',read(i)) end;return table.concat(t)
end
local function ret()
 local sp=memory.getregister('s')
 return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)
end
local function arm(pc)
 if exits[pc] then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if active then
   local sp=memory.getregister('s')
   for i=#pending,1,-1 do
    local p=pending[i]
    if p.pc==pc and p.sp==sp then
     p.after=state(active.actor);p.output=registers();p.exitOrder=active.order;active.order=active.order+1
     table.remove(pending,i)
    end
   end
   if active.pc==pc and active.sp==sp then
    assert(#pending==0,'unfinished descendant at normal-player return')
    local parts={}
    for _,c in ipairs(active.calls) do
     assert(c.after,'missing descendant return')
     parts[#parts+1]=string.format('{"name":"%s","address":%d,"entryOrder":%d,"exitOrder":%d,"input":%s,"output":%s,"before":%s,"after":%s}',c.name,c.address,c.entryOrder,c.exitOrder,c.input,c.output,c.before,c.after)
    end
    f:write(string.format('{"id":%d,"frame":%d,"actor":%d,"aimContext":%d,"before":%s,"after":%s,"beforePool":"%s","afterPool":"%s","calls":[%s]}\n',active.id,active.frame,active.actor,active.aimContext,active.before,state(active.actor),active.beforePool,pool(),table.concat(parts,',')))
    active=nil
   end
  end
  if entryHooks[pc] then entryHooks[pc]() end
 end)
end
entryHooks[0xd45e]=function()
 assert(not active,'recursive normal player')
 local actor=memory.getregister('x');assert(actor==0 or actor==1)
 local pc,sp=ret();serial=serial+1
 active={id=serial,frame=emu.framecount(),actor=actor,pc=pc,sp=sp,before=state(actor),beforePool=pool(),aimContext=read(8),calls={},order=0}
 arm(pc)
end
arm(0xd45e)
local function hook(address,name)
 entryHooks[address]=function()
  if not active then return end
  local pc,sp=ret()
  local p={pc=pc,sp=sp,address=address,name=name,input=registers(),before=state(active.actor),entryOrder=active.order}
  active.order=active.order+1;active.calls[#active.calls+1]=p;pending[#pending+1]=p
  arm(pc)
 end
 arm(address)
end
ENTRY_HOOKS
function finish_probe()
 assert(not active and #pending==0,'unfinished player call')
 f:close();maps:close()
end
'''


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    for mode in ('solo','coop'):
        source=ROOT/f'.local/contra/states/{mode}-play.fcs';target=BASE/f'states/{mode}-play.fcs'
        target.parent.mkdir(parents=True,exist_ok=True)
        if target.exists():assert target.read_bytes()==source.read_bytes()
        else:shutil.copyfile(source,target)
    hook=HOOK.replace('PLAYER_FIELDS','{'+','.join(hex(a) for _,a in PLAYER_FIELDS)+'}')
    hook=hook.replace('WORLD_FIELDS','{'+','.join(hex(a) for _,a in WORLD_FIELDS)+'}')
    hook=hook.replace('ENTRY_HOOKS','\n'.join(f'hook({hex(a)},"{n}")' for a,n in ENTRIES.items()))
    scenarios=[
      ('idle','solo',80,[]),
      ('ground-conflict','solo',120,[(1,30,1,['left','right']),(40,60,1,['left']),(70,90,1,['up','down']),(100,110,1,['right','down'])]),
      ('air-conflict','solo',160,[(1,1,1,['A','left','right']),(2,30,1,['left','right']),(90,90,1,['A']),(91,115,1,['right'])]),
      ('drop-water','solo',240,[(1,4,1,['down','A']),(60,90,1,['right','down']),(100,120,1,['right','A']),(130,200,1,['right'])]),
      ('water-escape','solo',700,[(1,4,1,['down','A']),(80,700,1,['right'])]),
      ('jump-return','solo',180,[(1,1,1,['A']),(20,40,1,['right']),(80,80,1,['A']),(81,110,1,['left']),(140,155,1,['right'])]),
      ('camera-peer','coop',420,[(1,420,1,['right','B']),(1,40,2,['left']),(240,420,2,['right','B'])]),
      ('walk-jump','coop',600,[(1,600,1,['right','B']),(1,600,2,['right','B'])]+[(n,n+1,p,['A']) for p in [1,2] for n in range(60,600,90)]),
    ]
    outputs=[];counts={};coverage={'water':set(),'fall':set(),'jump':set()}
    pidx={name:i for i,(name,_) in enumerate(PLAYER_FIELDS)}
    for name,mode,frames,segments in scenarios:
        spec={'name':f'playerstep-{args.label}-{name}','load_state':mode+'-play','frames':frames,
              'segments':[{'start':a,'end':b,'player':p,'keys':k} for a,b,p,k in segments],
              'captures':[1,frames],'hooks_lua':hook,'timeout_seconds':90}
        spec_path=BASE/(spec['name']+'.json');spec_path.write_text(json.dumps(spec,indent=2)+'\n')
        out=run(spec_path)
        steps=[json.loads(s) for s in (out/'player-steps.jsonl').read_text().splitlines()]
        maps=[json.loads(s) for s in (out/'player-maps.jsonl').read_text().splitlines()]
        assert steps
        for step in steps:
            for field in coverage:coverage[field].add(step['before']['player'][pidx[field]])
            for call in step['calls']:counts[call['name']]=counts.get(call['name'],0)+1
        outputs.append({'name':name,'source':json.loads((out/'source.json').read_text()),
                        'scenario':spec,'trace_sha256':hashlib.sha256((out/'player-steps.jsonl').read_bytes()).hexdigest(),
                        'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),'maps':maps,'steps':steps})
    fixture={'schema':1,'source_is_original_execution':True,'rom_sha256':outputs[0]['source']['rom_sha256'],
             'interventions':'none; controller inputs and observation hooks only',
             'player_fields':[{'name':name,'address':hex(address)} for name,address in PLAYER_FIELDS],
             'world_fields':[{'name':name,'address':hex(address)} for name,address in WORLD_FIELDS],
             'register_order':['a','x','y','p'],'normal_entry':'0xd45e','routines':{hex(a):n for a,n in ENTRIES.items()},
             'call_counts':counts,'observed_states':{k:sorted(v) for k,v in coverage.items()},'scenarios':outputs,
             'limitations':['Outdoor first-stage natural play only; no indoor or vertical-stage coverage.',
                            'Complete normal-player calls include bullet and sprite-stage side effects; helper isolation must preserve phase order.']}
    raw=json.dumps(fixture,separators=(',',':')).encode();compressed=gzip.compress(raw,mtime=0)
    path=ROOT/'docs/research/nes-contra/evidence/static-playerstep-complete-oracle.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()))
        for before,after in zip(old['scenarios'],fixture['scenarios'],strict=True):
            assert before['steps']==after['steps'] and before['maps']==after['maps'],f"oracle changed: {after['name']}"
    else:path.write_bytes(compressed)
    summary={'passed':True,'label':args.label,'steps':sum(len(x['steps']) for x in outputs),
             'call_counts':counts,'observed_states':fixture['observed_states'],
             'fixture_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
             'scenarios':[{'name':x['name'],'steps':len(x['steps']),'trace_sha256':x['trace_sha256'],'ram_sha256':x['ram_sha256']} for x in outputs]}
    (BASE/f'playerstep-{args.label}-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary))


if __name__=='__main__':main()
