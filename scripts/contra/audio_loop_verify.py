"""Verify bounded loop derivatives against original warmed PCM and driver state.

Failure scenarios, fixed before derivation: unprocessed phase discontinuities;
fading to silence instead of joining the true preceding samples; wrong warmup
offset; altered period or inconsistent periods across one song's stems; changed
prefix/body; energy cancellation/boost; new impulses/clipping; source or repeat
generation drift; and a numerical pass mislabeled as browser/listening approval.
"""
import argparse
import array
import csv
import hashlib
import json
import math
from pathlib import Path
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio-loops'
BLEND=1411
WINDOW=353
STRIDE=88


def sha(path):
    with Path(path).open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()


def pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        values=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':values.byteswap()
    return values


def seam_metrics(original, start, end, tail):
    """Independent readback measurements; this function does not create audio."""
    blend=len(tail)
    assert 16<=blend<=BLEND and start>=blend and end>=blend
    incoming=original[end-blend:end]
    outgoing=original[start-blend:start]
    boundary_error=abs((original[start]-tail[-1])-(original[start]-original[start-1]))
    tail_error=max(abs(a-b) for a,b in zip(tail[-16:],original[start-16:start]))
    source_step=max(abs(p[i]-p[i-1]) for p in (original[end-blend-1:end],original[start-blend-1:start+1]) for i in range(1,len(p)))
    derived=([original[end-blend-1]]+list(tail)+[original[start]])
    maximum_step=max(abs(derived[i]-derived[i-1]) for i in range(1,len(derived)))
    source_curvature=max(abs(p[i+1]-2*p[i]+p[i-1]) for p in (original[end-blend-1:end],original[start-blend-1:start+1]) for i in range(1,len(p)-1))
    maximum_curvature=max(abs(derived[i+1]-2*derived[i]+derived[i-1]) for i in range(1,len(derived)-1))
    rms=[];energy_ok=True
    for offset in range(0,blend-WINDOW+1,STRIDE):
        actual=math.sqrt(sum(v*v for v in tail[offset:offset+WINDOW])/WINDOW)
        a=incoming[offset:offset+WINDOW];b=outgoing[offset:offset+WINDOW]
        position=(offset+(WINDOW-1)/2)/(blend-1)
        weight=(1-math.cos(math.pi*position))/2
        expected=math.sqrt(((1-weight)*sum(v*v for v in a)+weight*sum(v*v for v in b))/WINDOW)
        if expected<8:
            passed=abs(actual-expected)<=2
            db=None if min(actual,expected)==0 else 20*math.log10(actual/expected)
        else:
            db=-999 if actual==0 else 20*math.log10(actual/expected)
            passed=-2<=db<=2
        energy_ok=energy_ok and passed
        rms.append({'offset':offset,'rms':actual,'referenceRMS':expected,'differenceDB':db,'passed':passed})
    peak=max(abs(v) for v in tail)
    passed=(boundary_error<=1 and tail_error<=2 and energy_ok and
            maximum_step<=source_step*1.05+2 and maximum_curvature<=source_curvature*1.05+4 and peak<32767)
    return {'passed':passed,'boundaryError':boundary_error,'tail16Error':tail_error,'maximumStep':maximum_step,
            'sourceMaximumStep':source_step,'maximumCurvature':maximum_curvature,'sourceMaximumCurvature':source_curvature,
            'peak':peak,'localRMS':rms}


def input_index():
    return json.loads((BASE/'inputs/index.json').read_text())


def retained_source(row, snapshot_key, original_key):
    candidate=ROOT/row[snapshot_key]
    return candidate if candidate.is_file() else ROOT/row[original_key]


