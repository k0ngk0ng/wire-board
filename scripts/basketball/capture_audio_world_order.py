"""Observe configured Court2 gameplay without injecting a ball or audio state.

Only initial legal teams/court are selected before their original initializers.
Every later input is a controller timeline. A separate configured reference
must match all observed per-frame RAM and screenshots. This does not claim the
original menu selected these settings, nor that a missing chain is impossible.
"""
import collections
import hashlib
import json
import sys
from pathlib import Path
from capture_reference import ROOT, BASE, run

CONFIG = r'''
local configured_teams=false
local configured_court=false
memory.registerexec(0xacbb,1,function()
 if not configured_teams and memory.readbyte(0xbfff)==3 then
  memory.writebyte(0x586,TEAM0);memory.writebyte(0x587,TEAM1);configured_teams=true
  writefile('configured-teams.json',string.format('{"teams":[TEAM0,TEAM1],"frame":%d,"scope":"initial original roster configuration only"}',emu.framecount()))
 end
end)
memory.registerexec(0xd1b5,1,function()
 if not configured_court and memory.readbyte(0x589)<128 then
  memory.writebyte(0x58b,2);configured_court=true
  writefile('configured-court.json',string.format('{"court":2,"frame":%d,"scope":"initial original court configuration only"}',emu.framecount()))
 end
end)
'''

