"""Capture the original four-byte random generator, including call order."""
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out.."/random.jsonl","w"));local pending=nil
local function state()
 local bytes={};for i=0x48,0x4b do bytes[#bytes+1]=tostring(memory.readbyte(i)) end
 return '['..table.concat(bytes,',')..']'
end
memory.registerexec(0xf9c3,1,function()
 assert(not pending);pending=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0xf9e7,1,function()
 assert(pending);f:write(pending..string.format('"after":%s,"value":%d}\n',state(),memory.getregister("a")));pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-random.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    records=[];sources=[]
    for name,frames,segments in [('shot',120,[dict(start=1,end=3,keys=['B'])]),('cpu',600,[])]:
        spec=dict(name='random-control-'+name,load_slot=7,frames=frames,segments=segments,captures=[frames])
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        reference=BASE/'runs'/spec['name']
        spec.update(name='random-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'random.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/'games/basketball/engine/testdata/random.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='fixed:f9c3',exit='fixed:f9e7',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name('random-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
