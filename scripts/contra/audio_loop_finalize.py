"""Bind numerical/native evidence to loop readiness without widening acceptance."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import shutil

from audio_loop_verify import ROOT,BASE,sha


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--browser',default='v3');args=parser.parse_args()
    target=ROOT/'games/contra/assets/audio-loops.json';before=target.read_bytes();manifest=json.loads(before)
    numeric_path=ROOT/'docs/research/nes-contra/evidence/audio-loop-numerical.json';numeric=json.loads(numeric_path.read_text())
    count=sum(len(s['stems'])for s in manifest['songs'])
    assert numeric['passed'] and len(numeric['stems'])==count
    browser_path=BASE/f'browser-{args.browser}/result.json';browser=json.loads(browser_path.read_text())
    assert browser['passed'] and browser['sourcesUnchangedDuringRun'] and len(browser['reports'])==len(manifest['songs'])
    assert browser['sourceSHA256']['web/src/audio.ts']==sha(ROOT/'web/src/audio.ts'),'platform audio changed; repeat native check'
    assert browser['sourceSHA256']['scripts/contra/audio_loop_browser.py']==sha(ROOT/'scripts/contra/audio_loop_browser.py')
    assert browser['sourceSHA256']['games/contra/assets/audio-loops.json']==hashlib.sha256(before).hexdigest()
    byid={r['id']:r for r in browser['reports']}
    summaries=[]
    for song in manifest['songs']:
        r=byid[song['id']]
        assert r['passed'] and len(r['tracks'])==len(song['stems'])
        assert len(r['prefix'])==len(song['stems']) and r['departure']
        assert all(0<p['time']<song['loopStartSeconds']for p in r['prefix'])
        assert abs(r['duration']-song['decodedSamples']/44100)<1e-12
        assert abs(r['loopStart']-song['loopStartSeconds'])<1e-12
        tracks=[]
        for index,track in enumerate(r['tracks']):
            assert track['ends']>=2 and track['maxBoundaryClockDebtMS']<=150
            assert not any(e['event']=='error' for e in track['events'])
            tracks.append({'channel':track['channel'],'nativeEnds':track['ends'],
                           'firstBoundaryPositionSeconds':r['firstTimes'][index],
                           'secondBoundaryPositionSeconds':r['secondTimes'][index],
                           'maximumBoundaryClockDebtMS':track['maxBoundaryClockDebtMS']})
        measurements={'crossTrackClockSpreadMS':r['crossTrackClockSpreadMS'],
                      'maximumBoundaryClockDebtMS':max(t['maximumBoundaryClockDebtMS']for t in tracks),'tracks':tracks,
                      'meaning':'Native media clock observations; not captured output PCM or a promise of sample-accurate synchrony.'}
        song.update(pcmSeamReady=True,nativeBoundaryReady=True,loopReady=True,nativeMeasurements=measurements,
                    multiTrackSyncAccepted=False,listeningAccepted=False)
        summaries.append({'id':song['id'],**measurements})
        for stem in song['stems']:assert sha(ROOT/stem['file'])==stem['sha256']
    manifest.update(multiTrackSyncAccepted=False,sampleAccurateSyncAccepted=False,nativeCoverageComplete=True,
                    readinessScope='loopReady means numerical PCM seam and per-track native suffix-loop position/progress/pause checks passed. It does not accept output-PCM continuity, multitrack synchronization, listening or mixed gameplay.',
                    numericalEvidence='docs/research/nes-contra/evidence/audio-loop-numerical.json',
                    nativeEvidence='docs/research/nes-contra/evidence/audio-loop-native.json.gz',
                    nativePlatformSHA256=browser['sourceSHA256']['web/src/audio.ts'])
    out=ROOT/'docs/research/nes-contra/evidence'
    (out/'audio-loop-before-readiness.json').write_bytes(before)
    raw=(json.dumps(browser,separators=(',',':'))+'\n').encode()
    (out/'audio-loop-native.json.gz').write_bytes(gzip.compress(raw,mtime=0))
    shutil.copyfile(BASE/f'browser-{args.browser}/native-loop.png',out/'audio-loop-native.png')
    target.write_text(json.dumps(manifest,indent=2)+'\n')
    summary={'passed':True,'perTrackBoundaries':count*2,'songs':summaries,
             'largestCrossTrackClockSpreadMS':max(r['crossTrackClockSpreadMS']for r in summaries),
             'largestBoundaryClockDebtMS':max(r['maximumBoundaryClockDebtMS']for r in summaries),
             'outputPCMCaptured':False,'multiTrackSyncAccepted':False,'listeningAccepted':False,
             'manifestBeforeReadinessSHA256':hashlib.sha256(before).hexdigest(),'manifestAfterReadinessSHA256':sha(target),
             'binding':'Only readiness flags, evidence links and measured native results were added after the run. PCM files, SHA values and the tested loop/prefix/sample parameters are unchanged.',
             'sourceSHA256':{str(p.relative_to(ROOT)):sha(p)for p in [ROOT/'scripts/contra/audio_loop_snapshot.py',ROOT/'scripts/contra/audio_loop_derive.py',ROOT/'scripts/contra/audio_loop_verify.py',ROOT/'scripts/contra/audio_loop_browser.py',Path(__file__)]}}
    (out/'audio-loop-native-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps({'passed':True,'readySongs':len(summaries),'stems':count,'largestClockSpreadMS':summary['largestCrossTrackClockSpreadMS'],'largestBoundaryClockDebtMS':summary['largestBoundaryClockDebtMS']}))


if __name__=='__main__':main()
