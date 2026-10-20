#!/usr/bin/env python3
"""Pixel oracle uses retained native OAM/PPU, never generator tile mapping.
See docs/research/battlecity-directions/PLAN.md for preimplementation failures.
Requires Pillow and capture-directions.py evidence; no source media in Git.
"""
import argparse, hashlib, importlib.util, json
from pathlib import Path
from PIL import Image, ImageDraw
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('generator',ROOT/'scripts/battlecity/prepare_assets.py');gen=importlib.util.module_from_spec(spec);spec.loader.exec_module(gen)
p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);p.add_argument('--atlas',type=Path);a=p.parse_args();a.out.mkdir(parents=True,exist_ok=True)
evidence=ROOT/'.local/battlecity/reference/directions-v2';cases=json.loads((evidence/'cases.json').read_text())
for name,want in cases['hashes'].items():assert hashlib.sha256((evidence/name).read_bytes()).hexdigest()==want,name
rom=gen.DEFAULT_ROM.read_bytes();assert hashlib.sha256(rom).hexdigest()==cases['romSha256']
frames=gen.build_frames(rom[0x4010:]);atlas=Image.open(a.atlas/'atlas.webp').convert('RGBA') if a.atlas else None
metadata=json.loads((a.atlas/'atlas.json').read_text()) if a.atlas else None
rows=[];sheet=Image.new('RGBA',(8*72,10*92),'#202020');draw=ImageDraw.Draw(sheet)
for i,c in enumerate(cases['cases'],1):
 raw=(evidence/f'case-{i:02d}-oam.bin').read_bytes();ppu=(evidence/f'case-{i:02d}-ppu.bin').read_bytes()
 slots=[list(raw[n:n+4]) for n in range(0,256,4) if raw[n]==112 and raw[n+2]&3==0 and raw[n+3] in (112,120)]
 assert len(slots)==2,(c,slots)
 assert sorted(s[1] for s in slots)==[c['base']+c['nativeDirection']*8+c['frame']*4,c['base']+c['nativeDirection']*8+c['frame']*4+2],(c,slots)
 for team,palette in [('top',[15,6,23,39]),('bottom',[15,12,18,33])]:
  expected=Image.new('RGBA',(16,16));
  for y,tile,attr,x in slots:
   start=(tile&1)*4096+(tile&254)*16
   for py in range(16):
    for px in range(8):
     sy=15-py if attr&128 else py;sx=7-px if attr&64 else px
     offset=start+(sy//8)*16+sy%8
     value=((ppu[offset]>>(7-sx))&1)|(((ppu[offset+8]>>(7-sx))&1)<<1)
     if value:expected.putpixel((x-112+px,py),gen.PALETTE[palette[value]]+(255,))
  name=f"tank-{c['class']}-{team}-{c['direction']}-{c['frame']}"
  actual=frames[name][0]
  if atlas:
   f=metadata['frames'][name];actual=atlas.crop((f['x'],f['y'],f['x']+f['w'],f['y']+f['h']))
  mismatch=sum(p!=q for p,q in zip(expected.getdata(),actual.getdata()))
  rows.append({'name':name,'mismatchedPixels':mismatch,'nativeOAM':slots,'expectedRGBA':hashlib.sha256(expected.tobytes()).hexdigest()})
  row=((i-1)//8)*2+(team=='bottom');col=(i-1)%8
  sheet.alpha_composite(actual.resize((64,64),Image.Resampling.NEAREST),(col*72,row*92));draw.text((col*72,row*92+65),f"{c['direction']} {c['frame']}",fill='white')
report={'passed':all(r['mismatchedPixels']==0 for r in rows),'romSha256':cases['romSha256'],'cases':len(rows),'failed':sum(r['mismatchedPixels']>0 for r in rows),'rows':rows}
(a.out/'report.json').write_text(json.dumps(report,indent=2)+'\n');sheet.save(a.out/'directions.png')
print(json.dumps({k:v for k,v in report.items() if k!='rows'}));raise SystemExit(0 if report['passed'] else 1)
