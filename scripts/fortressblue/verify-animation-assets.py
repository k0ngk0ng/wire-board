#!/usr/bin/env python3
"""Targeted, source-bound validation of the v3 animation pack; written before builder."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('original_assets', ROOT / 'scripts/fortressblue/prepare-original-assets.py')
codec = importlib.util.module_from_spec(spec)
spec.loader.exec_module(codec)
from PIL import Image, ImageDraw

def main():
    ci_spec = importlib.util.spec_from_file_location('v3_verify', ROOT / 'scripts/fortressblue/verify-v3-assets.py')
    ci = importlib.util.module_from_spec(ci_spec); ci_spec.loader.exec_module(ci); ci.verify()
    catalog = json.loads((ROOT / 'games/fortressblue/data/animations.json').read_text())
    assert catalog['version'] == 3
    assert catalog['pixelScale'] == 1
    assert catalog['timingEvidence'] == 'reconstructed-30hz'
    assert catalog['actionMappingEvidence'] == 'native-wasm-static'
    assert catalog['nativeAnimationMappingConfirmed'] is True
    assert set(catalog['tanks']) == {str(i) for i in range(1, 15)}
    entries = json.loads((ROOT / 'games/fortressblue/assets/manifest-v3.json').read_text())['entries']
    for entry in entries:
        p = ROOT / entry['output']
        assert p.stat().st_size == entry['bytes']
        assert hashlib.sha256(p.read_bytes()).hexdigest() == entry['sha256']
    checked = 0
    samples = Image.new("RGBA", (1120, 14*100), "#405366")
    labels = ImageDraw.Draw(samples)
    for tid, tank in catalog['tanks'].items():
        source = ROOT / tank['source']
        assert hashlib.sha256(source.read_bytes()).hexdigest() == tank['sourceSha256']
        raw = codec.frames(source)
        assert len(raw) == 296
        sheets = [Image.open(ROOT / ('games/fortressblue/assets/public' + url)).convert('RGBA') for url in tank['atlases']]
        assert all(im.width <= 2048 and im.height <= 2048 for im in sheets)
        assert len(tank['frames']) == len(raw) * 2
        for idx, frame in enumerate(tank['frames']):
            src = raw[frame['sourceIndex']]
            im, ox, oy = (src['image'], src['ox'], src['oy']) if frame['facing'] == 'right' else (src['other'] or (src['image'], src['ox'], src['oy']))
            assert (frame['w'], frame['h'], frame['ax'], frame['ay']) == (im.width, im.height, -ox, -oy), (tid, idx, 'anchor')
            sheet = sheets[frame['atlas']]
            x, y, w, h = (frame[k] for k in ('x', 'y', 'w', 'h'))
            assert x >= 0 and y >= 0 and x+w <= sheet.width and y+h <= sheet.height
            actual = sheet.crop((x, y, x+w, y+h))
            assert actual.getchannel('A').tobytes() == im.getchannel('A').tobytes(), (tid, idx, 'alpha')
            expected, got = im.tobytes(), actual.tobytes()
            assert all(expected[i+3] == 0 or expected[i:i+4] == got[i:i+4] for i in range(0,len(expected),4)), (tid, idx, 'pixels')
            checked += 1
        for col, name in enumerate(('idle', 'move', 'fire', 'special', 'damage', 'fall', 'death')):
            indices = tank['actions'][name]['right']
            selected = indices[len(indices)//2]
            frame = tank['frames'][selected]
            im = sheets[frame['atlas']].crop((frame['x'], frame['y'], frame['x']+frame['w'], frame['y']+frame['h']))
            origin_x, origin_y = col*160+60, (int(tid)-1)*100+80
            samples.alpha_composite(im, (origin_x-frame['ax'], origin_y-frame['ay']))
            labels.text((col*160+3, (int(tid)-1)*100+3), f"{tid} {name} [{frame['sourceIndex']}]", fill='white')
            labels.line((origin_x-3,origin_y,origin_x+3,origin_y),fill='#ff7184')
            labels.line((origin_x,origin_y-3,origin_x,origin_y+3),fill='#ff7184')
        assert set(tank['actions']) >= {'idle', 'move', 'fire', 'special', 'damage', 'fall', 'death'}
        for name, action in tank['actions'].items():
            assert action['evidence'] and action['durationMs'] > 0
            for facing in ('left', 'right'):
                assert action[facing]
                for index in action[facing]:
                    assert tank['frames'][index]['facing'] == facing
                    # Native groups are checked by the standard, source-free verifier.
    rows = (ROOT / catalog['characterSource']).read_text().splitlines()
    columns = rows[0].split('\t')
    tanks = json.loads((ROOT / 'games/fortressblue/data/tanks.json').read_text())['tanks']
    for tid in range(1, 15):
        row = list(map(int, rows[tid].split('\t')))
        original = tanks[str(tid)]['original']
        assert original['values'] == row
        assert original['columns'] == columns
        assert tanks[str(tid)]['minAngle'] == row[2]
        for field, col in [('hp',3),('speed',4),('defense',6),('range',7),('primary',8),('special',10),('angle',1)]:
            assert tanks[str(tid)][field] == row[col], (tid, field)
        assert original['roundMeaning'] == 'unverified'
    out = ROOT / '.local/fortressblue/assets-v3-evidence'
    out.mkdir(parents=True, exist_ok=True)
    samples.save(out / 'action-samples.png')
    # Reproducible diagnostic enlargement only; the shipped atlas remains native-sized.
    slope_tank = catalog['tanks']['1']
    slope_atlas = Image.open(ROOT / ('games/fortressblue/assets/public' + slope_tank['atlases'][0])).convert('RGBA')
    slope_sample = Image.new('RGBA', (800,300), '#405366')
    slope_labels = ImageDraw.Draw(slope_sample)
    for row, facing in enumerate(('right','left')):
        horizontal = slope_tank['actions']['idle'][facing][0]
        for col, slope in enumerate(range(-2,3)):
            f = slope_tank['frames'][horizontal+2*slope]
            frame_image = slope_atlas.crop((f['x'],f['y'],f['x']+f['w'],f['y']+f['h']))
            origin_x, origin_y = col*160+80, row*150+120
            slope_sample.alpha_composite(frame_image.resize((f['w']*3,f['h']*3), Image.Resampling.NEAREST), (origin_x-f['ax']*3,origin_y-f['ay']*3))
            slope_labels.text((col*160+4,row*150+4), f"{facing} source{f['sourceIndex']} slope{slope}", fill='white')
    slope_sample.save(out / 'slopes-facing.png')
    result = {'passed': True, 'tanks': 14, 'sourceFrames': checked//2, 'facingFrames': checked, 'manifestEntries': len(entries), 'pixelScale': 1}
    (out / 'verification.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps(result))

if __name__ == '__main__':
    main()
