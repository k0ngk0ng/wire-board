"""Observe one original E165/E168 call, retaining repeated actor calls separately."""
import hashlib,json
from capture_reference import BASE,ROOT,run
from capture_free_jump import prefix,hook as world_hook

eligibility=world_hook[world_hook.index('local function eligible'):world_hook.index('memory.registerexec')]
hook=prefix.replace('free-jump.jsonl','free-action.jsonl').replace('local pending={}','local pending=nil;local actor=nil')+eligibility+r'''
memory.registerexec(0xe165,1,function()
 local i=memory.getregister('x')
 if i>3 or not eligible(i) then return end
 assert(not pending);actor=i;pending=string.format('{"frame":%d,"actor":%d,"before":%s,',emu.framecount(),i,state(i))
end)
memory.registerexec(0xe168,1,function()
 if not pending then return end
 f:write(pending..'"after":'..state(actor)..'}\n');pending=nil
end)
function finish_probe() assert(not pending);f:close() end
'''

def main():
 reference=BASE/'runs/hoop-control-cpu-long'
 spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='free-action-cpu-long',hooks_lua=hook)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 records=[json.loads(line) for line in (observed/'free-action.jsonl').read_text().splitlines()]
 fixture=ROOT/'games/basketball/engine/testdata/free-action.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
 fixture.with_name('free-action-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='fixed:e165',exit='fixed:e168',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n');print('free-action',len(records))

if __name__=='__main__':main()
