"""Observe the original zone calculation itself, not its uninitialized cache."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/zones.jsonl","w"));local pending=nil
memory.registerexec(0x9ea4,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 local i=memory.getregister("x");assert(i<4 and not pending)
 local x=memory.readbyte(0x68+i)+256*memory.readbyte(0x75+i)+65536*memory.readbyte(0x82+i)
 local depth=memory.readbyte(0x8f+i)+256*memory.readbyte(0x9c+i)
 pending=string.format('{"actor":%d,"frame":%d,"x":%d,"depth":%d,',i,emu.framecount(),x,depth)
end)
memory.registerexec(0x9f12,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending);f:write(pending..string.format('"zone":%d}\n',memory.readbyte(0x66a+memory.getregister("x"))));pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-zone.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    reference=BASE/'runs/shot-observation'
    spec=json.loads((reference/'scenario.json').read_text());spec.update(name='ai-zone-calculation',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    rows=[json.loads(line) for line in (observed/'zones.jsonl').read_text().splitlines()]
    p=ROOT/'games/basketball/engine/testdata/ai-zones.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    p.with_name('ai-zones-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank5:9ea4',exit='bank5:9f12',records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')
    print(json.dumps(dict(records=len(rows))))

if __name__=='__main__':main()
