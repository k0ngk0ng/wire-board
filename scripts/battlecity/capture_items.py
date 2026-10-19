#!/usr/bin/env python3
"""Force-render each Battle City powerup item and capture evidence.

Research helper only.  Pokes the item state in emulator RAM (never the ROM) so
each of the six item IDs is drawn at a fixed position, then captures the
framebuffer, OAM and the item variables.  Writes only under .local.
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
    parser.add_argument("--name", default="items")
    parser.add_argument("--frames", type=int, default=720)
    parser.add_argument("--period", type=int, default=90)
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
    # Start stage 1, then every frame place item id (frame // period) % 6 at
    # (120, 104) and keep the pickup-flash timer clear.  Capture every 6 frames.
    lua = f'''local out = assert(os.getenv("NES_PROBE_OUT"))
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local function bytes(first,count)
  local t={{}}; for i=first,first+count-1 do t[#t+1]=string.char(memory.readbyte(i)) end
  return table.concat(t)
end
emu.poweron()
emu.speedmode("turbo")
for n=1,{args.frames} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  if n==120 or n==240 or n==360 then pad.start=true end
  joypad.set(1,pad)
  emu.frameadvance()
  if emu.framecount() >= 420 then
    memory.writebyte(0x86,120)
    memory.writebyte(0x87,104)
    memory.writebyte(0x88,math.floor((emu.framecount()-420)/{args.period})%6)
    memory.writebyte(0x62,0)
  end
  if emu.framecount() >= 430 and (emu.framecount()%6)==0 then
    local stem=string.format("frame-%04d",emu.framecount())
    writefile(stem..".gd",gui.gdscreenshot(true))
    writefile(stem.."-oam.bin",bytes(0x200,0x100))
    writefile(stem.."-palette.bin",bytes(0x3f00,32))
    writefile(stem.."-item.bin",bytes(0x86,3))
    writefile(stem.."-clock.bin",bytes(0x0b,1))
    writefile(stem.."-ram.bin",bytes(0x0000,0x0800))
  end
end
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
                    raise TimeoutError("item capture timed out")
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
        if len(raw) != 11 + width * height * 4:
            raise SystemExit(f"bad gd screenshot {gd}")
        Image.frombytes("RGBA", (width, height), raw[11:], "raw", "ARGB").convert("RGB").save(gd.with_suffix(".png"))
    hashes = {
        p.name: hashlib.sha256(p.read_bytes()).hexdigest()
        for p in sorted(out.iterdir())
        if p.suffix in {".bin", ".png", ".gd", ".lua"}
    }
    (out / "hashes.json").write_text(json.dumps(hashes, indent=2) + "\n")
    print(json.dumps({"name": args.name, "frames": args.frames}, indent=2))


if __name__ == "__main__":
    main()
