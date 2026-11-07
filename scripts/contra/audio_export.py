"""Render source-bound original Contra audio offline through the existing APU.

Small init/play wrappers are placed in verified unused FF padding of the sound
bank. A 120-update quiet warmup excludes unrelated APU cold-start transients.
The original driver, tables and complete fixed bank remain byte-identical.
No source ROM, user preferences or speaker output are modified. Files are local
research candidates; platform playback and listening are separate acceptance.
"""
import argparse
import array
import csv
import gzip
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import time
import wave
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio'
OUT=BASE/'export'
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
EFFECTS={0x0b:'landing',0x16:'scene-preview-effect-16',0x18:'scene-preview-effect-18',0x1a:'normal-shot',0x1c:'machine-shot',0x24:'laser-shot',0x26:'flame-shot',0x28:'spread-shot',0x2f:'enemy-explosion',0x33:'boss-siren',0x38:'weapon-pickup',0x72:'player-death',0x74:'pause',0x75:'large-explosion'}
EFFECTS.update({0x0d:'waterfall-rock',0x10:'interlude-impact',0x13:'interlude-effect',0x19:'flame-machine',0x20:'interlude-explosion',0x2a:'impact-2a',0x2b:'giant-jump',0x2c:'impact-2c',0x2e:'impact-2e',0x30:'ice-grenade',0x34:'electric-shock',0x36:'tank-shot',0x39:'life-bonus',0x3d:'bridge-explosion',0x3e:'base-wall'})
LOOP_EFFECTS={0x14:'intro-flight'}
MUSIC={0x46:'jungle-hangar',0x4a:'waterfall',0x4e:'snowfield',0x52:'energy-zone',0x56:'alien-lair',0x5a:'bases',0x5e:'base-boss'}
CUES={0x62:'stage-clear',0x66:'final-stage-clear',0x6e:'game-over',0x77:'boss-defeated',0x7b:'heart-defeated'}
PERCUSSION={0x02:'music-noise-tick',0x7f:'dpcm-hat',0x80:'dpcm-snare',0x81:'dpcm-extra'}

LUA=r'''
local out=assert(os.getenv('NES_AUDIO_OUT'))
local lib=assert(os.getenv('NES_AUDIO_BRIDGE'))
local apu=assert(io.open(out..'/apu.csv','w'));apu:write('frame,address,value\n')
local calls=assert(io.open(out..'/calls.csv','w'));calls:write('frame,kind,value\n')
local driver=assert(io.open(out..'/driver.bin','wb'))
memory.registerexec(0x87a2,1,function()calls:write(string.format('%d,request,%d\n',emu.framecount(),memory.getregister('a')))end)
memory.registerexec(0x80d5,1,function()calls:write(string.format('%d,update,0\n',emu.framecount()))end)
memory.registerexec(0x8719,1,function()calls:write(string.format('%d,reset,0\n',emu.framecount()))end)
memory.registerwrite(0x4000,0x18,function(a,s,v)apu:write(string.format('%d,%d,%d\n',emu.framecount(),a,v))end)
assert(package.loadlib(lib,'start'))()
local checkpoint=assert(package.loadlib(lib,'checkpoint'));checkpoint()
emu.speedmode('turbo')
for frame=1,tonumber(assert(os.getenv('NES_AUDIO_FRAMES')))do
 emu.frameadvance();checkpoint();driver:write(memory.readbyterange(0x100,130))
end
assert(package.loadlib(lib,'stop'))();apu:close();calls:close();driver:close()
local f=assert(io.open(out..'/complete.tmp','w'));f:write('complete');f:close();assert(os.rename(out..'/complete.tmp',out..'/complete'))
while true do end
'''


def sha(data):return hashlib.sha256(data).hexdigest()
def carrier(rom,sound):
    prg=rom[16:131088];data=bytearray(prg[0x4000:0x8000]+prg[0x1c000:])
    assert data[0x3fe0:0x3fef]==bytes([255])*15 and data[0x3f80:0x3f9a]==bytes([255])*26
    data[0x3fe0:0x3fef]=bytes([0x20,0x0b,0xf9,0x20,0x19,0x87,0xa9,0,0x8d,0xf0,5,0x8d,0xf1,5,0x60])
    data[0x3f80:0x3f9a]=bytes([0xad,0xf1,5,0xd0,0x12,0xee,0xf0,5,0xad,0xf0,5,0xc9,120,0xd0,8,0xee,0xf1,5,0xa9,sound,0x20,0xa2,0x87,0x4c,0xd5,0x80])
    header=bytearray(128);header[:8]=b'NESM\x1a\x01\x01\x01'
    struct.pack_into('<HHH',header,8,0x8000,0xbfe0,0xbf80)
    title=f'Contra Japan {sound:02x}'.encode();header[14:14+len(title)]=title
    struct.pack_into('<H',header,110,16639)
    return header+data


