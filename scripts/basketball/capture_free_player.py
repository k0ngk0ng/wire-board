"""Record the original world action phase for the unpossessed primary player.

The scenarios exercise movement, running, jumps and A/B actions for player
zero. No opponents, inputs, collisions or state are changed by this recorder.
"""
from pathlib import Path
import json
from capture_reference import BASE,run

hook='''
local f=assert(io.open(out.."/player.jsonl","w"));local pending=nil
local function word(lo,hi) return memory.readbyte(lo)+256*memory.readbyte(hi) end
local function wide(lo,mid,hi) return word(lo,mid)+65536*memory.readbyte(hi) end
local function state()
 local fields={action=0x512,request=0x51f,facing=0x46b,pose=0x5b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,boost=0x576,heading=0x4ec,jumpGrade=0x619,arena=0x58b,holder=0x53f,team=0x586,character=0x3e3}
 local parts={};for name,addr in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(addr)) end
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d',wide(0x68,0x75,0x82),word(0x8f,0x9c),wide(0xa9,0xb6,0xc3),word(0x478,0x485),word(0x492,0x49f),word(0x4ac,0x4b9),word(0x4c6,0x4d3))
 return '{'..table.concat(parts,',')..'}'
end
memory.registerexec(0xd345,1,function()
 assert(not pending);pending=string.format('{"frame":%d,"before":%s,',emu.framecount(),state())
end)
memory.registerexec(0xd348,1,function()
 assert(pending);f:write(pending..'"after":'..state()..'}\\n');pending=nil
end)
function finish_probe()
 if pending then writefile("incomplete-player-call.json",pending..'"incomplete":true}') end
 f:close()
end
'''
records=[]
for name in ['idle','right','left','jump','a','b','run-brake','running-jump','up','down','diagonal']:
    s=json.loads((BASE/f'control-{name}.json').read_text());s['name']='player-'+name;s['hooks_lua']=hook
    path=BASE/(s['name']+'.json');path.write_text(json.dumps(s,indent=2)+'\n');run(path)
    out=BASE/'runs'/s['name'];ref=BASE/'runs'/('control-'+name)
    assert (out/'ram.bin').read_bytes()==(ref/'ram.bin').read_bytes()
    for line in (out/'player.jsonl').read_text().splitlines():records.append({'scenario':name,**json.loads(line)})
path=Path('games/basketball/engine/testdata/free-player.json');path.write_text(json.dumps(records,separators=(',',':'))+'\n')
print(json.dumps({'original_world_updates':len(records),'observed_actions':sorted({r['after']['action']&127 for r in records}),'fixture':str(path)}))
