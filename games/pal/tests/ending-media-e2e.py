import gzip
import hashlib
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / '.local/pal/original-98-media'
OUT = ROOT / '.local/pal/ending-media-e2e'
REPORT = ROOT / '.local/pal/verification/ending-media-export.json'
MANIFEST = json.loads((ROOT / 'games/pal/assets/media-source-manifest.json').read_text())

def sha(data):
    return hashlib.sha256(data).hexdigest()

def decode(path, kind):
    args = ['ffmpeg', '-v', 'error', '-i', str(path)]
    if kind == 'video':
        args += ['-map', '0:v:0', '-fps_mode', 'passthrough', '-pix_fmt', 'rgb24', '-f', 'rawvideo', '-']
    else:
        args += ['-map', '0:a:0', '-acodec', 'pcm_u8', '-f', 'u8', '-']
    return subprocess.Popen(args, cwd=ROOT, stdout=subprocess.PIPE)

def digest_stream(process):
    digest = hashlib.sha256()
    count = 0
    while True:
        chunk = process.stdout.read(1048576)
        if not chunk:
            break
        digest.update(chunk)
        count += len(chunk)
    assert process.wait() == 0
    return count, digest.hexdigest()

before = {row['path']: sha((SOURCE / row['path']).read_bytes()) for row in MANIFEST['files']}
command = [sys.executable, '-B', 'games/pal/assets/export-ending.py', '--out', str(OUT)]
subprocess.run(command, cwd=ROOT, check=True)
index = json.loads((OUT / 'index.json').read_text())
assert index['version'] == 1
for name, row in index['files'].items():
    path = OUT / name
    assert path.resolve().is_relative_to(OUT.resolve())
    data = path.read_bytes()
    assert row['size'] == len(data) and row['sha256'] == sha(data)
    assert row['mime'] in ['application/gzip', 'application/json', 'audio/flac']
observed = []
for clip in [4, 5, 6]:
    metadata = json.loads((OUT / f'ending/{clip}.json').read_text())
    original = SOURCE / f'{clip}.avi'
    pinned = next(row for row in MANIFEST['files'] if row['path'] == original.name)
    assert metadata['sourceSHA256'] == pinned['sha256'] == before[original.name]
    assert metadata['width'] == 288 and metadata['height'] == 180
    assert metadata['rate'] == [15, 1]
    digest = hashlib.sha256()
    expected_start = 0
    for block in metadata['chunks']:
        assert block['first'] == expected_start and 0 < block['frames'] <= 15
        raw = gzip.decompress((OUT / block['path']).read_bytes())
        assert len(raw) == block['frames'] * 288 * 180 * 3
        digest.update(raw)
        expected_start += block['frames']
    expected_bytes, expected_hash = digest_stream(decode(original, 'video'))
    assert expected_start == metadata['frames']
    assert expected_bytes == expected_start * 288 * 180 * 3
    assert expected_hash == digest.hexdigest() == metadata['rgbSHA256']
    audio_source = digest_stream(decode(original, 'audio'))
    audio_export = digest_stream(decode(OUT / metadata['audio']['path'], 'audio'))
    assert audio_source == audio_export
    assert audio_export[1] == metadata['audio']['pcmSHA256']
    assert audio_export[0] == metadata['audio']['samples'] * metadata['audio']['channels']
    assert metadata['audio']['sampleRate'] == 22050
    probe = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-of', 'json', str(OUT / metadata['audio']['path'])]))
    assert probe['streams'][0]['codec_name'] == 'flac'
    assert int(probe['streams'][0]['bits_per_raw_sample']) == 8
    assert metadata['audio']['bits'] == 8
    assert int(probe['streams'][0]['sample_rate']) == metadata['audio']['sampleRate']
    assert probe['streams'][0]['channels'] == metadata['audio']['channels']
    observed.append({'clip': clip, 'frames': expected_start, 'chunks': len(metadata['chunks']), 'rgbSHA256': expected_hash, 'audioPCM': audio_export[1], 'audioBits': 8, 'sourceSHA256': before[original.name]})
assert before == {row['path']: sha((SOURCE / row['path']).read_bytes()) for row in MANIFEST['files']}
subprocess.run(command, cwd=ROOT, check=True)
assert index == json.loads((OUT / 'index.json').read_text())
for bad in ['games/pal/assets', '.local/pal/original-98-media', '.local/pal/verification/development-health-stage-23']:
    result = subprocess.run([*command[:-1], bad], cwd=ROOT, capture_output=True)
    assert result.returncode != 0, bad
REPORT.write_text(json.dumps({'passed': True, 'scope': 'all three pinned AVI clips exported to lossless RGB blocks and FLAC; complete FFmpeg RGB/PCM read-back, reproducibility and protected-output rejection; no browser or original-C pixel equivalence claim', 'clips': observed, 'files': len(index['files']), 'bytes': sum(row['size'] for row in index['files'].values())}, indent=2) + '\n')
print(REPORT.relative_to(ROOT))
