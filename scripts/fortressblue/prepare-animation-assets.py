#!/usr/bin/env python3
"""Losslessly pack every decoded original tank frame, preserving native anchors.

No source, v2 image, map or audio is rewritten. All outputs are under this repo.
Action sequences and timing evidence live in assets/animation-sequences.json.
"""
from __future__ import annotations
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / 'games/fortressblue/assets'
DATA = ROOT / 'games/fortressblue/data'
OUT = ASSETS / 'public/fortressblue/v3/tanks'
spec = importlib.util.spec_from_file_location('original_assets', ROOT / 'scripts/fortressblue/prepare-original-assets.py')
codec = importlib.util.module_from_spec(spec)
spec.loader.exec_module(codec)
from PIL import Image

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def pack_tank(tid):
    source = codec.LOCAL / 'downloaded' / f'char{tid:02d}.i16'
    raw = codec.frames(source)
    assert len(raw) == 296, (tid, len(raw))
    sheets = []
    frames = []
    sheet = Image.new('RGBA', (2048, 2048))
    x = y = row_height = used_width = 0
    for index, frame in enumerate(raw):
        for facing in ('right', 'left'):
            im, ox, oy = (frame['image'], frame['ox'], frame['oy']) if facing == 'right' else (frame['other'] or (frame['image'], frame['ox'], frame['oy']))
            w, h = im.size
            if x+w > 2048:
                x = 0
                y += row_height+1
                row_height = 0
            if y+h > 2048:
                sheets.append(sheet.crop((0,0,used_width,y+row_height)))
                sheet = Image.new('RGBA',(2048,2048))
                x = y = row_height = used_width = 0
            sheet.paste(im,(x,y))
            frames.append(dict(atlas=len(sheets),x=x,y=y,w=w,h=h,ax=-ox,ay=-oy,sourceIndex=index,facing=facing,sharedFacing=(facing == 'left' and frame['other'] is None)))
            x += w+1
            row_height = max(row_height,h)
            used_width = max(used_width,x-1)
    sheets.append(sheet.crop((0,0,used_width,y+row_height)))
    urls = []
    for i, im in enumerate(sheets):
        dest = OUT / f'{tid}-atlas-{i}.webp'
        im.save(dest,'WEBP',lossless=True,quality=100,method=6,exact=True)
        urls.append(f'/fortressblue/v3/tanks/{dest.name}')
    return dict(source=str(source.relative_to(ROOT)),sourceSha256=digest(source),atlases=urls,frames=frames)

def main():
    sequences = json.loads((ASSETS/'animation-sequences.json').read_text())
    OUT.mkdir(parents=True,exist_ok=True)
    source = codec.LOCAL/'extracted/package/Asset/character.txt'
    lines = source.read_text().splitlines()
    columns = lines[0].split('\t')
    tanks = json.loads((DATA/'tanks.json').read_text())
    animation = dict(version=3,pixelScale=1,timingEvidence=sequences['timingEvidence'],actionMappingEvidence=sequences['actionMappingEvidence'],nativeAnimationMappingConfirmed=sequences['nativeAnimationMappingConfirmed'],nativeEvidence=sequences['nativeEvidence'],updateHz=sequences['updateHz'],updatesPerAnimationFrame=sequences['updatesPerAnimationFrame'],characterSource=str(source.relative_to(ROOT)),characterSha256=digest(source),tanks={})
    for tid in range(1,15):
        tank = pack_tank(tid)
        tank['actions'] = {}
        mapping = sequences.get('overrides',{}).get(str(tid),sequences['actions'])
        for name, action in mapping.items():
            tank['actions'][name] = {k:v for k,v in action.items() if k != 'sourceIndices'}
            tank['actions'][name]['left'] = [i*2+1 for i in action['sourceIndices']]
            tank['actions'][name]['right'] = [i*2 for i in action['sourceIndices']]
        animation['tanks'][str(tid)] = tank
        values = list(map(int,lines[tid].split('\t')))
        metadata = tanks['tanks'][str(tid)]
        for field,col in [('hp',3),('speed',4),('defense',6),('range',7),('primary',8),('special',10),('angle',1),('minAngle',2)]:
            metadata[field] = values[col]
        metadata['original'] = dict(columns=columns,values=values,roundMeaning='unverified',source=animation['characterSource'],sourceSha256=animation['characterSha256'])
    (DATA/'tanks.json').write_text(json.dumps(tanks,ensure_ascii=False,indent=1)+'\n')
    (DATA/'animations.json').write_text(json.dumps(animation,ensure_ascii=False,separators=(',',':'))+'\n')
    entries = []
    for path in sorted(OUT.glob('*.webp')):
        entries.append(dict(file=str(path.relative_to(ASSETS/'public/fortressblue/v3')),urlPath='/'+str(path.relative_to(ASSETS/'public')),output=str(path.relative_to(ROOT)),sha256=digest(path),bytes=path.stat().st_size,mime='image/webp'))
    manifest = dict(version=3,license='原版素材：权利归原权利方；按本会话授权提取与转换，非原创素材。',conversion='Existing I16 LH5/RGB565 span decoder; both native facing blocks; no crop, resize, mirror, recolor or timing inference; lossless WebP atlas.',pixelScale=1,entries=entries)
    (ASSETS/'manifest-v3.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    print(f'Packed 14 tanks, {sum(len(t["frames"]) for t in animation["tanks"].values())} native facing frames, {len(entries)} atlases')

if __name__ == '__main__':
    main()
