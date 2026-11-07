#!/usr/bin/env python3
"""Observe original spawn, hit, death and cooperative life borrowing.

Failures defined before implementation: reducing lives on hit instead of after
death; erasing timers that original reset preserves; unsafe spawn placement;
death animation timer confusion; wrong donor eligibility or seat order; borrowing
on held A; accepting single-player borrowing; source or after-state injection.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-static/lifecycle-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

HOOK=r'''
local log=assert(io.open(out.."/lifecycle.jsonl","w"))
local maps=assert(io.open(out.."/lifecycle-maps.jsonl","w"))
local fields=PLAYER_FIELDS;local globals=WORLD_FIELDS
local active=nil;local callbacks={};local entries={};local serial=0;local mapCount=0;local mapIds={}
CONTROLLED_CASES
local caseIndex=0
local function rb(a) return memory.readbyte(a) end
local function vector(addresses,actor)
 local t={};for _,a in ipairs(addresses) do t[#t+1]=tostring(rb(a+actor)) end;return '['..table.concat(t,',')..']'
end
local function map_id()
 local t={};for i=0x680,0x6ff do t[#t+1]=string.format('%02x',rb(i)) end;local data=table.concat(t);local id=mapIds[data]
 if not id then mapCount=mapCount+1;id=mapCount;mapIds[data]=id;maps:write(string.format('{"id":%d,"bytes":"%s"}\n',id,data)) end
 return id
end
local function snapshot(actor)
 return string.format('{"player":%s,"peer":%s,"world":%s,"map":%d}',vector(fields,actor),vector(fields,1-actor),vector(globals,0),map_id())
end
local function registers()
 return string.format('[%d,%d,%d,%d]',memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.getregister('p'))
end
local function install(pc)
 if callbacks[pc] then return end;callbacks[pc]=true
 memory.registerexec(pc,1,function()
  if active and active.pc==pc and active.sp==memory.getregister('s') then
   local caseField=controlledCases and string.format(',"case":%d',active.case) or ''
   log:write(string.format('{"id":%d,"kind":"%s"%s,"frame":%d,"actor":%d,"aimContext":%d,"before":%s,"after":%s,"input":%s,"output":%s}\n',active.id,active.kind,caseField,active.frame,active.actor,active.aimContext,active.before,snapshot(active.actor),active.input,registers()))
   if active.restore then for a=0,2047 do memory.writebyte(a,active.restore[a]) end end
   active=nil
  end
  if entries[pc] then entries[pc]() end
 end)
end
local function observe(address,kind,shared)
 entries[address]=function()
  assert(not active,'nested lifecycle root')
  local restore=nil
  if controlledCases then
   if kind~='borrow' then return end
   local case=controlledCases[caseIndex+1];if not case then return end;caseIndex=caseIndex+1
   restore={};for a=0,2047 do restore[a]=rb(a) end
   for a,v in pairs(case) do memory.writebyte(a,v) end
  end
  local actor=shared and 0 or memory.getregister('x');assert(actor<2)
  local sp=memory.getregister('s')
  local pc=rb(0x100+AND(sp+1,255))+256*rb(0x100+AND(sp+2,255))+1
  serial=serial+1
  active={id=serial,kind=kind,case=caseIndex,restore=restore,frame=emu.framecount(),actor=actor,aimContext=rb(8),before=snapshot(actor),input=registers(),pc=pc,sp=AND(sp+2,255)}
  install(pc)
 end
 install(address)
end
observe(0xd3ed,'spawn',false)
observe(0xd4bd,'dying',false)
observe(0xd474,'hit',false)
observe(0xd30b,'borrow',true)
function finish_probe() assert(not active,'unfinished lifecycle call');log:close();maps:close() end
'''


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');parser.add_argument('--borrow-cases',action='store_true');args=parser.parse_args()
    metadata=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/static-playerstep-complete-oracle.json.gz').read_bytes()))
    for name in ('solo-play','coop-play','coop-start'):
        source=ROOT/f'.local/contra/states/{name}.fcs';target=BASE/f'states/{name}.fcs';target.parent.mkdir(parents=True,exist_ok=True)
        if target.exists():assert source.read_bytes()==target.read_bytes()
        else:shutil.copyfile(source,target)
    hook=HOOK.replace('PLAYER_FIELDS','{'+','.join(v['address'] for v in metadata['player_fields'])+'}')
    hook=hook.replace('WORLD_FIELDS','{'+','.join(v['address'] for v in metadata['world_fields'])+'}')
    cases=[]
    if args.borrow_cases:
        for target in [0,1]:
            donor=1-target
            for lives in [0,1,2]:
                for state in [0,1,2,3]:
                    for advance in [0,1]:
                        for pressed in [0,128]:
                            c={0x2c:4,0x22:1,0x38:0,0x39:0,0x32:0,0x33:0,0x90:1,0x91:1,
                               0xd0:0,0xd1:0,0xf5:0,0xf6:0,0xf1:0,0xf2:0}
                            c.update({0x38+target:1,0x90+target:0,0xaa+target:3,0xd2+target:254,0xd4+target:253,
                                      0x32+donor:lives,0x90+donor:state,0xd0+donor:advance,0xf5+target:pressed,0xf1+target:128})
                            cases.append(c)
        success=dict(cases[-1]);success.update({0x32:2,0x90:1,0xd0:0,0xf6:128})
        for address,value in [(0x22,0),(0x2c,3),(0x2c,5),(0xf6,0),(0x39,0)]:
            c=dict(success);c[address]=value;cases.append(c)
        c=dict(success);c[0x38]=1;c[0x32]=0;c[0xf5]=128;c[0xf6]=0;cases.append(c)
        lua='local controlledCases={'+','.join('{'+','.join(f'[{a}]={v}' for a,v in sorted(c.items()))+'}' for c in cases)+'}'
    else:lua='local controlledCases=nil'
    hook=hook.replace('CONTROLLED_CASES',lua)
    specs=[('entry','coop-start',300,[{'start':1,'end':3,'keys':['start']}]),
           ('solo-deaths','solo-play',2600,[{'start':1,'end':2600,'keys':['right']}]),
           ('coop-borrow','coop-play',3000,[{'start':1,'end':3000,'player':1,'keys':['right']},
                                         {'start':1,'end':3000,'player':2,'keys':['down']},
                                         {'start':1,'end':3,'player':2,'keys':['A']}]+
                                         [{'start':n,'end':n,'player':1,'keys':['A']} for n in range(1500,3000,90)])]
    if args.borrow_cases:specs=[('borrow-boundaries','coop-play',len(cases)+2,[])]
    scenarios=[];counts={};borrow_success=0
    for name,state,frames,segments in specs:
        spec={'name':f'lifecycle-{args.label}-{name}','load_state':state,'frames':frames,'segments':segments,
              'captures':[1,frames],'hooks_lua':hook,'timeout_seconds':90}
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        records=[json.loads(s) for s in (out/'lifecycle.jsonl').read_text().splitlines()]
        maps=[json.loads(s) for s in (out/'lifecycle-maps.jsonl').read_text().splitlines()]
        for r in records:
            counts[r['kind']]=counts.get(r['kind'],0)+1
            if r['kind']=='borrow' and (r['before']['player']!=r['after']['player'] or r['before']['peer']!=r['after']['peer']):borrow_success+=1
        scenarios.append({'name':name,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'steps':records,'maps':maps,
                          'trace_sha256':hashlib.sha256((out/'lifecycle.jsonl').read_bytes()).hexdigest(),
                          'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
    if args.borrow_cases:assert counts.get('borrow',0)==len(cases) and borrow_success>0
    else:assert counts.get('spawn',0)>0 and counts.get('dying',0)>100 and counts.get('hit',0)>0 and counts.get('borrow',0)>0
    scope='borrow-controlled' if args.borrow_cases else 'natural'
    fixture={'schema':1,'source_is_original_execution':True,'scope':scope,
             'interventions':'Borrow entry fields explicitly staged; full RAM restored after each observed original call. Not natural gameplay.' if args.borrow_cases else 'none; controller input only',
             'controlled_cases':cases,
             'rom_sha256':metadata['rom_sha256'],'player_fields':metadata['player_fields'],'world_fields':metadata['world_fields'],
             'scenarios':scenarios,'counts':counts,'borrow_successes':borrow_success}
    data=json.dumps(fixture,separators=(',',':')).encode();compressed=gzip.compress(data,mtime=0)
    path=ROOT/f'docs/research/nes-contra/evidence/static-lifecycle-{scope}.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()))
        for a,b in zip(old['scenarios'],scenarios,strict=True):assert a['steps']==b['steps'] and a['maps']==b['maps'],f"original replay changed: {b['name']}"
    else:path.write_bytes(compressed)
    summary={'passed':True,'label':args.label,'counts':counts,'borrow_successes':borrow_success,'fixture_sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
    (BASE/f'capture-{args.label}.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
