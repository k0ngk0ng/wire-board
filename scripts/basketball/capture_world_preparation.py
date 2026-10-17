"""Capture the complete contact/team/board preparation block before inputs."""
import csv
import gzip
import hashlib
import json
import struct
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out..'/world-preparation.csv','w'))
local raw=assert(io.open(out..'/world-preparation-ram.bin','wb'));local pending=nil;local sounds={}
f:write('frame,sounds\n')
local function snapshot()
 local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t))
end
memory.registerexec(0xd317,1,function()
 assert(not pending);pending=emu.framecount();sounds={};snapshot()
end)
memory.registerexec(0xc000,1,function()
 if pending then sounds[#sounds+1]=memory.getregister('a') end
end)
memory.registerexec(0xd326,1,function()
 assert(pending);f:write(tostring(pending)..','..table.concat(sounds,';')..'\n');snapshot();pending=nil
end)
function finish_probe() assert(not pending);f:close();raw:close() end
'''


def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
    spec.update(name='world-preparation-cpu-long',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name']
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for image in reference.glob('frame-*.png'):assert image.read_bytes()==(out/image.name).read_bytes()
    with (out/'world-preparation.csv').open() as f:rows=list(csv.DictReader(f))
    raw=(out/'world-preparation-ram.bin').read_bytes();assert len(raw)==4096*len(rows)
    target=ROOT/'games/basketball/engine/testdata/world-preparation.ram.gz'
    with target.open('wb') as f:
        with gzip.GzipFile(fileobj=f,mode='wb',mtime=0) as z:
            for i,row in enumerate(rows):
                sounds=[int(s) for s in row['sounds'].split(';') if s]
                assert len(sounds)<256
                z.write(struct.pack('<IB',int(row['frame']),len(sounds))+bytes(sounds)+raw[i*4096:(i+1)*4096])
    result=dict(source_is_original_execution=True,entry='fixed:d317',exit='fixed:d326',records=len(rows),ram_matches_uninstrumented=True,record_format='uint32le frame, uint8 sound count, sound IDs, 2048-byte before RAM, 2048-byte after RAM',trace_sha256=hashlib.sha256(raw).hexdigest(),fixture_sha256=hashlib.sha256(target.read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
    target.with_name('world-preparation-source.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))


if __name__=='__main__':main()
