#!/usr/bin/env python3
"""Pre-implementation acceptance of deterministic original Conquest audio."""
import hashlib
import json
import math
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import wave

ROOT = Path(__file__).resolve().parents[3]
ASSETS = ROOT / 'games/conquest/assets'
OUT = ROOT / '.local/conquest/audio'
NAMES = ('march', 'capture', 'round')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def inspect(path):
    with wave.open(str(path), 'rb') as stream:
        assert stream.getnchannels() == 1, path
        assert stream.getsampwidth() == 2, path
        assert stream.getframerate() == 22050, path
        assert stream.getcomptype() == 'NONE', path
        count = stream.getnframes()
        samples = struct.unpack('<' + 'h' * count, stream.readframes(count))
    duration = count / 22050
    peak = max(abs(s) for s in samples) / 32768
    rms = math.sqrt(sum(s * s for s in samples) / count) / 32768
    assert .1 < duration < .8, (path, duration)
    assert .05 < peak < .65, (path, peak)
    assert .01 < rms < .22, (path, rms)
    assert samples[0] == 0 and samples[-1] == 0, path
    assert max(abs(a-b) for a,b in zip(samples,samples[1:])) < 4500, path
    return {'frames':count,'seconds':duration,'peak':peak,'rms':rms,'sha256':digest(path),'bytes':path.stat().st_size}


def main():
    manifest = json.loads((ASSETS / 'manifest.json').read_text())
    generator = ROOT / manifest['generator']
    assert manifest['version'] == 'conquest/v1'
    assert manifest['generatorSha256'] == digest(generator)
    assert manifest['source']['kind'] == 'original-synthesis'
    assert manifest['source']['externalSamples'] == []
    assert manifest['license'] and manifest['source']['method']
    assert [Path(e['file']).stem for e in manifest['entries']] == list(NAMES)
    source = ASSETS / 'public/conquest/v1/audio'
    assert sorted(p.name for p in source.iterdir()) == sorted(n+'.wav' for n in NAMES)
    report = {'passed':False,'command':'python3 -B games/conquest/tests/audio_contract_test.py','files':{}}
    for entry in manifest['entries']:
        path = ROOT / entry['output']
        result = inspect(path)
        assert result['sha256'] == entry['sha256'] and result['bytes'] == entry['bytes'], entry
        assert entry['urlPath'] == '/conquest/v1/audio/' + entry['file']
        report['files'][entry['file']] = result
    temp = OUT / 'reproducibility'
    assert temp.resolve().is_relative_to(ROOT.resolve())
    try:
        for name in ['first', 'second']:
            destination = temp / name
            subprocess.run([sys.executable,'-B',str(generator),'--output-dir',str(destination)],cwd=ROOT,check=True,capture_output=True)
        for name in NAMES:
            relative = Path('public/conquest/v1/audio') / (name + '.wav')
            assert digest(temp/'first'/relative) == digest(temp/'second'/relative) == digest(ASSETS/relative), name
        assert (temp/'first/manifest.json').read_bytes() == (temp/'second/manifest.json').read_bytes() == (ASSETS/'manifest.json').read_bytes()
        report['repeatGenerationIdentical'] = True
    finally:
        if temp.exists():shutil.rmtree(temp)
    report['passed'] = True
    OUT.mkdir(parents=True,exist_ok=True)
    (OUT/'verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(report,ensure_ascii=False))


if __name__ == '__main__':
    main()
