#!/usr/bin/env python3
"""Check reproducible racing sources, atlas contract, audio and CDN fallbacks."""
import argparse, array, concurrent.futures, hashlib, json, math, struct, subprocess, urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / 'games/racing/assets'
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--dist', action='store_true')
p.add_argument('--oss', action='store_true')
p.add_argument('--decode-audio', action='store_true')
args = p.parse_args()
sha = lambda b: hashlib.sha256(b).hexdigest()
m = json.loads((ASSETS / 'manifest.json').read_text())
assert m['game'] == 'racing' and m['version'] == 1
sources = json.loads((ASSETS / 'source-manifest.json').read_text())
for e in sources['entries']:
    assert sha((ROOT / e['path']).read_bytes()) == e['sha256'], e['path']
    assert e.get('license') and e.get('author') and e.get('sourceURL'), e['path']
public = ASSETS / 'public/racing/v1'
atlas = json.loads((public / 'atlas.json').read_text())
assert atlas['cars']['cell'] == [192, 320]
assert atlas['cars']['columns'] == 6 and atlas['cars']['rows'] == 5
assert atlas['cars']['heading'] == 'up' and len(atlas['cars']['sprites']) == 30
assert {(s['seat'], s['body']) for s in atlas['cars']['sprites']} == {(s, b) for s in range(6) for b in range(5)}
for group in ('cars', 'scenery'):
    w, h = atlas[group]['size']
    for s in atlas[group]['sprites']:
        x, y, sw, sh = s['rect']
        assert sw > 0 and sh > 0 and x >= 0 and y >= 0 and x + sw <= w and y + sh <= h, s
names = {s['name'] for s in atlas['scenery']['sprites']}
assert {'tree', 'rock', 'cone', 'barrier', 'lights', 'grandstand', 'skid', 'smoke', 'spark', 'grass', 'dirt', 'sand', 'asphalt'} <= names
attribution = json.loads((public / 'attribution.json').read_text())
tire = next(e for e in attribution['sources'] if e['id'] == 'tom-haigh-tire')
assert tire['license'] == 'CC-BY-3.0' and 'Tom Haigh' in tire['author'] and tire['licenseURL'] and tire['modifications']
expected_audio = {'engine', 'tires', 'collision-light', 'collision-heavy', 'countdown', 'start', 'lap', 'finish'}
assert {e['name'] for e in m['entries'] if e['type'] == 'audio'} == expected_audio
assert {e['output'] for e in m['entries']} == {p.relative_to(ROOT).as_posix() for p in public.iterdir() if p.is_file()}
for f in public.rglob('*'):
    assert f.suffix.lower() not in {'.html', '.htm', '.swf'}, f

def check(e):
    file = ROOT / e['output']; data = file.read_bytes()
    assert file.relative_to(ASSETS / 'public').as_posix() == e['urlPath'].lstrip('/')
    assert e['urlPath'].startswith('/racing/v1/') and sha(data) == e['sha256'] and len(data) == e['bytes'], e['name']
    for source in e['sources']:
        assert sha((ROOT / source['path']).read_bytes()) == source['sha256'], source['path']
    result = {'name': e['name'], 'path': e['urlPath'], 'sha256': sha(data), 'bytes': len(data), 'passed': True}
    if e['type'] == 'image':
        assert data[:4] == b'RIFF' and data[8:12] == b'WEBP'
    if args.dist:
        assert sha((ROOT / 'web/dist' / e['urlPath'].lstrip('/')).read_bytes()) == e['sha256'], 'missing built fallback: ' + e['name']
    if e['type'] == 'audio' and args.decode_audio:
        r = subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(file), '-f', 'f32le', '-ac', '1', '-ar', '44100', '-'], check=True, capture_output=True)
        samples = array.array('f'); samples.frombytes(r.stdout)
        peak = max(abs(v) for v in samples); rms = math.sqrt(sum(v*v for v in samples) / len(samples))
        assert .02 < peak < .97 and .002 < rms < .45, (e['name'], peak, rms)
        assert .1 < len(samples) / 44100 < 6, e['name']
        result.update(seconds=len(samples)/44100, peak=peak, rms=rms)
        if e['name'] in {'engine', 'tires'}:
            seam = abs(samples[0] - samples[-1])
            assert seam < .035, (e['name'], 'loop discontinuity', seam)
            result['loopSeam'] = seam
    if args.oss:
        url = m['assetsBaseURL'] + e['urlPath']
        with urllib.request.urlopen(url, timeout=40) as response:
            assert sha(response.read()) == e['sha256'], url
            assert response.headers.get_content_type() == e['mime'], url
        result['cdnVerified'] = True
    return result

with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
    results = list(pool.map(check, m['entries']))
out = ROOT / '.local/racing/verification'; out.mkdir(parents=True, exist_ok=True)
target = out / ('oss-readback.json' if args.oss else 'assets.json')
target.write_text(json.dumps({'passed': True, 'dist': args.dist, 'decodeAudio': args.decode_audio, 'sources': len(sources['entries']), 'cars': 30, 'scenery': len(names), 'objects': results}, indent=2) + '\n')
print('Verified', len(results), 'racing objects:', target)
