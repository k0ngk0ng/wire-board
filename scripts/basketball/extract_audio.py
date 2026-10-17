"""Render selected original short effects offline; never used by the game.

Source scripts/env.sh first. Requires the existing FCEUX macOS installation,
Clang and the pinned research ROM. No network access or global configuration.
Outputs are research candidates until platform playback is separately accepted.
"""
from pathlib import Path
import array
import csv
import hashlib
import json
import os
import struct
import subprocess
import sys
import time
import wave

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/basketball/audio/export'
BASE = ROOT / '.local/nes-basket'
SOURCE_SHA = '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
CATALOG = ROOT / 'games/basketball/assets/audio-catalog.json'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def carrier(rom, sound_id):
    header = bytearray(128)
    header[:8] = b'NESM\x1a\x01\x01\x01'
    struct.pack_into('<HHH', header, 8, 0x8000, 0xfff0, 0x8003)
    name = f'Basketball sound {sound_id}'.encode()
    header[14:14+len(name)] = name
    struct.pack_into('<H', header, 110, 16639)
    prg = rom[16:16+131072]
    data = bytearray(prg[:16384] + prg[-16384:])
    # NSF calls init once and the original play entry once per NTSC frame.
    # Original bank0 is unchanged. The wrapper resides in the unused (for
    # these non-DPCM sounds) fixed-bank interrupt-vector area.
    data[0x7ff0:0x7ffb] = bytes([0xa9,0,0x20,0,0x80,0xa9,sound_id,0x20,0,0x80,0x60])
    return header + data


LUA = r'''local out = assert(os.getenv("NES_AUDIO_OUT"))
local lib = assert(os.getenv("NES_AUDIO_BRIDGE"))
local calls = assert(io.open(out.."/calls.csv","w"))
local apu = assert(io.open(out.."/apu.csv","w"))
local driver = assert(io.open(out.."/driver-ram.bin","wb"))
local jumps = assert(io.open(out.."/sequence-jumps.csv","w"))
local step=0
jumps:write("step,video_frame,channel,command,target\n")
calls:write("frame,kind,value\n"); apu:write("frame,address,value\n")
memory.registerexec(0x8000,1,function()
 calls:write(emu.framecount()..",init,"..memory.getregister("a").."\n")
end)
memory.registerexec(0x8003,1,function()
 calls:write(emu.framecount()..",play,0\n")
end)
memory.registerwrite(0x4000,0x18,function(address,size,value)
 apu:write(emu.framecount()..","..address..","..value.."\n")
end)
memory.registerexec(0x8359,1,function()
 if memory.getregister("a")~=0xd0 then return end
 local command=memory.readbyte(0xfc)+256*memory.readbyte(0xfd)+memory.getregister("y")
 assert(memory.readbyte(command)==0xd0)
 local target=memory.readbyte(command+1)+256*memory.readbyte(command+2)
 jumps:write(step..","..emu.framecount()..","..memory.readbyte(0x704)..","..command..","..target.."\n")
end)
assert(package.loadlib(lib,"start"))()
local checkpoint=assert(package.loadlib(lib,"checkpoint"))
checkpoint()
emu.speedmode("turbo")
for n=1,tonumber(assert(os.getenv("NES_AUDIO_FRAMES"))) do
 step=n
 emu.frameadvance()
 checkpoint()
 local bytes={};for i=0x700,0x7e6 do bytes[#bytes+1]=string.char(memory.readbyte(i)) end
 driver:write(table.concat(bytes))
end
assert(package.loadlib(lib,"stop"))()
calls:close(); apu:close(); driver:close(); jumps:close()
local f=assert(io.open(out.."/complete.tmp","w")); f:write("complete"); f:close()
assert(os.rename(out.."/complete.tmp",out.."/complete"))
while true do end
'''


