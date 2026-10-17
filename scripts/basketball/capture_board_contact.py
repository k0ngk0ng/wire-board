"""Capture every original board/rim object call, in descending world order."""
import csv,gzip,hashlib,json,struct
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/board-contact.csv','w'))
local raw=assert(io.open(out..'/board-contact-ram.bin','wb'));local pending=nil
f:write('frame,object\n')
local function snapshot()
 local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end
 raw:write(table.concat(t))
end
memory.registerexec(0x8e33,1,function()
 if memory.readbyte(0xbfff)~=4 then return end
 assert(not pending);pending={emu.framecount(),memory.getregister('x')};snapshot()
end)
memory.registerexec(0xfce9,1,function()
 assert(pending);f:write(pending[1]..','..pending[2]..'\n');snapshot();pending=nil
end)
function finish_probe()
 assert(not pending);f:close();raw:close()
end
'''
def main():
 ref=BASE/'runs/hoop-control-cpu-long'
 spec=json.loads((ref/'scenario.json').read_text());spec.pop('save_slot',None)
 spec.update(name='board-contact-cpu-long',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 out=BASE/'runs'/spec['name'];raw=(out/'board-contact-ram.bin').read_bytes()
 assert (out/'ram.bin').read_bytes()==(ref/'ram.bin').read_bytes()
 for p in ref.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
 rows=list(csv.DictReader((out/'board-contact.csv').open()));assert len(raw)==4096*len(rows)
 fixture=ROOT/'games/basketball/engine/testdata/board-contact.ram.gz'
 with fixture.open('wb') as f:
  with gzip.GzipFile(fileobj=f,mode='wb',mtime=0) as z:
   for i,row in enumerate(rows):z.write(struct.pack('<IB',int(row['frame']),int(row['object']))+raw[4096*i:4096*(i+1)])
 result=dict(records=len(rows),objects={str(i):sum(int(r['object'])==i for r in rows) for i in range(13)},source_is_original_execution=True,entry='bank4:8e33',exit='fixed:fce9',record_format='uint32le frame, uint8 object, 2048-byte before RAM, 2048-byte after RAM',ram_matches_uninstrumented=True,trace_sha256=hashlib.sha256(raw).hexdigest(),fixture_sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
 fixture.with_name('board-contact-source.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
