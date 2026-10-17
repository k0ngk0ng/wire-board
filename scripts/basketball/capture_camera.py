"""Observe camera, crowding and all-object projection without state injection."""
import csv
import hashlib
import json
from capture_reference import BASE, ROOT, run

HOOK = r'''
local captures={}
local function snapshot()
 local result={};for i=0,2047 do result[#result+1]=string.char(memory.readbyte(i)) end
 return table.concat(result)
end
for _,name in ipairs({"camera","crowding","projection"}) do
 captures[name]={raw=assert(io.open(out.."/"..name.."-ram.bin","wb")),
                csv=assert(io.open(out.."/"..name..".csv","w"))}
 captures[name].csv:write("frame,objects\n")
end
local function begin(name)
 local item=captures[name];assert(not item.frame);item.frame=emu.framecount();item.objects={};item.raw:write(snapshot())
end
local function finish(name)
 local item=captures[name];assert(item.frame);item.csv:write(item.frame..","..table.concat(item.objects,";").."\n")
 item.raw:write(snapshot());item.frame=nil
end
memory.registerexec(0xd314,1,function() begin("camera") end)
memory.registerexec(0xd317,1,function() finish("camera") end)
memory.registerexec(0xbf42,1,function() if memory.readbyte(0xbfff)==3 then begin("crowding") end end)
memory.registerexec(0xd32c,1,function() if captures.crowding.frame then finish("crowding") end end)
memory.registerexec(0xd348,1,function() begin("projection") end)
memory.registerexec(0xcd3c,1,function()
 if captures.projection.frame then table.insert(captures.projection.objects,memory.getregister("x")) end
end)
memory.registerexec(0xd352,1,function() finish("projection") end)
function finish_probe()
 for _,item in pairs(captures) do assert(not item.frame);item.csv:close();item.raw:close() end
end
'''


def main():
    reference = BASE/'runs/hoop-control-cpu-long'
    spec = json.loads((reference/'scenario.json').read_text())
    spec.update(name='camera-cpu-long', hooks_lua=HOOK)
    spec.pop('save_slot', None)
    path = BASE/(spec['name']+'.json')
    path.write_text(json.dumps(spec, indent=2)+'\n')
    run(path)
    observed = BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes() == (reference/'ram.bin').read_bytes()
    images = list(reference.glob('frame-*.png'))
    for image in images:
        assert image.read_bytes() == (observed/image.name).read_bytes()
    summary = dict(identical_reference_frames=spec['frames'], identical_screenshots=len(images),
                   source=json.loads((observed/'source.json').read_text()))
    for name in ['camera','crowding','projection']:
        rows = list(csv.DictReader((observed/f'{name}.csv').open()))
        raw = (observed/f'{name}-ram.bin').read_bytes()
        assert rows and len(raw) == len(rows)*4096
        if name == 'projection':
            assert all(row['objects'] == ';'.join(map(str,range(13))) for row in rows)
        summary[name] = dict(records=len(rows), sha256=hashlib.sha256(raw).hexdigest())
    dest = ROOT/'.local/basketball/rea/camera/capture.json'
    dest.write_text(json.dumps(summary, indent=2)+'\n')
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
