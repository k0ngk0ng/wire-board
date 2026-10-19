#!/usr/bin/env python3
"""Capture Battle City sound-queue and APU writes during scripted gameplay.

Used to identify sound IDs/pointers for the offline NSF renderer.  Not a
runtime dependency and never executed in the web game.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import time
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / ".local/battlecity"
ROM = BASE / "rom/Battle City (J).nes"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
FCEUX = "/opt/homebrew/bin/fceux"
SANDBOX = ROOT / ".local/nes-basket/workspace.sb"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--frames", type=int, default=1400)
    parser.add_argument("--name", default="sound-probe")
    args = parser.parse_args()
    if hashlib.sha256(ROM.read_bytes()).hexdigest() != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    out = BASE / "reference" / args.name
    out.mkdir(parents=True, exist_ok=True)
    home = BASE / "fceux-home"
    home.mkdir(parents=True, exist_ok=True)
    (home / "fceux.cfg").write_text("SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\n")
    (BASE / "runtime").mkdir(exist_ok=True)
    (ROOT / ".local/tmp").mkdir(exist_ok=True)
    lua = f'''local out = assert(os.getenv("NES_PROBE_OUT"))
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local function bytes(first,count,reader)
  local t={{}}; for i=first,first+count-1 do t[#t+1]=string.char(reader(i)) end
  return table.concat(t)
end
local q=assert(io.open(out.."/queue.csv","w")); q:write("frame,pc,address,value,a,x,y\\n")
local apu=assert(io.open(out.."/apu.csv","w")); apu:write("frame,pc,address,value\\n")
memory.registerwrite(0x300,0x100,function(a,s,v)
  q:write(string.format("%d,%04x,%04x,%02x,%02x,%02x,%02x\\n",emu.framecount(),memory.getregister("pc"),a,v,memory.getregister("a"),memory.getregister("x"),memory.getregister("y")))
end)
memory.registerwrite(0x4000,0x18,function(a,s,v)
  apu:write(string.format("%d,%04x,%04x,%02x\\n",emu.framecount(),memory.getregister("pc"),a,v))
end)
local captures={{[300]=true,[360]=true,[400]=true,[450]=true,[500]=true,[600]=true,[700]=true,[800]=true,[900]=true,[1000]=true,[1100]=true,[1200]=true,[1350]=true}}
emu.poweron()
emu.speedmode("turbo")
for n=1,{args.frames} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  if n==120 or n==240 or n==360 then pad.start=true end
  if n>=480 and n<={args.frames} then
    if n%12 < 2 then pad.A=true end
    if n>=540 and n<700 then pad.right=true end
    if n>=700 and n<850 then pad.up=true end
    if n>=850 and n<1000 then pad.left=true end
  end
  joypad.set(1,pad)
  emu.frameadvance()
  if captures[emu.framecount()] then
    local stem=string.format("frame-%04d",emu.framecount())
    writefile(stem..".gd",gui.gdscreenshot(true))
    writefile(stem.."-oam.bin",bytes(0x200,0x100,memory.readbyte))
    writefile(stem.."-ppu.bin",bytes(0,0x4000,ppu.readbyte))
    writefile(stem.."-palette.bin",bytes(0x3f00,32,ppu.readbyte))
    writefile(stem.."-name.bin",bytes(0x2000,0x1000,ppu.readbyte))
    writefile(stem.."-queue.bin",bytes(0x300,0x100,memory.readbyte))
  end
end
q:close(); apu:close()
writefile("capture-complete.tmp",'{{"status":"complete"}}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''
    script = out / "record.lua"
    script.write_text(lua)
    completion = out / "capture-complete.json"
    completion.unlink(missing_ok=True)
    env = dict(
        os.environ,
        FCEUX_HOME=str(home),
        FCEUX_CONFIG_DIR=str(home),
        QT_QPA_PLATFORM="offscreen",
        SDL_AUDIODRIVER="dummy",
        XDG_RUNTIME_DIR=str(BASE / "runtime"),
        NES_PROBE_OUT=str(out),
        TMPDIR=str(ROOT / ".local/tmp"),
    )
    cmd = ["sandbox-exec", "-f", str(SANDBOX), FCEUX, "--sound", "0", "--loadlua", str(script), str(ROM)]
    with (out / "emulator.log").open("w") as log:
        proc = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
        started = time.monotonic()
        completed = False
        try:
            while proc.poll() is None:
                if completion.is_file():
                    completed = True
                    break
                if time.monotonic() - started > 90:
                    raise TimeoutError("sound capture timed out")
                time.sleep(0.05)
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
    if not completed:
        raise SystemExit(f"capture failed; see {out}/emulator.log")
    for gd in sorted(out.glob("*.gd")):
        raw = gd.read_bytes()
        width = int.from_bytes(raw[2:4], "big")
        height = int.from_bytes(raw[4:6], "big")
        Image.frombytes("RGBA", (width, height), raw[11:], "raw", "ARGB").convert("RGB").save(gd.with_suffix(".png"))
    hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.iterdir()) if p.suffix in {".csv",".bin",".png",".gd",".lua"}}
    (out / "hashes.json").write_text(json.dumps(hashes, indent=2) + "\n")
    print(json.dumps({"name": args.name, "queue_lines": sum(1 for _ in (out/'queue.csv').open()), "apu_lines": sum(1 for _ in (out/'apu.csv').open())}, indent=2))


if __name__ == "__main__":
    main()