HOOK = r'''
local log=assert(io.open(out..'/audio-world-order.jsonl','w'))
local chains=assert(io.open(out..'/turn-chains.jsonl','w'))
local chainram=assert(io.open(out..'/turn-chains.ram','wb'))
local sequence=0
local request=nil
local motion=nil
local comparison=nil
local chaincount=0
local function raw(first,count)
 local t={};for i=first,first+count-1 do t[#t+1]=string.char(memory.readbyte(i)) end;return table.concat(t)
end
local function driver()
 local t={};for i=0x700,0x7e6 do t[#t+1]=string.format('%02x',memory.readbyte(i)) end;return table.concat(t)
end
local function event(kind,extra,with_driver)
 sequence=sequence+1
 local entry=string.format('{"order":%d,"frame":%d,"cycle":%.0f,"kind":"%s","work":%d,"court":%d,"phase":%d,"action":%d,"hoop":%d,"x":%d,"height":%d,"vx":%d,"upload":[%d,%d,%d]',sequence,emu.framecount(),debugger.getcyclescount(),kind,memory.readbyte(0xff),memory.readbyte(0x58b),memory.readbyte(0x589),memory.readbyte(0x518),memory.readbyte(0x55d),memory.readbyte(0x7b)+256*memory.readbyte(0x88),memory.readbyte(0xbc)+256*memory.readbyte(0xc9),memory.readbyte(0x47e)+256*memory.readbyte(0x48b),memory.readbyte(0x100),memory.readbyte(0x101),memory.readbyte(0x102))
 if with_driver then entry=entry..',"driver":"'..driver()..'"' end
 if extra then entry=entry..','..extra end
 entry=entry..'}';log:write(entry..'\n');return entry
end
memory.registerexec(0xf089,1,function()event('nmi')end)
memory.registerexec(0xf368,1,function()event('irq_before')end)
memory.registerexec(0xf36b,1,function()event('irq_after',nil,true)end)
memory.registerexec(0xfbcc,1,function()
 assert(not request);request=memory.getregister('a');event('request_before','"id":'..request)
end)
memory.registerexec(0xfbde,1,function()
 if request then event('request_after','"id":'..request,true);request=nil end
end)
for _,entry in ipairs({{0xd314,'world_start'},{0xd32c,'inputs_start'},{0xd345,'actions_start'},{0xd348,'actions_end'},{0xd352,'court_tail'}}) do
 local address,name=entry[1],entry[2]
 memory.registerexec(address,1,function()event(name)end)
end
memory.registerexec(0xbace,1,function()
 if memory.readbyte(0xbfff)==6 and memory.getregister('x')==6 then event('ball14_enter') end
end)
memory.registerexec(0xbaf3,1,function()
 if memory.readbyte(0xbfff)==6 and memory.getregister('x')==6 then motion={before=raw(0,2048),event=event('ball14_motion_before')} end
end)
memory.registerexec(0xbaf6,1,function()
 if memory.readbyte(0xbfff)==6 and memory.getregister('x')==6 then
  assert(motion);motion.after=raw(0,2048);motion.afterEvent=event('ball14_motion_after')
 end
end)
memory.registerexec(0xbdd2,1,function()
 if memory.readbyte(0xbfff)~=6 or memory.getregister('x')~=6 then return end
 local e=event('turn_compare',nil,true)
 if AND(memory.readbyte(0x518),127)==4 and motion then
  comparison={before=raw(0,2048),event=e,motion=motion};event('turn_alias_before')
 end
end)
memory.registerexec(0xbb08,1,function()
 if memory.readbyte(0xbfff)~=6 or memory.getregister('x')~=6 then return end
 local e=event('ball14_return')
 if comparison then
  chaincount=chaincount+1
  chainram:write(comparison.motion.before);chainram:write(comparison.motion.after);chainram:write(comparison.before);chainram:write(raw(0,2048))
  chains:write(string.format('{"index":%d,"motion_before":%s,"motion_after":%s,"compare":%s,"return":%s}\n',chaincount,comparison.motion.event,comparison.motion.afterEvent,comparison.event,e));comparison=nil
 end
 motion=nil
end)
local action=memory.readbyte(0x518)
memory.registerwrite(0x518,1,function(a,z,v)
 local old=action;action=v
 if AND(old,127)==14 or AND(v,127)==14 then event('ball_action_write',string.format('"pc":%d,"old":%d,"value":%d',memory.getregister('pc'),old,v)) end
end)
function finish_probe()log:close();chains:close();chainram:close()end
'''


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def capture(name,frames,team0,team1,segments=None):
    base=json.loads((BASE/'opening-from-menu.json').read_text())
    base.pop('save_slot',None)
    base['frames']=frames
    base['captures']=[n for n in [1450,2000,5000,10000,frames] if n<=frames]
    base['segments'] += segments if segments is not None else [dict(start=n,end=n+12,keys=['B']) for n in range(1450,frames-20,90)]
    config=CONFIG.replace('TEAM0',str(team0)).replace('TEAM1',str(team1))
    runs=[]
    for suffix,hook in [('reference',config),('observed',config+'\n'+HOOK)]:
        spec=dict(base,name=name+'-'+suffix,hooks_lua=hook)
        out=BASE/'runs'/spec['name'];path=ROOT/'.local/basketball/audio/world-order'/(spec['name']+'.json')
        path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(spec,indent=2)+'\n')
        if not ((out/'capture-complete.json').exists() and json.loads((out/'scenario.json').read_text())==spec):run(path)
        runs.append(out)
    reference,observed=runs
    assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
    for n in base['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
    events=[json.loads(line) for line in (observed/'audio-world-order.jsonl').read_text().splitlines()]
    chains=[json.loads(line) for line in (observed/'turn-chains.jsonl').read_text().splitlines()]
    counts=collections.Counter(event['kind'] for event in events)
    assert len((observed/'turn-chains.ram').read_bytes())==len(chains)*8192
    summary={'scope':'initial legal court/team configuration, then unchanged original gameplay and controller input; not natural menu selection',
        'name':name,'frames':frames,'teams':[team0,team1],'court':2,'ram_and_screenshots_match_reference':True,
        'ram_sha256':sha(observed/'ram.bin'),'events_sha256':sha(observed/'audio-world-order.jsonl'),
        'trace':str((observed/'audio-world-order.jsonl').relative_to(ROOT)),'counts':dict(counts),'chains':chains,
        'chain_ram_sha256':sha(observed/'turn-chains.ram'),'source':json.loads((observed/'source.json').read_text())}
    (ROOT/'.local/basketball/audio/world-order'/(name+'-summary.json')).write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps({key:summary[key] for key in ['name','frames','counts','ram_and_screenshots_match_reference']}))


if __name__=='__main__':
    capture(sys.argv[1] if len(sys.argv)>1 else 'audio-court2-team3',int(sys.argv[2]) if len(sys.argv)>2 else 18000,3,3,json.loads(Path(sys.argv[3]).read_text()) if len(sys.argv)>3 else None)
