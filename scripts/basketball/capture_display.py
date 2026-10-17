"""Read original semantic display boundaries and original glyph conversion calls."""
import gzip,hashlib,json
from capture_reference import ROOT,BASE,run
from capture_renderer import EXTRA
HOOK=r'''
local f=assert(io.open(out..'/display.jsonl','w'));local raw=assert(io.open(out..'/display.ram','wb'))
local function array(a,n)local t={};for i=0,n-1 do t[#t+1]=memory.readbyte(a+i) end;return '['..table.concat(t,',')..']' end
local function emit(kind,fields)f:write('{"kind":"'..kind..'","frame":'..emu.framecount()..','..fields..'}\n')end
local function snapshot()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t))end
local chr_pending=nil;local name_pending=nil;local tactic_pending=nil
memory.registerexec(0xce09,1,function()
 if memory.readbyte(0x588)~=1 then return end
 snapshot();emit('draw','"state":'..semantic_inputs()..',"surfaces":'..array(0x5ea,13)..',"flags":'..memory.readbyte(0x6d6)..',"detached":'..memory.readbyte(0xe4))
end)
memory.registerexec(0xf049,1,function()chr_pending=string.format('"before":%d,"pose":%d,"phase":%d',memory.readbyte(0x54),memory.readbyte(0x61),memory.readbyte(0x589))end)
memory.registerexec(0xf060,1,function()if chr_pending then emit('chr',chr_pending..',"after":'..memory.readbyte(0x54));chr_pending=nil end end)
memory.registerexec(0xb227,1,function()if memory.readbyte(0xbfff)==4 then name_pending=string.format('"team":%d,"character":%d',memory.readbyte(0x23),memory.readbyte(0x24)) end end)
memory.registerexec(0xb24d,1,function()if memory.readbyte(0xbfff)==4 and name_pending then emit('name',name_pending..',"upper":'..array(0x2c,4)..',"lower":'..array(0x3a,4));name_pending=nil end end)
memory.registerexec(0xb31e,1,function()if memory.readbyte(0xbfff)==4 then tactic_pending='"tactic":'..memory.readbyte(0x6c6+memory.getregister('y')) end end)
memory.registerexec(0xb33e,1,function()if memory.readbyte(0xbfff)==4 and tactic_pending then emit('tactic',tactic_pending..',"upper":'..array(0x2c,6)..',"lower":'..array(0x3a,6));tactic_pending=nil end end)
function finish_probe()f:close();raw:close()end
'''
def main():
 allrows=[];allraw=bytearray();sources=[]
 for reference_name in ['opening-from-menu','hoop-control-cpu-long']:
  reference=BASE/'runs'/reference_name;spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
  spec.update(name='display-'+reference_name,hooks_lua=EXTRA+'\n'+HOOK)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
  out=BASE/'runs'/spec['name'];assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for im in reference.glob('frame-*.png'):assert im.read_bytes()==(out/im.name).read_bytes()
  rows=[dict(scenario=reference_name,**json.loads(line)) for line in (out/'display.jsonl').read_text().splitlines()];data=(out/'display.ram').read_bytes();assert len(data)==sum(r['kind']=='draw' for r in rows)*2048
  allrows.extend(rows);allraw.extend(data);sources.append(json.loads((out/'source.json').read_text()))
 path=ROOT/'games/basketball/engine/testdata/display-stages.json.gz';path.write_bytes(gzip.compress(json.dumps(allrows,separators=(',',':')).encode(),mtime=0));path.with_name('display-stages.ram.gz').write_bytes(gzip.compress(allraw,mtime=0));counts={k:sum(r['kind']==k for r in allrows) for k in sorted({r['kind'] for r in allrows})}
 path.with_name('display-source.json').write_text(json.dumps({'kinds':counts,'source_is_original_execution':True,'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'ram_sha256':hashlib.sha256(allraw).hexdigest(),'ram_and_images_match':True,'sources':sources},indent=2)+'\n');print(counts)
if __name__=='__main__':main()
