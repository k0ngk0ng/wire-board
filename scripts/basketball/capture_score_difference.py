"""Retain original $9F20 score-difference phase without changing its inputs."""
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out..'/score-difference.jsonl','w'));local pending=nil
memory.registerexec(0x9f20,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending)
 pending=string.format('{"frame":%d,"scores":[%d,%d],',emu.framecount(),memory.readbyte(0x166)+256*memory.readbyte(0x168),memory.readbyte(0x167)+256*memory.readbyte(0x169))
end)
memory.registerexec(0x9f7d,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(pending);f:write(pending..'"difference":'..memory.readbyte(0x696)..'}\n');pending=nil
end)
function finish_probe() assert(not pending);f:close() end
'''


def main():
    reference=BASE/'runs/shot-observation'
    spec=json.loads((reference/'scenario.json').read_text());spec.update(name='score-difference-shot-observation',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name']
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(out/image.name).read_bytes()
    rows=[json.loads(line) for line in (out/'score-difference.jsonl').read_text().splitlines()]
    assert rows
    p=ROOT/'games/basketball/engine/testdata/score-difference.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    p.with_name('score-difference-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank5:9f20',exit='bank5:9f7d',records=len(rows),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text())),indent=2)+'\n')
    print(json.dumps(dict(records=len(rows))))


if __name__=='__main__': main()
