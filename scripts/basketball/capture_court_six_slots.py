"""Read-only original Court6 goal/slot boundaries after the declared court choice.
Keeps uninstrumented full RAM/screenshots identity; no forced score/reel/RNG.
"""
import gzip,hashlib,json
from capture_reference import BASE,ROOT,run
from capture_court_animation import HOOK,state,patches,scores

def main():
 reference=BASE/'runs/court-six-slot-complete-reference'
 spec=json.loads((reference/'scenario.json').read_text())
 hook='local pendingGoal=nil\n'+HOOK.replace('function finish_probe()','local function finish_animation_probe()')
 hook=hook.replace("memory.registerexec(0xc000,1,function()if pending then pending.sounds[#pending.sounds+1]=tostring(memory.getregister('a')) end end)","memory.registerexec(0xc000,1,function() local v=tostring(memory.getregister('a'));if pending then pending.sounds[#pending.sounds+1]=v end;if pendingGoal then pendingGoal.sounds[#pendingGoal.sounds+1]=v end end)")
 hook=hook.replace('pending={frame=emu.framecount(),before=ram(),queues={},sounds={},palette=false}', 'pending={frame=emu.framecount(),before=ram(),queues={},headers={},sounds={},palette=false}')
 hook=hook.replace('if pending then pending.queues[#pending.queues+1]=tostring(a) end', 'if pending and value~=255 then pending.queues[#pending.queues+1]=tostring(a);pending.headers[a]=value end')
 hook=hook.replace('"sounds":[%s]}', '"sounds":[%s],"headers":[%d,%d,%d]}')
 hook=hook.replace("table.concat(pending.sounds,',')))", "table.concat(pending.sounds,','),pending.headers[0x30b] or 0,pending.headers[0x32e] or 0,pending.headers[0x351] or 0))")
 hook+=r'''
local goalraw=assert(io.open(out..'/slot-goals.ram','wb'))
local goaljson=assert(io.open(out..'/slot-goals.jsonl','w'))
memory.registerexec(0x8e33,1,function()
 if memory.readbyte(0xbfff)==4 and memory.getregister('x')==6 and memory.readbyte(0x58b)==6 then
  assert(not pendingGoal);pendingGoal={before=ram(),frame=emu.framecount(),sounds={}}
 end
end)
memory.registerexec(0xfce9,1,function()
 if not pendingGoal then return end
 local after=ram()
 if pendingGoal.before:sub(0x166+1,0x16a)~=after:sub(0x166+1,0x16a) or pendingGoal.before:sub(0x6dd+1,0x6df)~=after:sub(0x6dd+1,0x6df) then
  goalraw:write(pendingGoal.before);goalraw:write(after)
  goaljson:write(string.format('{"frame":%d,"sounds":[%s]}\n',pendingGoal.frame,table.concat(pendingGoal.sounds,',')))
 end
 pendingGoal=nil
end)
function finish_probe()
 finish_animation_probe();assert(not pendingGoal);goalraw:close();goaljson:close()
end
'''
 spec.update(name='court-six-slot-observed-v2',hooks_lua=spec.get('hooks_lua','')+'\n'+hook)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 dest=BASE/'runs'/spec['name'];assert (dest/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for p in reference.glob('frame-*.png'):assert p.read_bytes()==(dest/p.name).read_bytes()
 raw=(dest/'court-animation-ram.bin').read_bytes();events=[json.loads(v) for v in (dest/'court-animation.jsonl').read_text().splitlines()];assert len(raw)==len(events)*4096
 rows=[];cleared=0
 for i,e in enumerate(events):
  b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096]
  if b[0x58b]!=6:continue
  produced=bytearray(a)
  for address,header in zip((0x30b,0x32e,0x351),e['headers']):
   if address in e['queueWrites']:
    assert 0x20<=header<0x30
    if produced[address]==255:produced[address]=header;cleared+=1
    else:assert produced[address]==header
  rows.append(dict(frame=e['frame'],before=state(b,6),context=dict(court=6,active=(b[0x589]&127) not in(0,2),videoFrame=b[0x300],cameraX=b[0x397],random=int.from_bytes(b[0x48:0x4c],'big'),scores=scores(b),hoopOwners=list(b[0x540:0x546]),hoopLocks=list(b[0x62d:0x633])),after=dict(state=state(a,6),patches=patches(produced,e['queueWrites']),palette=e['palette'],paletteID=a[0x304] if e['palette'] else 0,sounds=e['sounds'],random=int.from_bytes(a[0x48:0x4c],'big'),scores=scores(a))))
 out=ROOT/'games/basketball/engine/testdata';p=out/'court-six-slot-observed.json.gz';packed=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);p.write_bytes(packed)
 goals=(dest/'slot-goals.ram').read_bytes();metadata=[json.loads(v) for v in (dest/'slot-goals.jsonl').read_text().splitlines()];assert len(goals)==len(metadata)*4096
 gp=out/'court-six-slot-goals.ram.gz';gp.write_bytes(gzip.compress(goals,mtime=0));(out/'court-six-slot-goals.json').write_text(json.dumps(metadata,separators=(',',':'))+'\n')
 info=dict(source_is_original_execution=True,scope='one configured court choice at original D1B5; thereafter controller inputs only, passive hooks; no natural-menu or full-Go-gameplay claim',records=len(rows),nmiClearedHeadersRestoredFromObservedWrites=cleared,goals=len(metadata),randomChanged=sum(r['context']['random']!=r['after']['random'] for r in rows),scoreChanged=sum(r['context']['scores']!=r['after']['scores'] for r in rows),romSHA256=json.loads((dest/'source.json').read_text())['rom_sha256'],fixtureSHA256=hashlib.sha256(packed).hexdigest(),goalSHA256=hashlib.sha256(gp.read_bytes()).hexdigest(),ramAndScreenshotsMatchUninstrumented=True,referenceRAMSHA256=hashlib.sha256((reference/'ram.bin').read_bytes()).hexdigest(),source=json.loads((dest/'source.json').read_text()))
 (out/'court-six-slot-observed-source.json').write_text(json.dumps(info,indent=2)+'\n');print(json.dumps(info))
if __name__=='__main__':main()
