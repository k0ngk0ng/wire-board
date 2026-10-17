"""Verify an explicitly derived loop against untouched original PCM evidence."""
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


def pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        data=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':data.byteswap()
    return data


def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()


def verify():
    manifest=json.loads((OUT/'music-066-loop-source.json').read_text())
    assert manifest['accepted_for_release'] is False and manifest['listening_accepted'] is False
    assert manifest['processing']['requested_blend_ms']==32
    source=OUT/manifest['source']['recording']
    assert sha(source)==manifest['source']['sha256']
    assert len(manifest['source']['repeated_recordings'])==2
    assert all(sha(OUT/name)==manifest['source']['sha256'] for name in manifest['source']['repeated_recordings'])
    driver=(OUT/manifest['source']['driver']).read_bytes()
    assert hashlib.sha256(driver).hexdigest()==manifest['source']['driver_sha256']
    first,last=manifest['source']['frame_range']
    assert last-first==16384
    assert driver[(first-1)*231:first*231]==driver[(last-1)*231:last*231]
    cursors=list(csv.DictReader((OUT/manifest['source']['samples']).open()))
    start,end=manifest['source']['sample_range']
    assert [start,end]==[int(cursors[first]['samples']),int(cursors[last]['samples'])]
    original=pcm(source)
    result=pcm(OUT/manifest['output'])
    blend=manifest['processing']['blend_samples']
    assert blend==1411 and len(result)==end-start
    assert 272.6<len(result)/44100<272.7
    assert result[:-blend]==original[start:end-blend], 'samples outside transition changed'
    # The bridge must join the genuine preceding/next source samples, with
    # a vanishing derivative at the transition endpoints, not just fade to zero.
    boundary_error=abs((result[0]-result[-1])-(original[start]-original[start-1]))
    tail_error=max(abs(a-b) for a,b in zip(result[-16:],original[start-16:start]))
    assert boundary_error<=1 and tail_error<=2
    assert max(abs(v) for v in result)<32767
    incoming=original[end-blend:end]
    outgoing=original[start-blend:start]
    # Compare 8ms local energy at 2ms increments against the convex reference
    # energy of the two original passages. A >2dB dip/boost is not accepted.
    window,step=353,88
    rms=[]
    for index in range(0,blend-window+1,step):
        section=result[len(result)-blend+index:len(result)-blend+index+window]
        actual=math.sqrt(sum(v*v for v in section)/window)
        a=incoming[index:index+window];b=outgoing[index:index+window]
        position=(index+(window-1)/2)/(blend-1)
        w=(1-math.cos(math.pi*position))/2
        expected=math.sqrt(((1-w)*sum(v*v for v in a)+w*sum(v*v for v in b))/window)
        ratio=actual/expected if expected else 1
        assert 10**(-2/20)<=ratio<=10**(2/20), f'local energy deviation {20*math.log10(ratio):.3f}dB at {index}'
        rms.append({'offset_samples':index,'rms':actual,'reference_rms':expected,'difference_db':20*math.log10(ratio)})
    derived_step=max(abs(result[i]-result[i-1]) for i in range(len(result)-blend,len(result)))
    original_step=max(abs(x[i]-x[i-1]) for x in [original[end-blend-1:end],original[start-blend-1:start]] for i in range(1,len(x)))
    assert derived_step<=original_step*1.05+2, 'new transition impulse'
    hashes=[sha(OUT/p) for p in manifest['repeated_outputs']]
    assert len(hashes)==2 and hashes[0]==hashes[1]==sha(OUT/manifest['output'])==manifest['sha256']
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())
    entry=playback['music']['66']
    assert entry['wav']==manifest['output'] and entry['loop'] is True
    assert entry['sha256']==manifest['sha256']
    report={'status':'passed','scope':'deterministic original-PCM derivative, preserved period, seam and local-energy checks; no listening/browser claim',
        'seconds':len(result)/44100,'blend_samples':blend,'unchanged_fraction':(len(result)-blend)/len(result),
        'boundary_error':boundary_error,'tail_16_sample_error':tail_error,'maximum_transition_step':derived_step,
        'maximum_source_step':original_step,'local_rms':rms,'sha256':hashes[0],'listening_accepted':False,'accepted_for_release':False}
    (OUT/'music-066-loop-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report))


if __name__=='__main__':verify()
