"""Observe original shared AI reactions to the ball, preserving caller boundaries."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/reaction.jsonl","w"));local pending=nil;local returnPC=nil;local actor=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function position(i)
 return string.format('{"x":%d,"depth":%d,"height":%d}',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i))
end
local function state(i)
 local players={};for k=0,3 do players[#players+1]=position(k) end
 local random={};for k=0x48,0x4b do random[#random+1]=tostring(memory.readbyte(k)) end
 local team=memory.readbyte(0x586+math.floor(i/2));local character=memory.readbyte(0x3e3+i)
 local teamPtr=word(0x94af+team*2,0x94b0+team*2);local optionsPtr=word(teamPtr+character*2,teamPtr+character*2+1)
 local options={};for k=0,3 do options[#options+1]=string.format('{"destination":%d,"parameter":%d}',memory.readbyte(optionsPtr+k*2),memory.readbyte(optionsPtr+k*2+1)) end
 return string.format('{"action":%d,"request":%d,"heading":%d,"facing":%d,"mode":%d,"nextMode":%d,"counter":%d,"parameter":%d,"target":%d,"destination":%d,"position":%s,"players":[%s],"ball":%s,"ballAction":%d,"ballFacing":%d,"holder":%d,"landingX":%d,"landingDepth":%d,"random":[%s],"team":%d,"character":%d,"options":[%s],"carry":%d}',memory.readbyte(0x512+i),memory.readbyte(0x51f+i),memory.readbyte(0x4ec+i),memory.readbyte(0x46b+i),memory.readbyte(0x672+i),memory.readbyte(0x676+i),memory.readbyte(0x66e+i),memory.readbyte(0x682+i),memory.readbyte(0x67a+i),memory.readbyte(0x67e+i),position(i),table.concat(players,','),position(6),memory.readbyte(0x518),memory.readbyte(0x471),memory.readbyte(0x53f),word(0x699,0x69a),memory.readbyte(0x698),table.concat(random,','),team,character,table.concat(options,','),AND(memory.getregister("p"),1))
end
memory.registerexec(0xbacc,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending);actor=memory.getregister("x");assert(actor<4)
 local sp=memory.getregister("s")
 returnPC=word(0x100+AND(sp+1,255),0x100+AND(sp+2,255))+1
 local expected=returnPC
 pending=string.format('{"frame":%d,"actor":%d,"returnPC":%d,"before":%s,',emu.framecount(),actor,returnPC,state(actor))
 memory.registerexec(returnPC,1,function()
  if not pending or returnPC~=expected then return end
  assert(memory.getregister("x")==actor)
  f:write(pending..'"after":'..state(actor)..'}\n');pending=nil
 end)
end)
function finish_probe()
 if pending then writefile('incomplete-reaction.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    records=[];sources=[]
    for name,reference_name in [('cpu-long','hoop-control-cpu-long'),('pass','ball-control-pass'),('shot','ball-control-shot'),('jump-shot','ball-control-jump-shot')]:
        reference=BASE/'runs'/reference_name
        spec=json.loads((reference/'scenario.json').read_text());spec.update(name='ai-ball-reaction-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'reaction.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    p=ROOT/'games/basketball/engine/testdata/ai-ball-reaction.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n')
    p.with_name('ai-ball-reaction-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank5:bacc',exit='original caller return address',records=len(records),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
