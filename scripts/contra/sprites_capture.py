"""Capture natural sprite-builder calls, actual DMA and display state.

Uses the existing natural co-op opening state without modifying original RAM.
All output and emulator configuration belong to this task's workspace tree.
"""
from pathlib import Path
import hashlib
import json
import os
import shutil
import sys

ROOT = Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT', str(ROOT/'.local/contra-sprites/runtime'))
from capture_reference import BASE, run

HOOKS = r'''
local calls=assert(io.open(out..'/sprite-calls.jsonl','w'))
local raw=assert(io.open(out..'/sprite-calls.bin','wb'))
local dma=assert(io.open(out..'/dma.bin','wb'))
local display=assert(io.open(out..'/display.jsonl','w'))
local animation=assert(io.open(out..'/animation.jsonl','w'))
local oam=string.rep(string.char(0xf4),256)
local ctrl=0;local mask=0;local dmaframe=-1;local seq=0;local ordinal=0
local pending=nil
local player_pending=nil
local selected=SELECTED
local function pair(a) return memory.readbyte(a)+256*memory.readbyte(a+1) end
local function player(i)
 return string.format('{"actor":%d,"levelType":%d,"sequence":%d,"water":%d,"fall":%d,"jump":%d,"aim":%d,"animationTimer":%d,"animationFrame":%d,"specialTimer":%d,"recoil":%d,"hidden":%d,"spawnInvincibility":%d,"invincibility":%d,"electric":%d,"background":%d,"spriteCode":%d,"flip":%d,"displayCode":%d,"displayAttr":%d,"y":%d}',i,memory.readbyte(0x40),memory.readbyte(0xbc+i),memory.readbyte(0xb2+i),memory.readbyte(0xa4+i),memory.readbyte(0xa0+i),memory.readbyte(0xc2+i),memory.readbyte(0x9e+i),memory.readbyte(0xa6+i),memory.readbyte(0xd2+i),memory.readbyte(0xce+i),memory.readbyte(0xba+i),memory.readbyte(0xae+i),memory.readbyte(0xb0+i),memory.readbyte(0xc8+i),memory.readbyte(0xda+i),memory.readbyte(0xd6+i),memory.readbyte(0xd8+i),memory.readbyte(0x300+i),memory.readbyte(0x34e+i),memory.readbyte(0x31a+i))
end
memory.registerexec(0xa4f8,1,function()
 if memory.readbyte(0x8000)~=0x14 then return end
 assert(not player_pending)
 local i=memory.getregister('x');assert(i<2)
 player_pending={actor=i,before=player(i),frame=emu.framecount()}
 local ret=pair(0x100+AND(memory.getregister('s')+1,255))+1
 memory.registerexec(ret,1,function()
  if not player_pending then return end
  local p=player_pending;assert(memory.getregister('x')==p.actor)
  animation:write(string.format('{"frame":%d,"before":%s,"after":%s}\n',p.frame,p.before,player(p.actor)))
  player_pending=nil
 end)
end)
memory.registerexec(0x9284,1,function()
 if memory.readbyte(0x8000)~=0x14 then return end
 assert(not pending)
 local sp=memory.getregister('s')
 local ret=pair(0x100+AND(sp+1,255))+1
 pending={frame=emu.framecount(),code=memory.getregister('a'),slot=memory.readbyte(5),
  flags=memory.readbyte(0),y=memory.readbyte(1),x=memory.readbyte(2),
  offset=memory.readbyte(4),remaining=memory.readbyte(7),stride=memory.readbyte(0x10),
  before=memory.readbyterange(0x200,256)}
 memory.registerexec(ret,1,function()
  if not pending then return end
  local p=pending;raw:write(p.before);raw:write(memory.readbyterange(0x200,256))
  calls:write(string.format('{"index":%d,"frame":%d,"code":%d,"slot":%d,"flags":%d,"x":%d,"y":%d,"offset":%d,"remaining":%d,"stride":%d,"afterOffset":%d,"afterRemaining":%d}\n',ordinal,p.frame,p.code,p.slot,p.flags,p.x,p.y,p.offset,p.remaining,p.stride,memory.readbyte(4),memory.readbyte(7)))
  ordinal=ordinal+1;pending=nil
 end)
end)
memory.registerwrite(0x4014,1,function(a,s,v)
 oam=memory.readbyterange(v*256,256);dmaframe=emu.framecount()
end)
memory.registerwrite(0x2000,1,function(a,s,v)ctrl=v end)
memory.registerwrite(0x2001,1,function(a,s,v)mask=v end)
emu.registerafter(function()
 seq=seq+1;dma:write(oam)
 display:write(string.format('{"frame":%d,"emulatorFrame":%d,"dmaFrame":%d,"ctrl":%d,"mask":%d}\n',seq,emu.framecount(),dmaframe,ctrl,mask))
 if selected[seq] then
  writefile(string.format('display-%05d-ppu.bin',seq),bytes(0,0x4000,ppu.readbyte))
 end
end)
local colors={}
for i=0,63 do local r,g,b,a=gui.parsecolor(string.format('P%02x',i));colors[#colors+1]=string.char(r,g,b)end
writefile('nes-colors.rgb',table.concat(colors))
function finish_probe()assert(not pending);assert(not player_pending);calls:close();raw:close();dma:close();display:close();animation:close()end
'''


