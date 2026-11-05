#!/usr/bin/env python3
"""Independent test-first finite-audio catalog/source/WAV acceptance.

The fixed legacy catalog and standard-library WAV reader are the oracle.
This verifier never imports the exporter or the production Go decoder.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import wave

ROOT = Path(__file__).resolve().parents[2]
SOURCE_SHA = '617737f8ef07043401fa74f7f48ba551a64dd91cedd3a94dbfcf4883e0ff1541'
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
FORMAT = {'sample_rate': 44100, 'channels': 2, 'sample_width_bytes': 2, 'pcm_encoding': 'signed16-little-endian'}
FINITE_CODES = [*range(6, 127), 143, *range(160, 172)]


def require(value, message):
    if not value:
        raise ValueError(message)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, f'duplicate JSON field: {key}')
        result[key] = value
    return result


def read_json(path):
    return json.loads(path.read_bytes(), object_pairs_hook=pairs)


def original_oracle():
    source = ROOT / 'games/yuyuhakusho/data/audio-assets.json'
    require(digest(source.read_bytes()) == SOURCE_SHA, 'fixed source catalog differs')
    original = read_json(source)
    require(original['rom_sha256'] == ROM_SHA and original['format'] == FORMAT, 'source identity or WAV format differs')
    clips = []
    media = {}
    for row in original['assets']:
        code = row['code']
        require(code in FINITE_CODES and row['complete'] and row['runtime_asset_ready'] and row['pcm_replay_equal'],
                'unverified or nonfinite source entry')
        path = (ROOT / row['media_file']).resolve()
        canonical_root = ROOT / '.local/md-yuyuhakusho/audio-assets-final'
        require(path.is_relative_to(canonical_root), 'source WAV outside canonical acquisition tree')
        raw = path.read_bytes()
        require(digest(raw) == row['wav_sha256'], f'original WAV SHA differs: {code:02x}')
        with wave.open(io.BytesIO(raw), 'rb') as wav:
            require((wav.getnchannels(), wav.getsampwidth(), wav.getframerate(), wav.getnframes())
                    == (2, 2, 44100, row['stereo_frames']), f'original WAV format/count differs: {code:02x}')
            pcm = wav.readframes(wav.getnframes())
        require(digest(pcm) == row['pcm_sha256'] and len(pcm) == row['stereo_frames'] * 4,
                f'original PCM differs: {code:02x}')
        onset = next((index // 4 for index in range(0, len(pcm), 2) if pcm[index:index + 2] != b'\0\0'), None)
        require(onset is not None and (row.get('first_nonzero_stereo_frame') in (None, onset)),
                f'original onset differs: {code:02x}')
        if code == 143:
            require(onset == 971, 'finite8f onset differs from frozen source observation')
        clips.append({'code': code, 'kind': row['classification'], 'stereo_frames': row['stereo_frames'],
                      'wav_sha256': row['wav_sha256'], 'pcm_sha256': row['pcm_sha256'],
                      'first_nonzero_stereo_frame': onset, 'loop': False})
        media[code] = {'path': str(path.relative_to(ROOT)), 'bytes': len(raw), 'wav_sha256': digest(raw),
                       'pcm_sha256': digest(pcm), 'first_nonzero_stereo_frame': onset}
    clips.sort(key=lambda item: item['code'])
    require([clip['code'] for clip in clips] == FINITE_CODES, 'source finite code set differs')
    bindings = [{'character': character, 'codes': sorted(row['code'] for row in original['animation_bindings']
                                                       if character in row['characters'])} for character in range(11)]
    require(len({code for binding in bindings for code in binding['codes']}) == 78, 'source animation union differs')
    expected = {'schema': 1, 'rom_sha256': ROM_SHA, 'source_catalog_sha256': SOURCE_SHA,
                'format': FORMAT, 'source_origin': 'isolated-capture-start', 'clips': clips, 'animation_bindings': bindings}
    return expected, media


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--catalog', type=Path, default=ROOT / 'games/yuyuhakusho/data/audio-finite-catalog.json')
    parser.add_argument('--manifest', type=Path, default=ROOT / 'games/yuyuhakusho/data/audio-finite-catalog-manifest.json')
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    report = {'scope': 'All134 finite original WAVs and source-character animation references; no playback or BGM loop acceptance',
              'verifier_sha256': digest(Path(__file__).read_bytes()), 'source_catalog_sha256': SOURCE_SHA}
    try:
        require(args.catalog.exists(), 'missing finite runtime catalog: audio-finite-catalog.json')
        expected, media = original_oracle()
        actual = read_json(args.catalog)
        require(actual == expected, 'runtime finite catalog differs from complete original/WAV oracle')
        manifest = read_json(args.manifest)
        require(manifest.get('catalog_sha256') == digest(args.catalog.read_bytes())
                and manifest.get('source_catalog_sha256') == SOURCE_SHA and manifest.get('rom_sha256') == ROM_SHA,
                'finite manifest digest or source identity differs')
        require(manifest.get('clips') == 134 and manifest.get('animation_unique_codes') == 78,
                'finite manifest coverage differs')
        report.update({'pass': True, 'catalog_sha256': digest(args.catalog.read_bytes()),
                       'catalog_bytes': args.catalog.stat().st_size, 'finite_clips': 134,
                       'animation_characters': 11, 'animation_unique_codes': 78,
                       'wav_files_verified': len(media), 'wav_bytes_verified': sum(item['bytes'] for item in media.values()),
                       'finite8f_onset': media[143]['first_nonzero_stereo_frame'],
                       'media': [{'code': code, **item} for code, item in sorted(media.items())]})
    except (ValueError, KeyError, OSError, wave.Error) as error:
        report.update({'pass': False, 'error': str(error)})
    raw = json.dumps(report, indent=2) + '\n'
    if args.report:
        target = args.report.resolve()
        require(target.is_relative_to(ROOT), 'report must remain inside worktree')
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(raw)
    print(raw, end='')
    raise SystemExit(0 if report['pass'] else 1)


if __name__ == '__main__':
    main()
