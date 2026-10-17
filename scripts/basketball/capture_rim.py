"""Record original rim search/outcomes after the separate backboard phase."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/rim.jsonl","w"));local pending=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function body(i)
 local fields={action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,link=0x539,cooldown=0x626}
 local parts={};for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a+i)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i))
 return '{'..table.concat(parts,',')..'}'
end
local function state()
 local hoops={};for i=7,12 do hoops[#hoops+1]=body(i) end
 local rng={};for i=0x48,0x4b do rng[#rng+1]=tostring(memory.readbyte(i)) end
 local shooter=memory.readbyte(0x6d1)
 local ability=0
 if shooter<2 then
  local char=memory.readbyte(0x3e3+shooter)
  local ptr=word(0x94d8+char*2,0x94d9+char*2)
  ability=memory.readbyte(ptr+2)
 end
 return string.format('{"ball":%s,"hoops":[%s],"rng":[%s],"scores":[%d,%d],"holder":%d,"bounces":%d,"deflection":%d,"previousAction":%d,"selectedHoop":%d,"shooter":%d,"shooterAction":%d,"skill":%d,"scoreAbility":%d,"originX":%d,"soundDelay":%d}',body(6),table.concat(hoops,','),table.concat(rng,','),word(0x166,0x168),word(0x167,0x169),memory.readbyte(0x53f),memory.readbyte(0x511),memory.readbyte(0x564),memory.readbyte(0x6cf),memory.readbyte(0x55d),shooter,memory.readbyte(0x512+shooter),memory.readbyte(0x61d+shooter),ability,word(0x55e,0x55f),memory.readbyte(0xe3))
end
memory.registerexec(0x9035,1,function()
 if memory.readbyte(0xbfff)~=4 or memory.getregister("x")~=6 then return end
 assert(not pending);pending=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0x9290,1,function()
 if memory.readbyte(0xbfff)~=4 or not pending then return end
 f:write(pending..'"after":'..state()..'}\n');pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-rim.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    records=[];sources=[]
    for name in ['shot','quick-shot','left-shot','jump-shot','pass','cpu-match']:
        reference=BASE/'runs'/('ball-control-'+name)
        spec=json.loads((reference/'scenario.json').read_text())
        spec.update(name='rim-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(),'Rim probe changed original execution'
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'rim.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/'games/basketball/engine/testdata/rim.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='bank4:9035',exit='bank4:9290',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name('rim-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