def record(nsf, run, bridge, frames, volume):
    run.mkdir(parents=True, exist_ok=True)
    marker = run / 'complete'
    marker.unlink(missing_ok=True)
    script = run / 'record.lua'
    script.write_text(LUA)
    home = OUT / 'fceux-home'
    home.mkdir(exist_ok=True)
    # The Qt/OpenGL default crashes once the offscreen window's delayed first
    # paint runs. Use FCEUX's software painter in this private capture profile.
    (home/'fceux.cfg').write_text('SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\n')
    env = dict(os.environ, QT_QPA_PLATFORM='offscreen', SDL_AUDIODRIVER='dummy',
               FCEUX_HOME=str(home), FCEUX_CONFIG_DIR=str(home),
               XDG_RUNTIME_DIR=str(BASE/'runtime'), TMPDIR=str(ROOT/'.local/tmp'),
               NES_AUDIO_OUT=str(run), NES_AUDIO_WAV=str(run/'full.wav'), NES_AUDIO_BRIDGE=str(bridge),
               NES_AUDIO_FRAMES=str(frames), NES_AUDIO_FRAME_COUNTS=str(run/'sample-cursors.csv'))
    cmd = ['sandbox-exec', '-f', str(BASE/'workspace.sb'), '/opt/homebrew/bin/fceux',
           '--pal', '0', '--sound', '1', '--soundrate', '44100',
           '--soundq', '2', '--volume', str(volume), '--loadlua', str(script), str(nsf)]
    with (run/'emulator.log').open('w') as log:
        process = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
        started = time.monotonic()
        try:
            while process.poll() is None and not marker.is_file():
                # Long music superperiods can span many original sequence
                # loops. Keep a bounded deadline proportional to declared work.
                if time.monotonic()-started > max(30,frames/1000):
                    raise TimeoutError(f'audio capture timed out: {run}')
                if 'Lua thread bombed out' in (run/'emulator.log').read_text():
                    raise RuntimeError(f'Lua capture failed: {run}')
                time.sleep(.05)
            assert marker.read_text() == 'complete', f'incomplete audio capture: {run}'
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()
    return {'wav': str((run/'full.wav').relative_to(OUT)),
            'calls': str((run/'calls.csv').relative_to(OUT)),
            'apu': str((run/'apu.csv').relative_to(OUT)),
            'driver': str((run/'driver-ram.bin').relative_to(OUT)),
            'jumps': str((run/'sequence-jumps.csv').relative_to(OUT)),
            'samples': str((run/'sample-cursors.csv').relative_to(OUT))}


