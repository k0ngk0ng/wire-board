#!/usr/bin/env python3
"""Record complete original DB81 streaming, resources and deferred queue output.

Failures specified before implementation: wrong map variant before Boss scene;
using 256 instead of 240 vertical pixels; wrong partial collision rows or
attribute nibbles; packet grouping/budget drift; repeated alternate triggers;
losing palette/CHR request order; treating queued presentation as immediate;
invalid resources or capacity leaving a partial phase.

Controller-only samples and controlled scroll/transition inputs are separate.
Controlled runs retain actual original map decoding and all intervention data.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-terrain-scroll/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

FIELDS=[('frame',0x1a),('level',0x30),('sceneType',0x40),('scrollType',0x41),('alternate',0x48),
        ('axis',0x60),('remaining',0x61),('tileLo',0x62),('tileHi',0x63),('screen',0x64),('screenScroll',0x65),
        ('attrLo',0x66),('attrHi',0x67),('frameScroll',0x68),('mapOffset',0x69),('screenCleared',0x6a),
        ('indoorScroll',0x73),('paletteTimer',0x74),('autoA',0x75),('autoB',0x76),('tankScroll',0x77),
        ('enemyOffset',0x82),('bossScrollComplete',0x84),('force0',0xca),('force1',0xcb),
        ('scrollY',0xfc),('scrollX',0xfd),('ppuControl',0xff),('paletteCount',0x36),('generationBias',0x06),('stopScroll',0x58)]

HOOK=r'''
local log=assert(io.open(out..'/terrain-scroll.jsonl','w'))
local fields=FIELDS;local pending=nil;local registered={};local entries={};local injected=false;local done=false
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function hex(a,n,reader)local t={};reader=reader or read;for i=a,a+n-1 do t[#t+1]=string.format('%02x',reader(i))end;return table.concat(t)end
local function snapshot()
 local values={};for _,a in ipairs(fields)do values[#values+1]=tostring(read(a))end
 local variant=(word(0x42)==0x90e3 and word(0x44)==0xb68a and word(0x46)==0xbe8a)and 1 or 0
 return string.format('{"state":[%s],"variant":%d,"header":"%s","pointers":"%s","maps":"%s","collision":"%s","queue":"%s","palette":"%s","chr":"%s"}',
 table.concat(values,','),variant,hex(0x49,15),hex(0x42,6),hex(0x600,128),hex(0x680,128),hex(0x700,read(0x21)),hex(0x7c0,32),hex(0x7f0,8))
end
local function patch(a,v,changes)
 changes[#changes+1]=string.format('{"address":%d,"before":%d,"after":%d}',a,read(a),v);memory.writebyte(a,v)
end
local function event(kind,value)
 if pending then pending.effects[#pending.effects+1]=string.format('{"kind":"%s","value":%d}',kind,value or 0)end
end
local function install(pc)
 if registered[pc]then return end;registered[pc]=true
 memory.registerexec(pc,1,function()
  if pending and pending.pc==pc and pending.sp==memory.getregister('s')then
   local p=pending;pending=nil
   local unchanged=p.ppu==hex(0x2000,2048,ppu.readbyte)..hex(0x3f00,32,ppu.readbyte);assert(unchanged,'DB81 changed presentation before its display boundary')
   log:write(string.format('{"frame":%d,"interventions":[%s],"before":%s,"after":%s,"effects":[%s],"ppuUnchanged":%s}\n',p.frame,table.concat(p.interventions,','),p.before,snapshot(),table.concat(p.effects,','),tostring(unchanged)))
  end
  if entries[pc]then entries[pc]()end
 end)
end
entries[0xfa3a]=function()
 assert(not pending)
 local changes={}
 BEFORE_PHASE
 local sp=memory.getregister('s');local pc=read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1
 pending={pc=pc,sp=AND(sp+2,255),frame=emu.framecount(),interventions=changes,before=snapshot(),effects={},ppu=hex(0x2000,2048,ppu.readbyte)..hex(0x3f00,32,ppu.readbyte)}
 install(pc)
end
install(0xfa3a)
for pc,kind in pairs({[0xc778]='chr-preset',[0xcee3]='palette-load',[0xf9bc]='sound'})do
 local address,name=pc,kind;entries[address]=function()event(name,memory.getregister('a'))end;install(address)
end
entries[0xf9d4]=function()event('audio-reset',0)end;install(0xf9d4)
for _,pc in ipairs({0xdc13,0xdc40,0xdd37})do
 local address=pc;entries[address]=function()event('palette-timer',memory.getregister('a'))end;install(address)
end
function finish_probe()assert(not pending);log:close()end
'''


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',required=True)
    parser.add_argument('--scope',choices=['natural','boundaries','counters'],required=True);args=parser.parse_args()
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,20}',args.label):parser.error('invalid capture label')
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    entries=json.loads((ROOT/'docs/research/nes-contra/evidence/campaign-controlled-entries.json').read_text())['entries']
    stage_info={e['stage']:e for e in entries}
    for stage in range(1,9):
        src=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';dest=BASE/f'states/stage-{stage}.fcs'
        if dest.exists():assert src.read_bytes()==dest.read_bytes()
        else:shutil.copyfile(src,dest)
    captures=[]
    if args.scope=='natural':
        modes=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/static-player-modes.json.gz').read_bytes()))
        selected=['base2-progress','waterfall-climb','base2-boss']
        for scenario in modes['scenarios']:
            if scenario['name'] not in selected:continue
            spec=scenario['scenario'];setup=''
            captures.append({'name':scenario['name'],'stage':scenario['stage'],'frames':spec['frames'],
                             'segments':spec['segments'],'before_phase':setup,'source_scenario':scenario,
                             'initial_boss_setup':bool(scenario['additional_intervention'])})
        for stage in [1,5,6,7,8]:
            segments=[{'start':1,'end':400,'player':p,'keys':['right','B']}for p in [1,2]]
            segments +=[{'start':n,'end':n+1,'player':p,'keys':['A']}for p in [1,2]for n in range(60,400,90)]
            captures.append({'name':f'stage-{stage}-walk','stage':stage,'frames':400,'segments':segments,'before_phase':''})
    elif args.scope=='boundaries':
        resources=json.loads((ROOT/'games/contra/engine/leveldata/levels.json').read_text())
        for s in resources['stages']:
            stage=s['stage'];vertical=s['scrollType']==1;base=s['sceneType']==1
            if base:
                code="if not injected then patch(0x6a,128,changes);patch(0x61,0,changes);patch(0x73,1,changes);patch(0x64,read(0x48),changes);patch(0x65,3,changes);injected=true end"
                frames=40
            else:
                code="if not injected then patch(0x32,99,changes);patch(0x33,99,changes);patch(0xb0,255,changes);patch(0xb1,255,changes);injected=true end;if read(0x48)~=255 then patch(0x68,7,changes)else done=true;patch(0x68,0,changes)end"
                frames=(s['alternate']['triggerScreen']*(240 if vertical else 256)+6)//7+70
            captures.append({'name':f'stage-{stage}-alternate','stage':stage,'frames':frames,'segments':[],'before_phase':code})
    else:
        cases=[('auto-a-finishes',1,{0x75:1,0x76:0,0x68:0,0x77:0}),
               ('auto-a-continues',1,{0x75:2,0x76:0,0x68:0,0x77:0}),
               ('auto-b-continues',1,{0x75:2,0x76:2,0x68:3,0x77:0}),
               ('auto-b-to-a',1,{0x75:2,0x76:1,0x68:3,0x77:0}),
               ('auto-b-retains-scroll',1,{0x75:0,0x76:1,0x68:3,0x77:0}),
               ('tank-only',5,{0x75:0,0x76:0,0x68:0,0x77:1}),
               ('tank-wrap',5,{0x75:0,0x76:0,0x68:255,0x77:1}),
               ('vertical-auto',3,{0x75:1,0x76:2,0x68:9,0x77:3}),
               ('vertical-auto-continues',3,{0x75:2,0x76:2,0x68:9,0x77:3}),
               ('base-positive-clear',2,{0x6a:1,0x61:0,0x73:1}),
               ('base-no-clear',2,{0x6a:0,0x61:0,0x73:1}),
               ('base-waits-for-players',2,{0x6a:128,0x61:0,0x73:0})]
        for name,stage,changes in cases:
            code='if not injected then '+''.join(f'patch({a},{v},changes);' for a,v in changes.items())+'injected=true end'
            captures.append({'name':name,'stage':stage,'frames':2,'segments':[],'before_phase':code})
    for capture in captures:
        name=f'terrain-scroll-{args.scope}-{args.label}-{capture["name"]}'
        hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in FIELDS)+'}').replace('BEFORE_PHASE',capture['before_phase'])
        if capture.get('initial_boss_setup'):
            hook="local a=memory.readbyte(0x64);local b=memory.readbyte(0x6a);local c=memory.readbyte(0xb0);local d=memory.readbyte(0xb1);memory.writebyte(0x64,memory.readbyte(0x58)-1);memory.writebyte(0x6a,128);memory.writebyte(0xb0,255);memory.writebyte(0xb1,255);writefile('setup.json',string.format('{\"screenBefore\":%d,\"clearBefore\":%d,\"barrierBefore\":[%d,%d],\"after\":[4,128,255,255]}',a,b,c,d))\n"+hook
        spec={'name':name,'load_state':f'stage-{capture["stage"]}','frames':capture['frames'],'segments':capture['segments'],
              'captures':[1,capture['frames']],'hooks_lua':hook,'timeout_seconds':120}
        path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
        records=[json.loads(line)for line in (observed/'terrain-scroll.jsonl').read_text().splitlines()]
        assert records and all(r['ppuUnchanged']for r in records)
        capture.update(records=records,source=json.loads((observed/'source.json').read_text()),scenario=spec,
                       initial_stage_source=stage_info[capture['stage']],ram_sha256=hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest())
        if capture.get('source_scenario'):
            assert capture['ram_sha256']==capture['source_scenario']['ram_sha256'],'observer changed existing original scenario'
            capture['source_scenario']={'name':capture['source_scenario']['name'],'ram_sha256':capture['source_scenario']['ram_sha256']}
    result={'source_is_original_execution':True,'scope':args.scope,
            'description':'Controller inputs from declared stage initializers; boss source retains its declared room/barrier setup.' if args.scope=='natural' else 'Explicit per-phase scroll inputs and final base transition; original maps are loaded by the game itself; not natural traversal.',
            'fields':[{'name':n,'address':hex(a)}for n,a in FIELDS],'captures':captures}
    destination=ROOT/f'docs/research/nes-contra/evidence/terrain-scroll-{args.scope}.json.gz'
    if destination.exists():
        old=json.loads(gzip.decompress(destination.read_bytes()))
        for a,b in zip(old['captures'],captures,strict=True):assert a['records']==b['records']and a['ram_sha256']==b['ram_sha256']
    else:destination.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'passed':True,'scope':args.scope,'captures':len(captures),'records':sum(len(c['records'])for c in captures),
                      'sha256':hashlib.sha256(destination.read_bytes()).hexdigest()}))


if __name__=='__main__':main()
