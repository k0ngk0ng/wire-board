"""Observe original character setup and on-court referee opening."""
import gzip,hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/opening.jsonl','w'));local raw=assert(io.open(out..'/opening.ram','wb'));local pending=nil
local function snapshot()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;return table.concat(t)end
local function begin(kind)assert(not pending);pending={kind=kind,frame=emu.framecount(),before=snapshot(),ticks=0,sounds={}}end
local function finish(kind)
 if not pending or pending.kind~=kind then return end
 raw:write(pending.before);raw:write(snapshot());f:write(string.format('{"kind":"%s","frame":%d,"ticks":%d,"sounds":[%s]}\n',kind,pending.frame,pending.ticks,table.concat(pending.sounds,',')));pending=nil
end
memory.registerexec(0xacbb,1,function()if memory.readbyte(0xbfff)==3 then begin('grades')end end)
memory.registerexec(0xad13,1,function()if memory.readbyte(0xbfff)==3 then finish('grades')end end)
memory.registerexec(0xd1b5,1,function()if memory.readbyte(0x589)<128 then begin('court')end end)
memory.registerexec(0xd230,1,function()finish('court')end)
memory.registerexec(0xd231,1,function()begin('start')end)
memory.registerexec(0xd248,1,function()finish('start')end)
memory.registerexec(0xd527,1,function()begin('throw')end)
memory.registerexec(0xd5c2,1,function()finish('throw')end)
memory.registerexec(0xd257,1,function()begin('period')end)
memory.registerexec(0xd303,1,function()finish('period')end)
memory.registerexec(0xd40b,1,function()begin('team')end)
memory.registerexec(0xd41d,1,function()finish('team')end)
memory.registerexec(0xf25c,1,function()if pending and memory.readbyte(0x589)~=2 then pending.ticks=pending.ticks+1 end end)
memory.registerexec(0xfbcc,1,function()if pending then pending.sounds[#pending.sounds+1]=memory.getregister('a') end end)
function finish_probe()assert(not pending);f:close();raw:close()end
'''
def main():
 reference=BASE/'runs/opening-from-menu'
 expected=dict(name='opening-from-menu',load_slot=2,frames=1550,segments=[dict(start=30,end=31,keys=['right']),dict(start=60,end=61,keys=['A']),dict(start=180,end=181,keys=['start']),dict(start=450,end=451,keys=['A'])],captures=[1260,1305,1400,1420,1550])
 if not (reference/'scenario.json').exists():
  baseline=BASE/'opening-from-menu.json';baseline.write_text(json.dumps(expected,indent=2)+'\n');run(baseline)
 spec=json.loads((reference/'scenario.json').read_text());assert spec==expected,'opening reference input timeline differs';spec.update(name='opening-observation',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 out=BASE/'runs'/spec['name'];assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for p in reference.glob('frame-*.png'):assert p.read_bytes()==(out/p.name).read_bytes()
 rows=[json.loads(l) for l in (out/'opening.jsonl').read_text().splitlines()];raw=(out/'opening.ram').read_bytes();assert len(raw)==len(rows)*4096
 fixture=ROOT/'games/basketball/engine/testdata/opening.ram.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));fixture.with_name('opening.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
 metadata=dict(records=len(rows),kinds={k:sum(r['kind']==k for r in rows) for k in sorted({r['kind'] for r in rows})},source_is_original_execution=True,initial_state_sha256=hashlib.sha256((BASE/'states/slot-2.fcs').read_bytes()).hexdigest(),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),ram_matches_uninstrumented=True,source=json.loads((out/'source.json').read_text()))
 fixture.with_name('opening-source.json').write_text(json.dumps(metadata,indent=2)+'\n');print(json.dumps(metadata))
if __name__=='__main__':main()