def main():
    original = ROOT/'.local/contra/states/coop-play.fcs'
    assert original.exists(), 'run capture_bootstrap.py before sprite capture'
    (BASE/'states').mkdir(parents=True, exist_ok=True)
    target = BASE/'states/coop-play.fcs'
    if target.exists():
        assert target.read_bytes()==original.read_bytes()
    else:
        shutil.copyfile(original,target)
    captures = [1,2,10,20,40,60,80,100,120,140,160,180,200,220,240]
    name = sys.argv[1] if len(sys.argv)>1 else 'natural-sprites'
    spec = {'name':name,'load_state':'coop-play','frames':240,'captures':captures,
            'segments':[
                {'start':1,'end':55,'player':1,'keys':['right','B']},
                {'start':5,'end':5,'player':1,'keys':['A']},
                {'start':1,'end':25,'player':2,'keys':['left','B']},
                {'start':26,'end':80,'player':2,'keys':['up','B']},
                {'start':65,'end':90,'player':1,'keys':['down','B']},
                {'start':100,'end':150,'player':1,'keys':['right','up','B']},
                {'start':100,'end':150,'player':2,'keys':['right','B']},
                {'start':165,'end':190,'player':1,'keys':['left','up','B']},
                {'start':165,'end':190,'player':2,'keys':['left','down','B']},
                {'start':205,'end':205,'player':1,'keys':['A']},
                {'start':205,'end':205,'player':2,'keys':['A']},
            ],
            'hooks_lua':HOOKS.replace('SELECTED','{'+','.join(f'[{n}]=true' for n in captures)+'}')}
    path=BASE/'scenario.json';path.write_text(json.dumps(spec,indent=2)+'\n')
    out=run(path)
    rows=[json.loads(l) for l in (out/'sprite-calls.jsonl').read_text().splitlines()]
    assert (out/'sprite-calls.bin').stat().st_size==len(rows)*512
    assert (out/'dma.bin').stat().st_size==spec['frames']*256
    source={'initial_state_sha256':hashlib.sha256(original.read_bytes()).hexdigest(),
            'calls':len(rows),'codes':sorted({r['code'] for r in rows}),
            'interventions':'none; ordinary controllers only','capture_source':json.loads((out/'source.json').read_text())}
    (out/'sprite-source.json').write_text(json.dumps(source,indent=2)+'\n')
    print(json.dumps(source))


if __name__=='__main__':
    main()
