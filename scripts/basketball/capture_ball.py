"""Observe ball action phases, including ordinary shots and floor bounces."""
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK=r'''
local f=assert(io.open(out.."/ball.jsonl","w"));local pending=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function state()
 local i=6
 local parts={}
 local fields={action=0x518,request=0x525,pose=0x61,facing=0x471,clock=0x43d,cursor=0x44a,repeatCount=0x457,nextAction=0x532,air=0x5fd,attachment=0x648,bounces=0x511,holder=0x53f,hoop=0x55d,deflection=0x564}
 for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i))
 local owner=AND(memory.readbyte(0x53f),0x7f)
 local shooter=owner
 if shooter>3 then shooter=0 end
 local rim={};for k=7,12 do rim[#rim+1]=tostring(memory.readbyte(0x6e9+k)) end
 parts[#parts+1]=string.format('"shooter":%d,"shooterHeight":%d,"character":%d,"team":%d,"specialFlags":%d,"equipment":%d,"rimFlags":[%s],"targetX":%d,"targetDepth":%d,"targetHeight":%d,"arcHeight":%d',shooter,wide(0xa9+shooter,0xb6+shooter,0xc3+shooter),memory.readbyte(0x3e3+shooter),memory.readbyte(0x586+math.floor(shooter/2)),memory.readbyte(0x554+shooter),memory.readbyte(0x6e4),table.concat(rim,','),word(0x69f,0x6a0),memory.readbyte(0x6a1),word(0x6a2,0x6a3),memory.readbyte(0x6a4))
 return '{'..table.concat(parts,',')..'}'
end
memory.registerexec(0xe149,1,function()
 assert(not pending);pending=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0xe14c,1,function()
 assert(pending);f:write(pending..'"after":'..state()..'}\n');pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-ball.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def capture(spec):
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    return BASE/'runs'/spec['name']

def main():
    cases={
        'shot':dict(load_slot=7,frames=360,segments=[dict(start=1,end=30,keys=['B'])]),
        'quick-shot':dict(load_slot=7,frames=360,segments=[dict(start=1,end=3,keys=['B'])]),
        'left-shot':dict(load_slot=7,frames=360,segments=[dict(start=1,end=9,keys=['left']),dict(start=10,end=18,keys=['B'])]),
        'jump-shot':dict(load_slot=7,frames=360,segments=[dict(start=1,end=9,keys=['A','B']),dict(start=16,end=24,keys=['B'])]),
        'pass':dict(load_slot=7,frames=360,segments=[dict(start=1,end=9,keys=['A'])]),
        'cpu-match':dict(load_slot=5,frames=1800,segments=[]),
    }
    records=[];sources=[]
    for name,spec in cases.items():
        spec=dict(spec,captures=[spec['frames']])
        reference=capture(dict(spec,name='ball-control-'+name))
        observed=capture(dict(spec,name='ball-'+name,hooks_lua=HOOK))
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(),'Probe changed original execution'
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'ball.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/'games/basketball/engine/testdata/ball.json'
    fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='fixed:e149',exit='fixed:e14c',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name('ball-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(dict(records=len(records),scenarios=len(cases))))

if __name__=='__main__':
    main()
