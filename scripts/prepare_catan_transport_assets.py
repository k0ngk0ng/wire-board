#!/usr/bin/env python3
"""Extract original 2025 transport pieces and full printed card faces.

No recoloring: original four player-color wagons, commodity tiles and cargo.
Five/six-player assets are not implied by this three/four-player acceptance set.
"""
import argparse
import hashlib
import json
from pathlib import Path
import pymupdf
from PIL import Image, ImageDraw, ImageChops
from prepare_catan_attack_assets import composite

SOURCE = 'catan-traders-2025'
PIECES = {
    'wagon-blue': (45188, (80, 92)), 'wagon-orange': (45187, (80, 92)),
    'wagon-white': (45182, (80, 92)), 'wagon-red': (45189, (80, 92)),
    'site-quarry': (45176, (237, 265)), 'site-glassworks': (45186, (237, 265)),
    'site-castle': (45193, (237, 265)),
    'cargo-marble': (32259, (82, 82)), 'cargo-sand': (32261, (82, 82)),
    'cargo-tools': (32263, (82, 82)), 'cargo-glass': (32265, (82, 82)),
    'card-back': (32233, (173, 246)),
}
CARDS = {
    'knight': (102.66803, 401.479065, 167.901428, 494.66095),
    'road': (167.609344, 401.479065, 232.842742, 494.66095),
    'swift': (232.55066, 401.479065, 297.78406, 494.66095),
    'vp': (351.1, 404.5, 409.0, 491.4),
}

def prepare(output, rules):
    root = Path(__file__).resolve().parent.parent
    source = json.loads((root/'docs/board-expansion-rule-sources.json').read_text())[SOURCE]
    data = (rules/f'{SOURCE}.pdf').read_bytes()
    if hashlib.sha256(data).hexdigest()!=source['sha256']:
        raise ValueError('Rulebook changed; review components before updating the source pin')
    target=output/'catan/transport'
    target.mkdir(parents=True,exist_ok=True)
    provenance={}
    with pymupdf.open(stream=data,filetype='pdf') as doc:
        page=doc[19]
        rows={r[0]:r for r in page.get_images(full=True)}
        def save(name,art,detail):
            path=target/f'{name}-v1.webp'
            art.save(path,lossless=True,method=6)
            provenance[name]={'source':SOURCE,'page':20,**detail,'size':list(art.size),'bytes':path.stat().st_size,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
        for name,(xref,size) in PIECES.items():
            row=rows[xref]
            if tuple(row[2:4])!=size: raise ValueError(f'Component dimensions changed: {name}')
            pix=pymupdf.Pixmap(doc,xref)
            if pix.colorspace.n>3: pix=pymupdf.Pixmap(pymupdf.csRGB,pix)
            art=Image.frombytes('RGBA' if pix.alpha else 'RGB',size,pix.samples)
            if row[1]:
                mask=pymupdf.Pixmap(doc,row[1]);art.putalpha(Image.frombytes('L',(mask.width,mask.height),mask.samples).resize(size))
            if name.startswith('wagon-') and (not row[1] or art.getchannel('A').getextrema()!=(0,255)):
                raise ValueError('Missing wagon alpha')
            save(name,art,{'method':'native-image','object':xref,'soft_mask':row[1] or None})
        for name,bounds in CARDS.items():
            expected={'knight':'KNIGHT','road':'ROAD BUILDING','swift':'SWIFT JOURNEY','vp':'TOOLMAKING'}[name]
            if expected not in page.get_text(clip=pymupdf.Rect(bounds)):
                raise ValueError(f'Wrong printed card region: {name}')
            art=composite(page,bounds,(196,280))
            mask=pymupdf.Pixmap(doc,32235)
            alpha=Image.frombytes('L',(mask.width,mask.height),mask.samples)
            region=alpha.point(lambda v:255 if v else 0)
            for x in range(region.width):
                for y in (0,region.height-1):
                    if region.getpixel((x,y))==0: ImageDraw.floodfill(region,(x,y),128)
            for y in range(region.height):
                for x in (0,region.width-1):
                    if region.getpixel((x,y))==0: ImageDraw.floodfill(region,(x,y),128)
            alpha=ImageChops.lighter(alpha,region.point(lambda v:255 if v==0 else 0))
            if name == 'vp':
                # The printed VP cards overlap. Crop exactly the foremost card;
                # its native frame mask also includes the other card's shadow.
                alpha=Image.new('L',art.size,0)
                ImageDraw.Draw(alpha).rounded_rectangle((0,0,art.width-1,art.height-1),radius=5,fill=255)
            art.putalpha(alpha.resize(art.size,Image.Resampling.BICUBIC))
            save(f'card-{name}',art,{'method':'page-composite','clip':bounds,'language':'Original English; Chinese rules in UI','note':'VP uses original Toolmaking face for the shared victory-point card type' if name=='vp' else ''})
    (target/'sources.json').write_text(json.dumps({'source':source,'artwork':provenance},ensure_ascii=False,indent=2)+'\n')
    print(f'Prepared {len(provenance)} original transport images')

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output',type=Path)
    parser.add_argument('--rules-directory',type=Path,required=True)
    args=parser.parse_args();prepare(args.output,args.rules_directory)
