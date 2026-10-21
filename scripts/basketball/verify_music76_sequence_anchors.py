"""Bounded original-sequence-boundary candidates; no waveform anchor search."""
from pathlib import Path
import array, csv, json, math, sys, wave
from capture_music76_assets import ASSETS, ROOT, NAMES, sha, save

PERIOD, BLEND, RATE = 36864, 1411, 44100

def read_pcm(path):
    with wave.open(str(path)) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate()) == (1,2,RATE)
        pcm = array.array('h', f.readframes(f.getnframes()))
    if sys.byteorder != 'little': pcm.byteswap()
    return pcm

def blend_tail(original,start,end):
    return array.array('h', (round((1-(1-math.cos(math.pi*i/(BLEND-1)))/2)*original[end-BLEND+i]+
                                  ((1-math.cos(math.pi*i/(BLEND-1)))/2)*original[start-BLEND+i]) for i in range(BLEND)))

def check_tail(original,start,end,tail):
    incoming,outgoing = original[end-BLEND:end],original[start-BLEND:start]
    boundary = abs((original[start]-tail[-1])-(original[start]-original[start-1]))
    final = max(abs(a-b) for a,b in zip(tail[-16:],original[start-16:start]))
    rms=[]
    for k in range(0,BLEND-353+1,88):
        weight=(1-math.cos(math.pi*(k+176)/(BLEND-1)))/2
        reference=((1-weight)*sum(x*x for x in incoming[k:k+353])+weight*sum(x*x for x in outgoing[k:k+353]))/353
        actual=sum(x*x for x in tail[k:k+353])/353
        db=10*math.log10(actual/reference) if actual and reference else 0
        rms.append({'offset_samples':k,'difference_db':db,'rms':math.sqrt(actual),'reference_rms':math.sqrt(reference)})
    step=max(abs(a-b) for a,b in zip([original[end-BLEND-1]]+list(tail),tail))
    source_step=max(abs(x[i]-x[i-1]) for x in [original[end-BLEND-1:end],original[start-BLEND-1:start]] for i in range(1,len(x)))
    peak=max(max(abs(x) for x in original[:end-BLEND]),max(abs(x) for x in tail))
    checks={'boundary_error_le_1':boundary<=1,'tail_16_error_le_2':final<=2,'no_clipping':peak<32767,
            'transition_step_le_source_1_05_plus_2':step<=source_step*1.05+2,'local_rms_within_2_db':all(abs(x['difference_db'])<=2 for x in rms)}
    return {'passed':all(checks.values()),'checks':checks,'boundary_error':boundary,'tail_16_sample_error':final,
            'peak_abs':peak,'maximum_transition_step':step,'maximum_source_step':source_step,'local_rms':rms,
            'max_abs_rms_db':max(abs(x['difference_db']) for x in rms)}

def main():
    previous=ASSETS/'music76-asset-manifest.json'
    assert sha(previous)=='3a1f0ff79cb17d675dd4eb8593b95e40968eace0ef504ec0059289b1e968b524'
    capture=json.loads((ASSETS/'capture-verification.json').read_text())
    runs=[ASSETS/f'capture-45000-run-{n}' for n in (1,2)]
    for run in runs: assert all(sha(run/k)==capture['files'][k] for k in NAMES)
    original=read_pcm(runs[0]/'full.wav');driver=(runs[0]/'driver-ram.bin').read_bytes()
    cursors=list(csv.DictReader((runs[0]/'sample-cursors.csv').open()))
    jumps=[{k:int(v) for k,v in r.items()} for r in csv.DictReader((runs[0]/'sequence-jumps.csv').open())]
    mainjumps=[r for r in jumps if r['channel']==0]
    candidates=[r['step'] for r in mainjumps if r['step']+PERIOD<=45000]
    assert candidates==[5342,6396,7449]
    reports=[]
    for first in candidates:
        last=first+PERIOD
        index=next(i for i,r in enumerate(mainjumps) if r['step']==first)
        assert mainjumps[index+35]['step']==last
        assert {r['channel'] for r in jumps if r['step']==first}=={0,1,2,3}
        assert {r['channel'] for r in jumps if r['step']==last}=={0,1,2,3}
        state=driver[(first-1)*231:first*231]
        assert state==driver[(last-1)*231:last*231]
        start,end=[int(cursors[i]['samples']) for i in (first,last)]
        result=check_tail(original,start,end,blend_tail(original,start,end))
        reports.append(dict(result,source_frames=[first,last],loop_start_sample=start,loop_end_sample=end,
                            loop_samples=end-start,sample_rate=RATE,prefix_seconds=start/RATE,
                            loop_seconds=(end-start)/RATE,seconds=end/RATE,driver_state_sha256=__import__('hashlib').sha256(state).hexdigest(),
                            boundary_jumps=[r for r in jumps if r['step'] in [first,last]]))
    accepted=[r for r in reports if r['passed']]
    result={'status':'passed' if accepted else 'no-accepted-source-boundary','candidate_source_frames':candidates,
            'selection':'first passing chronological original synchronized D0 jump with complete period inside both existing captures',
            'selected':accepted[0] if accepted else None,'candidates':reports,'period_frames':PERIOD,'musical_loops':35,
            'blend_samples':BLEND,'source_capture_verification_sha256':sha(ASSETS/'capture-verification.json'),
            'rejected4304_manifest_sha256':sha(previous),'script_sha256':sha(Path(__file__)),
            'command':'PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music76_sequence_anchors.py'}
    save(ASSETS/'sequence-anchor-verification.json',result)
    print(json.dumps({'status':result['status'],'candidates':[{k:r[k] for k in ['source_frames','passed','max_abs_rms_db','loop_start_sample','loop_end_sample','loop_samples']} for r in reports]}))
    assert accepted,'no candidate passed; stop without deriving'

if __name__=='__main__':main()