def verify_raw(ids=None):
    rows=[]
    for song in input_index()['songs']:
        if ids is not None and song['id'] not in ids:continue
        start,end=song['baseSampleRange']
        for stem in song['stems']:
            original=pcm(retained_source(stem,'snapshot','source'))
            values=seam_metrics(original,start,end,original[end-BLEND:end])
            rows.append({'id':song['id'],'channel':stem['channel'],**values})
    out=ROOT/'docs/research/nes-contra/evidence'/('audio-loop-intro-raw-red.json' if ids=={20} else 'audio-loop-raw-red.json')
    out.write_text(json.dumps({'scope':'Unprocessed original loop candidate boundaries; expected red before derivation.','stems':rows},indent=2)+'\n')
    assert all(r['passed'] for r in rows),f'{sum(not r["passed"]for r in rows)} unprocessed stems fail explicit PCM seam criteria'


def verify():
    manifest=json.loads((ROOT/'games/contra/assets/audio-loops.json').read_text())
    sources={s['id']:s for s in input_index()['songs']}
    assert {s['id']for s in manifest['songs']}==set(sources) and manifest['acceptedForRelease'] is False
    assert manifest['listeningAccepted'] is False and manifest['mixedGameplayAccepted'] is False
    result=[]
    for song in manifest['songs']:
        source=sources[song['id']]
        first,last=song['driverFrames'];start,end=song['loopStartSample'],song['decodedSamples']
        assert last-first==source['periodTicks']
        assert end-start==source['periodSamples']==song['periodSamples']
        driver_path=retained_source(source,'driverSnapshot','driverSource')
        driver=driver_path.read_bytes()
        assert sha(driver_path)==source['driverSHA256']
        assert driver[(first-1)*130:first*130]==driver[(last-1)*130:last*130]
        assert last+source['periodTicks']<=len(driver)//130
        assert driver[(first-1)*130:(last-1)*130]==driver[(last-1)*130:(last+source['periodTicks']-1)*130]
        cursor_path=retained_source(source,'cursorSnapshot','cursorSource')
        assert sha(cursor_path)==source['cursorSHA256']
        cursors=list(csv.DictReader(cursor_path.open()))
        assert [start,end]==[int(cursors[n]['samples'])-source['originSample'] for n in (first,last)]
        expected={'pulse1','pulse2'} if song['id']==20 else {'pulse1','pulse2','triangle'}
        assert {s['channel']for s in song['stems']}==expected
        for stem in song['stems']:
            reference=next(s for s in source['stems'] if s['channel']==stem['channel'])
            original_path=retained_source(reference,'snapshot','source')
            assert sha(original_path)==reference['sha256']
            original=pcm(original_path)
            path=ROOT/stem['file'];output=pcm(path)
            assert len(output)==end<=len(original)
            assert stem['blendSamples']==BLEND
            assert output[:-BLEND]==original[:end-BLEND],'unmodified prefix/body changed'
            metrics=seam_metrics(original,start,end,output[-BLEND:])
            assert metrics['passed'],(song['id'],stem['channel'],metrics)
            assert max(abs(v)for v in output)<32767
            assert sha(path)==stem['sha256']
            repeat=ROOT/stem['repeatFile'];assert repeat.read_bytes()==path.read_bytes(),'independent generation differs'
            result.append({'id':song['id'],'channel':stem['channel'],'sha256':stem['sha256'],**metrics})
    assert len(result)==sum(len(s['stems'])for s in sources.values())
    report={'passed':True,'scope':f'{len(result)} original-PCM derivatives, preserved tick/sample periods and prefix, bounded 32ms raised-cosine tail only; numerical seam acceptance, not mixed/listening approval.',
            'stems':result,'listeningAccepted':False,'mixedGameplayAccepted':False}
    (ROOT/'docs/research/nes-contra/evidence/audio-loop-numerical.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':True,'songs':len(manifest['songs']),'stems':len(result)}))


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--probe-raw',action='store_true');parser.add_argument('--ids');args=parser.parse_args()
    verify_raw({int(n,0)for n in args.ids.split(',')}if args.ids else None) if args.probe_raw else verify()
