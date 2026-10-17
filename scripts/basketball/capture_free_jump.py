"""Observe unlinked, unpossessed jump/falling world phases without RAM writes."""
import hashlib
import json
from capture_reference import BASE,ROOT,run
from capture_hoop_player import HOOK

prefix=HOOK[:HOOK.index('memory.registerexec')].replace('hoop-player.jsonl','free-jump.jsonl')
prefix=prefix.replace('local pending=nil;local actor=nil','local pending={}')
hook=prefix+r'''
local function eligible(i)
 local a=AND(memory.readbyte(0x512+i),127)
 if a~=4 and a~=5 and a~=15 then return false end
 if memory.readbyte(0x642+i)>=128 or memory.readbyte(0x53f)==i then return false end
 for j=0x53d,0x546 do if memory.readbyte(j)==i then return false end end
 return true
end
memory.registerexec(0xd345,1,function()
 pending={}
 for i=0,3 do
  if eligible(i) then pending[i]=string.format('{"frame":%d,"actor":%d,"before":%s,',emu.framecount(),i,state(i)) end
 end
end)
memory.registerexec(0xd348,1,function()
 for i=0,3 do if pending[i] then f:write(pending[i]..'"after":'..state(i)..'}\n') end end
 pending={}
end)
function finish_probe() f:close() end
'''

def main():
 name='hoop-control-cpu-long'
 reference=BASE/'runs'/name
 spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='free-jump-'+name,hooks_lua=hook)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 records=[dict(scenario=name,**json.loads(line)) for line in (observed/'free-jump.jsonl').read_text().splitlines()]
 fixture=ROOT/'games/basketball/engine/testdata/free-jump.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
 fixture.with_name('free-jump-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='fixed:d345',exit='fixed:d348',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')
 print('free-jump',len(records))

if __name__=='__main__': main()
