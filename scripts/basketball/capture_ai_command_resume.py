"""Capture every natural original mode21/22 resume call and AI return."""
import argparse,csv,gzip,hashlib,json
from capture_reference import ROOT,BASE,run
from extract_world_inputs import decode_world_input_state
from extract_world_preparation import context as preparation_context

HOOK=r'''
local raw=assert(io.open(out..'/shot-plan2-ram.bin','wb'))
local calls=assert(io.open(out..'/shot-plan2.csv','w'));calls:write('frame,actor\n')
local pending=nil
local function snapshot() local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;raw:write(table.concat(t)) end
for _,entry in ipairs({0xa8c1}) do
 memory.registerexec(entry,1,function()
  if memory.readbyte(0xbfff)~=5 then return end
  assert(not pending);pending={emu.framecount(),memory.getregister('x')};snapshot()
 end)
end
memory.registerexec(0xfd8e,1,function()
 if not pending then return end
 assert(memory.getregister('x')==pending[2]);snapshot();calls:write(table.concat(pending,',')..'\n');pending=nil
end)
function finish_probe() assert(not pending);raw:close();calls:close() end
'''


def main():
 stem='command-resume';entry='A8C1';hook=HOOK.replace('shot-plan2',stem)
 reference=BASE/'runs/special-ball-cpu-command'
 spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='ai-'+stem+'-natural',hooks_lua=hook)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
 raw=(observed/(stem+'-ram.bin')).read_bytes();rows=list(csv.DictReader((observed/(stem+'.csv')).open()));assert len(raw)==len(rows)*4096
 records=[]
 for i,row in enumerate(rows):
  b,a=raw[i*4096:i*4096+2048],raw[i*4096+2048:(i+1)*4096]
  records.append(dict(frame=int(row['frame']),actor=int(row['actor']),before=decode_world_input_state(b),after=decode_world_input_state(a),context=dict(world=preparation_context(b)['actions'],difficulty=b[0x58c],scoreDifference=b[0x696],cachedZones=list(b[0x66a:0x66e]),objectKinds=list(b[0x3d6:0x3e3]),hoopLinks=list(b[0x649:0x64f]),humanTactics=list(b[0x666:0x668]))))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/('ai-'+stem+'.json.gz')).write_bytes(packed)
 summary=dict(source_is_original_execution=True,records=len(records),entry='bank5 '+entry,exit='fixed FD8E',trace_sha256=hashlib.sha256(raw).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),reference_ram_sha256=hashlib.sha256((reference/'ram.bin').read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text()))
 (out/('ai-'+stem+'-source.json')).write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
