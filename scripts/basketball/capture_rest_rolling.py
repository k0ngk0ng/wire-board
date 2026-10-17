"""Capture complete natural resting/rolling ball dispatches without RAM writes."""
import argparse,csv,gzip,hashlib,json
from capture_reference import BASE,ROOT,run
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context as preparation_context

HOOK=r'''
local f=assert(io.open(out..'/rest-rolling-ram.bin','wb'))
local c=assert(io.open(out..'/rest-rolling.csv','w'));c:write('frame\n')
local pending=nil
local function snapshot() local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;f:write(table.concat(t)) end
memory.registerexec(0xe149,1,function()
 local action=AND(memory.readbyte(0x518),127)
 if action~=0 and action~=5 then return end
 assert(not pending);pending=emu.framecount();snapshot()
end)
memory.registerexec(0xe14c,1,function()
 if not pending then return end
 snapshot();c:write(pending..'\n');pending=nil
end)
function finish_probe() assert(not pending);f:close();c:close() end
'''


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--extract',action='store_true');args=parser.parse_args()
 reference=BASE/'runs/special-ball-cpu-command'
 spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
 spec.update(name='ball-rest-rolling-natural',hooks_lua=HOOK)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
 if not args.extract:run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
 raw=(observed/'rest-rolling-ram.bin').read_bytes();rows=list(csv.DictReader((observed/'rest-rolling.csv').open()));assert len(raw)==len(rows)*4096
 records=[]
 for i,row in enumerate(rows):
  before,after=raw[i*4096:i*4096+2048],raw[i*4096+2048:(i+1)*4096]
  records.append(dict(frame=int(row['frame']),before=decode_world_action_state(before),after=decode_world_action_state(after),context=preparation_context(before)['actions'],cameraShake=before[0x6d5]))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'ball-rest-rolling.json.gz').write_bytes(packed)
 summary=dict(source_is_original_execution=True,records=len(records),entry='fixed E149 before original ball request/action dispatch',exit='fixed E14C',trace_sha256=hashlib.sha256(raw).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),reference_ram_sha256=hashlib.sha256((reference/'ram.bin').read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text()))
 (out/'ball-rest-rolling-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
