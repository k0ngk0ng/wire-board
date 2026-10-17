"""Passive original Court7 D257..D303 opening capture, identical full reference."""
import gzip,hashlib,json
from capture_reference import BASE,ROOT,run
HOOK=r'''
local f=assert(io.open(out..'/court-seven-opening.ram','wb'));local pending=nil;local count=0
local function snap()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i))end;return table.concat(t)end
memory.registerexec(0xd257,1,function()assert(not pending);pending=snap()end)
memory.registerexec(0xd303,1,function()if pending then f:write(pending);f:write(snap());pending=nil;count=count+1 end end)
function finish_probe()assert(not pending);assert(count==1);f:close()end
'''
def main():
 ref=BASE/'runs/court-7-reference';spec=json.loads((ref/'scenario.json').read_text());spec.update(name='court-seven-opening',hooks_lua=spec.get('hooks_lua','')+'\n'+HOOK)
 p=BASE/'court-seven-opening.json';p.write_text(json.dumps(spec,indent=2)+'\n');run(p)
 out=BASE/'runs'/spec['name'];assert(out/'ram.bin').read_bytes()==(ref/'ram.bin').read_bytes()
 for image in ref.glob('frame-*.png'):assert image.read_bytes()==(out/image.name).read_bytes()
 raw=(out/'court-seven-opening.ram').read_bytes();assert len(raw)==4096 and raw[0x58b]==7 and raw[2048+0x58b]==7
 fixture=ROOT/'games/basketball/engine/testdata/court-seven-opening.ram.gz';fixture.write_bytes(gzip.compress(raw,mtime=0))
 meta=dict(source_is_original_execution=True,scope='passive D257..D303 after once-configured Court7; unchanged controller inputs, no natural-menu claim',records=1,fixtureSHA256=hashlib.sha256(fixture.read_bytes()).hexdigest(),beforeAuxKinds=list(raw[0x3da:0x3dc]),afterAuxKinds=list(raw[2048+0x3da:2048+0x3dc]),ramAndScreenshotsMatchUninstrumented=True,referenceRAMSHA256=hashlib.sha256((ref/'ram.bin').read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
 fixture.with_name('court-seven-opening-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