def main():
    rom = (BASE/'basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest() == SOURCE_SHA
    assert rom[:4] == b'NES\x1a' and rom[4] == 8
    OUT.mkdir(parents=True, exist_ok=True)
    bridge = OUT / 'wave_bridge.dylib'
    subprocess.run(['clang', '-dynamiclib', '-Wall', '-Wextra', '-Werror', '-o', str(bridge),
                    str(ROOT/'scripts/basketball/audio_wave_bridge.c')], check=True)
    catalog = json.loads(CATALOG.read_text())
    manifest = {'rom_sha256': SOURCE_SHA, 'accepted_for_release': False, 'catalog_sha256': digest(CATALOG),
        'method': 'Unmodified bank0 sound driver/data in offline NSF; FCEUX original APU renderer; fixed-vector init wrapper resets driver then requests one ID; sample-exact trimming only.',
        'tool': {'name': 'FCEUX', 'version': '2.6.6_12 / git 34eb7601c415b81901fd02afbd5cfdc84b5047ac',
                 'sha256': digest(Path('/opt/homebrew/Cellar/fceux/2.6.6_12/libexec/fceux'))},
        'source_scripts': {str(p.relative_to(ROOT)): digest(p) for p in [Path(__file__).resolve(), ROOT/'scripts/basketball/audio_wave_bridge.c', ROOT/'scripts/basketball/verify_audio.py']},
        'controls': catalog['controls'],
        'limitations': ['Only currently implemented effect IDs and observed NY music ID66 are exported; remaining IDs are not accepted.', 'DPCM is accepted only when every fetched range lies wholly in unmodified source bytes.', 'Concurrent channel-priority behavior in original gameplay is not reproduced by isolated clips.', 'Sequence jump markers and a repeating driver state are not an accepted seamless PCM loop.', 'Original ROM ownership/licensing is not established by extraction; no public/OSS upload performed.', 'Browser platform audio integration and listening acceptance remain pending.'],
        'sounds': []}
    for definition in catalog['effects'] + catalog['music']:
        sound_id,frames = definition['id'],definition['capture_frames']
        volume=definition.get('render_volume',255)
        nsf = OUT / f'sound-{sound_id:03}.nsf'
        nsf.write_bytes(carrier(rom, sound_id))
        runs = [record(nsf, OUT/f'sound-{sound_id:03}-run-{n}', bridge,frames,volume) for n in (1,2)]
        with wave.open(str(OUT/runs[0]['wav'])) as f:
            samples = array.array('h', f.readframes(f.getnframes()))
        if sys.byteorder != 'little':
            samples.byteswap()
        active = [i for i,s in enumerate(samples) if abs(s) >= 8]
        assert active, f'silent sound {sound_id}'
        first, end = max(0, active[0]-882), min(len(samples),active[-1]+4411)
        loop = None
        if definition['kind']=='music':
            first,end=0,len(samples)
            driver=(OUT/runs[0]['driver']).read_bytes()
            seen={}
            loop={'status':'unresolved','pcm_seam_accepted':False}
            for index in range(3,frames):
                state=driver[index*231:(index+1)*231]
                if state in seen and index-seen[state]>60:
                    start,finish=seen[state]+1,index+1
                    counts=list(csv.DictReader((OUT/runs[0]['samples']).open()))
                    loop.update(status='driver-state-repeat',driver_frames=[start,finish],sample_range=[int(counts[start]['samples']),int(counts[finish]['samples'])])
                    break
                seen.setdefault(state,index)
            if loop['status']=='unresolved':
                jumps=[{k:int(v) for k,v in row.items()} for row in csv.DictReader((OUT/runs[0]['jumps']).open()) if int(row['channel'])==0]
                if len(jumps)>=2 and jumps[0]['target']==jumps[1]['target']:
                    counts=list(csv.DictReader((OUT/runs[0]['samples']).open()))
                    loop.update(status='sequence-loop-candidate',markers=jumps,
                                sample_range=[int(counts[jumps[0]['step']]['samples']),int(counts[jumps[1]['step']]['samples'])],
                                boundary='end of video frame containing original channel0 D0 jump; oscillator and tempo phase continuity not accepted')
        pcm = samples[first:end]
        if sys.byteorder != 'little':
            pcm.byteswap()
        wav = OUT/definition['wav']
        with wave.open(str(wav), 'wb') as f:
            f.setparams((1,2,44100,0,'NONE','not compressed'))
            f.writeframes(pcm.tobytes())
        files = [nsf.name, wav.name] + [name for run in runs for name in run.values()]
        address=length=None;regions=set()
        for row in csv.DictReader((OUT/runs[0]['apu']).open()):
            register,value=int(row['address']),int(row['value'])
            if register==0x4012:address=0xc000+value*64
            if register==0x4013:length=value*16+1
            if register==0x4015 and value&16:
                assert address is not None and length is not None
                regions.add((address,length))
        entry={'id': sound_id, 'kind':definition['kind'],'sources':definition['sources'],'capture_frames':frames,'nsf': nsf.name, 'wav': wav.name,
            'dpcm_regions':sorted(regions),'render_volume':volume,
            'sample_range': [first,end], 'duration_seconds': (end-first)/44100,
            'runs': runs, 'sha256': {name:digest(OUT/name) for name in files}}
        if loop is not None:entry['loop']=loop
        manifest['sounds'].append(entry)
        print(f'exported original sound {sound_id}: {(end-first)/44100:.3f}s', flush=True)
    (OUT/'manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')


if __name__ == '__main__':
    main()
