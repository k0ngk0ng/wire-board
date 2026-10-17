"""Capture all four actor inputs and the subsequent object-input stage."""
import csv
import gzip
import hashlib
import json
from capture_reference import BASE,ROOT,run

HOOK=r'''
local raw=assert(io.open(out.."/world-inputs-ram.bin","wb"))
local phases=assert(io.open(out.."/world-inputs.csv","w"));phases:write("frame,objects\n")
local pending=nil;local objects={}
local function snapshot()
 local bytes={};for i=0,2047 do bytes[#bytes+1]=string.char(memory.readbyte(i)) end
 return table.concat(bytes)
end
memory.registerexec(0xd32c,1,function()
 assert(not pending);pending=emu.framecount();objects={};raw:write(snapshot())
end)
memory.registerexec(0x8000,1,function()
 if pending and memory.readbyte(0xbfff)==6 then objects[#objects+1]=memory.getregister("x") end
end)
memory.registerexec(0x8467,1,function()
 if pending and memory.readbyte(0xbfff)==6 then objects[#objects+1]=memory.getregister("x") end
end)
memory.registerexec(0xd345,1,function()
 assert(pending);phases:write(tostring(pending)..","..table.concat(objects,";").."\n");raw:write(snapshot());pending=nil
end)
function finish_probe() assert(not pending);raw:close();phases:close() end
'''


def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text());spec.update(name='world-inputs-cpu-long',hooks_lua=HOOK);spec.pop('save_slot',None)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    images=list(reference.glob('frame-*.png'))
    for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
    rows=list(csv.DictReader((observed/'world-inputs.csv').open()));raw_path=observed/'world-inputs-ram.bin';raw=raw_path.read_bytes()
    assert len(rows)==1998 and len(raw)==len(rows)*4096
    assert all(r['objects']=='0;1;2;3;4;5;6;7;8;9' for r in rows)
    compressed=gzip.compress(raw,mtime=0);raw_path.with_suffix('.bin.gz').write_bytes(compressed)
    raw_path.unlink()
    summary=dict(complete_phases=len(rows),object_input_calls=len(rows)*10,identical_frames=spec['frames'],identical_screenshots=len(images),trace_sha256=hashlib.sha256(raw).hexdigest(),compressed_trace_sha256=hashlib.sha256(compressed).hexdigest(),source=json.loads((observed/'source.json').read_text()))
    out=ROOT/'.local/basketball/rea/world-inputs';out.mkdir(parents=True,exist_ok=True);(out/'capture.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
