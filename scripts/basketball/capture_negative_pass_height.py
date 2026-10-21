"""Capture the actual original pass after one declared failed-runtime origin."""
import sys,json,hashlib
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'scripts/basketball'))
from capture_reference import BASE,run
from capture_rest_stall_original import CONFIG
from encode_world_probe import encode_world_probe
from extract_world_loop import loop_state
from compare_rest_stall_worlds import normalized,differences
out=ROOT/'.local/basketball/rea/negative-pass-height'
origin=json.loads((ROOT/'.local/basketball/rea/suspended-repair-proposal/retained-newyork-failure.json').read_text());patch,names=encode_world_probe(origin)
writes=';'.join(f'memory.writebyte({a},{v})' for a,v in sorted(patch.items()))
hook=r'''
local aligned=false;local count=0;local pending=false
local trace=assert(io.open(out..'/pass-ram.bin','wb'));local index=assert(io.open(out..'/pass.jsonl','w'))
local function raw()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i))end;return table.concat(t)end
local function word(a,b)return memory.readbyte(a)+256*memory.readbyte(b)end
memory.registerexec(0xd355,1,function()
 if not aligned and court and memory.readbyte(0x589)==129 then
  writefile('alignment-before.ram',raw());PATCH
  aligned=true;writefile('alignment-after.ram',raw());writefile('alignment-frame.json',string.format('{"frame":%d}',emu.framecount()))
 end
end)
memory.registerexec(0x8c23,1,function()
 if not aligned or memory.readbyte(0xbfff)~=5 then return end
 assert(not pending);pending=true;trace:write(raw())
 local i=memory.getregister('x')
 index:write(string.format('{"frame":%d,"actor":%d,"x":%d,"depth":%d,"height":%d,"targetX":%d,"targetDepth":%d,"targetHeight":%d,"arc":%d}\n',emu.framecount(),i,word(0x75+i,0x82+i),memory.readbyte(0x9c+i),word(0xb6+i,0xc3+i),word(0x69f,0x6a0),memory.readbyte(0x6a1),word(0x6a2,0x6a3),memory.readbyte(0x6a4)))
end)
memory.registerexec(0x8cf1,1,function()
 if not aligned or memory.readbyte(0xbfff)~=5 or not pending then return end
 trace:write(raw());pending=false;count=count+1
end)
function finish_probe()assert(aligned);trace:close();index:close();writefile('pass-result.json',string.format('{"calls":%d,"pending":%s}',count,tostring(pending)))end
'''.replace('PATCH',writes)
spec=json.loads((BASE/'opening-from-menu.json').read_text());spec.pop('save_slot',None);spec.update(name='negative-pass-height-observed',frames=3000,captures=[1500,3000],hooks_lua=CONFIG+'\n'+hook)
p=out/'observed-spec.json';p.write_text(json.dumps(spec,indent=2)+'\n');run(p)
observed=BASE/'runs'/spec['name'];first=(observed/'alignment-after.ram').read_bytes();assert not list(differences(normalized(loop_state(first,origin['playing']['world']['Updates'])),normalized(origin['playing']['world'])))
rows=[json.loads(l) for l in (observed/'pass.jsonl').read_text().splitlines()];print('source calls',rows[:4],json.loads((observed/'pass-result.json').read_text()))
