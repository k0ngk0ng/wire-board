"""Bind retained MP3 browser evidence to verified distribution assets.

Reject missing/failed cases, master or encoded hash drift, changed bundles,
duration/sample mismatch, WAV requests, incomplete native wraps, restart on
resume, missing finite end, uncleared sources, or unexercised CDN fallback.
This validates retained evidence; accept_audio_loop.py executes the browser.
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
    report = {
        'status': 'passed', 'scope': 'retained local Chromium native-media evidence bound to exact MP3 assets and compiled harnesses',
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
            'The 76 intro/loop preview has no runtime mapping; masters/repeat encodes are not delivery assets.'
        ], 'accepted_for_release': False, 'listening_accepted': False,
    }
    destination = AUDIO / 'compression/acceptance-summary.json'
    destination.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'status': report['status'], 'cases': len(cases), 'summary': str(destination.relative_to(ROOT)),
                      'encoded_bytes': report['encoded_bytes']}))


if __name__ == '__main__':
    main()
