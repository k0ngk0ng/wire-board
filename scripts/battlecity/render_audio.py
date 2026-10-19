#!/usr/bin/env python3
"""Capture isolated Battle City sound streams from the ROM.

FCEUX's Qt build has the WAV auto-start disabled, so this script records the
SDL disk-audio driver's raw S16LE stereo output instead.  Every frame it
clears all sound triggers except the requested slots; at --start frame it
writes trigger 1 to those slots.  With no other triggers active, the game's
title/battle music is silenced and the requested stream can be analysed in
isolation.

Research helper only: output stays under .local.  Nothing is synthesized.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import time
import wave
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / ".local/battlecity"
ROM = BASE / "rom/Battle City (J).nes"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
FCEUX = "/opt/homebrew/bin/fceux"
SANDBOX = ROOT / ".local/nes-basket/workspace.sb"
RATE = 44100


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", required=True)
    parser.add_argument("--slots", required=True, help="comma-separated sound slots, e.g. 1,2,3")
    parser.add_argument("--seconds", type=float, default=6.0)
    parser.add_argument("--warmup", type=int, default=300)
    parser.add_argument("--start", type=int, default=360)
    parser.add_argument("--no-clear", action="store_true", help="leave game-triggered sounds alone")
    parser.add_argument("--press-start", default="", help="comma-separated frames to press Start for two frames")
    parser.add_argument("--press-a", default="", help="comma-separated frames to press A for two frames")
    parser.add_argument("--log-triggers", action="store_true", help="record writes to $0300-$031B")
    parser.add_argument("--poke", default="", help="comma-separated frame:addr:value writes")
    parser.add_argument("--screenshot-at", default="", help="comma-separated frames to save screenshots")
    parser.add_argument("--out", type=Path, default=None)
    args = parser.parse_args()
    if sha256(ROM) != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    slots = sorted({int(x, 0) for x in args.slots.split(",") if x.strip()})
    if any(not 0 <= slot <= 27 for slot in slots):
        raise SystemExit("slots must be 0..27")
    out = args.out or BASE / "audio-raw" / args.name
    out.mkdir(parents=True, exist_ok=True)
    home = BASE / "fceux-audio-home"
    home.mkdir(parents=True, exist_ok=True)
    (home / "fceux.cfg").write_text(
        "SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\n"
        "SDL.Sound = 1\nSDL.Sound.Mute = 0\nSDL.Sound.Volume = 255\n"
        f"SDL.Sound.Rate = {RATE}\nSDL.Sound.Quality = 1\n"
        "SDL.Sound.Square1Volume = 255\nSDL.Sound.Square2Volume = 255\n"
        "SDL.Sound.TriangleVolume = 255\nSDL.Sound.NoiseVolume = 255\n"
        "SDL.Sound.PCMVolume = 255\nSDL.Sound.BufSize = 128\n"
        "SDL.Sound.RecordFile =\n"
    )
    (BASE / "runtime").mkdir(exist_ok=True)
    (ROOT / ".local/tmp").mkdir(exist_ok=True)
    total_frames = max(args.start, 0) + int(args.seconds * 60) + 30
    slot_list = "{" + ",".join(str(s) for s in slots) + "}"
    start_frames = [int(x) for x in args.press_start.split(",") if x.strip()]
    a_frames = [int(x) for x in args.press_a.split(",") if x.strip()]
    start_list = "{" + ",".join(str(x) for x in start_frames) + "}"
    a_list = "{" + ",".join(str(x) for x in a_frames) + "}"
    pokes = []
    for item in args.poke.split(","):
        if not item.strip():
            continue
        frame, addr, value = (int(x, 0) for x in item.split(":"))
        pokes.append((frame, addr, value))
    poke_list = "{" + ",".join(f"{{{f},{a},{v}}}" for f, a, v in pokes) + "}"
    shot_frames = [int(x) for x in args.screenshot_at.split(",") if x.strip()]
    shot_list = "{" + ",".join(str(x) for x in shot_frames) + "}"
    lua = f'''local out = assert(os.getenv("NES_PROBE_OUT"))
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local slots = {slot_list}
local press_start = {start_list}
local press_a = {a_list}
local pokes = {poke_list}
local shots = {shot_list}
local triglog = {{}}
local function pressed(list, n)
  for _,f in ipairs(list) do if n >= f and n < f + 2 then return true end end
  return false
end
local function want_shot(f)
  for _,v in ipairs(shots) do if f == v then return true end end
  return false
end
if {str(args.log_triggers).lower()} then
  for i=0,27 do
    memory.registerwrite(0x300+i,1,function(a,s,v)
      triglog[#triglog+1]=string.format("%d %03X %d",emu.framecount(),a,v)
    end)
  end
end
emu.poweron()
emu.speedmode("normal")
for n=1,{total_frames} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  if pressed(press_start,n) then pad.start = true end
  if pressed(press_a,n) then pad.A = true end
  joypad.set(1,pad)
  emu.frameadvance()
  local f = emu.framecount()
  memory.writebyte(0x6D,0)
  if not {str(args.no_clear).lower()} and f >= {args.warmup} then
    for i=0,27 do
      local keep=false
      for _,s in ipairs(slots) do if i==s then keep=true end end
      if not keep then memory.writebyte(0x300+i,0) end
    end
  end
  if {args.start} > 0 and f == {args.start} then
    for _,s in ipairs(slots) do memory.writebyte(0x300+s,1) end
  end
  for _,p in ipairs(pokes) do
    if f == p[1] then memory.writebyte(p[2],p[3]) end
  end
  if want_shot(f) then writefile(string.format("shot-%04d.gd",f),gui.gdscreenshot(true)) end
end
if {str(args.log_triggers).lower()} then writefile("triggers.log",table.concat(triglog,"\\n").."\\n") end
writefile("capture-complete.tmp",'{{"status":"complete"}}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''
    script = out / "record.lua"
    script.write_text(lua)
    raw = out / "capture.raw"
    raw.unlink(missing_ok=True)
    completion = out / "capture-complete.json"
    completion.unlink(missing_ok=True)
    env = dict(
        os.environ,
        FCEUX_HOME=str(home),
        FCEUX_CONFIG_DIR=str(home),
        QT_QPA_PLATFORM="offscreen",
        SDL_AUDIODRIVER="disk",
        SDL_DISKAUDIOFILE=str(raw),
        XDG_RUNTIME_DIR=str(BASE / "runtime"),
        NES_PROBE_OUT=str(out),
        TMPDIR=str(ROOT / ".local/tmp"),
    )
    cmd = ["sandbox-exec", "-f", str(SANDBOX), FCEUX, "--sound", "1", "--loadlua", str(script), str(ROM)]
    with (out / "emulator.log").open("w") as log:
        proc = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
        started = time.monotonic()
        completed = False
        try:
            while proc.poll() is None:
                if completion.is_file():
                    completed = True
                    break
                if time.monotonic() - started > args.seconds + 30:
                    raise TimeoutError("audio capture timed out")
                time.sleep(0.05)
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
    if not completed:
        raise SystemExit(f"capture failed; see {out}/emulator.log")
    for gd in sorted(out.glob("shot-*.gd")):
        data = gd.read_bytes()
        width = int.from_bytes(data[2:4], "big")
        height = int.from_bytes(data[4:6], "big")
        if len(data) != 11 + width * height * 4:
            raise SystemExit(f"bad screenshot {gd}")
        Image.frombytes("RGBA", (width, height), data[11:], "raw", "ARGB").convert("RGB").save(gd.with_suffix(".png"))
    pcm = raw.read_bytes()
    if len(pcm) < RATE * 4:
        raise SystemExit(f"short audio capture: {len(pcm)} bytes")
    wav_path = out / "capture.wav"
    with wave.open(str(wav_path), "wb") as wav:
        wav.setnchannels(2)
        wav.setsampwidth(2)
        wav.setframerate(RATE)
        wav.writeframes(pcm[: len(pcm) // 4 * 4])
    peak = max(abs(int.from_bytes(pcm[i : i + 2], "little", signed=True)) for i in range(0, len(pcm) // 4 * 4, 2))
    meta = {
        "name": args.name,
        "slots": slots,
        "start_frame": args.start,
        "seconds": args.seconds,
        "press_start": start_frames,
        "press_a": a_frames,
        "pokes": [{"frame": f, "addr": f"0x{a:04X}", "value": v} for f, a, v in pokes],
        "screenshots": shot_frames,
        "rom_sha256": ROM_SHA256,
        "capture_format": {"codec": "pcm_s16le", "channels": 2, "rate": RATE},
        "raw_bytes": len(pcm),
        "raw_sha256": sha256(raw),
        "wav_bytes": wav_path.stat().st_size,
        "wav_sha256": sha256(wav_path),
        "peak": peak,
    }
    (out / "capture.json").write_text(json.dumps(meta, indent=2) + "\n")
    print(json.dumps(meta, indent=2))


if __name__ == "__main__":
    main()
