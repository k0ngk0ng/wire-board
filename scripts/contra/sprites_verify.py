"""Compare recovered sprite decoding with recorded original builder and pixels.

Created before sprites_decode.py. Coverage is explicit: natural stage-one
standing/running/jumping/aiming/crouching/water and ordinary bullet observations.
"""
from pathlib import Path
import hashlib
import json
import sys
import zipfile
import base64
import gzip

from PIL import Image
from sprites_decode import SpriteBank, render_oam

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-sprites'


def main():
    if len(sys.argv)>1 and sys.argv[1]=='--evidence':
        bundle=json.loads(gzip.decompress(Path(sys.argv[2]).read_bytes()))
        run=BASE/'recheck'
        run.mkdir(parents=True,exist_ok=True)
        for name,item in bundle['files'].items():
            assert Path(name).name==name and name not in ('.','..')
            data=base64.b64decode(item['base64'],validate=True)
            assert hashlib.sha256(data).hexdigest()==item['sha256'],name
            (run/name).write_bytes(data)
    else:
        run=Path(sys.argv[1]) if len(sys.argv)>1 else BASE/'runtime/runs/natural-sprites'
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:
        rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    bank=SpriteBank(rom[16:16+131072])
    rows=[json.loads(s) for s in (run/'sprite-calls.jsonl').read_text().splitlines()]
    raw=(run/'sprite-calls.bin').read_bytes()
    assert len(raw)==len(rows)*512
    codes=set();flags=set();strides=set();counts={}
    for i,row in enumerate(rows):
        actual,offset,remaining=bank.compose(row,raw[i*512:i*512+256])
        expected=raw[i*512+256:(i+1)*512]
        assert actual==expected,(i,row,'OAM mismatch',[(a,b,c)for a,(b,c)in enumerate(zip(actual,expected))if b!=c][:10])
        assert (offset,remaining)==(row['afterOffset'],row['afterRemaining']),(i,row,'cursor mismatch')
        codes.add(row['code']);flags.add(row['flags']);strides.add(row['stride'])
        kind=bank.sprite(row['code'])['kind'];counts[kind]=counts.get(kind,0)+1
    displays=[json.loads(s)for s in (run/'display.jsonl').read_text().splitlines()]
    dma=(run/'dma.bin').read_bytes();rgb=(run/'nes-colors.rgb').read_bytes()
    pixels=[]
    for p in sorted(run.glob('display-*-ppu.bin')):
        frame=int(p.name.split('-')[1]);state=displays[frame-1]
        oam=dma[(frame-1)*256:frame*256]
        ppu=p.read_bytes()
        image,front=render_oam(oam,ppu,rgb,state['ctrl'],state['mask'])
        expected=Image.open(run/f'frame-{frame:05d}.png').convert('RGB')
        differences=[]
        for y in range(240):
            for x in range(256):
                if front[y*256+x] and image.getpixel((x,y))[:3]!=expected.getpixel((x,y)):
                    differences.append((x,y))
        image.save(run/f'recovered-sprites-{frame:05d}.png')
        assert not differences,(frame,'sprite pixel mismatch',len(differences),differences[:10])
        pixels.append({'frame':frame,'opaque_foreground_pixels':sum(front),'mismatches':0,
                       'dma_frame':state['dmaFrame'],'emulator_frame':state['emulatorFrame']})
    assert len(rows)>=500 and len(codes)>=10 and len(pixels)>=10
    assert counts['compact']>0 and counts['regular']>0
    report={'passed':True,'builder_calls':len(rows),'codes':sorted(codes),'flags':sorted(flags),
            'strides':sorted(strides),'counts':counts,'pixels':pixels,
            'scope':'recorded natural first-stage execution only',
            'not_proven':['unobserved sprites/animation phases','other weapons/levels','whole-frame background rendering']}
    (run/'verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report))


if __name__=='__main__':main()
