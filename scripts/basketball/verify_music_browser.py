"""Bind historical and separately selected native runs to saved bundles/assets.

Reject missing/failed cases, master or encoded hash drift, changed bundles,
duration/sample mismatch, WAV requests, incomplete native wraps, restart on
resume, missing finite end, uncleared sources, or unexercised CDN fallback.
Historical cases never establish current-player coverage. Separately validate
the retained76 suffix case and selected current-run66/75 regressions, without
claiming sample-gapless speaker output, listening or device acceptance.
"""
from pathlib import Path
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
AUDIO = ROOT / '.local/basketball/audio'


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def artifact(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path)}


def verify_music76(catalog):
    directory=ROOT/'.local/music76-source/browser';path=directory/'result.json'
    result=json.loads(path.read_text());entry=catalog['76'];rate=entry['sample_rate']
    delivery=json.loads((AUDIO/'export/music76-delivery-manifest.json').read_text())
    for key in ['sample_rate','loop_start_sample','loop_end_sample','loop_samples','decoded_samples']:
        assert type(entry[key]) is int and entry[key]>0 and entry[key]==delivery[key]
    assert rate==44100 and entry['loop_start_sample']+entry['loop_samples']==entry['loop_end_sample']==entry['decoded_samples']
    start=entry['loop_start_sample']/rate;period=entry['loop_samples']/rate;duration=entry['decoded_samples']/rate
    assert result['status']=='passed' and result['assetFile']==entry['file']=='music-076-intro-loop.mp3'
    assert result['assetSHA256']==entry['distribution_sha256']==delivery['distribution_sha256']==sha(AUDIO/'export'/entry['file'])
    assert result['bundleSHA256']==sha(directory/'harness.js')
    assert result['loopStartSeconds']==start and result['loopSeconds']==period and result['durationSeconds']==duration
    checks={c['kind']:c for c in result['checks']}
    assert len(result['checks'])==len(checks)==11
    assert set(checks)=={'native-initial','continuous-prefix-to-body','two-native-suffix-wraps','source-pause-resume',
                         'wall-clock-restore','paused-snapshot-restore','mute-clears-no-stale-restart','native-hidden-clears',
                         'owner-replacement','dispose','real-source-fallback'}
    initial=checks['native-initial']['state'];src=initial['src']
    assert src.endswith('/audio/'+entry['file']) and initial['attribute']=='/audio/'+entry['file']
    assert abs(initial['duration']-duration)<=1/rate and initial['rate']==1 and not initial['paused'] and not initial['loop']
    prefix=checks['continuous-prefix-to-body']
    assert prefix['state']['src']==src and prefix['state']['time']>start and not any(e['kind']=='ended' for e in prefix['events'])
    gaps=checks['two-native-suffix-wraps']['gaps'];assert len(gaps)==2
    for ordinal,gap in enumerate(gaps,1):
        assert gap['loop']==ordinal;state=gap['state'];events=gap['events']
        assert state['src']==src and state['mediaCount']==initial['mediaCount'] and state['rate']==1
        assert not state['paused'] and not state['loop'] and not state['ended'] and start<state['time']<start+4
        ended=[e for e in events if e['kind']=='ended'];assert len(ended)==1;ended=ended[0]
        assert ended['ended'] and abs(ended['time']-duration)<=1/rate
        for event_name,reported_name in [('playing','endedToPlayingMS'),('seeked','endedToSeekedMS')]:
            event=next(e for e in events if e['kind']==event_name and e['at']>=ended['at'])
            assert event['src']==src and abs(event['time']-start)<.01
            assert gap[reported_name]>=0 and abs(event['at']-ended['at']-gap[reported_name])<.000001
    pause=checks['source-pause-resume'];assert pause['paused']['paused'] and not pause['resumed']['paused']
    assert pause['paused']['src']==pause['resumed']['src']==src and abs(pause['paused']['time']-pause['resumed']['time'])<.25
    restore=checks['wall-clock-restore'];elapsed=(restore['elapsedMS']+restore['serverDeltaMS'])/1000
    mapped=elapsed if elapsed<start else start+(elapsed-start)%period
    assert not restore['state']['paused'] and mapped<=restore['state']['time']<mapped+1.5
    frozen=checks['paused-snapshot-restore']['state'];assert frozen['paused'] and abs(frozen['time']-(start-.5))<.001
    for state in [checks['mute-clears-no-stale-restart']['muted'],checks['mute-clears-no-stale-restart']['enabled'],
                  checks['native-hidden-clears']['state'],checks['dispose']['state']]:
        assert state['attribute'] is None and state['paused']
    assert any(e['hidden'] for e in checks['native-hidden-clears']['events'])
    owner=checks['owner-replacement']['state'];assert owner['src'].endswith('/'+catalog['75']['file']) and owner['time']>=3 and not owner['paused']
    fallback=checks['real-source-fallback']['state'];assert fallback['src'].endswith('/audio/'+entry['file']) and not fallback['paused']
    paths=[r['path'] for r in result['requests']];assert not any('.wav' in p.lower() for p in paths)
    failed=paths.index('/cdn-unavailable/basketball/v1/audio/'+entry['file'])
    assert '/audio/'+entry['file'] in paths[failed+1:]
    recordings=[];assert len(result['recordings'])==2
    for number,record in enumerate(result['recordings'],1):
        assert record['file']==f'wrap-{number}.webm' and record['error']=='' and record['mime']=='audio/webm;codecs=opus'
        recorded=directory/record['file'];assert sha(recorded)==record['sha256'] and recorded.stat().st_size==record['bytes']>0
        recordings.append(artifact(recorded))
    assert not result['accepted_for_release'] and not result['listening_accepted']
    return {'status':'passed','result':artifact(path),'bundle':artifact(directory/'harness.js'),
            'screenshot':artifact(directory/'verified.png'),'asset_sha256':result['assetSHA256'],
            'loop_start_sample':entry['loop_start_sample'],'loop_end_sample':entry['loop_end_sample'],
            'loop_samples':entry['loop_samples'],'checks':len(checks),'native_suffix_boundaries':len(gaps),
            'ended_to_playing_ms':[g['endedToPlayingMS'] for g in gaps],
            'gap_scope':'native event latency only; not continuous speaker-output gap measurement',
            'recordings':recordings,'recording_inspection':artifact(directory/'recording-inspection.json'),
            'recording_scope':'approximately0.5-second EOF tails; recordings do not establish continuous suffix-restart or gapless output',
            'validated_by_this_verifier':True,'scope':'separate selected Chromium76 run bound to saved bundle and current registered interval',
            'sample_gapless_accepted':False,'listening_accepted':False,'device_acceptance':False}


