"""Capture original animation function inputs/outputs, including loop endings."""
from pathlib import Path
import json
from capture_reference import BASE,run

hook='''
local f=assert(io.open(out.."/animation.jsonl","w"))
local pending=nil
local function state()
 local x=memory.getregister("x")
 return string.format('{"action":%d,"next":%d,"clock":%d,"cursor":%d,"repeat":%d,"pose":%d}',memory.readbyte(0x512+x),memory.readbyte(0x52c+x),memory.readbyte(0x437+x),memory.readbyte(0x444+x),memory.readbyte(0x451+x),memory.readbyte(0x5b+x))
end
memory.registerexec(0x8741,1,function()
 local x=memory.getregister("x")
 if memory.readbyte(0xbfff)~=6 or x>3 then return end
 assert(not pending)
 local addr=memory.readbyte(0x2a)+256*memory.readbyte(0x2b)
 local frames={};local i=0
 while memory.readbyte(addr+i)<0xfd do
  frames[#frames+1]=string.format('{"pose":%d,"ticks":%d}',memory.readbyte(addr+i),memory.readbyte(addr+i+1));i=i+2;assert(i<128)
 end
 local ending=memory.readbyte(addr+i)
 local limit=ending==0xfd and memory.readbyte(addr+i+1) or 0
 pending=string.format('{"frame":%d,"actor":%d,"address":%d,"facing":%d,"spec":{"frames":[%s],"end":%d,"limit":%d},"before":%s,',emu.framecount(),x,addr,memory.readbyte(0x46b+x),table.concat(frames,","),ending,limit,state())
end)
local function finish()
 if memory.readbyte(0xbfff)~=6 or memory.getregister("x")>3 then return end
 assert(pending);f:write(pending..'"after":'..state()..'}\\n');pending=nil
end
memory.registerexec(0x8758,1,finish)
memory.registerexec(0x879e,1,finish)
function finish_probe() assert(not pending);f:close() end
'''
specs={};records=[]
for name in ['idle','right','left','jump','a','b']:
    spec=json.loads((BASE/f'control-{name}.json').read_text());spec['name']='animation-'+name;spec['hooks_lua']=hook
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    out=BASE/'runs'/spec['name'];reference=BASE/'runs'/('control-'+name)
    assert (out/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    for line in (out/'animation.jsonl').read_text().splitlines():
        row=json.loads(line);row['scenario']=name
        address=str(row['address']);seq=row.pop('spec')
        if address in specs:assert specs[address]==seq
        specs[address]=seq;records.append(row)
out=Path('games/basketball/engine/testdata/animation.json')
out.write_text(json.dumps({'source':'bank 6 $8741, exits $8758/$879E','specs':specs,'records':records},separators=(',',':'))+'\n')
print(json.dumps({'animation_specs':len(specs),'original_calls':len(records),'fixture':str(out)}))
