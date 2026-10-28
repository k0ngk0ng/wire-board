"""Prove the retained complete $6E cue contains the authoritative percussion.

Failure cases before terminal adapter implementation: a tonal stem is mistaken
for the full mix; wrong source/hash; truncated sample range or tail; omitted or
shifted internal requests; per-slot table and original trace disagree.
"""
import array
import csv
import hashlib
import json
from pathlib import Path
import wave

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio/export'

def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        return array.array('h',f.readframes(f.getnframes()))

def main():
    manifest=json.loads((BASE/'manifest.json').read_text())
    source=json.loads((ROOT/'games/contra/assets/audio-source.json').read_text())
    control=json.loads((ROOT/'games/contra/engine/audio_control_data.json').read_text())
    capture=next(s for s in manifest['sounds'] if s['id']==0x6e)
    catalog=next(s for s in source['sounds'] if s['id']==0x6e)
    slot=next(s for d in control['definitions'] if d['id']==0x6e for s in d['slots'] if s['slot']==3)
    run,repeat=(BASE/p for p in capture['runs'])
    requests=[{'tick':int(r['frame'])-capture['requestFrame'],'id':int(r['value'])} for r in csv.DictReader((run/'calls.csv').open()) if r['kind']=='request' and int(r['value'])!=0x6e]
    assert len(requests)==39 and requests==slot['prefix']
    assert requests[0]['tick']==7 and requests[-1]['tick']==287
    for name in ['calls.csv','apu.csv','driver.bin','samples.csv','full.wav']:
        assert (run/name).read_bytes()==(repeat/name).read_bytes()
    path=BASE/capture['file'];first,last=capture['sampleRange']
    assert first==capture['requestSampleOrigin']
    assert sha(path)==catalog['sha256']==capture['sha256']=='c5c16a2b174c64c4a2d2a2fef2ba5507311c8db66f09efd01ed62240731f3fe8'
    assert pcm(path)==pcm(run/'full.wav')[first:last]
    assert max(abs(v) for v in pcm(path)[-4410:])<8
    result={'passed':True,'id':110,'scope':'Original complete full-mix finite cue, including the recorded internal noise/DPCM requests; no new runtime tick or device-listening claim.',
            'romSHA256':manifest['romSHA256'],'file':str(path.relative_to(ROOT)),'sha256':sha(path),'sampleRange':[first,last],
            'requestCount':len(requests),'noiseRequests':sum(r['id']==2 for r in requests),'dpcmRequests':sum(r['id']>=127 for r in requests),'requests':requests,
            'sourceFiles':{str(p.relative_to(ROOT)):sha(p) for p in [run/'calls.csv',run/'apu.csv',run/'full.wav',ROOT/'games/contra/engine/audio_control_data.json',ROOT/'games/contra/assets/audio-source.json',Path(__file__)]}}
    target=ROOT/'docs/research/nes-contra/evidence/audio-terminal-cue-verification.json';target.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({k:result[k] for k in ['passed','requestCount','noiseRequests','dpcmRequests','sha256']}))
if __name__=='__main__':main()
