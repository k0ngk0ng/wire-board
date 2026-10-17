"""Retain whole original action-phase states, without injecting any world data."""
import csv
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out..'/world-actions.csv','w'))
local raw=assert(io.open(out..'/world-actions-ram.bin','wb'));local pending=nil
f:write('frame\n')
local function snapshot()
 local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end
 raw:write(table.concat(t))
end
memory.registerexec(0xd345,1,function()
 assert(not pending);pending=emu.framecount();snapshot()
end)
memory.registerexec(0xd348,1,function()
 assert(pending);f:write(tostring(pending)..'\n');snapshot();pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-world-action.json','{"frame":'..pending..'}') end
 f:close();raw:close()
end
'''


def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
    spec.update(name='world-actions-cpu-long',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name']
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(out/image.name).read_bytes()
    with (out/'world-actions.csv').open() as f:rows=list(csv.DictReader(f))
    raw=(out/'world-actions-ram.bin').read_bytes()
    assert len(raw)>=len(rows)*4096 and len(raw)<len(rows)*4096+4096
    summary=dict(records=len(rows),complete_bytes=len(rows)*4096,ram_matches_uninstrumented=True,source=json.loads((out/'source.json').read_text()),trace_sha256=hashlib.sha256(raw).hexdigest())
    target=ROOT/'.local/basketball/world-actions';target.mkdir(parents=True,exist_ok=True)
    (target/'capture.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary))


if __name__=='__main__':main()
