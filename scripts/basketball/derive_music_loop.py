"""Make a disclosed original-PCM loop derivative; never overwrite source audio.

The five-cycle period is kept sample-for-sample. Only the final 32ms is bridged
to the original passage immediately before the beginning, using a raised-cosine
blend of the two original passages at the same musical position. No new notes,
oscillators, time stretching or silent padding are introduced.
"""
from pathlib import Path
import array
import csv
import hashlib
import json
import math
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'
START,END=3279,19663
BLEND=1411


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def read_pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        pcm=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':pcm.byteswap()
    return pcm


def derive(run,output):
    cursors=list(csv.DictReader((run/'sample-cursors.csv').open()))
    start,end=int(cursors[START]['samples']),int(cursors[END]['samples'])
    driver=(run/'driver-ram.bin').read_bytes()
    assert driver[(START-1)*231:START*231]==driver[(END-1)*231:END*231]
    original=read_pcm(run/'full.wav')
    result=original[start:end]
    for index in range(BLEND):
        weight=(1-math.cos(math.pi*index/(BLEND-1)))/2
        a=original[end-BLEND+index]
        b=original[start-BLEND+index]
        result[len(result)-BLEND+index]=round((1-weight)*a+weight*b)
    if sys.byteorder!='little':result.byteswap()
    with wave.open(str(output),'wb') as f:
        f.setparams((1,2,44100,0,'NONE','not compressed'))
        f.writeframes(result.tobytes())
    return start,end


def main():
    runs=[OUT/f'music-loop-boundary-run-{n}' for n in (1,2)]
    names=['full.wav','driver-ram.bin','sample-cursors.csv','sequence-jumps.csv','apu.csv','calls.csv']
    for name in names:assert sha(runs[0]/name)==sha(runs[1]/name),f'original rerender differs: {name}'
    outputs=[OUT/f'music-066-loop-derived-{n}.wav' for n in (1,2)]
    ranges=[derive(run,output) for run,output in zip(runs,outputs)]
    assert ranges[0]==ranges[1] and sha(outputs[0])==sha(outputs[1])
    final=OUT/'music-066-loop.wav'
    final.write_bytes(outputs[0].read_bytes())
    manifest={'schema':1,'accepted_for_release':False,'listening_accepted':False,'original_sound_id':66,
        'source':{'recording':str((runs[0]/'full.wav').relative_to(OUT)),'sha256':sha(runs[0]/'full.wav'),
            'driver':str((runs[0]/'driver-ram.bin').relative_to(OUT)),'driver_sha256':sha(runs[0]/'driver-ram.bin'),
            'samples':str((runs[0]/'sample-cursors.csv').relative_to(OUT)),'frame_range':[START,END],'sample_range':list(ranges[0]),
            'render_volume':192,'rom_sha256':sha(ROOT/'.local/nes-basket/basketball.nes'),
            'repeated_recordings':[str((run/'full.wav').relative_to(OUT)) for run in runs],
            'repeated_capture_sha256':{name:sha(runs[0]/name) for name in names}},
        'processing':{'method':'raised-cosine convex blend of original corresponding passages; final window only; unchanged period',
            'requested_blend_ms':32,'blend_samples':BLEND,'actual_blend_ms':BLEND*1000/44100,
            'source_tail_range':[ranges[0][1]-BLEND,ranges[0][1]],'source_target_range':[ranges[0][0]-BLEND,ranges[0][0]],
            'formula':'round((1-w)*tail[i] + w*pre_start[i]), w=(1-cos(pi*i/(N-1)))/2',
            'script_sha256':sha(Path(__file__).resolve())},
        'output':final.name,'sha256':sha(final),'repeated_outputs':[p.name for p in outputs],
        'limitations':['This is an explicitly processed derivative, not a sample-identical original slice.','Listening acceptance and browser-loop timing remain separate; no OSS upload.']}
    (OUT/'music-066-loop-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    playback_path=ROOT/'games/basketball/assets/audio-playback.json'
    playback=json.loads(playback_path.read_text()) if playback_path.exists() else {'schema':1,'accepted_for_release':False,'music':{}}
    playback['music']['66']={'wav':final.name,'loop':True,'sha256':sha(final),
        'source_manifest':'music-066-loop-source.json','processing':'original five-cycle PCM with a disclosed 32 ms raised-cosine tail bridge',
        'listening_accepted':False}
    playback_path.write_text(json.dumps(playback,indent=2)+'\n')
    print(json.dumps({'output':final.name,'sha256':sha(final),'sample_range':ranges[0],'seconds':(ranges[0][1]-ranges[0][0])/44100}))


if __name__=='__main__':main()
