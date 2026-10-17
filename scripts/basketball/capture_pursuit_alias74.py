"""Read-only alias74 write/probe watch; retains complete original RAM and frames."""
import json,csv,hashlib
from pathlib import Path
from capture_reference import BASE,ROOT,run
HOOK=r'''
local writes=assert(io.open(out..'/alias-writes.csv','w'));writes:write('frame,bank,pc,address,value\n')
local reads=assert(io.open(out..'/alias-probes.csv','w'));reads:write('frame,actor,index,actionByte,depthByte\n')
for _,address in ipairs({0x55c,0xe6}) do
 memory.registerwrite(address,1,function(a,size,value)
  writes:write(string.format('%d,%d,%04x,%04x,%d\n',emu.framecount(),memory.readbyte(0xbfff),memory.getregister('pc'),a,value or -1))
 end)
end
memory.registerexec(0xb10a,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 local index=memory.getregister('y')
 if index<4 then return end
 reads:write(string.format('%d,%d,%d,%d,%d\n',emu.framecount(),memory.getregister('x'),index,memory.readbyte(0x55c),memory.readbyte(0xe6)))
end)
function finish_probe() writes:close();reads:close() end
'''
def main():
    result=[]
    for name in ['hoop-control-cpu-long','rider-normal-reference']:
     reference=BASE/'runs'/name
     spec=json.loads((reference/'scenario.json').read_text());spec.update(name='alias-watch-'+name,hooks_lua=HOOK);spec.pop('save_slot',None)
     path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
     observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
     for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
     writes=list(csv.DictReader((observed/'alias-writes.csv').open()));probes=list(csv.DictReader((observed/'alias-probes.csv').open()))
     result.append(dict(reference=name,frames=spec['frames'],writes=writes,probes=probes,reference_sha256=hashlib.sha256((reference/'ram.bin').read_bytes()).hexdigest(),unchanged=True))
     print(name,'writes',len(writes),'probes',len(probes),flush=True)
    (ROOT/'.local/basketball/rea/pursuit-alias74/write-observation.json').write_text(json.dumps(result,indent=2)+'\n')


if __name__=='__main__':main()
