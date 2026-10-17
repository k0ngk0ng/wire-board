"""Observe all six hoop action calls during normal original matches."""
import hashlib
import json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/hoop.jsonl","w"));local pending=nil
local locks=assert(io.open(out.."/locks.jsonl","w"));local lockPending=nil
local function lockState()
 local values={};for i=0x62d,0x632 do values[#values+1]=tostring(memory.readbyte(i)) end
 return '['..table.concat(values,',')..']'
end
memory.registerexec(0x8ebb,1,function()
 if memory.readbyte(0xbfff)~=4 or memory.getregister("x")~=6 then return end
 assert(not lockPending);lockPending=string.format('{"frame":%d,"holder":%d,"before":%s,',emu.framecount(),memory.readbyte(0x53f),lockState())
end)
memory.registerexec(0x8ece,1,function()
 if memory.readbyte(0xbfff)~=4 or not lockPending then return end
 locks:write(lockPending..'"after":'..lockState()..'}\n');lockPending=nil
end)
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function state(i)
 local fields={action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,link=0x539,cooldown=0x626,durability=0x577}
 local parts={};for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a+i)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i))
 local flags={};for k=0,5 do flags[#flags+1]=tostring(memory.readbyte(0x6f0+k)) end
 parts[#parts+1]=string.format('"detached":%d,"holder":%d,"contactLatch":%d,"firstClimber":%d,"secondClimber":%d,"mountFlags":[%s]',memory.readbyte(0xe4),memory.readbyte(0x53f),memory.readbyte(0x571),memory.readbyte(0x53d),memory.readbyte(0x53e),table.concat(flags,','))
 local owner=memory.readbyte(0x539+i);if owner>3 then owner=0 end
 parts[#parts+1]=string.format('"selectedFlags":%d',memory.readbyte(0x6e9+memory.readbyte(0x55d)))
 parts[#parts+1]=string.format('"ownerAction":%d,"ownerPose":%d,"ownerFacing":%d,"ownerX":%d,"ownerDepth":%d,"ownerHeight":%d,"ownerComputer":%d,"ownerSpecial":%d,"selectedHoop":%d,"targetX":%d,"targetDepth":%d,"targetHeight":%d,"arcHeight":%d',memory.readbyte(0x512+owner),memory.readbyte(0x5b+owner),memory.readbyte(0x46b+owner),wide(0x68+owner,0x75+owner,0x82+owner),word(0x8f+owner,0x9c+owner),wide(0xa9+owner,0xb6+owner,0xc3+owner),memory.readbyte(0x539+owner),memory.readbyte(0x554+owner),memory.readbyte(0x55d),word(0x69f,0x6a0),memory.readbyte(0x6a1),word(0x6a2,0x6a3),memory.readbyte(0x6a4))
 return '{'..table.concat(parts,',')..'}'
end
memory.registerexec(0xe151,1,function()
 local i=memory.getregister("x")
 assert(i>=7 and i<=12 and not pending)
 pending=string.format('{"frame":%d,"actor":%d,"before":%s,',emu.framecount(),i,state(i))
end)
memory.registerexec(0xe154,1,function()
 assert(pending)
 f:write(pending..'"after":'..state(memory.getregister("x"))..'}\n');pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-hoop.json',pending..'"incomplete":true}') end
 if lockPending then writefile('incomplete-lock.json',lockPending..'"incomplete":true}') end
 locks:close()
 f:close()
end
'''

def main():
    records=[];sources=[];locks=[]
    for name,slot,frames,segments in [('shot',7,360,[dict(start=1,end=3,keys=['B'])]),('cpu-long',5,6000,[])]:
        spec=dict(name='hoop-control-'+name,load_slot=slot,frames=frames,segments=segments,captures=[frames])
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        reference=BASE/'runs'/spec['name']
        spec.update(name='hoop-'+name,hooks_lua=HOOK)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(),'Hoop capture changed original behavior'
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'hoop.jsonl').read_text().splitlines())
        locks.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'locks.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/'games/basketball/engine/testdata/hoop.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='fixed:e151',exit='fixed:e154',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name('hoop-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    lockfile=fixture.with_name('hoop-locks.json');lockfile.write_text(json.dumps(locks,separators=(',',':'))+'\n')
    lockfile.with_name('hoop-locks-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='bank4:8ebb',exit='bank4:8ece',records=len(locks),sha256=hashlib.sha256(lockfile.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
