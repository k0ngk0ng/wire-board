"""Observe natural bank-4 8249 floor resolution, before XY commit."""
import gzip,hashlib,json,struct
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/court-floor.jsonl','w'))
local raw=assert(io.open(out..'/court-floor.ram','wb'));local pending=nil
local function snapshot()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i))end;return table.concat(t)end
memory.registerexec(0x8249,1,function()
 if memory.readbyte(0xbfff)~=4 then return end
 assert(not pending);pending={frame=emu.framecount(),object=memory.getregister('x'),before=snapshot(),sounds={}}
end)
local function finish()
 if not pending or memory.readbyte(0xbfff)~=4 then return end
 raw:write(pending.before);raw:write(snapshot())
 f:write(string.format('{"frame":%d,"object":%d,"sounds":[%s]}\n',pending.frame,pending.object,table.concat(pending.sounds,',')));pending=nil
end
for _,a in ipairs({0x82e1,0x831d,0x839d,0x83cb,0x840a})do memory.registerexec(a,1,finish)end
memory.registerexec(0xfbcc,1,function()if pending then pending.sounds[#pending.sounds+1]=memory.getregister('a')end end)
function finish_probe()assert(not pending);f:close();raw:close()end
'''
def main():
 ref=BASE/'runs/hoop-control-cpu-long';spec=json.loads((ref/'scenario.json').read_text());spec.pop('save_slot',None)
 spec.update(name='court-floor-natural',hooks_lua=HOOK);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 out=BASE/'runs'/spec['name'];assert(out/'ram.bin').read_bytes()==(ref/'ram.bin').read_bytes()
 for p in ref.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
 rows=[json.loads(l) for l in(out/'court-floor.jsonl').read_text().splitlines()];raw=(out/'court-floor.ram').read_bytes();assert len(raw)==4096*len(rows)
 fixture=ROOT/'games/basketball/engine/testdata/court-floor-natural.ram.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));fixture.with_name('court-floor-natural.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
 meta=dict(records=len(rows),objects={str(i):sum(r['object']==i for r in rows)for i in range(13)},source_is_original_execution=True,natural_gameplay=True,ram_matches_uninstrumented=True,fixture_sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),source=json.loads((out/'source.json').read_text()))
 fixture.with_name('court-floor-natural-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
