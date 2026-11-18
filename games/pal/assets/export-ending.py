import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import subprocess
import wave

ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / '.local/pal/original-98-media'
MANIFEST = Path(__file__).with_name('media-source-manifest.json')

def sha(data):
    return hashlib.sha256(data).hexdigest()

def pcm_digest(path):
    process = subprocess.Popen(['ffmpeg', '-v', 'error', '-i', str(path), '-map', '0:a:0', '-acodec', 'pcm_u8', '-f', 'u8', '-'], stdout=subprocess.PIPE)
    digest = hashlib.sha256()
    count = 0
    while True:
        chunk = process.stdout.read(1048576)
        if not chunk:
            break
        digest.update(chunk)
        count += len(chunk)
    if process.wait() != 0:
        raise ValueError('audio decoding failed')
    return count, digest.hexdigest()

def export(output):
    source = json.loads(MANIFEST.read_text())
    originals = {}
    for clip in [4, 5, 6]:
        item = next(row for row in source['files'] if row['path'] == f'{clip}.avi')
        path = SOURCE / item['path']
        data = path.read_bytes()
        if len(data) != item['size'] or sha(data) != item['sha256']:
            raise ValueError('original ending source integrity mismatch: ' + item['path'])
        originals[clip] = item
    output.mkdir(parents=True, exist_ok=True)
    files = {}
    def write(name, mime, data):
        target = output / name
        if not target.resolve().is_relative_to(output):
            raise ValueError('ending output escapes through symlink')
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        files[name] = {'size': len(data), 'sha256': sha(data), 'mime': mime}
    for clip, item in originals.items():
        path = SOURCE / item['path']
        probe = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-of', 'json', str(path)]))
        video = next(stream for stream in probe['streams'] if stream['codec_type'] == 'video')
        audio = next(stream for stream in probe['streams'] if stream['codec_type'] == 'audio')
        if (video['width'], video['height'], video['avg_frame_rate'], video['codec_name'], audio['codec_name']) != (288, 180, '15/1', 'msvideo1', 'pcm_u8'):
            raise ValueError('pinned ending media format changed')
        process = subprocess.Popen(['ffmpeg', '-v', 'error', '-i', str(path), '-map', '0:v:0', '-fps_mode', 'passthrough', '-pix_fmt', 'rgb24', '-f', 'rawvideo', '-'], stdout=subprocess.PIPE)
        chunks = []
        frames = 0
        frame_bytes = 288 * 180 * 3
        digest = hashlib.sha256()
        try:
            while True:
                data = process.stdout.read(frame_bytes * 15)
                if not data:
                    break
                if len(data) % frame_bytes:
                    raise ValueError('ending RGB stream contains a partial frame')
                count = len(data) // frame_bytes
                name = f'ending/{clip}/{frames}.rgb.gz'
                write(name, 'application/gzip', gzip.compress(data, compresslevel=6, mtime=0))
                chunks.append({'first': frames, 'frames': count, 'path': name})
                digest.update(data)
                frames += count
            if process.wait() != 0 or frames != int(video['nb_frames']):
                raise ValueError('ending frame count or decoder failed')
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait()
        audio_name = f'ending/{clip}.flac'
        audio_path = output / audio_name
        if not audio_path.resolve().is_relative_to(output):
            raise ValueError('ending audio output escapes through symlink')
        if int(audio['bits_per_sample']) != 8:
            raise ValueError('original ending PCM depth changed')
        samples = subprocess.check_output(['ffmpeg', '-v', 'error', '-i', str(path), '-map', '0:a:0', '-c:a', 'pcm_u8', '-f', 'u8', '-'])
        container = io.BytesIO()
        with wave.open(container, 'wb') as wav:
            wav.setnchannels(audio['channels'])
            wav.setsampwidth(1)
            wav.setframerate(int(audio['sample_rate']))
            wav.writeframes(samples)
        subprocess.run(['flac', '-8', '--verify', '--no-padding', '--no-seektable', '--silent', '--force', '--output-name=' + str(audio_path), '-'], input=container.getvalue(), check=True, capture_output=True)
        compressed = audio_path.read_bytes()
        if compressed[:4] != b'fLaC' or ((int.from_bytes(compressed[18:26], 'big') >> 36) & 31) + 1 != 8:
            raise ValueError('ending FLAC depth differs from original PCM')
        pcm = pcm_digest(path)
        if pcm != pcm_digest(audio_path):
            raise ValueError('ending audio is not lossless')
        write(audio_name, 'audio/flac', audio_path.read_bytes())
        metadata = {'version': 1, 'sourceSHA256': item['sha256'], 'width': 288, 'height': 180, 'rate': [15, 1], 'frames': frames, 'rgbSHA256': digest.hexdigest(), 'chunks': chunks,
                    'audio': {'path': audio_name, 'sampleRate': int(audio['sample_rate']), 'channels': audio['channels'], 'bits': 8, 'samples': pcm[0] // audio['channels'], 'pcmSHA256': pcm[1]}}
        write(f'ending/{clip}.json', 'application/json', (json.dumps(metadata, indent=2) + '\n').encode())
    index = {'version': 1, 'sourceCommit': source['commit'], 'files': dict(sorted(files.items()))}
    write('index.json', 'application/json', (json.dumps(index, indent=2) + '\n').encode())
    return {'clips': len(originals), 'files': len(index['files']), 'bytes': sum(row['size'] for row in index['files'].values())}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    output = args.out.resolve()
    allowed = (ROOT / '.local/pal').resolve()
    if output.parent != allowed or not output.name.startswith('ending-media'):
        parser.error('output must be a dedicated .local/pal/ending-media* directory inside the workspace')
    print(json.dumps(export(output)))

if __name__ == '__main__':
    main()
