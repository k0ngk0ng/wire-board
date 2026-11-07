"""Create bounded original-PCM loop derivatives only after the raw red probe.

The original song period and all samples outside a 32ms tail remain unchanged.
One shared driver-frame anchor is searched for all three melodic stems; failure
is explicit rather than weakening the acceptance limits or shifting one track.
"""
import argparse
import array
import csv
import hashlib
import json
import math
import os
from pathlib import Path
import wave

from audio_loop_verify import ROOT,BASE,BLEND,input_index,pcm,seam_metrics,sha,retained_source


WEIGHTS=[(1-math.cos(math.pi*i/(BLEND-1)))/2 for i in range(BLEND)]


def blend_tail(source,start,end):
    a=source[end-BLEND:end];b=source[start-BLEND:start]
    return array.array('h',(round(x*(1-w)+y*w) for x,y,w in zip(a,b,WEIGHTS)))


def write_wave(path,values):
    temporary=path.with_name(path.name+'.writing')
    with wave.open(str(temporary),'wb') as f:
        f.setparams((1,2,44100,0,'NONE','not compressed'));f.writeframes(values.tobytes())
    temporary.replace(path)


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--ids');args=parser.parse_args()
    ids={int(n,0)for n in args.ids.split(',')}if args.ids else None
    red='audio-loop-intro-raw-red.json'if ids=={20}else'audio-loop-raw-red.json'
    assert (ROOT/'docs/research/nes-contra/evidence'/red).is_file(),'run the pre-implementation raw seam probe first'
    source=input_index();output=BASE/'output';output.mkdir(parents=True,exist_ok=True)
    manifest={'format':'contra-j-audio-loops-v1','romSHA256':source['romSHA256'],'sampleRate':44100,
              'acceptedForRelease':False,'listeningAccepted':False,'mixedGameplayAccepted':False,
              'algorithm':'A shared original driver-frame anchor for each song; 1411-sample raised-cosine bridge from source end tail to the genuine source prefix immediately before loopStart. No resampling, normalization or synthesis.',
              'songs':[]}
    search=[]
    target=ROOT/'games/contra/assets/audio-loops.json'
    if ids is not None:
        manifest=json.loads(target.read_text())
        manifest['songs']=[s for s in manifest['songs']if s['id']not in ids]
        search=[s for s in json.loads((ROOT/'docs/research/nes-contra/evidence/audio-loop-search.json').read_text())['songs']if s['id']not in ids]
        manifest['nativeCoverageComplete']=False
    for song in source['songs']:
        if ids is not None and song['id']not in ids:continue
        originals={s['channel']:pcm(retained_source(s,'snapshot','source')) for s in song['stems']}
        driver=retained_source(song,'driverSnapshot','driverSource').read_bytes()
        cursors=list(csv.DictReader(retained_source(song,'cursorSnapshot','cursorSource').open()))
        first,last=song['baseDriverFrames'];limit=min(song['periodTicks'],len(cursors)-last-1)
        selected=None;attempts=0;skip_period=0;fail_counts={channel:0 for channel in originals}
        for offset in range(limit):
            a,b=first+offset,last+offset
            start,end=(int(cursors[n]['samples'])-song['originSample'] for n in (a,b))
            if end-start!=song['periodSamples']:
                skip_period+=1;continue
            if start<BLEND or end>min(len(v)for v in originals.values()):continue
            assert driver[(a-1)*130:a*130]==driver[(b-1)*130:b*130],('driver period lost',song['id'],offset)
            attempts+=1;results={};passed=True
            for channel,original in originals.items():
                tail=blend_tail(original,start,end)
                metrics=seam_metrics(original,start,end,tail)
                results[channel]=(tail,metrics)
                if not metrics['passed']:
                    fail_counts[channel]+=1;passed=False
            if passed:
                selected=(a,b,start,end,results,offset);break
        if selected is None:
            failure={'id':song['id'],'attempts':attempts,'samePeriodCandidatesSkipped':skip_period,'failuresByChannel':fail_counts,'scope':'No shared anchor passed all fixed numerical limits; no asset emitted.'}
            (ROOT/'docs/research/nes-contra/evidence/audio-loop-search-failure.json').write_text(json.dumps(failure,indent=2)+'\n')
            raise RuntimeError(json.dumps(failure))
        a,b,start,end,results,offset=selected
        row={'id':song['id'],'label':song['label'],'kind':song.get('kind','music'),'driverFrames':[a,b],'driverOffsetTicks':offset,
             'loopStartSample':start,'decodedSamples':end,'periodSamples':end-start,'periodTicks':b-a,
             'loopStartSeconds':start/44100,'periodSeconds':(end-start)/44100,'pcmSeamReady':False,
             'nativeBoundaryReady':False,'loopReady':False,'stems':[]}
        for stem in song['stems']:
            channel=stem['channel'];original=originals[channel]
            tail,metrics=results[channel]
            derived=array.array('h',original[:end]);derived[-BLEND:]=tail
            prefix=song.get('kind','music')
            path=output/f'{prefix}-{song["id"]:02x}-{channel}-loop.wav'
            repeat=output/f'{prefix}-{song["id"]:02x}-{channel}-repeat.wav'
            write_wave(path,derived)
            # Generate independently from the untouched source, not a file copy.
            again=array.array('h',original[:end]);again[-BLEND:]=blend_tail(original,start,end)
            write_wave(repeat,again)
            assert not os.path.samefile(path,repeat),'separate generation must not share writable storage'
            assert path.read_bytes()==repeat.read_bytes()
            row['stems'].append({'channel':channel,'file':str(path.relative_to(ROOT)),'repeatFile':str(repeat.relative_to(ROOT)),
                'sha256':sha(path),'sourceFile':stem['source'],'sourceSHA256':stem['sha256'],'blendSamples':BLEND,
                'changedFractionMaximum':BLEND/end,'periodChangedFractionMaximum':BLEND/(end-start),
                'metrics':metrics})
        manifest['songs'].append(row)
        search.append({'id':song['id'],'attempts':attempts,'samePeriodCandidatesSkipped':skip_period,'failuresByChannel':fail_counts,'selectedOffsetTicks':offset})
        print(json.dumps({'id':song['id'],'offsetTicks':offset,'attempts':attempts,'loopStart':start,'periodSamples':end-start}),flush=True)
    target.write_text(json.dumps(manifest,indent=2)+'\n')
    (ROOT/'docs/research/nes-contra/evidence/audio-loop-search.json').write_text(json.dumps({'songs':search},indent=2)+'\n')


if __name__=='__main__':main()
