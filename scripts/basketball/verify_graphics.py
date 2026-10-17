"""Compare independently decoded visible sprite pixels with original rendering.

No threshold relaxation: every claimed visible pixel must match. This check
does not establish gameplay or full scene reconstruction.
"""
from pathlib import Path
import argparse,csv,json
from PIL import Image
from graphics import render_sprites,render_background,render_scene

p=argparse.ArgumentParser();p.add_argument('capture',type=Path);p.add_argument('--output',type=Path,required=True)
args=p.parse_args();args.output.mkdir(parents=True,exist_ok=True)
frames={int(r['frame']):int(r['emulator_frame']) for r in csv.DictReader((args.capture/'frames.csv').open())}
colors=(args.capture/'nes-colors.rgb').read_bytes()
report=[]
for reference in sorted(args.capture.glob('frame-*.png')):
    frame=int(reference.stem.split('-')[1]);emulator_frame=frames[frame]
    prefix=args.capture/('ppu-%05d'%emulator_frame)
    ppu=Path(str(prefix)+'-ppu.bin').read_bytes();oam=Path(str(prefix)+'-oam.bin').read_bytes();ctrl=Path(str(prefix)+'-ctrl.bin').read_bytes()[0]
    sprite=render_sprites(ppu,oam,ctrl,colors)
    sprite.save(args.output/(reference.stem+'-sprites.png'))
    render_background(ppu,colors,(ctrl>>4)&1).save(args.output/(reference.stem+'-background.png'))
    original=Image.open(reference).convert('RGB');matched=0;mismatch=[]
    for y in range(192):
        for x in range(256):
            rgba=sprite.getpixel((x,y))
            if rgba[3]:
                if rgba[:3]==original.getpixel((x,y)):matched+=1
                else:mismatch.append([x,y])
    scene_mismatches=None
    field_path=Path(str(prefix)+'-field.bin')
    if field_path.exists():
        scroll=Path(str(prefix)+'-scroll.bin').read_bytes()
        scene=render_scene(field_path.read_bytes(),ppu,oam,ctrl,colors,scroll)
        scene.save(args.output/(reference.stem+'-scene.png'))
        scene_mismatches=sum(scene.getpixel((x,y))!=original.getpixel((x,y)) for y in range(240) for x in range(256))
    report.append({'frame':frame,'emulator_frame':emulator_frame,'matching_pixels':matched,'mismatching_pixels':len(mismatch),'first_mismatches':mismatch[:10],'scene_mismatches':scene_mismatches})
(args.output/'comparison.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
assert report and all(r['matching_pixels']>0 and r['mismatching_pixels']==0 for r in report),'Original-frame sprite comparison failed'
scenes=[r for r in report if r['scene_mismatches'] is not None]
assert scenes and all(r['scene_mismatches']==0 for r in scenes),'Original-frame complete-scene comparison failed'
