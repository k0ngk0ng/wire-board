#!/usr/bin/env python3
"""Prepare original map/ticket artwork outside source; labels remain live UI text.

Requires Pillow, OpenCV and NumPy. Source is a local reference artwork directory.
No credentials, artwork, or server configuration are written to the repository.
"""
import argparse
import json
from pathlib import Path
import cv2
import numpy as np
from PIL import Image

MAPS=('europe','india','switzerland','nordiccountries','legendaryasia')
def prepare(source, output):
    labels=json.loads(Path(__file__).with_name('rail_map_labels.json').read_text())
    for name in MAPS:
        folder=output/name
        (folder/'tickets').mkdir(parents=True,exist_ok=True)
        original=Image.open(source/name/'map.webp').convert('RGB')
        pixels=np.array(original)
        mask=np.zeros(pixels.shape[:2],np.uint8)
        for label,(x,y,w,h) in labels[name].items():
            # Country names are decorative geography, sometimes repeated/rotated.
            if name=='switzerland' and label in ('Italia','Österreich','Deutschland','France'):
                continue
            x0,y0=max(0,x-2),max(0,y-2)
            x1,y1=min(mask.shape[1],x+w+2),min(mask.shape[0],y+h+2)
            crop=pixels[y0:y1,x0:x1].astype(np.int16)
            red,green,blue=crop[:,:,0],crop[:,:,1],crop[:,:,2]
            if name=='india': ink=(green<115)&(red>green*1.35)&(blue>green*1.1)&(red<175)
            elif name=='legendaryasia': ink=(green<110)&(red>green*1.5)&(blue<130)&(red<190)
            elif name=='nordiccountries': ink=(red<130)&(green<100)&(blue<160)
            else: ink=(red<115)&(green<115)&(blue<115)
            glyph=np.zeros_like(mask);glyph[y0:y1,x0:x1]=ink.astype(np.uint8)*255
            glyph=cv2.dilate(glyph,cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(9,9)))
            mask=cv2.bitwise_or(mask,glyph)
        clean=Image.fromarray(cv2.inpaint(pixels,mask,5,cv2.INPAINT_TELEA))
        clean.save(folder/'map.webp',quality=88,method=6)
        preview=original.copy();preview.thumbnail((480,320));preview.save(folder/'preview.webp',quality=80,method=6)
        if name=='europe':
            stations=Image.open(source/name/'station-icons.png').convert('RGBA')
            sw=stations.width//5
            for color,index in [('blue',0),('red',3),('green',1),('yellow',4),('black',2)]:
                station=stations.crop((sw*index,0,sw*(index+1),stations.height))
                if color=='black':
                    rgba=np.array(station);grey=(np.array(station.convert('L'))*.55).astype(np.uint8)
                    rgba[:,:,:3]=grey[:,:,None];station=Image.fromarray(rgba)
                station.save(folder/f'station-{color}.webp',lossless=True,method=6)
        data=json.loads((Path('internal/game/rail_maps')/(name+'.json')).read_text())
        atlases={}
        for t in data['tickets']:
            deck=2 if t.get('long') else 1
            if deck not in atlases:atlases[deck]=Image.open(source/name/f'destinations-{deck}-0.jpg')
            w,h=(161,250) if name in ('india','nordiccountries') else (250,161)
            index=t['art']-1
            x,y=(index%10)*w,(index//10)*h
            assert x+w<=atlases[deck].width and y+h<=atlases[deck].height,(name,t)
            card=atlases[deck].crop((x,y,x+w,y+h))
            card.save(folder/'tickets'/f"{t['id']}.webp",quality=90,method=6)
        print(f"{name}: map, preview and {len(data['tickets'])} tickets",flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('source',type=Path);p.add_argument('output',type=Path)
    a=p.parse_args();prepare(a.source,a.output)
