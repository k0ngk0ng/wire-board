"""Create development review sheets; never add captions to the product PNGs."""
import argparse
import json
from pathlib import Path
from PIL import Image, ImageDraw

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--assets',type=Path,default=ROOT/'.local/basketball/selection-assets')
parser.add_argument('--output',type=Path,default=ROOT/'.local/basketball/selection-assets-evidence')
args=parser.parse_args()
if not args.output.resolve().is_relative_to(ROOT):
    raise ValueError('review output must remain in the workspace')
args.output.mkdir(parents=True,exist_ok=True)
doc=json.loads((args.assets/'manifest.json').read_text())
for key,cell,size,name in [('fullBody',(152,200),(128,160),'players'),('portrait',(112,120),(96,96),'portraits')]:
    sheet=Image.new('RGB',(8*cell[0],4*cell[1]),'#e6e0d3');draw=ImageDraw.Draw(sheet)
    for team in doc['teams']:
        for character in team['characters']:
            source=(args.assets/character[key]['file']).resolve()
            if not source.is_relative_to(args.assets.resolve()):raise ValueError('invalid asset path')
            image=Image.open(source).convert('RGBA')
            if image.size!=size:raise ValueError('unexpected player dimensions')
            x=team['id']*cell[0];y=character['character']*cell[1]
            sheet.paste(image,(x+(cell[0]-size[0])//2,y+22),image)
            draw.text((x+8,y+5),f"{team['id']}/{character['character']}",fill='#252820')
    sheet.save(args.output/f'{name}-contact-sheet.png')
sheet=Image.new('RGB',(1024,1248),'#e6e0d3');draw=ImageDraw.Draw(sheet)
for court in doc['courts']:
    source=(args.assets/court['preview']['file']).resolve()
    if not source.is_relative_to(args.assets.resolve()):raise ValueError('invalid asset path')
    image=Image.open(source).convert('RGBA')
    if image.size!=(512,288):raise ValueError('unexpected court dimensions')
    x=(court['id']%2)*512;y=(court['id']//2)*312
    sheet.paste(image,(x,y+24));draw.text((x+8,y+5),f"court {court['id']}",fill='#252820')
sheet.save(args.output/'courts-contact-sheet.png')
print(json.dumps(dict(courts=8,characters=32,sheets=3)))
