"""Record the whole contact-response phase and every actual attack response."""
import csv
import hashlib
import json
from capture_reference import BASE, ROOT, run

HOOK=r'''
local phases=assert(io.open(out.."/combat-phases.csv","w"))
local phaseRaw=assert(io.open(out.."/combat-phase-ram.bin","wb"))
local calls=assert(io.open(out.."/combat-calls.csv","w"))
local callRaw=assert(io.open(out.."/combat-call-ram.bin","wb"))
phases:write("frame,sounds\n");calls:write("frame,source,target\n")
local phase=nil;local pending=nil;local sounds={}
local function snapshot()
 local result={};for i=0,2047 do result[#result+1]=string.char(memory.readbyte(i)) end
 return table.concat(result)
end
memory.registerexec(0x84b9,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not phase);phase=emu.framecount();sounds={};phaseRaw:write(snapshot())
end)
memory.registerexec(0xc000,1,function()
 if memory.readbyte(0xbfff)==5 and phase then sounds[#sounds+1]=memory.getregister("a") end
end)
memory.registerexec(0x8653,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(phase and not pending)
 local x=memory.getregister("x");local target=memory.readbyte(0x3c+x)
 if target>=128 then return end
 pending={emu.framecount(),x,target};callRaw:write(snapshot())
end)
memory.registerexec(0x84be,1,function()
 if memory.readbyte(0xbfff)~=5 or not pending then return end
 calls:write(table.concat(pending,",").."\n");callRaw:write(snapshot());pending=nil
end)
memory.registerexec(0x84c3,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(phase and not pending);phases:write(tostring(phase)..","..table.concat(sounds,";").."\n");phaseRaw:write(snapshot());phase=nil
end)
function finish_probe() assert(not phase and not pending);phases:close();phaseRaw:close();calls:close();callRaw:close() end
'''


def main():
    reference=BASE/'runs/shot-observation'
    spec=json.loads((reference/'scenario.json').read_text())
    spec.update(name='combat-shot-observation',hooks_lua=HOOK);spec.pop('save_slot',None)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    images=list(reference.glob('frame-*.png'))
    for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
    summary=dict(identical_reference_frames=spec['frames'],identical_screenshots=len(images),source=json.loads((observed/'source.json').read_text()))
    for kind in ['phase','call']:
        rows=list(csv.DictReader((observed/f'combat-{kind}s.csv').open()))
        raw=(observed/f'combat-{kind}-ram.bin').read_bytes()
        assert len(raw)==len(rows)*4096
        summary[kind]=dict(records=len(rows),sha256=hashlib.sha256(raw).hexdigest())
    out=ROOT/'.local/basketball/rea/combat';out.mkdir(parents=True,exist_ok=True)
    (out/'capture.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
