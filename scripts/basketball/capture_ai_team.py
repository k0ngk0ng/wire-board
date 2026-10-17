"""Observe the complete original team-AI phase without injecting game state."""
import csv
import hashlib
import json
from capture_reference import BASE, ROOT, run


HOOK = r'''
local trace=assert(io.open(out.."/team-ai.csv","w"))
local raw=assert(io.open(out.."/team-ai-ram.bin","wb"))
trace:write("frame,team0_branch,team1_branch\n")
local pending=nil
local function snapshot()
 local bytes={};for i=0,2047 do bytes[#bytes+1]=string.char(memory.readbyte(i)) end
 return table.concat(bytes)
end
memory.registerexec(0x99f0,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending,"nested team AI entry")
 pending={emu.framecount(),-1,-1};raw:write(snapshot())
end)
memory.registerexec(0x9af6,1,function()
 if memory.readbyte(0xbfff)~=5 or not pending then return end
 local x=memory.getregister("x");assert(x<2,"invalid team index")
 assert(pending[x+2]==-1,"repeated team dispatch")
 pending[x+2]=memory.readbyte(0x663+x)
end)
memory.registerexec(0x8026,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending and pending[2]>=0 and pending[3]>=0,"incomplete team phase")
 trace:write(table.concat(pending,",").."\n");raw:write(snapshot());pending=nil
end)
function finish_probe() assert(not pending);trace:close();raw:close() end
'''


def main():
    reference = BASE/'runs/shot-observation'
    spec = json.loads((reference/'scenario.json').read_text())
    spec.update(name='team-ai-shot-observation', hooks_lua=HOOK)
    spec.pop('save_slot', None)
    path = BASE/(spec['name']+'.json')
    path.write_text(json.dumps(spec, indent=2)+'\n')
    run(path)
    recorded = BASE/'runs'/spec['name']
    assert (recorded/'ram.bin').read_bytes() == (reference/'ram.bin').read_bytes()
    screenshots = list(reference.glob('frame-*.png'))
    for image in screenshots:
        assert image.read_bytes() == (recorded/image.name).read_bytes()
    rows = list(csv.DictReader((recorded/'team-ai.csv').open()))
    raw = (recorded/'team-ai-ram.bin').read_bytes()
    assert len(raw) == len(rows)*4096 and rows
    summary = dict(calls=len(rows), identical_reference_frames=spec['frames'],
                   identical_screenshots=len(screenshots),
                   trace_sha256=hashlib.sha256(raw).hexdigest(),
                   source=json.loads((recorded/'source.json').read_text()))
    out = ROOT/'.local/basketball/rea/team-ai'
    out.mkdir(parents=True, exist_ok=True)
    (out/'capture.json').write_text(json.dumps(summary, indent=2)+'\n')
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
