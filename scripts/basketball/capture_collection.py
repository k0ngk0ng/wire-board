"""Capture a fixed original AI pickup/carry/throw interval across whole phases."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/collection.jsonl","w"));local pending=nil;local randomBefore=nil;local randomAfter=nil
local function rng()
 local bytes={};for i=0x48,0x4b do bytes[#bytes+1]=tostring(memory.readbyte(i)) end
 return '['..table.concat(bytes,',')..']'
end
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function object(i)
 local fields={action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,heading=0x4ec,link=0x539,cooldown=0x626,durability=0x577,shotCycle=0x572,aimWindow=0x54c,runGrade=0x615,jumpGrade=0x619}
 local parts={};for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a+i)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i))
 return '{'..table.concat(parts,',')..'}'
end
local function state()
 local i=2;local hoops={};for k=7,12 do hoops[#hoops+1]=object(k) end
 local mounts={};for k=0,5 do mounts[#mounts+1]=tostring(memory.readbyte(0x6f0+k)) end
 local players={};for k=0,3 do players[#players+1]=object(k) end
 return string.format('{"player":%s,"control":{"mode":%d,"nextMode":%d,"counter":%d,"parameter":%d,"target":%d,"destination":%d},"hoops":[%s],"players":[%s],"ball":%s,"holder":%d,"mountFlags":[%s],"contactLatch":%d,"firstClimber":%d,"secondClimber":%d,"selectedFlags":%d}',object(i),memory.readbyte(0x672+i),memory.readbyte(0x676+i),memory.readbyte(0x66e+i),memory.readbyte(0x682+i),memory.readbyte(0x67a+i),memory.readbyte(0x67e+i),table.concat(hoops,','),table.concat(players,','),object(6),memory.readbyte(0x53f),table.concat(mounts,','),memory.readbyte(0x571),memory.readbyte(0x53d),memory.readbyte(0x53e),memory.readbyte(0x6e9+memory.readbyte(0x55d)))
end
memory.registerexec(0xd32c,1,function()
 local frame=emu.framecount()
 if frame<5450 or frame>5530 then return end
 assert(not pending);randomBefore=nil;randomAfter=nil;pending=string.format('{"frame":%d,"before":%s,',frame,state())
end)
memory.registerexec(0xa05d,1,function()
 if pending and memory.readbyte(0xbfff)==5 and memory.getregister("x")==2 then randomBefore=rng() end
end)
memory.registerexec(0xfd8e,1,function()
 if pending and memory.getregister("x")==2 then randomAfter=rng() end
end)
memory.registerexec(0xd348,1,function()
 if not pending then return end
 assert(randomBefore and randomAfter)
 f:write(pending..'"actorRandomBefore":'..randomBefore..',"actorRandomAfter":'..randomAfter..',"after":'..state()..'}\n');pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-collection.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text());spec.update(name='collection-cpu-long',hooks_lua=HOOK)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    rows=[json.loads(line) for line in (observed/'collection.jsonl').read_text().splitlines()]
    p=ROOT/'games/basketball/engine/testdata/collection.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    p.with_name('collection-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='fixed:d32c',exit='fixed:d348',records=len(rows),actor=2,frame_start=5450,frame_end=5530,sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n')
    print(json.dumps(dict(records=len(rows))))

if __name__=='__main__':main()
