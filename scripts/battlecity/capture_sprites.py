#!/usr/bin/env python3
"""Capture scripted Battle City sprite frames (player directions, enemies).

Research helper only.  Writes screenshots, OAM, PPU and palette dumps under
.local/battlecity; no derived media enters Git.
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
    parser.add_argument("--name", default="sprite-probe")
    parser.add_argument("--frames", type=int, default=1000)
    parser.add_argument("--captures", default="")
    parser.add_argument("--script", default="player-turns")
    parser.add_argument("--stage", type=int, default=1)
    parser.add_argument("--out", type=Path, default=None)
    args = parser.parse_args()
    if hashlib.sha256(ROM.read_bytes()).hexdigest() != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    out = args.out if args.out else BASE / "reference" / args.name
    out.mkdir(parents=True, exist_ok=True)
    home = BASE / "fceux-home"
    home.mkdir(parents=True, exist_ok=True)
    (home / "fceux.cfg").write_text("SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\n")
    (BASE / "runtime").mkdir(exist_ok=True)
    (ROOT / ".local/tmp").mkdir(exist_ok=True)
    captures = sorted({int(x) for x in args.captures.split(",") if x.strip()})
    capture_set = "{" + ",".join(f"[{n}]=true" for n in captures) + "}"
    if args.script == "player-turns":
        script = """
    if n == 120 or n == 240 or n == 360 then pad.start = true end
    if n >= 470 and n <= 560 then pad.right = true end
    if n >= 580 and n <= 670 then pad.up = true end
    if n >= 690 and n <= 780 then pad.down = true end
    if n >= 800 and n <= 890 then pad.left = true end
    if n >= 470 and n % 24 < 2 then pad.A = true end
"""
    elif args.script == "stage2":
        script = """
    if n == 120 or n == 240 or n == 360 then pad.start = true end
    if n == 420 then pad.start = true end
    if n >= 480 and n <= 900 then
      if n % 16 < 2 then pad.A = true end
      if n >= 500 and n < 700 then pad.right = true end
      if n >= 700 and n < 900 then pad.up = true end
    end
"""
    elif args.script == "players2":
        script = """
    if n == 160 then pad.down = true end
    if n == 260 or n == 320 or n == 380 then pad.start = true end
    if n >= 440 and n % 36 < 2 then pad.A = true end
"""
    elif args.script == "select-stage":
        script = f"""
    if n == 120 or n == 240 or n == 360 then pad.start = true end
    if n >= 360 and n <= 470 then memory.writebyte(0x85,{args.stage}) end
"""
    else:
        raise SystemExit(f"unknown script {args.script}")
    lua = f'''local out = assert(os.getenv("NES_PROBE_OUT"))
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local function bytes(first,count,reader)
  local t={{}}; for i=first,first+count-1 do t[#t+1]=string.char(reader(i)) end
  return table.concat(t)
end
local captures = {capture_set}
local oam = string.rep(string.char(255),256)
local ctrl = 0
memory.registerwrite(0x4014,1,function(a,s,v) oam = bytes(v*256,256,memory.readbyte) end)
memory.registerwrite(0x2000,1,function(a,s,v) ctrl = v end)
emu.poweron()
emu.speedmode("turbo")
for n=1,{args.frames} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  {script}
  joypad.set(1,pad)
  emu.frameadvance()
  if captures[emu.framecount()] then
    local stem=string.format("frame-%04d",emu.framecount())
    writefile(stem..".gd",gui.gdscreenshot(true))
    writefile(stem.."-oam.bin",oam)
    writefile(stem.."-ppu.bin",bytes(0,0x4000,ppu.readbyte))
    writefile(stem.."-palette.bin",bytes(0x3f00,32,ppu.readbyte))
    writefile(stem.."-name.bin",bytes(0x2000,0x1000,ppu.readbyte))
    writefile(stem.."-ctrl.bin",string.char(ctrl))
    writefile(stem.."-ram.bin",bytes(0x0000,0x0800,memory.readbyte))
  end
end
writefile("capture-complete.tmp",'{{"status":"complete"}}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''
    script_path = out / "record.lua"
    script_path.write_text(lua)
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
    cmd = ["sandbox-exec", "-f", str(SANDBOX), FCEUX, "--sound", "0", "--loadlua", str(script_path), str(ROM)]
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
                    raise TimeoutError("sprite capture timed out")
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
    (out / "source.json").write_text(
        json.dumps(
            {"rom_sha256": ROM_SHA256, "frames": args.frames, "captures": captures, "script": args.script},
            indent=2,
        )
        + "\n"
    )
    print(json.dumps({"name": args.name, "captures": captures}, indent=2))


if __name__ == "__main__":
    main()
