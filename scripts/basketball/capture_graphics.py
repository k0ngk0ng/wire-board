"""Capture separate field/HUD pattern mappings and current sprites from FCEUX.

Requires the pinned ROM and initial match savestate prepared by the research
workflow. All output stays in .local/nes-basket; it is not a runtime dependency.
"""
from pathlib import Path
import argparse,json
from capture_reference import run,BASE

p=argparse.ArgumentParser();p.add_argument('--scenario',type=Path,default=BASE/'control-right.json')
args=p.parse_args();spec=json.loads(args.scenario.read_text());spec['name']='graphics-'+spec['name'].removeprefix('control-')
spec['hooks_lua']='''
local oam=string.rep(string.char(255),256)
local ctrl=memory.readbyte(0x56)
local field=nil
local scrollx,scrolly=0,0
local fieldx,fieldy=0,0
local field_phase=false
local owners={};for i=1,64 do owners[i]=255 end
local rendered_owners=string.rep(string.char(255),64)
local object_state={}
local rendered_objects="{}"
local order_reversed=false
local rendered_order=false
local function readbytes(addr,len,reader)
 local t={};for i=addr,addr+len-1 do t[#t+1]=string.char(reader(i)) end;return table.concat(t)
end
memory.registerexec(0xf002,1,function()
 local reversed={};for i=1,64 do reversed[i]=owners[65-i] end;owners=reversed
 order_reversed=not order_reversed
end)
memory.registerexec(0xce09,1,function() order_reversed=false end)
memory.registerwrite(0x200,0x100,function(a,s,v)
 if memory.getregister("pc")==0xcedb and a%4==0 then
  local x=memory.getregister("x")
  owners[(a-0x200)/4+1]=x
  local px=memory.readbyte(0x68+x)+256*memory.readbyte(0x75+x)+65536*memory.readbyte(0x82+x)
  local py=memory.readbyte(0x8f+x)+256*memory.readbyte(0x9c+x)
  local pz=memory.readbyte(0xa9+x)+256*memory.readbyte(0xb6+x)+65536*memory.readbyte(0xc3+x)
  object_state[x]=string.format('"%d":{"x":%d,"y":%d,"z":%d,"ram005b":%d,"ram03d6":%d}',x,px,py,pz,memory.readbyte(0x5b+x),memory.readbyte(0x3d6+x))
 end
end)
memory.registerwrite(0x4014,1,function(a,s,v)
 oam=readbytes(v*256,256,memory.readbyte)
 local t={};for i=1,64 do t[i]=string.char(owners[i]) end;rendered_owners=table.concat(t)
 local states={};for i=0,12 do if object_state[i] then states[#states+1]=object_state[i] end end
 rendered_objects="{"..table.concat(states,",").."}"
 rendered_order=order_reversed
end)
memory.registerwrite(0x2000,1,function(a,s,v) ctrl=v end)
memory.registerwrite(0x2005,1,function(a,s,v)
 local pc=memory.getregister("pc")
 if pc==0xf2b1 then scrollx=v end
 if pc==0xf2b7 then scrolly=v end
end)
memory.registerwrite(0x8000,1,function(a,s,v)
 if field_phase and memory.getregister("pc")==0xf281 then
  field=readbytes(0,0x4000,ppu.readbyte)
  fieldx,fieldy=scrollx,scrolly
 end
end)
memory.registerwrite(0x8001,1,function(a,s,v)
 local pc=memory.getregister("pc")
 if pc==0xf213 then field_phase=true end
 if pc==0xf334 then field_phase=false end
end)
emu.registerafter(function()
 local stem=string.format("ppu-%05d",emu.framecount())
 writefile(stem.."-oam.bin",oam)
 writefile(stem.."-ppu.bin",readbytes(0,0x4000,ppu.readbyte))
 writefile(stem.."-ctrl.bin",string.char(ctrl))
 writefile(stem.."-owners.bin",rendered_owners)
 writefile(stem.."-objects.json",rendered_objects)
 writefile(stem.."-order.bin",string.char(rendered_order and 1 or 0))
 if field then
  writefile(stem.."-field.bin",field)
  writefile(stem.."-scroll.bin",string.char(fieldx,fieldy))
 end
end)
local colors={}
for i=0,63 do local r,g,b,a=gui.parsecolor(string.format("P%02x",i)); colors[#colors+1]=string.char(r,g,b) end
writefile("nes-colors.rgb",table.concat(colors))
'''
path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
run(path)