def record(nsf,path,bridge,frames,lua=LUA,channel_volumes=None):
    if path.exists():
        assert (path/'complete').read_text()=='complete'
        return
    path.mkdir(parents=True)
    (path/'record.lua').write_text(lua)
    home=path/'fceux-home';home.mkdir(exist_ok=True)
    (home/'fceux.cfg').write_text('SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\nSDL.NoConfig = 1\n')
    env=dict(os.environ,QT_QPA_PLATFORM='offscreen',SDL_AUDIODRIVER='dummy',FCEUX_HOME=str(home),FCEUX_CONFIG_DIR=str(home),
             XDG_RUNTIME_DIR=str(BASE/'runtime'),TMPDIR=str(BASE/'tmp'),NES_AUDIO_OUT=str(path),NES_AUDIO_WAV=str(path/'full.wav'),
             NES_AUDIO_BRIDGE=str(bridge),NES_AUDIO_FRAMES=str(frames),NES_AUDIO_FRAME_COUNTS=str(path/'samples.csv'))
    cmd=['sandbox-exec','-f',str(BASE/'workspace.sb'),'/opt/homebrew/bin/fceux','--pal','0','--sound','1','--soundrate','44100','--soundq','2','--volume','192','--loadlua',str(path/'record.lua'),str(nsf)]
    if channel_volumes:
        for channel,value in channel_volumes.items():cmd[-1:-1]=['--'+channel+'vol',str(value)]
    with (path/'emulator.log').open('w') as log:
        process=subprocess.Popen(cmd,env=env,stdout=log,stderr=subprocess.STDOUT)
        start=time.monotonic()
        try:
            while process.poll() is None and not (path/'complete').exists():
                if time.monotonic()-start>max(40,frames/600):raise TimeoutError(str(path))
                if 'Lua thread bombed out' in (path/'emulator.log').read_text():raise RuntimeError(str(path))
                time.sleep(.05)
            assert (path/'complete').read_text()=='complete'
        finally:
            if process.poll() is None:process.kill()
            process.wait()


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--ids',default=','.join(str(i)for i in list(EFFECTS)+list(MUSIC)+list(CUES)+list(PERCUSSION)+list(LOOP_EFFECTS)));parser.add_argument('--music-frames',type=int,default=8000);args=parser.parse_args()
    ids=[int(s,0) for s in args.ids.split(',')];assert all(s in EFFECTS or s in MUSIC or s in CUES or s in PERCUSSION or s in LOOP_EFFECTS for s in ids)
    for p in [OUT,BASE/'runtime',BASE/'tmp']:p.mkdir(parents=True,exist_ok=True)
    (BASE/'workspace.sb').write_text('(version 1)\n(allow default)\n(deny file-write*)\n'+f'(allow file-write* (subpath {json.dumps(str(ROOT))}) (literal "/dev/null") (literal "/dev/tty"))\n')
    bridge=OUT/'wave-bridge.dylib'
    subprocess.run(['sandbox-exec','-f',str(BASE/'workspace.sb'),'clang','-dynamiclib','-Wall','-Wextra','-Werror','-o',str(bridge),str(ROOT/'scripts/contra/audio_wave_bridge.c')],env=dict(os.environ,TMPDIR=str(BASE/'tmp'),CLANG_MODULE_CACHE_PATH=str(BASE/'clang-cache')),check=True)
    rom_path=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip');before=sha(rom_path.read_bytes())
    with zipfile.ZipFile(rom_path) as archive:rom=archive.read('Contra (J).nes')
    assert sha(rom)==SHA
    manifest={'romSHA256':SHA,'archiveSHA256':before,'acceptedForRelease':False,
              'method':'Original sound driver/data executed offline in FCEUX APU; bounded init/play wrappers in verified FF padding; 120-update quiet warmup then exactly one request; sample-exact trimming only.',
              'renderVolume':192,'sampleRate':44100,'channels':1,'bits':16,'speakerOutput':'SDL_AUDIODRIVER=dummy',
              'toolSHA256':sha(Path('/opt/homebrew/bin/fceux').read_bytes()),'bridgeSHA256':sha(bridge.read_bytes()),'sounds':[]}
    executable=Path('/opt/homebrew/Cellar/fceux/2.6.6_12/libexec/fceux')
    manifest['tool']={'version':'FCEUX 2.6.6_12 Homebrew','launcher':'/opt/homebrew/bin/fceux',
                      'launcherSHA256':manifest['toolSHA256'],'executable':str(executable),'executableSHA256':sha(executable.read_bytes())}
    manifest_path=OUT/'manifest.json'
    if manifest_path.exists():
        old=json.loads(manifest_path.read_text());assert old['romSHA256']==SHA
        manifest['sounds']=[s for s in old['sounds'] if s['id'] not in ids]
    for sound in ids:
        kind='effect' if sound in EFFECTS else 'cue' if sound in CUES else 'percussion' if sound in PERCUSSION else 'loop-effect' if sound in LOOP_EFFECTS else 'music'
        content_frames=360 if kind in ('effect','percussion') else 600 if kind=='cue' else 1200 if kind=='loop-effect' else args.music_frames
        frames=content_frames+120
        nsf=OUT/f'sound-{sound:02x}.nsf';nsf.write_bytes(carrier(rom,sound))
        runs=[f'sound-{sound:02x}-warm120-{frames}-run-{n}' for n in (1,2)]
        for name in runs:record(nsf,OUT/name,bridge,frames)
        with wave.open(str(OUT/runs[0]/'full.wav')) as wav:samples=array.array('h',wav.readframes(wav.getnframes()))
        if sys.byteorder!='little':samples.byteswap()
        requests=[r for r in csv.DictReader((OUT/runs[0]/'calls.csv').open()) if r['kind']=='request' and int(r['value'])==sound]
        assert len(requests)==1,(sound,'target must be requested exactly once')
        request_frame=int(requests[0]['frame']);cursors=list(csv.DictReader((OUT/runs[0]/'samples.csv').open()))
        origin=int(cursors[request_frame]['samples'])
        assert origin>4410 and max(abs(v)for v in samples[origin-4410:origin])<8,(sound,'warmup did not settle')
        active=[i for i in range(origin,len(samples)) if abs(samples[i])>=8];assert active,(sound,'silent')
        # Keep the real request-to-note delay. Removing initial in-game silence
        # would make a correctly scheduled media event audibly early.
        first,last=(origin,min(len(samples),active[-1]+4411)) if kind not in ('music','loop-effect') else (origin,len(samples))
        selected=samples[first:last];outfile=OUT/f'{kind}-{sound:02x}.wav'
        with wave.open(str(outfile),'wb') as wav:
            wav.setparams((1,2,44100,0,'NONE','not compressed'));wav.writeframes(selected.tobytes())
        entry={'id':sound,'label':(EFFECTS|MUSIC|CUES|PERCUSSION|LOOP_EFFECTS)[sound],'kind':kind,'captureFrames':frames,'carrier':nsf.name,'file':outfile.name,
               'sampleRange':[first,last],'durationSeconds':len(selected)/44100,'sha256':sha(outfile.read_bytes()),'runs':runs,
               'warmupFrames':120,'requestFrame':request_frame,'requestSampleOrigin':origin}
        if kind in ('music','loop-effect'):
            driver=(OUT/runs[0]/'driver.bin').read_bytes();assert len(driver)==frames*130
            seen={};loop={'status':'unresolved','pcmSeamAccepted':False}
            for n in range(request_frame,frames):
                state=driver[n*130:(n+1)*130]
                if any(state[6:12]) and state in seen and n-seen[state]>60:
                    start,end=seen[state]+1,n+1;cursors=list(csv.DictReader((OUT/runs[0]/'samples.csv').open()))
                    source_range=[int(cursors[start]['samples']),int(cursors[end]['samples'])]
                    loop.update(status='driver-state-repeat',driverFrames=[start,end],sourceSampleRange=source_range,sampleRange=[n-first for n in source_range]);break
                seen.setdefault(state,n)
            entry['loop']=loop
        manifest['sounds'].append(entry);manifest['sounds'].sort(key=lambda s:s['id'])
        manifest_path.write_text(json.dumps(manifest,indent=2)+'\n')
        print(json.dumps({'id':sound,'kind':kind,'seconds':entry['durationSeconds'],'loop':entry.get('loop')}),flush=True)
    assert sha(rom_path.read_bytes())==before
    assert sha(executable.read_bytes())==manifest['tool']['executableSHA256']


if __name__=='__main__':main()
