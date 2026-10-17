"""Audit original music loop candidates without modifying any PCM sample.

Prerequisite: record sound-066.nsf with extract_audio.record for 20,000 frames,
volume 192, under export/music-loop-boundary-run-1. This is a bounded analysis
result, not permission to enable looping or mark material release-ready.
"""
from pathlib import Path
import array
import csv
import hashlib
import json
import sys
import wave

ROOT=Path(__file__).resolve().parents[2]


def main():
    root=ROOT/'.local/basketball/audio'
    run=root/'export/music-loop-boundary-run-1'
    driver=(run/'driver-ram.bin').read_bytes()
    assert len(driver)==20000*231
    cursor=[int(r['samples']) for r in csv.DictReader((run/'sample-cursors.csv').open())]
    assert len(cursor)==20001
    jumps=[{k:int(v) for k,v in r.items()} for r in csv.DictReader((run/'sequence-jumps.csv').open()) if int(r['channel'])==0]
    assert [r['step'] for r in jumps]==[3280,6556,9833,13110,16387,19664]
    assert all(r['target']==0xabb0 and r['command']==0xac1d for r in jumps)
    with wave.open(str(run/'full.wav')) as f:
        assert (f.getnchannels(),f.getsampwidth(),f.getframerate())==(1,2,44100)
        pcm=array.array('h',f.readframes(f.getnframes()))
    if sys.byteorder!='little':pcm.byteswap()
    assert cursor[-1]==len(pcm)
    candidates=[]
    for start,end in [(3280,6556),(3280,19664)]:
        a,b=cursor[start],cursor[end]
        equal=driver[(start-1)*231:start*231]==driver[(end-1)*231:end*231]
        delta=pcm[a]-pcm[b-1]
        candidates.append({'driver_frames':[start,end],'sample_range':[a,b],'seconds':(b-a)/44100,
            'driver_state_equal':equal,'pcm_seam_delta':delta,'natural_next_delta':pcm[b]-pcm[b-1],
            'slope_difference':(pcm[a+1]-pcm[a])-(pcm[b-1]-pcm[b-2]),
            'seam_delta_dbfs':None if not delta else __import__('math').log10(abs(delta)/32768)*20})
    assert not candidates[0]['driver_state_equal'] and candidates[1]['driver_state_equal']
    # Exact original sequence/tempo recurrence alone is insufficient: the
    # analog-output approximation still has a different oscillator phase.
    result={'status':'analysis-complete','seamless_loop_accepted':False,
        'reason':'Raw original loop boundaries have different PCM phase; no crossfade or sample replacement was applied.',
        'source_rom_sha256':hashlib.sha256((ROOT/'.local/nes-basket/basketball.nes').read_bytes()).hexdigest(),
        'candidates':candidates,
        'source_sha256':{name:hashlib.sha256((run/name).read_bytes()).hexdigest() for name in ['full.wav','driver-ram.bin','sample-cursors.csv','sequence-jumps.csv','apu.csv','calls.csv']}}
    (root/'seam-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result))


if __name__=='__main__':main()
