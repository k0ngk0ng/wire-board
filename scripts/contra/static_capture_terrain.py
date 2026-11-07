#!/usr/bin/env python3
"""Record natural map streaming and collision queries in the original ROM.

Predefined failures: stale state, a changed oracle ROM, hook return mismatch,
missing source collision byte, empty coverage, or accidental writes to another
recorder's mutable FCEUX directory. Hooks observe memory; they never change it.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / '.local/contra-static/oracle'
os.environ['CONTRA_REFERENCE_ROOT'] = str(BASE)
from capture_reference import run

HOOK = r'''
local queries=assert(io.open(out.."/collision.jsonl","w"))
local mapfile=assert(io.open(out.."/collision-maps.jsonl","w"))
local streams=assert(io.open(out.."/streaming.jsonl","w"))
local seen={};local mapcount=0;local pending=nil;local streaming=nil;local exits={}
local function read(a) return memory.readbyte(a) end
local function hex(a,n)
 local t={};for k=a,a+n-1 do t[#t+1]=string.format("%02x",read(k)) end;return table.concat(t)
end
local function return_pc()
 local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1
end
local function arm(pc)
 if exits[pc] then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if pending and pending.pc==pc then
   local q=pending;pending=nil
   queries:write(string.format('{"frame":%d,"x":%d,"y":%d,"scrollX":%d,"scrollY":%d,"ppuControl":%d,"map":%d,"code":%d,"carry":%d,"negative":%d,"sourceIndex":%d,"sourceByte":%d,"column":%d}\n',
    q.frame,q.x,q.y,q.scrollX,q.scrollY,q.ppuControl,q.map,memory.getregister('a'),AND(memory.getregister('p'),1),AND(memory.getregister('p'),128),read(0x13),read(0x680+read(0x13)),read(0x12)))
  end
  if streaming and streaming.pc==pc then
   local s=streaming;streaming=nil
   streams:write(string.format('{"frame":%d,"screen":%d,"offset":%d,"vertical":%d,"indices":"%s"}\n',s.frame,s.screen,s.offset,s.vertical,hex(0x600+s.offset,s.vertical==0 and 56 or 64)))
  end
 end)
end
memory.registerexec(0xdfec,1,function()
 assert(not pending,'nested collision query')
 local state=hex(0x680,128);local id=seen[state]
 if not id then mapcount=mapcount+1;id=mapcount;seen[state]=id;mapfile:write(string.format('{"id":%d,"bytes":"%s"}\n',id,state)) end
 local pc=return_pc()
 pending={pc=pc,frame=emu.framecount(),x=memory.getregister('a'),y=memory.getregister('y'),scrollX=read(0xfd),scrollY=read(0xfc),ppuControl=read(0xff),map=id}
 arm(pc)
end)
memory.registerexec(0xe09c,1,function()
 assert(not streaming,'nested map decode')
 local pc=return_pc()
 streaming={pc=pc,frame=emu.framecount(),screen=memory.getregister('a'),offset=read(0x69),vertical=read(0x41)}
 arm(pc)
end)
function finish_probe()
 assert(not pending and not streaming,'unfinished observed call')
 queries:close();mapfile:close();streams:close()
end
'''


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--name', default='terrain-walk-jump-01')
    parser.add_argument('--state', type=Path, default=ROOT/'.local/contra/states/coop-play.fcs')
    args=parser.parse_args()
    state=args.state.resolve()
    assert state.is_relative_to(ROOT) and state.is_file()
    target=BASE/'states/coop-play.fcs';target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists():
        assert target.read_bytes()==state.read_bytes()
    else:
        shutil.copyfile(state,target)
    segments=[]
    for player in (1,2):
        segments.append({'start':1,'end':600,'player':player,'keys':['right','B']})
        for frame in range(60,600,90):
            segments.append({'start':frame,'end':frame+1,'player':player,'keys':['A']})
    spec={'name':args.name,'load_state':'coop-play','frames':600,'segments':segments,
          'captures':[1,120,240,360,480,600],'hooks_lua':HOOK,'timeout_seconds':60}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
    output=run(path)
    counts={}
    for name in ['collision.jsonl','collision-maps.jsonl','streaming.jsonl']:
        raw=(output/name).read_bytes();rows=[json.loads(line) for line in raw.splitlines()]
        counts[name]={'records':len(rows),'sha256':hashlib.sha256(raw).hexdigest()}
    assert counts['collision.jsonl']['records']>0
    assert counts['streaming.jsonl']['records']>0
    (output/'terrain-source.json').write_text(json.dumps({'passed':True,'state_sha256':hashlib.sha256(state.read_bytes()).hexdigest(),'outputs':counts},indent=2)+'\n')
    print(json.dumps(counts))


if __name__=='__main__':main()
