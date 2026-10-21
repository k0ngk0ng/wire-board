"""Verify only the approved music76 anchor; retain failures and never move it.

No WAV derivative/MP3 is written by this probe. A rejected fixed anchor requires
explicit parent/root review before any alternative derivation is attempted.
"""
from pathlib import Path
import array, csv, hashlib, json, math, sys, wave
from capture_music76_assets import ASSETS, ROOT, OLD, NAMES, sha, save

START, END, BLEND, RATE = 4304, 41168, 1411, 44100

def main():
    capture = json.loads((ASSETS/'capture-verification.json').read_text())
    assert capture['status'] == 'passed'
    runs = [ASSETS/f'capture-45000-run-{n}' for n in (1,2)]
    for run in runs:
        assert all(sha(run/k) == capture['files'][k] for k in NAMES)
    old_prefixes = []
    for repeat in (1,2):
        old = OLD/f'court-music-076-20000-run-{repeat}'
        for k in NAMES[1:]: assert (runs[0]/k).read_bytes().startswith((old/k).read_bytes()), k
        with wave.open(str(runs[0]/'full.wav')) as a, wave.open(str(old/'full.wav')) as b:
            assert (b.getnchannels(), b.getsampwidth(), b.getframerate()) == (1,2,RATE)
            n = b.getnframes(); assert a.readframes(n) == b.readframes(n)
        old_prefixes.append({'path': str(old.relative_to(ROOT)), 'samples': n,
                             'files': {k: sha(old/k) for k in NAMES}})
    cursors = list(csv.DictReader((runs[0]/'sample-cursors.csv').open()))
    start, end = [int(cursors[i]['samples']) for i in (START,END)]
    assert start == 3157480 and END-START == 36864
    driver = (runs[0]/'driver-ram.bin').read_bytes()
    assert driver[(START-1)*231:START*231] == driver[(END-1)*231:END*231]
    with wave.open(str(runs[0]/'full.wav')) as f:
        assert (f.getnchannels(), f.getsampwidth(), f.getframerate()) == (1,2,RATE)
        original = array.array('h', f.readframes(f.getnframes()))
    if sys.byteorder != 'little': original.byteswap()
    incoming, outgoing = original[end-BLEND:end], original[start-BLEND:start]
    candidate = []
    for i in range(BLEND):
        weight = (1-math.cos(math.pi*i/(BLEND-1)))/2
        candidate.append(round((1-weight)*incoming[i]+weight*outgoing[i]))
    boundary_error = abs((original[start]-candidate[-1])-(original[start]-original[start-1]))
    tail_error = max(abs(a-b) for a,b in zip(candidate[-16:], original[start-16:start]))
    levels = []
    for k in range(0,BLEND-353+1,88):
        weight = (1-math.cos(math.pi*(k+176)/(BLEND-1)))/2
        reference = ((1-weight)*sum(x*x for x in incoming[k:k+353])+
                     weight*sum(x*x for x in outgoing[k:k+353]))/353
        actual = sum(x*x for x in candidate[k:k+353])/353
        db = 10*math.log10(actual/reference) if actual and reference else 0
        levels.append({'offset_samples': k, 'rms': math.sqrt(actual),
                       'reference_rms': math.sqrt(reference), 'difference_db': db,
                       'passed': abs(db) <= 2})
    transition_step = max(abs(a-b) for a,b in zip([original[end-BLEND-1]]+candidate,candidate))
    source_step = max(abs(section[i]-section[i-1])
                      for section in [original[end-BLEND-1:end],original[start-BLEND-1:start]]
                      for i in range(1,len(section)))
    peak = max(max(abs(v) for v in original[:end-BLEND]), max(abs(v) for v in candidate))
    checks = {'boundary_error_le_1': boundary_error <= 1, 'tail_16_error_le_2': tail_error <= 2,
              'no_clipping': peak < 32767, 'transition_step_le_source_1_05_plus_2': transition_step <= source_step*1.05+2,
              'local_rms_within_2_db': all(v['passed'] for v in levels)}
    passed = all(checks.values())
    result = {'status': 'passed' if passed else 'rejected-approved-anchor', 'id': 76,
              'anchor_moved': False, 'source_frames': [START,END], 'phase_period_frames': END-START,
              'musical_loops': 35, 'sample_rate': RATE, 'loop_start_sample': start,
              'loop_end_sample': end, 'loop_samples': end-start, 'source_samples': end,
              'decoded_samples': None, 'prefix_seconds': start/RATE, 'loop_seconds': (end-start)/RATE,
              'seconds': end/RATE, 'blend_samples': BLEND, 'blend_seconds': BLEND/RATE,
              'checks': checks, 'boundary_error': boundary_error, 'tail_16_sample_error': tail_error,
              'maximum_transition_step': transition_step, 'maximum_source_step': source_step,
              'peak_abs': peak, 'local_rms': levels, 'max_abs_rms_db': max(abs(x['difference_db']) for x in levels),
              'source': {'capture_manifest': 'capture-verification.json',
                         'recordings': [str(p.relative_to(ASSETS)) for p in runs],
                         'files': capture['files'], 'settings': capture['settings'], 'old_prefixes': old_prefixes},
              'processing': {'method': 'preflight only; fixed raised-cosine convex blend of source passages in memory',
                             'source_tail_range': [end-BLEND,end], 'source_target_range': [start-BLEND,start],
                             'formula': 'round((1-w)*tail[i]+w*pre_start[i]), w=(1-cos(pi*i/(1411-1)))/2'},
              'planned_public_filename': 'music-076-intro-loop.mp3', 'file': None, 'sha256': None,
              'master': None, 'master_sha256': None, 'derivative_written': False, 'mp3_encoded': False,
              'command': 'PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_music76_asset_anchor.py',
              'script_sha256': sha(Path(__file__)), 'accepted_for_release': False,
              'listening_accepted': False, 'browser_accepted': False,
              'limitations': ['Offline isolated original driver capture, not earned campaign/gameplay evidence.',
                              'Rejected fixed anchor: no alternate anchor, derivative or MP3 was produced.',
                              'Human listening, native browser boundary timing and device acceptance remain pending.']}
    save(ASSETS/'music76-asset-manifest.json', result)
    print(json.dumps({k: result[k] for k in ['status','loop_start_sample','loop_end_sample','loop_samples',
                                           'seconds','checks','max_abs_rms_db','derivative_written']}))
    return 0 if passed else 2

if __name__ == '__main__': raise SystemExit(main())
