"""Render original physical-channel stems without changing driver or game data.

Channel isolation is a host-renderer setting only. Native driver/APU traces must
remain identical to the full capture. This does not claim that independent
linear media playback recreates the original nonlinear pulse/TND mixer.
"""
import array
import argparse
import hashlib
import json
from pathlib import Path
import wave
import zipfile
from audio_export import ROOT,BASE,OUT,record

CHANNELS={'pulse1':'square1','pulse2':'square2','triangle':'triangle','noise':'noise','dpcm':'pcm'}


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--ids');args=parser.parse_args()
    selected_ids=set(int(n,0)for n in args.ids.split(',')) if args.ids else None
    base=BASE/'stems';base.mkdir(parents=True,exist_ok=True)
    full=json.loads((OUT/'manifest.json').read_text())
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:rom=z.read('Contra (J).nes')
    bank=rom[16+0x4000:16+0x8000];table=int.from_bytes(bank[0x717:0x719],'little')
    manifest={'romSHA256':full['romSHA256'],'acceptedForRelease':False,'mixingLimit':'Native pulse/TND mixer is nonlinear; summing isolated stems is not bit-identical to original full mix. Noise/DPCM reference stems do not implement request-level priority suppression.','sounds':[]}
    manifest_path=base/'manifest.json'
    if selected_ids and manifest_path.exists():manifest['sounds']=[s for s in json.loads(manifest_path.read_text())['sounds'] if s['id'] not in selected_ids]
    for sound in full['sounds']:
        sid=sound['id']
        if selected_ids is not None and sid not in selected_ids:continue
        if sid>=0x7f:channels=['dpcm']
        elif sound['kind'] in ('music','cue'):channels=list(CHANNELS)
        else:
            offset=table-0x8000+sid*3;count=((bank[offset]>>3)&3)+1
            slots=[bank[offset+i*3]&7 for i in range(count)]
            channels=list(dict.fromkeys('pulse1' if slot==4 else 'noise' if slot==5 else {0:'pulse1',1:'pulse2',2:'triangle',3:'noise'}[slot] for slot in slots))
        entry={'id':sid,'kind':sound['kind'],'sampleRange':sound['sampleRange'],'stems':[]}
        for channel in channels:
            volumes={name:255 if name==CHANNELS[channel] else 0 for name in CHANNELS.values()}
            runs=[f'sound-{sid:02x}-{channel}-run-{n}' for n in (1,2)]
            for name in runs:record(OUT/sound['carrier'],base/name,OUT/'wave-bridge.dylib',sound['captureFrames'],channel_volumes=volumes)
            with wave.open(str(base/runs[0]/'full.wav')) as f:values=array.array('h',f.readframes(f.getnframes()))
            first,last=sound['sampleRange'];selected=values[first:last];path=base/f'{sound["kind"]}-{sid:02x}-{channel}.wav'
            with wave.open(str(path),'wb') as f:f.setparams((1,2,44100,0,'NONE','not compressed'));f.writeframes(selected.tobytes())
            entry['stems'].append({'channel':channel,'file':path.name,'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
                                   'runs':runs,'channelVolumes':volumes,'silent':max(abs(v)for v in selected)<8,'seconds':len(selected)/44100})
            print(json.dumps({'id':sid,'channel':channel,'seconds':len(selected)/44100}),flush=True)
        manifest['sounds'].append(entry)
        manifest['sounds'].sort(key=lambda s:s['id'])
        (base/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')


if __name__=='__main__':main()
