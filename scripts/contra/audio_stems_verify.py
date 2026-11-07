"""Verify source-channel isolation without claiming a linear original mix.

Failure inventory: rendering controls change emulated driver/APU writes; stem
timelines drift; independent trimming destroys alignment; missing/silent tracks
are hidden; original nonlinear mixing is incorrectly claimed to equal stem sum.
"""
import array
import hashlib
import json
from pathlib import Path
import wave

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio'


def pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        return array.array('h',f.readframes(f.getnframes()))


def main():
    catalog=json.loads((BASE/'stems/manifest.json').read_text());reports=[]
    ids=[s['id'] for s in catalog['sounds']]
    assert len(ids)==len(set(ids)), 'Duplicate sound IDs in stem catalog'
    full=json.loads((BASE/'export/manifest.json').read_text());byid={s['id']:s for s in full['sounds']}
    for item in catalog['sounds']:
        original=byid[item['id']]
        reference=BASE/'export'/original['runs'][0]
        for stem in item['stems']:
            a,b=[BASE/'stems'/r for r in stem['runs']]
            for name in ['driver.bin','apu.csv','calls.csv','samples.csv']:
                assert (a/name).read_bytes()==(reference/name).read_bytes(),(item['id'],stem['channel'],name)
                assert (a/name).read_bytes()==(b/name).read_bytes()
            assert (a/'full.wav').read_bytes()==(b/'full.wav').read_bytes()
            raw=pcm(a/'full.wav');first,last=original['sampleRange'];output=BASE/'stems'/stem['file']
            values=pcm(output);assert values==raw[first:last]
            assert hashlib.sha256(output.read_bytes()).hexdigest()==stem['sha256']
            assert all(-32768<v<32767 for v in values)
        reports.append({'id':item['id'],'stems':len(item['stems']),'sameDriverAPUTimeline':True})
    result={'passed':True,'scope':'Original-channel isolation with identical emulated driver/APU/timing; independent media mix and seams not accepted.',
            'sounds':reports,'stemCount':sum(r['stems'] for r in reports)}
    (BASE/'stems/verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'stems':result['stemCount']}))


if __name__=='__main__':main()
