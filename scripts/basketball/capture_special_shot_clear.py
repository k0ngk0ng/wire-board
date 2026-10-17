"""Read-only SpecialShot write provenance over the retained controlled-origin match."""
import hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local writes=assert(io.open(out..'/special-shot-writes.jsonl','w'))
local before={};for i=0,3 do before[i]=memory.readbyte(0x554+i) end
for actor=0,3 do
 local i=actor
 memory.registerwrite(0x554+i,1,function(a,z,v)
  writes:write(string.format('{"frame":%d,"pc":%d,"bank":%d,"actor":%d,"before":%d,"after":%d,"x":%d,"action":%d,"holder":%d,"ballAction":%d}\n',emu.framecount(),memory.getregister('pc'),memory.readbyte(0xbfff),i,before[i],v,memory.getregister('x'),memory.readbyte(0x512+i),memory.readbyte(0x53f),memory.readbyte(0x518)))
  before[i]=v
 end)
end
local original_finish=finish_probe
function finish_probe()original_finish();writes:close()end
'''
def main():
 out=ROOT/'.local/basketball/rea/special-shot-clear'
 reference=BASE/'runs/ball-rest-team5-aligned';spec=json.loads((reference/'scenario.json').read_text());spec.update(name='special-shot-clear-team5',hooks_lua=spec['hooks_lua']+'\n'+HOOK)
 path=out/'spec.json';path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for n in spec['captures']:assert (observed/f'frame-{n:05d}.png').read_bytes()==(reference/f'frame-{n:05d}.png').read_bytes()
 worlds=[json.loads(line) for line in (observed/'world-after.jsonl').read_text().splitlines()]
 low,high=worlds[1012]['frame'],worlds[1013]['frame']
 writes=[json.loads(line) for line in (observed/'special-shot-writes.jsonl').read_text().splitlines()]
 selected=[r for r in writes if low<=r['frame']<=high and r['actor']==1]
 result=dict(scope='read-only writes in prior controlled first-world origin match; not natural menu path',ramAndScreenshotsEqual=True,priorWorldFrame=low,firstDifferentWorldFrame=high,writes=selected,allWriteCount=len(writes),traceSHA256=hashlib.sha256((observed/'special-shot-writes.jsonl').read_bytes()).hexdigest())
 (out/'provenance.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
