import argparse
from fractions import Fraction
import gzip
import hashlib
import io
import json
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import wave

ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
ASSETS = Path(__file__).parent


def sha(data):
    return hashlib.sha256(data).hexdigest()


def safe_output(path):
    output = path.resolve()
    if not LOCAL.resolve().is_relative_to(ROOT) or output.parent != LOCAL.resolve() or not output.name.startswith('audio-media'):
        raise ValueError('output must be a dedicated .local/pal/audio-media* directory inside the workspace')
    if path.is_symlink() or (output.exists() and (not output.is_dir() or any(item.is_symlink() for item in output.rglob('*')))):
        raise ValueError('audio output contains a symbolic link or is not a directory')
    return output


def pinned(path, item, size='size', digest='sha256'):
    data = path.read_bytes()
    if len(data) != item[size] or sha(data) != item[digest]:
        raise ValueError('audio source integrity mismatch: ' + str(path))
    return data


def font_source(path, manifest):
    path = path.resolve()
    if not path.is_relative_to(LOCAL.resolve()) or not path.is_relative_to(ROOT):
        raise ValueError('font cache must stay inside .local/pal')
    default = path == LOCAL / 'audio-synthesis-source'
    required = manifest['soundFont']['archives']
    for item in required:
        target = path / item['output']
        if target.exists():
            pinned(target, item, 'outputSize', 'outputSHA256')
        elif not default:
            raise ValueError('missing pinned font source: ' + str(target))
    path.mkdir(parents=True, exist_ok=True)
    for item in required:
        archive_path = path / item['path']
        if archive_path.exists():
            data = pinned(archive_path, item)
        elif default:
            data = urllib.request.urlopen(item['url'], timeout=45).read()
            if len(data) != item['size'] or sha(data) != item['sha256']:
                raise ValueError('downloaded font archive integrity mismatch')
            archive_path.write_bytes(data)
        else:
            raise ValueError('missing pinned font archive: ' + str(archive_path))
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            member = archive.getmember(item['member'])
            if not member.isfile():
                raise ValueError('font archive member is not a regular file')
            payload = archive.extractfile(member).read()
        if len(payload) != item['outputSize'] or sha(payload) != item['outputSHA256']:
            raise ValueError('font archive member integrity mismatch')
        target = path / item['output']
        if target.is_symlink():
            raise ValueError('font source cannot be a symbolic link')
        if not target.exists():
            target.write_bytes(payload)
    config = path / 'timidity.cfg'
    expected = manifest['renderer']['config'].encode()
    if config.is_symlink() or (config.exists() and config.read_bytes() != expected):
        raise ValueError('synthesis config does not match the pinned configuration')
    if not config.exists():
        config.write_bytes(expected)
    return path


