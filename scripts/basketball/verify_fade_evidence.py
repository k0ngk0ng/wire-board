"""Audit retained original fade observations, renderer coverage and exported LUT.

Run after accept_renderer.py. This does not execute the original ROM or replace
the browser comparison; it binds that comparison to its retained source inputs.
"""
from collections import Counter
from hashlib import sha256
import json
from pathlib import Path
from PIL import Image
from palette_fade import build_palette_fade_lut

ROOT = Path(__file__).resolve().parents[2]
ART = ROOT / '.local/basketball'


def digest(path):
    return sha256(path.read_bytes()).hexdigest()


def read(path):
    return json.loads(path.read_text())


def main():
    capture = read(ART / 'fade/capture.json')
    rom_digest = digest(ROOT / '.local/nes-basket/basketball.nes')
    frames = []
    for record in capture['records']:
        observed = ROOT / '.local/nes-basket/runs' / record['run']
        reference = observed.parent / ('fade-' + record['label'] + '-reference')
        assert (observed / 'capture-complete.json').is_file()
        assert (reference / 'capture-complete.json').is_file()
        source = read(observed / 'source.json')
        assert source == record['source'] and source['rom_sha256'] == rom_digest
        assert source['scenario_sha256'] == digest(observed / 'scenario.json')
        assert (observed / 'ram.bin').read_bytes() == (reference / 'ram.bin').read_bytes()
        for frame in record['frames']:
            name = f"frame-{frame['frame']:05d}.png"
            assert (observed / name).read_bytes() == (reference / name).read_bytes()
            frames.append({'label': record['label'], 'frame': frame['frame'],
                           'sha256': digest(observed / name)})
    assert len(frames) == 112
    fixture = read(ART / 'renderer/browser/fade-cases.json')
    result = read(ART / 'renderer/browser/result.json')
    assert result['status'] == 'passed' and result['firstFailure'] is None
    expected = {(x['label'], x['frame']) for x in frames}
    selected = {(x['label'], x['frame']) for x in fixture['cases']}
    excluded = {(x['run'].removeprefix('fade-v2-').removesuffix('-observed'), x['frame'])
                for x in fixture['excluded']}
    assert len(selected) == 107 and len(excluded) == 5
    assert not selected & excluded and selected | excluded == expected
    assert all(x['paletteLevel'] == 0 for x in fixture['excluded'])
    reports = result['fadeFramesDetail']
    assert {(x['label'], x['frame']) for x in reports} == selected
    assert all(x['mismatches'] == 0 for x in reports)
    groups = Counter((x['label'], x['paletteID'], x['direction'], x['result']) for x in reports)
    levels = {(x['paletteID'], x['direction'], x['level']) for x in reports}
    for palette in (2, 9, 10):
        assert all((palette, 'out', level) in levels for level in range(5))
    for palette in (2, 5, 9):
        assert all((palette, 'in', level) in levels for level in range(1, 5))
    raster_path = ART / 'raster/manifest.json'
    raster = read(raster_path)
    colors = ROOT / '.local/nes-basket/runs/graphics-right/nes-colors.rgb'
    assert raster['accepted_for_release'] is False
    assert raster['source']['rom_sha256'] == rom_digest
    assert raster['paletteFade']['rgb_sha256'] == digest(colors)
    assert raster['paletteFade']['indexCount'] == 64
    assert raster['paletteFade']['entries'] == build_palette_fade_lut(colors.read_bytes())
    atlas = raster_path.parent / raster['atlas']['file']
    assert digest(atlas) == raster['atlas']['sha256']
    with Image.open(atlas) as image:
        assert image.mode == 'RGBA' and list(image.size) == raster['atlas']['size']
    counts = [result[key] for key in ('frames', 'courtFrames', 'irqHoldoutFrames',
                                     'resultFrames', 'resultPaletteFrames', 'fadeFrames')]
    assert counts == [83, 48, 34, 15, 8, 107]
    extra_frames = result.get('resultCourtSixFrames', 0)
    assert extra_frames in (0, 8)
    if extra_frames:
        assert all(x['mismatches'] == 0 for x in result['resultCourtSixFramesDetail'])
    inputs = [raster_path, ART / 'fade/capture.json', ART / 'fade/index-oracle.json',
              ART / 'renderer/browser/fade-cases.json', ART / 'renderer/browser/result.json',
              ART / 'renderer/browser/renderer.js', ROOT / 'games/basketball/web/renderer.ts']
    report = {'status': 'passed', 'scope': 'retained original render inputs and browser pixel comparison; not Go world equivalence',
              'sourceFrames': 112, 'excludedPreviousMenuFrames': 5, 'fadeFrames': 107,
              'fadePixels': 107 * 256 * 240, 'fadeRegressionFrames': sum(counts), 'additionalCourtSixResultFrames': extra_frames,
              'totalFrames': sum(counts) + extra_frames,
              'totalPixels': (sum(counts) + extra_frames) * 256 * 240, 'rgbClasses': 54,
              'accepted_for_release': False, 'frames': frames,
              'groups': [{'label': k[0], 'paletteID': k[1], 'direction': k[2],
                          'result': k[3], 'frames': v} for k, v in sorted(groups.items())],
              'inputs': {str(p.relative_to(ROOT)): digest(p) for p in inputs}}
    (ART / 'fade/evidence-verification.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({k: v for k, v in report.items() if k not in ('frames', 'groups', 'inputs')}))


if __name__ == '__main__':
    main()
