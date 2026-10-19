#!/usr/bin/env python3
"""Render one captured sound-queue slot from a synthetic NSF carrier.

The NSF contains the original PRG twice (NROM-128 mirroring) so the sound
engine's mirrored jumps work.  A tiny wrapper at $BF00 calls the original
sound INIT ($AA51), writes the captured 8-byte slot state, and returns; PLAY
is the original $AA7E.  No game code runs, so no music is requested.  This is
an offline research tool; it is not part of the web game.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import struct
import subprocess
import time
import wave
from pathlib import Path

import array

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / ".local/battlecity"
ROM = BASE / "rom/Battle City (J).nes"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
FCEUX = "/opt/homebrew/bin/fceux"
SANDBOX = ROOT / ".local/nes-basket/workspace.sb"
RATE = 44100
WRAP = 0xBF00
TABLE = 0xBF20


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def carrier(prg: bytes, slot: int, flag: int, data: bytes) -> bytes:
    if len(prg) != 16384 or len(data) != 8:
        raise ValueError("bad PRG/slot data")
    code = bytearray()
    code += bytes((0x20, 0x51, 0xAA))  # JSR $AA51
    code += bytes((0xA2, 0x07))  # LDX #$07
    loop = WRAP + len(code)
    code += bytes((0xBD, TABLE & 0xFF, TABLE >> 8))  # LDA table,X
    target = 0x031C + slot * 8
    code += bytes((0x9D, target & 0xFF, target >> 8))  # STA target,X
    code += bytes((0xCA,))  # DEX
    code += bytes((0x10, (loop - (WRAP + len(code) + 2)) & 0xFF))  # BPL loop
    code += bytes((0xA9, flag & 0xFF))  # LDA #flag
    code += bytes((0x8D, (0x0300 + slot) & 0xFF, (0x0300 + slot) >> 8))  # STA $0300+slot
    code += bytes((0x60,))  # RTS
    if WRAP + len(code) > TABLE:
        raise ValueError("wrapper overlaps table")
    image = bytearray(prg + prg)
    image[WRAP - 0x8000 : WRAP - 0x8000 + len(code)] = code
    image[TABLE - 0x8000 : TABLE - 0x8000 + 8] = data
    header = bytearray(128)
    header[:8] = b"NESM\x1a\x01\x01\x01"
    struct.pack_into("<HHH", header, 8, 0x8000, WRAP, 0xAA7E)
    name = f"Battle City slot {slot}".encode()
    header[14 : 14 + len(name)] = name
    struct.pack_into("<H", header, 110, 16639)
    return bytes(header) + bytes(image)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", required=True)
    parser.add_argument("--slot", type=int, required=True)
    parser.add_argument("--queue", type=Path, required=True)
    parser.add_argument("--seconds", type=float, default=2.0)
    parser.add_argument("--skip", type=float, default=0.0)
    args = parser.parse_args()
    rom = ROM.read_bytes()
    if hashlib.sha256(rom).hexdigest() != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    queue_path = args.queue.resolve()
    queue = queue_path.read_bytes()
    if len(queue) != 256:
        raise SystemExit("queue snapshot must be 256 bytes")
    flag = queue[args.slot]
    data = queue[0x1C + args.slot * 8 : 0x1C + args.slot * 8 + 8]
    if flag == 0 or not any(data):
        raise SystemExit("slot has no captured state")
    out = BASE / "audio-nsf" / args.name
    out.mkdir(parents=True, exist_ok=True)
    nsf = out / "sound.nsf"
    nsf.write_bytes(carrier(rom[16 : 16 + 16384], args.slot, flag, data))
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
    frames = int(args.seconds * 60) + 30
    lua = f'''local out=assert(os.getenv("NES_PROBE_OUT"))
emu.speedmode("normal")
for n=1,{frames} do emu.frameadvance() end
local f=assert(io.open(out.."/capture-complete.tmp","w")); f:write("complete"); f:close()
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''
    lua_path = out / "record.lua"
    lua_path.write_text(lua)
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
    cmd = ["sandbox-exec", "-f", str(SANDBOX), FCEUX, "--sound", "1", "--loadlua", str(lua_path), str(nsf)]
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
                    raise TimeoutError("NSF render timed out")
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
    with wave.open(str(wav), "rb") as w:
        channels = w.getnchannels()
        rate = w.getframerate()
        samples = array.array("h", w.readframes(w.getnframes()))
    mono = samples[::channels]
    skip = int(args.skip * rate)
    active = [i for i in range(skip, len(mono)) if abs(mono[i]) >= 8]
    if not active:
        raise SystemExit(f"{args.name}: silent")
    first = max(skip, active[0] - int(0.01 * rate))
    end = min(len(mono), active[-1] + int(0.02 * rate))
    sfx = out / "sfx.wav"
    with wave.open(str(sfx), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        w.writeframes(mono[first:end].tobytes())
    meta = {
        "name": args.name, "slot": args.slot, "flag": flag, "data": list(data),
        "queue": str(queue_path.relative_to(ROOT)), "queue_sha256": sha(queue_path),
        "nsf_sha256": sha(nsf), "raw_sha256": sha(raw), "wav_sha256": sha(wav),
        "sfx_sha256": sha(sfx), "sfx_bytes": sfx.stat().st_size,
        "sfx_duration_s": round((end - first) / rate, 4),
        "rom_sha256": ROM_SHA256,
    }
    (out / "render.json").write_text(json.dumps(meta, indent=2) + "\n")
    print(json.dumps(meta, indent=2))


if __name__ == "__main__":
    main()
