"""Compare a normalized raster-region renderer with retained original pixels.

This is offline verification, not runtime frame playback. Both planes are
computed from normalized PPU bytes/OAM and the recovered scene commands.
"""
from pathlib import Path
import csv,gzip,json,hashlib
from PIL import Image
from campaign_scene_raster_analyze import pixel,sprite_hit
from sprites_decode import render_oam
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence';RUN=ROOT/'.local/contra-raster/oracle/runs/raster-v1'
def main():
 events=json.loads(gzip.decompress((E/'campaign-scene-raster-original.json.gz').read_bytes()))['events'];frames={int(r['frame']):int(r['emulator_frame'])for r in csv.DictReader((RUN/'frames.csv').open())};ram=(RUN/'ram.bin').read_bytes();rgb=(ROOT/'.local/contra-background/runs/background-v1/background-colors.rgb').read_bytes();checks=[]
 for n in [6500,6550,6600]:
  frame=frames[n];es=[r for r in events if r['frame']==frame];effect=next(r for r in es if r['kind']=='pc-9fe5');state=ram[(n-2)*2048:(n-1)*2048];oam=state[0x200:0x300];assert list(oam[:4])==effect['oam0'];ppu=(RUN/f'frame-{n:05d}-ppu.bin').read_bytes();hit=sprite_hit(ppu,oam[:4]);control_y=hit[1]+1
  sprites,front=render_oam(oam,ppu,rgb,effect['control'],effect['mask']);actual=Image.open(RUN/f'frame-{n:05d}.png').convert('RGB');expected=Image.new('RGB',(256,240));mismatch=[]
  for y in range(240):
   for x in range(256):
    color,opaque=pixel(ppu,x,y,nt0=1 if y>=control_y else 0);value=tuple(rgb[color*3:color*3+3]);sp=sprites.getpixel((x,y))
    if y<hit[1]and sp[3]and(front[y*256+x]or not opaque):value=sp[:3]
    expected.putpixel((x,y),value)
    if actual.getpixel((x,y))!=value:mismatch.append([x,y])
  dest=E/f'campaign-scene-raster-interlude-{n}.png';expected.save(dest)
  counts={};
  for x,y in mismatch:counts[y]=counts.get(y,0)+1
  checks.append({'frame':n,'emulatorFrame':frame,'hit':hit,'maskFromLine':hit[1],'controlFromLine':control_y,'pixelsCompared':61440,'mismatches':len(mismatch),'mismatchRows':counts,'exampleMismatches':mismatch[:12],'renderSHA256':hashlib.sha256(dest.read_bytes()).hexdigest()})
 print(json.dumps(checks))
 (E/'campaign-scene-raster-pixels.json').write_text(json.dumps({'scope':'Exact visual semantics of the pinned FCEUX 2.6.6 old PPU oracle; not a hardware subdot timing claim.','checks':checks,'passed':all(r['mismatches']==0 for r in checks)},indent=2)+'\n')
 if any(r['mismatches']for r in checks):raise SystemExit(1)
if __name__=='__main__':main()
