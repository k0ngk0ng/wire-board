import array
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
import tempfile
import urllib.request
import wave

ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / '.local/pal/original-98'
MEDIA = ROOT / '.local/pal/original-98-media'
FONT = ROOT / '.local/pal/audio-synthesis-source'
OUT = ROOT / '.local/pal/audio-media-e2e'
REPORT = ROOT / '.local/pal/verification/audio-media-export.json'
EXPORTER = ROOT / 'games/pal/assets/export-audio.py'
SYNTH_BUILDER = ROOT / 'games/pal/build/audio-synth.py'
ORACLE_URL = 'https://files.pythonhosted.org/packages/b5/6d/e18a5b59ff086e1cd61d7fbf943d86c5f593a4e68bfc60215ab74210b22b/mido-1.2.10-py2.py3-none-any.whl'
ORACLE_SHA = '0e618232063e0a220249da4961563c7636fea00096cfb3e2b87a4231f0ac1a9e'
UNMAPPED = {
    42: (['No instrument mapped to drum set 16, program 26', 'No instrument mapped to drum set 0, program 26 - this instrument will not be heard'], {26: 64}),
    43: (['No instrument mapped to drum set 16, program 26', 'No instrument mapped to drum set 0, program 26 - this instrument will not be heard'], {26: 96}),
    49: (['No instrument mapped to drum set 0, program 95 - this instrument will not be heard'], {95: 1}),
    54: (['No instrument mapped to drum set 16, program 1', 'No instrument mapped to drum set 0, program 1 - this instrument will not be heard'], {1: 1}),
    64: (['No instrument mapped to drum set 16, program 89', 'No instrument mapped to drum set 16, program 92', 'No instrument mapped to drum set 0, program 89 - this instrument will not be heard', 'No instrument mapped to drum set 0, program 92 - this instrument will not be heard'], {89: 1, 92: 1}),
    66: (['No instrument mapped to drum set 16, program 25', 'No instrument mapped to drum set 0, program 25 - this instrument will not be heard'], {25: 128}),
}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def decode(path, bits):
    encoding = {8: 'u8', 16: 's16le'}[bits]
    return subprocess.check_output(['ffmpeg', '-v', 'error', '-i', str(path), '-map', '0:a:0', '-c:a', 'pcm_' + encoding, '-f', encoding, '-'])


def probe(path):
    return json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_streams', '-of', 'json', str(path)]))['streams'][0]


def run(extra=(), good=True):
    command = [sys.executable, '-B', str(EXPORTER), '--out', str(OUT), *map(str, extra)]
    result = subprocess.run(command, cwd=ROOT, text=True, capture_output=True)
    if good:
        assert result.returncode == 0, result.stdout + result.stderr
    else:
        assert result.returncode != 0, command
    return {'command': command, 'code': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr}


def oracle():
    FONT.mkdir(parents=True, exist_ok=True)
    target = FONT / 'mido-1.2.10-py2.py3-none-any.whl'
    data = target.read_bytes() if target.exists() else urllib.request.urlopen(ORACLE_URL, timeout=45).read()
    assert len(data) == 51094 and sha(data) == ORACLE_SHA
    target.write_bytes(data)
    sys.path.insert(0, str(target))
    import mido
    return mido


def midi_duration(mido, data):
    midi = mido.MidiFile(file=io.BytesIO(data))
    assert midi.type in (0, 1) and midi.ticks_per_beat > 0
    tempo = 500000
    elapsed = Fraction(0)
    count = 0
    for event in mido.merge_tracks(midi.tracks):
        elapsed += Fraction(event.time * tempo, midi.ticks_per_beat * 1000000)
        if event.type == 'set_tempo':
            tempo = event.tempo
        elif not event.is_meta:
            count += 1
    samples = (elapsed.numerator * 44100 + elapsed.denominator - 1) // elapsed.denominator
    return samples, count


