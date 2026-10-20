#!/usr/bin/env python3
"""Run the original ROM renderer for 40 controlled tank cases; retain OAM/PPU.
Only fixture RAM (class/direction/tread/position) is set at $DFE9. ROM code is
unchanged. The native $DFEB/$DA73/$DA7B routines write actual OAM.
"""
import hashlib, json, os, subprocess, time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
ROM=ROOT/'.local/battlecity/rom/Battle City (J).nes'
OUT=ROOT/'.local/battlecity/reference/directions-v2'
SHA='a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c'
assert hashlib.sha256(ROM.read_bytes()).hexdigest()==SHA
OUT.mkdir(parents=True,exist_ok=True)
cases=[{'class':c,'base':base,'direction':d,'nativeDirection':n,'frame':f} for c,base in [('balanced',32),('basic',128),('fast',160),('power',192),('armor',224)] for n,d in enumerate(['up','left','down','right']) for f in range(2)]
lua='''local out=assert(os.getenv("NES_PROBE_OUT"))
local function save(name,data) local f=assert(io.open(out.."/"..name,"wb"));f:write(data);f:close() end
local function bytes(a,n,read) local t={} for i=a,a+n-1 do t[#t+1]=string.char(read(i)) end return table.concat(t) end
local cases=CASES
local current=0
local rendered=false
local oam=string.rep(string.char(255),256)
memory.registerwrite(0x4014,1,function(a,s,v) oam=bytes(v*256,256,memory.readbyte) end)
memory.registerexec(0xDFE9,1,function()
 if current==0 or memory.getregister("x")~=0 then return end
 local c=cases[current]
 memory.writebyte(0xA8,c[1]); memory.writebyte(0xA0,0xA0+c[2]); memory.writebyte(0xB0,c[3]*4)
 memory.writebyte(0x90,120); memory.writebyte(0x98,120); rendered=true
end)
emu.poweron();emu.speedmode("turbo")
for n=1,620 do
 local pad={};if n==120 or n==240 or n==360 then pad.start=true end
 joypad.set(1,pad)
 if n>=500 and n<620 then current=math.floor((n-500)/3)+1;rendered=false;memory.writebyte(0x6F,0) end
 emu.frameadvance()
 if current>0 and (n-500)%3==2 then
  assert(rendered,"native renderer not reached")
  local stem=string.format("case-%02d",current)
  save(stem.."-oam.bin",oam);save(stem.."-ppu.bin",bytes(0,0x2000,ppu.readbyte));save(stem..".gd",gui.gdscreenshot(true))
 end
end
save("complete.json",'{"passed":true}');while true do end
'''.replace('CASES','{'+','.join('{'+f"{c['base']},{c['nativeDirection']},{c['frame']}"+'}' for c in cases)+'}')
(OUT/'record.lua').write_text(lua)
(OUT/'complete.json').unlink(missing_ok=True)
env=dict(os.environ,FCEUX_HOME=str(ROOT/'.local/battlecity/fceux-home'),FCEUX_CONFIG_DIR=str(ROOT/'.local/battlecity/fceux-home'),QT_QPA_PLATFORM='offscreen',SDL_AUDIODRIVER='dummy',XDG_RUNTIME_DIR=str(ROOT/'.local/battlecity/runtime'),NES_PROBE_OUT=str(OUT),TMPDIR=str(ROOT/'.local/tmp'))
with (OUT/'emulator.log').open('w') as log:
 p=subprocess.Popen(['sandbox-exec','-f',str(ROOT/'.local/nes-basket/workspace.sb'),'/opt/homebrew/bin/fceux','--sound','0','--loadlua',str(OUT/'record.lua'),str(ROM)],env=env,stdout=log,stderr=subprocess.STDOUT)
 try:
  start=time.monotonic()
  while p.poll() is None and not (OUT/'complete.json').exists():
   if time.monotonic()-start>60:raise TimeoutError('capture timed out')
   time.sleep(.1)
 finally:
  if p.poll() is None:p.kill();p.wait()
assert (OUT/'complete.json').exists(),(OUT/'emulator.log').read_text()
(OUT/'cases.json').write_text(json.dumps({'romSha256':SHA,'cases':cases,'hashes':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(OUT.glob('*')) if p.suffix in ['.bin','.lua','.gd']}},indent=2)+'\n')
print(json.dumps({'passed':True,'cases':len(cases),'out':str(OUT)}))