def verify_selected_regression(ident,folder,track,catalog):
    directory=ROOT/'.local/music76-source'/folder;path=directory/'result.json';r=json.loads(path.read_text())
    assert r['status']=='passed' and r['music_id']==ident and r['asset_file']==track['file']==catalog[str(ident)]['file']
    assert r['asset_sha256']==track['sha256']==catalog[str(ident)]['distribution_sha256']==sha(AUDIO/'export'/track['file'])
    assert r['wav_sha256']==track['master_sha256']==sha(AUDIO/'export'/track['master']) and r['bundle_sha256']==sha(directory/'loop.js')
    initial=r['initial'];assert abs(initial['duration']-track['decoded_samples']/44100)<=1/44100
    assert initial['src'].endswith('/'+track['file']) and not initial['paused']
    assert r['paused']['paused'] and not r['resumed']['paused'] and initial['src']==r['paused']['src']==r['resumed']['src']
    assert abs(r['paused']['time']-r['resumed']['time'])<.25 and r['disposed']=={'paused':True,'src':None}
    paths=[e['path'] for e in r['requests']];assert '/audio/'+track['file'] in paths and not any('.wav' in p.lower() for p in paths)
    if ident==66:
        assert initial['loop'] and len(r['wraps'])==2
        for ordinal,wrap in enumerate(r['wraps'],1):
            assert wrap['loops']==ordinal and not wrap['ended'] and not wrap['paused'] and 0<=wrap['time']<1
            assert wrap['src']==initial['src'] and wrap['mediaCount']==initial['mediaCount']
    else:
        assert ident==75 and not initial['loop'] and r['wraps']==[]
        assert r['finite_ended']['ended'] and r['finite_ended']['paused'] and abs(r['finite_ended']['time']-initial['duration'])<=1/44100
    assert not r['cdn_failure_exercised'] and not r['accepted_for_release'] and not r['listening_accepted']
    return {'status':'passed','music_id':ident,'result':artifact(path),'bundle':artifact(directory/'loop.js'),
            'screenshot':artifact(directory/'verified.png'),'native_wraps':len(r['wraps']),'finite_ended':ident==75,
            'scope':'selected current-run regression, bound to saved bundle; no other historical cases promoted'}