def main():
    assert EXPORTER.is_file(), 'audio exporter has not been implemented'
    assert SYNTH_BUILDER.is_file(), 'deterministic MIDI renderer builder has not been implemented'
    synthesis = json.loads(subprocess.check_output([sys.executable, '-B', str(SYNTH_BUILDER)], cwd=ROOT, text=True))
    assert synthesis['recipe'].get('explicitConfigurationOnly') is True
    renderer = ROOT / synthesis['binary']
    assert sha(renderer.read_bytes()) == synthesis['binarySHA256']
    mido = oracle()
    originals = {}
    source_index = json.loads((ROOT / 'games/pal/assets/source-manifest.json').read_text())
    media_index = json.loads((ROOT / 'games/pal/assets/media-source-manifest.json').read_text())
    for base, index in [(SOURCE, source_index), (MEDIA, media_index)]:
        for item in index['files']:
            data = (base / item['path']).read_bytes()
            assert len(data) == item['size'] and sha(data) == item['sha256']
            originals[str(base / item['path'])] = sha(data)
    first = run()
    index_bytes = (OUT / 'index.json').read_bytes()
    index = json.loads(index_bytes)
    assert index['version'] == 1
    assert len(index['sounds']) == 363 and len(index['music']) == 87 and len(index['cd']) == 8
    for name, row in index['files'].items():
        target = OUT / name
        assert target.resolve().is_relative_to(OUT.resolve())
        data = target.read_bytes()
        assert len(data) == row['size'] and sha(data) == row['sha256'], name
        assert row['mime'] in ('audio/flac', 'audio/wav', 'audio/ogg', 'application/gzip')
    archive = (SOURCE / 'sounds.mkf').read_bytes()
    offsets = struct.unpack_from('<' + 'I' * (struct.unpack_from('<I', archive)[0] // 4), archive)
    sounds = []
    for number, (start, end) in enumerate(zip(offsets, offsets[1:])):
        if start == end:
            assert str(number) not in index['sounds']
            continue
        row = index['sounds'][str(number)]
        original = archive[start:end]
        with wave.open(io.BytesIO(original)) as sound:
            expected = sound.readframes(sound.getnframes())
            shape = [sound.getframerate(), sound.getnchannels(), sound.getsampwidth() * 8, sound.getnframes()]
        assert row['sourceSHA256'] == sha(original)
        assert [row['sampleRate'], row['channels'], row['bits'], row['samples']] == shape
        exported = OUT / row['path']
        assert decode(exported, row['bits']) == expected
        assert row['pcmSHA256'] == sha(expected)
        stream = probe(exported)
        assert int(stream['sample_rate']) == shape[0] and stream['channels'] == shape[1]
        assert int(stream.get('bits_per_raw_sample', stream.get('bits_per_sample', 0))) == shape[2]
        assert row['wavBytes'] == struct.unpack_from('<I', original, 4)[0] + 8
        assert index['files'][row['path']]['size'] == min(row['wavBytes'], row['flacBytes'])
        sounds.append({'id': number, 'path': row['path'], 'pcmSHA256': row['pcmSHA256'], 'shape': shape})
    music = []
    with tempfile.TemporaryDirectory(prefix='audio-media-oracle-', dir=ROOT / '.local/pal') as temporary:
        for number in range(1, 88):
            row = index['music'][str(number)]
            original_path = SOURCE / f'musics/{number:03d}.mid'
            original = original_path.read_bytes()
            assert row['sourceSHA256'] == sha(original)
            assert gzip.decompress((OUT / row['midiPath']).read_bytes()) == original
            samples, events = midi_duration(mido, original)
            assert row['samples'] == samples and row['events'] == events
            assert row['sampleRate'] == 44100 and row['bits'] == 16 and row['channels'] == 2
            pcm = decode(OUT / row['path'], 16)
            assert len(pcm) == samples * 4 and sha(pcm) == row['pcmSHA256']
            values = array.array('h', pcm)
            assert any(values), number
            reference = Path(temporary) / 'reference.wav'
            process = subprocess.run([str(renderer), '-c', 'timidity.cfg', '-OwS1l', '-s', '44100', '-A', '70', '-p', '256', '-S', '0', '--no-polyphony-reduction', '--preserve-silence', '-EFreverb=d', '-EFchorus=d', '-EFdelay=d', '-EFresamp=l', '-EFns=0', '-o', str(reference), str(original_path)], cwd=FONT, capture_output=True, text=True)
            assert process.returncode == 0, process.stderr
            log = process.stdout + process.stderr
            assert 'Notes lost totally: 0' in log
            assert 'Notes cut: 0' in log
            warnings = [line for line in log.splitlines() if 'No instrument' in line]
            expected_warnings = UNMAPPED[number][0] if number in UNMAPPED else []
            assert warnings == expected_warnings and "Can't open" not in log
            assert row['unmappedRequests'] == expected_warnings
            if number in UNMAPPED:
                midi = mido.MidiFile(file=io.BytesIO(original))
                expected_notes = UNMAPPED[number][1]
                notes = [event for event in mido.merge_tracks(midi.tracks) if event.type == 'note_on' and event.channel == 9 and event.note in expected_notes and event.velocity > 0]
                assert {key: len([event for event in notes if event.note == key]) for key in expected_notes} == expected_notes
            with wave.open(str(reference)) as sound:
                assert sound.getnframes() >= samples
                assert sound.readframes(samples) == pcm, number
            music.append({'id': number, 'samples': samples, 'events': events, 'pcmSHA256': sha(pcm), 'independentRenderCompared': True})
    cd = []
    for number in range(2, 10):
        row = index['cd'][str(number)]
        original = (MEDIA / f'cd/{number:02d}.ogg').read_bytes()
        assert (OUT / row['path']).read_bytes() == original
        assert row['sourceSHA256'] == sha(original)
        assert row['gzipBytes'] == len(gzip.compress(original, compresslevel=9, mtime=0))
        assert row['gzipNativePlayable'] is False
        assert row['pcmSHA256'] == sha(decode(OUT / row['path'], 16))
        cd.append({'id': number, 'sha256': sha(original), 'bytes': len(original)})
    assert before_unchanged(originals)
    second = run()
    assert (OUT / 'index.json').read_bytes() == index_bytes
    for name, item in index['files'].items():
        assert sha((OUT / name).read_bytes()) == item['sha256']
    negative = []
    for bad in ['games/pal/assets', '.local/pal/original-98', '.local/pal/original-98-media', '../audio-media-outside']:
        negative.append(run(['--out', bad], good=False))
    with tempfile.TemporaryDirectory(prefix='audio-media-negative-', dir=ROOT / '.local/pal') as temporary:
        scratch = Path(temporary)
        for name in ['sounds.mkf', 'musics/001.mid']:
            target = scratch / 'source' / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(SOURCE / name, target)
        damaged = scratch / 'source/sounds.mkf'
        data = bytearray(damaged.read_bytes())
        data[-1] ^= 1
        damaged.write_bytes(data)
        negative.append(run(['--source', damaged.parent], good=False))
        shutil.copyfile(SOURCE / 'sounds.mkf', damaged)
        damaged = scratch / 'source/musics/001.mid'
        data = bytearray(damaged.read_bytes())
        data[-2] ^= 1
        damaged.write_bytes(data)
        negative.append(run(['--source', scratch / 'source'], good=False))
        (scratch / 'font').mkdir()
        (scratch / 'font/TimGM6mb.sf2').write_bytes(b'invalid SoundFont')
        negative.append(run(['--font-root', scratch / 'font'], good=False))
        protected = scratch / 'protected'
        protected.mkdir()
        (protected / 'sentinel').write_text('unchanged')
        linked = ROOT / '.local/pal/audio-media-e2e-linked'
        assert not linked.exists() and not linked.is_symlink()
        linked.symlink_to(protected, target_is_directory=True)
        try:
            negative.append(run(['--out', linked], good=False))
        finally:
            linked.unlink()
        nested = OUT / 'sounds/forbidden'
        nested.symlink_to(protected, target_is_directory=True)
        try:
            negative.append(run(good=False))
        finally:
            nested.unlink()
        assert (protected / 'sentinel').read_text() == 'unchanged'
        assert len(list(protected.iterdir())) == 1
    assert (OUT / 'index.json').read_bytes() == index_bytes
    assert before_unchanged(originals)
    REPORT.parent.mkdir(parents=True, exist_ok=True)
    REPORT.write_text(json.dumps({'passed': True, 'scope': 'all original PCM effects, original-MIDI-derived deterministic synthesis and byte-identical CD Ogg; independent decoding, Mido timing, reproducibility and protected-output rejection; browser playback and saved audio timeline remain unverified', 'indexSHA256': sha(index_bytes), 'files': len(index['files']), 'bytes': sum(row['size'] for row in index['files'].values()), 'sounds': sounds, 'music': music, 'cd': cd, 'oracleSHA256': ORACLE_SHA, 'runs': [first, second], 'negative': negative, 'synthesis': index['synthesis']}, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': True, 'sounds': len(sounds), 'music': len(music), 'cd': len(cd), 'report': str(REPORT.relative_to(ROOT))}))


def before_unchanged(originals):
    return all(sha(Path(name).read_bytes()) == digest for name, digest in originals.items())


if __name__ == '__main__':
    main()
