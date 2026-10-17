"""Record the original human-input interpretation before world action updates."""
from pathlib import Path
import json
import hashlib
from capture_reference import BASE,ROOT,run

hook='''
local f=assert(io.open(out.."/input.jsonl","w"));local pending=nil
local function state()
 local fields={request=0x51f,heading=0x4ec,tapTimer=0x4f9,lastTap=0x509,attackTapTimer=0x4fd,lastAttackTap=0x505,buttonDelay=0x501,pendingButtons=0x50d,boost=0x576}
 local values={};for k,a in pairs(fields) do values[#values+1]=string.format('"%s":%d',k,memory.readbyte(a)) end
 return '{'..table.concat(values,',')..'}'
end
memory.registerexec(0x805c,1,function()
 if memory.readbyte(0xbfff)~=6 or memory.getregister("x")~=0 then return end
 assert(not pending)
 local pad=memory.readbyte(0x4e8)
 pending=string.format('{"frame":%d,"held":%d,"pressed":%d,"action":%d,"pose":%d,"air":%d,"holder":%d,"before":%s,',emu.framecount(),memory.readbyte(pad),memory.readbyte(pad+4),memory.readbyte(0x512),memory.readbyte(0x5b),memory.readbyte(0x5f7),memory.readbyte(0x53f),state())
end)
memory.registerexec(0x8085,1,function()
 if memory.readbyte(0xbfff)~=6 or memory.getregister("x")~=0 or not pending then return end
 f:write(pending..'"after":'..state()..'}\\n');pending=nil
end)
function finish_probe()
 if pending then writefile("incomplete-input-call.json",pending..'"incomplete":true}') end
 f:close()
end
'''
records=[]
scenarios=json.loads((ROOT/'scripts/basketball/reference_scenarios.json').read_text())
for name,spec in scenarios.items():
    spec['name']='input-'+name;spec['hooks_lua']=hook
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name'];reference=BASE/'runs'/('control-'+name)
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for line in (out/'input.jsonl').read_text().splitlines():records.append({'scenario':name,**json.loads(line)})
path=ROOT/'games/basketball/engine/testdata/input.json';path.write_text(json.dumps(records,separators=(',',':'))+'\n')
manifest={'rom_sha256':json.loads((out/'source.json').read_text())['rom_sha256'],
          'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'records':len(records),
          'scenarios':sorted({r['scenario'] for r in records}),
          'source_is_original_execution':True,'entry':'bank6:805c','return':'bank6:8085'}
path.with_name('input-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'input_calls':len(records),'fixture':str(path)}))
