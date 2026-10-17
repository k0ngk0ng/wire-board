"""Capture complete natural result calls including all four celebration actors."""
import gzip,hashlib,json,struct
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/match-transition.jsonl','w'))
local raw=assert(io.open(out..'/match-transition.ram','wb'));local pending=nil
local function snapshot()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;return table.concat(t)end
local function begin(kind)assert(not pending);pending={kind=kind,frame=emu.framecount(),before=snapshot(),ticks=0,sounds={}}end
local function finish(kind)
 if not pending or pending.kind~=kind then return end
 raw:write(pending.before);raw:write(snapshot())
 f:write(string.format('{"kind":"%s","frame":%d,"ticks":%d,"sounds":[%s]}\n',kind,pending.frame,pending.ticks,table.concat(pending.sounds,',')));pending=nil
end
memory.registerexec(0xba40,1,function()if memory.readbyte(0xbfff)==4 then begin('result')end end)
memory.registerexec(0xbca0,1,function()if memory.readbyte(0xbfff)==4 then finish('result')end end)
memory.registerexec(0xf25c,1,function()if pending and memory.readbyte(0x589)~=2 then pending.ticks=pending.ticks+1 end end)
memory.registerexec(0xfbcc,1,function()if pending then pending.sounds[#pending.sounds+1]=memory.getregister('a') end end)
function finish_probe()assert(not pending);f:close();raw:close()end
'''
def main():
 reference=BASE/'runs/match-clock-complete'
 spec=json.loads((reference/'scenario.json').read_text());spec.update(name='match-result',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 out=BASE/'runs'/spec['name'];assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for p in reference.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
 rows=[json.loads(l) for l in (out/'match-transition.jsonl').read_text().splitlines()];raw=(out/'match-transition.ram').read_bytes();assert len(raw)==len(rows)*4096
 fixture=ROOT/'games/basketball/engine/testdata/match-result.ram.gz';fixture.write_bytes(gzip.compress(raw,mtime=0))
 fixture.with_name('match-result.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
 metadata=dict(records=len(rows),kinds={k:sum(r['kind']==k for r in rows) for k in sorted({r['kind'] for r in rows})},source_is_original_execution=True,sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),ram_matches_uninstrumented=True,source=json.loads((out/'source.json').read_text()))
 fixture.with_name('match-result-source.json').write_text(json.dumps(metadata,indent=2)+'\n');print(json.dumps(metadata))
if __name__=='__main__':main()
