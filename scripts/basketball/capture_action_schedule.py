"""Observe every action invocation and attachment flag across original world steps."""
import hashlib
import json
from capture_reference import BASE, ROOT, run

HOOK=r'''
local f=assert(io.open(out..'/action-schedule.jsonl','w'))
local pending=nil;local world=nil;local calls={}
local function flags()
 local t={};for i=0,6 do t[#t+1]=tostring(memory.readbyte(0x642+i)) end
 return '['..table.concat(t,',')..']'
end
memory.registerexec(0xd345,1,function()
 assert(not world and not pending);calls={}
 world=string.format('{"frame":%d,"before":%s,',emu.framecount(),flags())
end)
memory.registerexec(0xe18b,1,function()
 if not world then return end
 assert(not pending)
 local stack=memory.getregister('s')
 local address=memory.readbyte(0x100+((stack+1)%256))+256*memory.readbyte(0x100+((stack+2)%256))+1
 assert(address==0xe168 or address==0xe14c or address==0xe154)
 pending=string.format('{"actor":%d,"before":%s,',memory.getregister('x'),flags())
end)
local function returned()
 if not world then return end
 assert(pending)
 calls[#calls+1]=pending..'"after":'..flags()..'}';pending=nil
end
memory.registerexec(0xe168,1,returned)
memory.registerexec(0xe14c,1,returned)
memory.registerexec(0xe154,1,returned)
memory.registerexec(0xd348,1,function()
 assert(world and not pending)
 f:write(world..'"calls":['..table.concat(calls,',')..'],"after":'..flags()..'}\n');world=nil;calls={}
end)
function finish_probe()
 if world then writefile('incomplete-action-schedule.json',world..'"incomplete":true}') end
 f:close()
end
'''


def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
    spec.update(name='action-schedule-cpu-long',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name']
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(out/image.name).read_bytes()
    rows=[json.loads(line) for line in (out/'action-schedule.jsonl').read_text().splitlines()]
    assert rows
    p=ROOT/'games/basketball/engine/testdata/action-schedule.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='fixed:d345',exit='fixed:d348',records=len(rows),calls=sum(len(r['calls']) for r in rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
    p.with_name('action-schedule-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({k:manifest[k] for k in ['records','calls']}))


if __name__=='__main__': main()
