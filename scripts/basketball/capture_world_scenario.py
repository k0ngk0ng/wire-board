"""Retain all core phase boundaries for an existing unmodified input scenario."""
import argparse
import csv
import hashlib
import json
import re
from capture_reference import BASE, run

HOOK = r'''
local phases={}
local function snapshot()
 local parts={};for i=0,2047 do parts[#parts+1]=string.char(memory.readbyte(i)) end
 return table.concat(parts)
end
for _,name in ipairs({'world-preparation','world-inputs','world-actions','projection'}) do
 phases[name]={raw=assert(io.open(out..'/'..name..'-ram.bin','wb')),csv=assert(io.open(out..'/'..name..'.csv','w'))}
 phases[name].csv:write('frame\n')
end
local function begin(name)
 local p=phases[name];assert(not p.frame);p.frame=emu.framecount();p.before=snapshot()
end
local function finish(name)
 local p=phases[name];assert(p.frame);p.csv:write(p.frame..'\n');p.raw:write(p.before);p.raw:write(snapshot());p.frame=nil;p.before=nil
end
memory.registerexec(0xd317,1,function() begin('world-preparation') end)
memory.registerexec(0xd326,1,function() finish('world-preparation') end)
memory.registerexec(0xd32c,1,function() begin('world-inputs') end)
memory.registerexec(0xd345,1,function() finish('world-inputs');begin('world-actions') end)
memory.registerexec(0xd348,1,function() finish('world-actions');begin('projection') end)
memory.registerexec(0xd352,1,function() finish('projection') end)
function finish_probe()
 local tails={}
 for name,p in pairs(phases) do
  if p.frame then
   local f=assert(io.open(out..'/'..name..'-incomplete-ram.bin','wb'));f:write(p.before);f:close()
   tails[#tails+1]=string.format('"%s":%d',name,p.frame)
  end
  p.csv:close();p.raw:close()
 end
 local f=assert(io.open(out..'/incomplete-phases.json','w'));f:write('{'..table.concat(tails,',')..'}');f:close()
end
'''


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--reference-run',required=True)
    parser.add_argument('--name',required=True)
    args=parser.parse_args()
    for value in (args.reference_run,args.name):
        if not re.fullmatch('[a-z][a-z0-9-]*',value):
            raise ValueError('run names must be directory basenames')
    if args.reference_run==args.name:raise ValueError('capture must not overwrite reference')
    reference=BASE/'runs'/args.reference_run
    spec=json.loads((reference/'scenario.json').read_text())
    spec.pop('save_slot',None)
    spec.update(name=args.name,hooks_lua=HOOK)
    path=BASE/(args.name+'.json')
    path.write_text(json.dumps(spec,indent=2)+'\n')
    run(path)
    observed=BASE/'runs'/args.name
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    images=list(reference.glob('frame-*.png'))
    for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
    counts={}
    for name in ('world-preparation','world-inputs','world-actions','projection'):
        rows=list(csv.DictReader((observed/(name+'.csv')).open()))
        raw=(observed/(name+'-ram.bin')).read_bytes()
        assert len(raw)==len(rows)*4096
        counts[name]=dict(records=len(rows),sha256=hashlib.sha256(raw).hexdigest())
    report=dict(reference=args.reference_run,ram_identical=True,screenshots_identical=len(images),phases=counts,incomplete_at_capture_end=json.loads((observed/'incomplete-phases.json').read_text()))
    (observed/'world-capture-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report))


if __name__=='__main__':main()