def midi_timing(data):
    if len(data) < 14 or data[:4] != b'MThd':
        raise ValueError('invalid MIDI header')
    header = struct.unpack_from('>I', data, 4)[0]
    version, count, division = struct.unpack_from('>HHH', data, 8)
    if header < 6 or version not in (0, 1) or count < 1 or (version == 0 and count != 1) or division == 0 or division & 0x8000:
        raise ValueError('unsupported MIDI timing or track format')
    cursor = 8 + header
    messages = []
    event_count = 0
    for track in range(count):
        if cursor + 8 > len(data) or data[cursor:cursor + 4] != b'MTrk':
            raise ValueError('invalid MIDI track header')
        size = struct.unpack_from('>I', data, cursor + 4)[0]
        cursor += 8
        finish = cursor + size
        if finish > len(data):
            raise ValueError('truncated MIDI track')
        tick = 0
        status = 0
        ended = False

        def byte():
            nonlocal cursor
            if cursor >= finish:
                raise ValueError('truncated MIDI event')
            value = data[cursor]
            cursor += 1
            return value

        def variable():
            value = 0
            for unused in range(4):
                digit = byte()
                value = (value << 7) | (digit & 127)
                if not digit & 128:
                    return value
            raise ValueError('invalid MIDI variable integer')

        while cursor < finish:
            if ended:
                raise ValueError('MIDI data after end of track')
            tick += variable()
            current = data[cursor] if cursor < finish else byte()
            if current & 128:
                current = byte()
                if current < 240:
                    status = current
                elif current != 255:
                    status = 0
            elif status:
                current = status
            else:
                raise ValueError('MIDI running status without a channel message')
            tempo = None
            if current == 255:
                kind = byte()
                length = variable()
                if cursor + length > finish:
                    raise ValueError('truncated MIDI meta event')
                payload = data[cursor:cursor + length]
                cursor += length
                if kind == 81:
                    if length != 3 or int.from_bytes(payload, 'big') == 0:
                        raise ValueError('invalid MIDI tempo')
                    tempo = int.from_bytes(payload, 'big')
                elif kind == 47:
                    if length:
                        raise ValueError('invalid MIDI end-of-track event')
                    ended = True
            elif current in (240, 247):
                length = variable()
                cursor += length
                if cursor > finish:
                    raise ValueError('truncated MIDI system event')
                event_count += 1
            elif 128 <= current < 240:
                length = 1 if current & 240 in (192, 208) else 2
                for unused in range(length):
                    if byte() & 128:
                        raise ValueError('invalid MIDI channel parameter')
                event_count += 1
            else:
                raise ValueError('unsupported MIDI system event')
            messages.append((tick, track, len(messages), tempo))
        if not ended:
            raise ValueError('MIDI track lacks end-of-track event')
    padding = len(data) - cursor
    if padding != 0 and not (padding == 1 and cursor % 2 == 1):
        raise ValueError('invalid MIDI archive padding')
    tempo = 500000
    previous = 0
    elapsed = Fraction(0)
    for tick, track, order, change in sorted(messages):
        elapsed += Fraction((tick - previous) * tempo, division * 1000000)
        previous = tick
        if change is not None:
            tempo = change
    samples = (elapsed.numerator * 44100 + elapsed.denominator - 1) // elapsed.denominator
    if samples <= 0 or samples > 44100 * 3600:
        raise ValueError('invalid MIDI duration')
    return {'samples': samples, 'events': event_count, 'duration': [elapsed.numerator, elapsed.denominator]}


def decode(path, bits):
    encoding = {8: 'u8', 16: 's16le'}[bits]
    return subprocess.check_output(['ffmpeg', '-v', 'error', '-i', str(path), '-map', '0:a:0', '-c:a', 'pcm_' + encoding, '-f', encoding, '-'])


def compress(wav, output):
    subprocess.run(['flac', '-8', '--verify', '--no-padding', '--no-seektable', '--silent', '--force', '--output-name=' + str(output), '-'], input=wav, check=True, capture_output=True)
    return output.read_bytes()


def tool(name, arguments):
    binary = shutil.which(name)
    if binary is None:
        raise ValueError('missing audio tool: ' + name)
    result = subprocess.run([binary, *arguments], capture_output=True, text=True, check=True)
    return {'name': name, 'version': (result.stdout + result.stderr).strip().splitlines()[0], 'binarySHA256': sha(Path(binary).resolve().read_bytes())}


