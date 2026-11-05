#!/usr/bin/env python3
"""Export only the134 already-accepted finite WAV metadata, without recapture."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import wave

ROOT = Path(__file__).resolve().parents[2]
SOURCE_SHA = '617737f8ef07043401fa74f7f48ba551a64dd91cedd3a94dbfcf4883e0ff1541'
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
MEDIA_ROOT = ROOT / '.local/md-yuyuhakusho/audio-assets-final'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def require(value, message):
    if not value:
        raise ValueError(message)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    require(out.is_relative_to(ROOT), 'outputs must remain inside this worktree')
    source_path = ROOT / 'games/yuyuhakusho/data/audio-assets.json'
    source_raw = source_path.read_bytes()
    require(sha(source_raw) == SOURCE_SHA, 'immutable original source catalog differs')
    source = json.loads(source_raw)
    expected_format = {'sample_rate': 44100, 'channels': 2, 'sample_width_bytes': 2, 'pcm_encoding': 'signed16-little-endian'}
    require(source['rom_sha256'] == ROM_SHA and source['format'] == expected_format, 'original identity/format differs')
    clips, media_bytes, pcm_bytes = [], 0, 0
    for original in sorted(source['assets'], key=lambda row: row['code']):
        code = original['code']
        kind = ('pcm-sample-command' if 6 <= code <= 126 else
                'finite-fm-psg-sequence' if 160 <= code <= 171 else
                'finite-music-cue' if code == 143 else None)
        require(kind is not None and original['classification'] == kind and original['complete']
                and original['runtime_asset_ready'] and original['pcm_replay_equal'], 'nonfinite or unverified original entry')
        path = (ROOT / original['media_file']).resolve()
        require(path.is_relative_to(MEDIA_ROOT) and original['logical_asset'] == f'yuyuhakusho/audio/{code:02x}.wav',
                'original media identity differs')
        raw = path.read_bytes()
        require(sha(raw) == original['wav_sha256'], f'original WAV SHA differs: {code:02x}')
        with wave.open(io.BytesIO(raw), 'rb') as wav:
            require((wav.getnchannels(), wav.getsampwidth(), wav.getframerate(), wav.getnframes(), wav.getcomptype())
                    == (2, 2, 44100, original['stereo_frames'], 'NONE'), f'original WAV format/count differs: {code:02x}')
            pcm = wav.readframes(wav.getnframes())
        require(len(pcm) == original['stereo_frames'] * 4 and sha(pcm) == original['pcm_sha256'],
                f'original PCM SHA/count differs: {code:02x}')
        onset = None
        for frame in range(len(pcm) // 4):
            if pcm[frame * 4:frame * 4 + 4] != b'\0\0\0\0':
                onset = frame
                break
        require(onset is not None, 'accepted finite original is unexpectedly silent')
        old_onset = original.get('first_nonzero_stereo_frame')
        require((old_onset == onset) if code != 143 else (old_onset is None and onset == 971),
                f'original onset differs: {code:02x}')
        clips.append({'code': code, 'kind': kind, 'stereo_frames': original['stereo_frames'],
                      'wav_sha256': sha(raw), 'pcm_sha256': sha(pcm),
                      'first_nonzero_stereo_frame': onset, 'loop': False})
        media_bytes += len(raw)
        pcm_bytes += len(pcm)
    require([row['code'] for row in clips] == [*range(6, 127), 143, *range(160, 172)], 'finite code set differs')
    bindings = []
    for character in range(11):
        codes = sorted(row['code'] for row in source['animation_bindings'] if character in row['characters'])
        require(codes and len(codes) == len(set(codes)), 'invalid source-character animation references')
        bindings.append({'character': character, 'codes': codes})
    require(len({code for row in bindings for code in row['codes']}) == 78, 'animation-reference union differs')
    catalog = {'schema': 1, 'rom_sha256': ROM_SHA, 'source_catalog_sha256': SOURCE_SHA,
               'format': expected_format, 'source_origin': 'isolated-capture-start',
               'clips': clips, 'animation_bindings': bindings}
    canonical = (json.dumps(catalog, separators=(',', ':')) + '\n').encode()
    manifest = {'schema': 1, 'pass': True, 'scope': 'Complete finite original media metadata only; no sound scheduling, mixed output or continuous BGM acceptance',
                'rom_sha256': ROM_SHA, 'source_catalog_sha256': SOURCE_SHA,
                'source_catalog': 'games/yuyuhakusho/data/audio-assets.json',
                'producer': 'scripts/yuyuhakusho/export_finite_audio_catalog.py',
                'producer_sha256': sha(Path(__file__).read_bytes()), 'catalog_sha256': sha(canonical),
                'catalog_bytes': len(canonical), 'clips': len(clips),
                'clip_kinds': {'pcm-sample-command': 121, 'finite-fm-psg-sequence': 12, 'finite-music-cue': 1},
                'animation_characters': 11, 'animation_unique_codes': 78,
                'animation_reference_count': sum(len(row['codes']) for row in bindings),
                'wav_files_verified': 134, 'wav_bytes_verified': media_bytes, 'pcm_bytes_verified': pcm_bytes,
                'source_origin': 'isolated-capture-start', 'finite8f_onset_stereo_frame': 971,
                'excluded': {'controls': '00..05', 'silent_diagnostic': '7f', 'bounded_music': ['82', '87'],
                             'other_music': 'All other BGM codes'},
                'authentication_boundary': 'Caller authenticates immutable manifest/catalog digest; ROM/source fields alone do not authenticate arbitrary clip metadata',
                'limits': ['Animation references are not a complete gameplay reachability or required preload set',
                           'Original leading delay and trailing samples are preserved; duration is derived from stereo_frames/44100',
                           'No WAV body, local/absolute media path, ROM address, runtime synthesis or playback code in this catalog',
                           'No original source metadata or WAV was modified; no media was published']}
    out.mkdir(parents=True, exist_ok=True)
    (out / 'audio-finite-catalog.json').write_bytes(canonical)
    (out / 'audio-finite-catalog-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps(manifest, indent=2), flush=True)


if __name__ == '__main__':
    main()
