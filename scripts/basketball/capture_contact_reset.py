"""Record the original pre-contact reset, including links and arbitration state."""
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out..'/contact-reset.jsonl','w'));local pending=nil
local function array(a)
 local t={};for i=0,6 do t[#t+1]=tostring(memory.readbyte(a+i)) end;return '['..table.concat(t,',')..']'
end
local function state()
 return string.format('{"rotation":%d,"marks":%s,"candidates":%s,"links":%s,"startingBallAction":%d,"parity":%d}',memory.readbyte(0x633),array(0x634),array(0x3c),array(0x642),memory.readbyte(0x665),memory.readbyte(0x6a7))
end
memory.registerexec(0x8469,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending);pending=string.format('{"frame":%d,"ball":%d,"before":%s,',emu.framecount(),memory.readbyte(0x518),state())
end)
memory.registerexec(0x8498,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending);f:write(pending..'"after":'..state()..'}\n');pending=nil
end)
function finish_probe() assert(not pending);f:close() end
'''


def main():
    reference=BASE/'runs/shot-observation'
    spec=json.loads((reference/'scenario.json').read_text());spec.update(name='contact-reset-shot-observation',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
    rows=[json.loads(line) for line in (observed/'contact-reset.jsonl').read_text().splitlines()]
    assert rows
    p=ROOT/'games/basketball/engine/testdata/contact-reset.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    p.with_name('contact-reset-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank5:8469',exit='bank5:8498',records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')
    print(json.dumps(dict(records=len(rows))))


if __name__=='__main__':main()
