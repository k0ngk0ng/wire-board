#!/usr/bin/env python3
"""Independent media checks authored before the world-v2 generator."""
import argparse
from array import array
import hashlib
import json
from pathlib import Path
import subprocess
import wave
import xml.etree.ElementTree as ET
from PIL import Image

ROOT = Path(__file__).resolve().parent
PUBLIC = ROOT.parent / 'public/starfort/world-v2'
BIOMES = ('forest', 'canyon', 'river', 'snow', 'ruins', 'marsh')
PROPS = ('tree', 'pine', 'rock', 'bridge', 'highground', 'watchtower', 'mine', 'cart', 'gate', 'ruins', 'reeds')
ENEMIES = ('healer', 'banner', 'splitter', 'engineer')
EVENTS = ('fog', 'frost', 'surge', 'star')


def validate():
    expected = {f'tile-{name}': (256, 256) for name in BIOMES}
    expected.update({f'choice-{name}': (256, 160) for name in BIOMES})
    expected.update({f'prop-{name}': (128, 128) for name in PROPS})
    expected.update({f'enemy-{name}-{pose}': (128, 128) for name in ENEMIES for pose in range(3)})
    expected.update({f'event-{name}': (64, 64) for name in EVENTS})
    expected['effect-boss-aura'] = (128, 128)
    metadata = json.loads((PUBLIC / 'atlas.json').read_text())
    atlas = Image.open(PUBLIC / 'atlas.webp').convert('RGBA')
    assert atlas.size == (metadata['meta']['width'], metadata['meta']['height'])
    assert set(metadata['frames']) == set(expected)
    checks, digests, occupied = [], {}, []
    for name, size in expected.items():
        source = ET.parse(ROOT / 'src' / (name + '.svg')).getroot()
        assert source.attrib['viewBox'] == f'0 0 {size[0]} {size[1]}'
        for element in source.iter():
            assert element.tag.split('}')[-1] not in ('image', 'script', 'foreignObject', 'text')
            assert not any(k.endswith('href') for k in element.attrib)
        image = Image.open(PUBLIC / (name + '.webp')).convert('RGBA')
        image.load()
        assert image.size == size, (name, image.size)
        assert len(image.getcolors(size[0] * size[1]) or []) > 24, name
        alpha = image.getchannel('A')
        bounds = alpha.getbbox()
        assert bounds, (name, 'empty')
        if name.startswith('tile-'):
            assert alpha.getextrema() == (255, 255), (name, 'tile not opaque')
            # Texture motifs are inset; matching outer pixels give a seam-free repeat.
            assert image.crop((0, 0, 1, 256)).tobytes() == image.crop((255, 0, 256, 256)).tobytes(), name
            assert image.crop((0, 0, 256, 1)).tobytes() == image.crop((0, 255, 256, 256)).tobytes(), name
        else:
            assert alpha.getextrema() == (0, 255), (name, 'alpha missing')
            assert 0 < bounds[0] < bounds[2] < size[0] and 0 < bounds[1] < bounds[3] < size[1], (name, 'clipped', bounds)
        frame = metadata['frames'][name]
        x, y, w, h = (frame[k] for k in ('x', 'y', 'w', 'h'))
        assert (w, h) == size and x >= 0 and y >= 0 and x + w <= atlas.width and y + h <= atlas.height
        assert image.tobytes() == atlas.crop((x, y, x + w, y + h)).tobytes(), (name, 'crop mismatch')
        assert not any(x < xx + ww and x + w > xx and y < yy + hh and y + h > yy for xx, yy, ww, hh in occupied)
        occupied.append((x, y, w, h))
        digests[name] = hashlib.sha256(image.tobytes()).hexdigest()
        checks.append({'asset': name, 'size': size, 'alphaBounds': bounds})
    for kind in ENEMIES:
        assert len({digests[f'enemy-{kind}-{pose}'] for pose in range(3)}) == 3, kind
    assert len({digests['tile-' + name] for name in BIOMES}) == len(BIOMES)
    for name in ('expand', 'event', 'objective'):
        with wave.open(str(PUBLIC / 'audio' / (name + '.wav'))) as wav:
            assert (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) == (1, 2, 22050)
            samples = array('h', wav.readframes(wav.getnframes()))
            peak, duration = max(map(abs, samples)), len(samples) / wav.getframerate()
            assert 100 < peak < 32000 and .1 <= duration <= 2.5, (name, peak, duration)
            assert abs(samples[0]) < 100 and abs(samples[-1]) < 100, (name, 'edge click')
            checks.append({'asset': 'audio/' + name, 'seconds': duration, 'peak': peak})
    manifest = json.loads((ROOT / 'manifest.json').read_text())
    actual = {str(p.relative_to(PUBLIC)) for p in PUBLIC.rglob('*') if p.is_file()}
    assert actual == {item['path'] for item in manifest['files']}
    for item in manifest['files']:
        data = (PUBLIC / item['path']).read_bytes()
        assert len(data) == item['bytes'] and hashlib.sha256(data).hexdigest() == item['sha256'], item['path']
    return {'status': 'passed', 'checks': checks, 'files': len(actual), 'manifestSHA256': hashlib.sha256((ROOT / 'manifest.json').read_bytes()).hexdigest()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--report', type=Path)
    parser.add_argument('--reproduce', action='store_true')
    args = parser.parse_args()
    result = validate()
    if args.reproduce:
        before = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for base in (PUBLIC, ROOT / 'src') for p in base.rglob('*') if p.is_file()}
        import sys
        subprocess.run([sys.executable, '-B', str(ROOT / 'generate.py')], check=True)
        after = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for base in (PUBLIC, ROOT / 'src') for p in base.rglob('*') if p.is_file()}
        assert before == after, 'generation not reproducible'
        assert validate()['manifestSHA256'] == result['manifestSHA256']
        result['reproducible'] = True
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'status': result['status'], 'checks': len(result['checks']), 'files': result['files'], 'reproducible': result.get('reproducible', False)}))

if __name__ == '__main__':
    main()