def main():
    manifest_path = AUDIO / 'export/music-distribution-manifest.json'
    verification_path = AUDIO / 'export/music-distribution-verification.json'
    manifest = json.loads(manifest_path.read_text())
    material = json.loads(verification_path.read_text())
    catalog = json.loads((ROOT / 'games/basketball/assets/audio-playback.json').read_text())['music']
    tracks = {item['id']: item for item in manifest['tracks']}
    assert set(tracks) == {59, 60, 61, 62, 63, 64, 65, 66, 75}
    assert material['status'] == 'passed' and material['tracks'] == 9
    cases = []
    for ident, fallback in [(ident, False) for ident in sorted(tracks)] + [(66, True)]:
        directory = AUDIO / f'encoded-loop-browser-{ident}{"-cdn-fallback" if fallback else ""}'
        path = directory / 'result.json'
        result = json.loads(path.read_text())
        track = tracks[ident]
        assert result['status'] == 'passed' and result['music_id'] == ident
        assert result['asset_file'] == track['file'] == catalog[str(ident)]['file']
        assert result['asset_sha256'] == track['sha256'] == catalog[str(ident)]['distribution_sha256'] == sha(AUDIO / 'export' / track['file'])
        assert result['wav_sha256'] == track['master_sha256'] == sha(AUDIO / 'export' / track['master'])
        assert result['bundle_sha256'] == sha(directory / 'loop.js')
        assert abs(result['initial']['duration'] - track['source_samples'] / 44100) <= 1 / 44100
        assert track['source_samples'] == track['decoded_samples']
        paths = [request['path'] for request in result['requests']]
        assert not any('.wav' in value.lower() for value in paths)
        assert f'/audio/{track["file"]}' in paths
        initial = result['initial']
        assert initial['src'].endswith('/' + track['file']) and not initial['paused']
        assert result['paused']['paused'] and not result['resumed']['paused']
        assert initial['src'] == result['paused']['src'] == result['resumed']['src']
        assert abs(result['paused']['time'] - result['resumed']['time']) < .25
        assert result['disposed'] == {'paused': True, 'src': None}
        if ident == 75:
            assert not initial['loop'] and result['wraps'] == []
            assert result['finite_ended']['ended'] and result['finite_ended']['paused']
            assert abs(result['finite_ended']['time'] - initial['duration']) <= 1 / 44100
        else:
            assert initial['loop'] and len(result['wraps']) == 2
            for ordinal, wrap in enumerate(result['wraps'], 1):
                assert wrap['loops'] == ordinal and not wrap['ended'] and not wrap['paused']
                assert wrap['src'] == initial['src'] and wrap['mediaCount'] == initial['mediaCount']
                assert 0 <= wrap['time'] < 1
        assert result['cdn_failure_exercised'] == fallback
        if fallback:
            failed_path = '/cdn-unavailable/basketball/v1/audio/' + track['file']
            assert paths.index(failed_path) < paths.index('/audio/' + track['file'])
        assert not result['accepted_for_release'] and not result['listening_accepted']
        cases.append({'music_id': ident, 'cdn_fallback': fallback, 'result': artifact(path),
                      'screenshot': artifact(directory / 'verified.png'), 'bundle': artifact(directory / 'loop.js'),
                      'duration': initial['duration'], 'native_wraps': len(result['wraps']),
                      'finite_ended': ident == 75, 'pause_resume': True, 'source_cleared': True,
                      'wav_requested': False, 'asset_sha256': result['asset_sha256']})
    music76=verify_music76(catalog)
    selected=[verify_selected_regression(66,'whole-loop-66',tracks[66],catalog),
              verify_selected_regression(75,'finite-75',tracks[75],catalog)]
    report = {
        'status': 'passed', 'scope': 'historical nine-track batch plus separately bound76 suffix and selected66/75 native regression runs',
        'historical_music_ids':sorted(tracks),'current_shared_player_verified':False,'separate_music76_evidence':music76,
        'selected_current_run_regressions':selected,
        'manifest': artifact(manifest_path), 'material_verification': artifact(verification_path),
        'source_bytes': material['source_bytes'], 'encoded_bytes': material['encoded_bytes'],
        'reduction_percent': material['reduction_percent'], 'cases': cases,
        'other_evidence': [artifact(AUDIO / 'compression' / name) for name in [
            'adapter-before.log', 'no-wav-fallback-before.log', 'material-before.log',
            'adapter-after.log', 'material-after.log', 'typecheck.log', 'native-media.log']],
        'limitations': [
            'Browser tests seek near the tail at 1x; they do not play every long track from its beginning.',
            'Evidence is bound to saved loop.js hashes, not later concurrent edits to audio.ts.',
            'Real Safari/iOS/Android devices, human listening and complete match mixing remain unverified.',
            'The fallback case uses a local simulated CDN failure; no OSS upload was performed.',
            'The preserved333-second76 preview is unregistered; the compound76 result is validated separately against its saved bundle and current interval.',
            'Only the separately named76/66/75 native runs cover the selected new player revision; other saved cases remain historical.',
            'The76 recordings contain approximately0.5-second EOF tails, not continuous end-to-suffix audio;4.4ms/1.2ms event latencies do not establish audible or sample-gapless output.'
        ], 'accepted_for_release': False, 'listening_accepted': False,
    }
    destination = AUDIO / 'compression/acceptance-summary.json'
    destination.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'status': report['status'], 'cases': len(cases), 'summary': str(destination.relative_to(ROOT)),
                      'encoded_bytes': report['encoded_bytes'],'music76_checks':music76['checks'],
                      'selected_current_regressions':[r['music_id'] for r in selected]}))


if __name__ == '__main__':
    main()
