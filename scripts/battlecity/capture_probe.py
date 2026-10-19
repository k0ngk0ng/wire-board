#!/usr/bin/env python3
"""Headless FCEUX reference capture for Battle City research.

The emulator is only a development/reference tool.  It runs with a private
config and writes only under .local/battlecity.  No ROM data enters Git.
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


def parse_frames(value: str) -> list[int]:
    frames: set[int] = set()
    for part in value.split(","):
        part = part.strip()
        if not part:
            continue
        if "-" in part:
            a, b = part.split("-", 1)
            frames.update(range(int(a), int(b) + 1))
        else:
            frames.add(int(part))
    return sorted(frames)


def lua_script(frames: int, captures: list[int], presses: dict[int, str], out: Path) -> str:
    capture_set = "{" + ",".join(f"[{n}]=true" for n in captures) + "}"
    segments = []
    for frame, key in sorted(presses.items()):
        segments.append(f"if n == {frame} then pad[{json.dumps(key)}]=true end")
    segments_lua = "\n".join(segments)
    return f'''local out = assert(os.getenv("NES_PROBE_OUT"))
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
local function after()
  local n = emu.framecount()
  if captures[n] then
    writefile(string.format("frame-%04d.gd",n), gui.gdscreenshot(true))
    writefile(string.format("frame-%04d-oam.bin",n), oam)
    writefile(string.format("frame-%04d-ppu.bin",n), bytes(0,0x4000,ppu.readbyte))
    writefile(string.format("frame-%04d-palette.bin",n), bytes(0x3f00,32,ppu.readbyte))
    writefile(string.format("frame-%04d-name.bin",n), bytes(0x2000,0x1000,ppu.readbyte))
    writefile(string.format("frame-%04d-ctrl.bin",n), string.char(ctrl))
  end
end
memory.registerwrite(0x4014,1,function(a,s,v)
  oam = bytes(v*256,256,memory.readbyte)
end)
memory.registerwrite(0x2000,1,function(a,s,v) ctrl=v end)
local colors={{}}
for i=0,63 do local r,g,b,a=gui.parsecolor(string.format("P%02x",i)); colors[#colors+1]=string.char(r,g,b) end
writefile("nes-colors.rgb",table.concat(colors))
emu.poweron()
emu.speedmode("turbo")
for n=1,{frames} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  {segments_lua}
  joypad.set(1,pad)
  emu.frameadvance()
  after()
end
writefile("capture-complete.tmp",'{{"status":"complete"}}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", default="probe")
    parser.add_argument("--frames", type=int, default=900)
    parser.add_argument("--captures", default="60,120,180,240,300,360,420,480,540,600,660,720,780,840,900")
    parser.add_argument("--press", action="append", default=[], help="frame:key, e.g. 120:start")
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
    presses = {}
    for item in args.press:
        frame, key = item.split(":", 1)
        presses[int(frame)] = key
    captures = parse_frames(args.captures)
    script = out / "record.lua"
    script.write_text(lua_script(args.frames, captures, presses, out))
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
    cmd = [
        "sandbox-exec",
        "-f",
        str(SANDBOX),
        FCEUX,
        "--sound",
        "0",
        "--loadlua",
        str(script),
        str(ROM),
    ]
    with (out / "emulator.log").open("w") as log:
        proc = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
        started = time.monotonic()
        completed = False
        try:
            while proc.poll() is None:
                if completion.is_file():
                    completed = True
                    break
                if time.monotonic() - started > 60:
                    raise TimeoutError("FCEUX capture timed out")
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
        if p.suffix in {".bin", ".png", ".gd", ".lua", ".rgb"}
    }
    (out / "hashes.json").write_text(json.dumps(hashes, indent=2) + "\n")
    (out / "source.json").write_text(
        json.dumps(
            {
                "rom_sha256": ROM_SHA256,
                "frames": args.frames,
                "captures": captures,
                "presses": {str(k): v for k, v in sorted(presses.items())},
            },
            indent=2,
        )
        + "\n"
    )
    print(json.dumps({"name": args.name, "png": [p.name for p in sorted(out.glob("*.png"))]}, indent=2))


if __name__ == "__main__":
    main()