def export(args):
    output = safe_output(args.out)
    source_manifest = json.loads((ASSETS / 'source-manifest.json').read_text())
    media_manifest = json.loads((ASSETS / 'media-source-manifest.json').read_text())
    synthesis_bytes = (ASSETS / 'audio-synthesis-manifest.json').read_bytes()
    synthesis_manifest = json.loads(synthesis_bytes)
    originals = {item['path']: pinned(args.source / item['path'], item) for item in source_manifest['files'] if item['path'] == 'sounds.mkf' or item['path'].startswith('musics/')}
    cd_sources = {item['path']: pinned(args.media_source / item['path'], item) for item in media_manifest['files'] if item['path'].startswith('cd/')}
    timings = {name: midi_timing(data) for name, data in originals.items() if name.startswith('musics/')}
    synth = json.loads(subprocess.check_output([sys.executable, '-B', str(ROOT / 'games/pal/build/audio-synth.py')], text=True, cwd=ROOT))
    renderer = ROOT / synth['binary']
    tools = {name: tool(name, ['--version'] if name == 'flac' else ['-version']) for name in ('flac', 'ffmpeg')}
    tools['timidity'] = tool(str(renderer), ['--version'])
    if tools['timidity']['version'] != synthesis_manifest['renderer']['versionLine']:
        raise ValueError('unsupported TiMidity++ version')
    font = font_source(args.font_root, synthesis_manifest)
    archive = originals['sounds.mkf']
    table_size = struct.unpack_from('<I', archive)[0]
    if table_size < 8 or table_size % 4 or table_size > len(archive):
        raise ValueError('invalid sound MKF index')
    offsets = struct.unpack_from('<' + 'I' * (table_size // 4), archive)
    if offsets[0] != table_size or offsets[-1] != len(archive) or any(a > b for a, b in zip(offsets, offsets[1:])):
        raise ValueError('invalid sound MKF bounds')
    index = {'version': 1, 'sourceCommit': source_manifest['commit'], 'mediaCommit': media_manifest['commit'], 'sourceManifestSHA256': sha((ASSETS / 'source-manifest.json').read_bytes()), 'mediaManifestSHA256': sha((ASSETS / 'media-source-manifest.json').read_bytes()), 'soundSlots': len(offsets) - 1, 'emptySoundSlots': [], 'synthesis': {'manifestSHA256': sha(synthesis_bytes), 'soundFontSHA256': sha((font / 'TimGM6mb.sf2').read_bytes()), 'configSHA256': sha((font / 'timidity.cfg').read_bytes()), 'arguments': synthesis_manifest['renderer']['arguments'], 'tools': tools, 'rendererBuild': synth, 'sampleRate': 44100, 'channels': 2, 'bits': 16, 'loopBoundary': 'ceil-original-max-track-end-at-44100Hz', 'originalPCM': False}, 'sounds': {}, 'music': {}, 'cd': {}, 'files': {}}
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.work-', dir=output) as temporary:
        work = Path(temporary)

        def write(name, mime, data):
            target = output / name
            if not target.resolve().is_relative_to(output) or target.is_symlink():
                raise ValueError('audio output escapes through symbolic link')
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            index['files'][name] = {'mime': mime, 'size': len(data), 'sha256': sha(data)}

        for number, (start, end) in enumerate(zip(offsets, offsets[1:])):
            if start == end:
                index['emptySoundSlots'].append(number)
                continue
            chunk = archive[start:end]
            length = struct.unpack_from('<I', chunk, 4)[0] + 8
            if length > len(chunk) or len(chunk) - length not in (0, length % 2):
                raise ValueError('invalid original RIFF alignment')
            with wave.open(io.BytesIO(chunk)) as audio:
                channels, width, rate, frames = audio.getnchannels(), audio.getsampwidth(), audio.getframerate(), audio.getnframes()
                pcm = audio.readframes(frames)
                if width not in (1, 2) or len(pcm) != frames * channels * width:
                    raise ValueError('unsupported or truncated original PCM')
            flac = compress(chunk[:length], work / 'sound.flac')
            if decode(work / 'sound.flac', width * 8) != pcm:
                raise ValueError('sound FLAC differs from original PCM')
            streaminfo = flac[8:42]
            packed = int.from_bytes(streaminfo[10:18], 'big')
            if ((packed >> 36) & 31) + 1 != width * 8:
                raise ValueError('FLAC changed original sound bit depth')
            use_flac = len(flac) < length
            name = f'sounds/{number}.flac' if use_flac else f'sounds/{number}.wav'
            write(name, 'audio/flac' if use_flac else 'audio/wav', flac if use_flac else chunk[:length])
            index['sounds'][str(number)] = {'path': name, 'sourceSHA256': sha(chunk), 'sampleRate': rate, 'channels': channels, 'bits': width * 8, 'samples': frames, 'pcmSHA256': sha(pcm), 'wavBytes': length, 'flacBytes': len(flac)}
        print(json.dumps({'stage': 'sounds', 'count': len(index['sounds'])}), flush=True)
        for name, timing in sorted(timings.items()):
            number = int(Path(name).stem)
            wav_path = work / 'music.wav'
            result = subprocess.run([str(renderer), *synthesis_manifest['renderer']['arguments'], '-o', str(wav_path), str((args.source / name).resolve())], cwd=font, capture_output=True, text=True, check=True)
            log = result.stdout + result.stderr
            expected = synthesis_manifest.get('unmappedRequests', {}).get(name)
            if expected and sha(originals[name]) != expected['sourceSHA256']:
                raise ValueError('unmapped MIDI request source identity changed')
            warnings = [line for line in log.splitlines() if 'No instrument' in line]
            if 'Notes lost totally: 0' not in log or 'Notes cut: 0' not in log or warnings != (expected['warnings'] if expected else []) or "Can't open" in log:
                raise ValueError('MIDI synthesis lost instruments or notes: ' + name + '\n' + log)
            with wave.open(str(wav_path)) as audio:
                if (audio.getnchannels(), audio.getsampwidth(), audio.getframerate()) != (2, 2, 44100) or audio.getnframes() < timing['samples']:
                    raise ValueError('MIDI synthesis has invalid PCM parameters or duration')
                pcm = audio.readframes(timing['samples'])
            if not any(pcm):
                raise ValueError('MIDI synthesis produced silence')
            container = io.BytesIO()
            with wave.open(container, 'wb') as audio:
                audio.setparams((2, 2, 44100, timing['samples'], 'NONE', 'not compressed'))
                audio.writeframes(pcm)
            flac = compress(container.getvalue(), work / 'music.flac')
            if decode(work / 'music.flac', 16) != pcm:
                raise ValueError('music FLAC differs from generated PCM')
            path = f'music/{number}.flac'
            midi_path = f'midi/{number}.mid.gz'
            write(path, 'audio/flac', flac)
            write(midi_path, 'application/gzip', gzip.compress(originals[name], compresslevel=9, mtime=0))
            index['music'][str(number)] = {'path': path, 'midiPath': midi_path, 'sourceSHA256': sha(originals[name]), 'sampleRate': 44100, 'channels': 2, 'bits': 16, **timing, 'pcmSHA256': sha(pcm), 'notesLost': 0, 'notesCut': 0, 'unmappedRequests': warnings}
            print(json.dumps({'stage': 'music', 'track': number}), flush=True)
        for name, data in sorted(cd_sources.items()):
            number = int(Path(name).stem)
            pcm = decode(args.media_source / name, 16)
            stream = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-of', 'json', str(args.media_source / name)]))['streams'][0]
            write(name, 'audio/ogg', data)
            index['cd'][str(number)] = {'path': name, 'sourceSHA256': sha(data), 'sampleRate': int(stream['sample_rate']), 'channels': stream['channels'], 'samples': len(pcm) // (stream['channels'] * 2), 'pcmSHA256': sha(pcm), 'pcmEncoding': 'decoded-reference-s16le', 'gzipBytes': len(gzip.compress(data, compresslevel=9, mtime=0)), 'gzipNativePlayable': False}
    index['files'] = dict(sorted(index['files'].items()))
    (output / 'index.json').write_text(json.dumps(index, ensure_ascii=False, indent=2) + '\n')
    return {'sounds': len(index['sounds']), 'music': len(index['music']), 'cd': len(index['cd']), 'files': len(index['files']), 'bytes': sum(item['size'] for item in index['files'].values())}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--source', type=Path, default=LOCAL / 'original-98')
    parser.add_argument('--media-source', type=Path, default=LOCAL / 'original-98-media')
    parser.add_argument('--font-root', type=Path, default=LOCAL / 'audio-synthesis-source')
    args = parser.parse_args()
    print(json.dumps(export(args)))


if __name__ == '__main__':
    main()
