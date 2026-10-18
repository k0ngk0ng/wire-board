"""Observe parameter stores and B7CD in the existing unchanged reference run."""
import collections,hashlib,json
from capture_reference import ROOT,BASE,run
HOOK=r'''
local f=assert(io.open(out..'/parameter-writes.jsonl','w'))
local calls=assert(io.open(out..'/evasion-dispatches.jsonl','w'))
memory.registerwrite(0x682,4,function(a,z,v)
 local actor=a-0x682
 f:write(string.format('{"frame":%d,"pcAfter":%d,"bank":%d,"x":%d,"y":%d,"actor":%d,"value":%d,"mode":%d}\n',emu.framecount(),memory.getregister('pc'),memory.readbyte(0xbfff),memory.getregister('x'),memory.getregister('y'),actor,v or -1,memory.readbyte(0x672+actor)))
end)
memory.registerexec(0xb7cd,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 local actor=memory.getregister('x');calls:write(string.format('{"frame":%d,"actor":%d,"mode":%d,"parameter":%d}\n',emu.framecount(),actor,memory.readbyte(0x672+actor),memory.readbyte(0x682+actor)))
end)
function finish_probe()f:close();calls:close()end
'''
def main():
 out=ROOT/'.local/basketball/rea/evasion-direct';reference=BASE/'runs/special-ball-cpu-command';spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='evasion-setter-audit',hooks_lua=HOOK);path=out/'capture-spec.json';path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes();images=list(reference.glob('frame-*.png'))
 for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
 writes=[json.loads(x) for x in (observed/'parameter-writes.jsonl').read_text().splitlines()];calls=[json.loads(x) for x in (observed/'evasion-dispatches.jsonl').read_text().splitlines()]
 counts=collections.Counter((r['bank'],r['pcAfter'],r['x'],r['mode'],r['value']) for r in writes)
 result=dict(scope='bounded existing original reference, no gameplay injection; not all-roster reachability proof',referenceRAMAndScreenshotsMatch=True,frames=spec['frames'],screenshots=len(images),writes=len(writes),writeGroups=[dict(bank=k[0],pcAfter=hex(k[1]),x=k[2],mode=k[3],value=k[4],count=n) for k,n in sorted(counts.items())],evasionCalls=calls,remainingPlanCalls=[r for r in calls if r['parameter'] in [6,7,8,9,10,12,13]],traceSHA256=hashlib.sha256((observed/'parameter-writes.jsonl').read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text()));(out/'caller-capture.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({k:v for k,v in result.items() if k not in ['writeGroups','evasionCalls']}))
if __name__=='__main__':main()
