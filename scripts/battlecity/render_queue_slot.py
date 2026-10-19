#!/usr/bin/env python3
"""Render a captured sound-queue slot in isolation for research.

The game is started normally, then the 256-byte queue is cleared and one
slot's captured flag/data is written back.  SDL disk audio records the result.
This is an offline research tool; it is not used by the web game.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import time
import wave
import array
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / ".local/battlecity"
ROM = BASE / "rom/Battle City (J).nes"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
FCEUX = "/opt/homebrew/bin/fceux"
SANDBOX = ROOT / ".local/nes-basket/workspace.sb"
RATE = 44100


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", required=True)
    parser.add_argument("--slot", type=int, required=True)
    parser.add_argument("--queue", type=Path, required=True)
    parser.add_argument("--seconds", type=float, default=2.0)
    parser.add_argument("--start", type=int, default=600)
    parser.add_argument("--hold", type=int, default=16)
    parser.add_argument("--skip", type=float, default=2.0)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()
    if sha(ROM) != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    if not 0 <= args.slot <= 27:
        raise SystemExit("slot must be 0..27")
    queue_path = args.queue.resolve()
    queue = queue_path.read_bytes()
    if len(queue) != 256:
        raise SystemExit("queue snapshot must be 256 bytes ($0300-$03FF)")
    flag = queue[args.slot]
    data = queue[0x1C + args.slot * 8 : 0x1C + args.slot * 8 + 8]
    out = args.out or BASE / "audio-slots" / args.name
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
    total = args.start + int(args.seconds * 60) + 30
    data_lua = ",".join(str(b) for b in data)
    lua = f'''local out = assert(os.getenv("NES_PROBE_OUT"))
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local data={{{data_lua}}}
emu.poweron()
emu.speedmode("normal")
for n=1,{total} do
  local pad={{A=false,B=false,start=false,select=false,up=false,down=false,left=false,right=false}}
  if n==120 or n==240 or n==360 then pad.start=true end
  joypad.set(1,pad)
  emu.frameadvance()
  local f=emu.framecount()
  if f=={args.start} then
    for i=0,255 do memory.writebyte(0x300+i,0) end
    memory.writebyte(0x300+{args.slot},{flag})
    for i=1,8 do memory.writebyte(0x31C+{args.slot}*8+i-1,data[i]) end
  end
  if f=={args.start}+{args.hold} then
    memory.writebyte(0x300+{args.slot},0)
    for i=1,8 do memory.writebyte(0x31C+{args.slot}*8+i-1,0) end
  end
  if f>={args.start} then
    for i=0,27 do
      if i~={args.slot} then memory.writebyte(0x300+i,0) end
    end
  end
end
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
                if time.monotonic() - started > args.seconds + 45:
                    raise TimeoutError("slot render timed out")
                time.sleep(0.05)
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
    if not completed:
        raise SystemExit(f"render failed; see {out}/emulator.log")
    pcm = raw.read_bytes()
    wav = out / "capture.wav"
    with wave.open(str(wav), "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(pcm[: len(pcm) // 4 * 4])
    # Trim the isolated injected segment, ignoring the power-on pop and the
    # initial title/demo silence.  The game clears the slot after --hold frames.
    with wave.open(str(wav), "rb") as w:
        channels, rate, frames = w.getnchannels(), w.getframerate(), w.getnframes()
        samples = array.array("h", w.readframes(frames))
    if channels > 1:
        mono = samples[::channels]
    else:
        mono = samples
    skip = int(args.skip * rate)
    active = [i for i in range(skip, len(mono)) if abs(mono[i]) >= 8]
    if not active:
        raise SystemExit(f"{args.name}: no isolated audio after {args.skip}s")
    first = max(skip, active[0] - int(0.01 * rate))
    end = min(len(mono), active[-1] + int(0.02 * rate))
    sfx = wav.with_name("sfx.wav")
    with wave.open(str(sfx), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        w.writeframes(mono[first:end].tobytes())
    meta = {
        "name": args.name, "slot": args.slot, "flag": flag, "data": list(data),
        "hold_frames": args.hold, "sfx_bytes": sfx.stat().st_size,
        "sfx_sha256": sha(sfx), "sfx_duration_s": round((end-first)/rate, 4),
        "queue": str(queue_path.relative_to(ROOT)), "queue_sha256": sha(queue_path),
        "rom_sha256": ROM_SHA256, "raw_sha256": sha(raw), "wav_sha256": sha(wav),
        "wav_bytes": wav.stat().st_size,
    }
    (out / "render.json").write_text(json.dumps(meta, indent=2) + "\n")
    print(json.dumps(meta, indent=2))


if __name__ == "__main__":
    main()
