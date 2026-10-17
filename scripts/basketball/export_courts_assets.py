"""Export all eight original courts from their own immutable ROM tables."""
from pathlib import Path
from hashlib import sha256
import json
from static_sprites import SpriteROM,DIGEST
from static_court import build_court,build_court_tiles,build_hud_template,build_hud_glyphs,initial_court_ring,stream_court_column
ROOT=Path(__file__).resolve().parents[2];out=ROOT/'.local/basketball/courts';rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
def digest(path):return sha256(path.read_bytes()).hexdigest()
log=ROOT/'.local/basketball/irq-raster/python-after.log';
if not log.exists():log=ROOT/'.local/basketball/courts-after.log'
capture=json.loads((out/'capture.json').read_text());assert sum(len(r['frames']) for r in capture['records'])==48
pixel_path=ROOT/'.local/basketball/renderer/browser/result.json';pixel_report=json.loads(pixel_path.read_text()) if pixel_path.exists() else {'status':'not_run'}
records=[]
for court in range(8):
 directory=out/str(court);directory.mkdir(exist_ok=True);assets={}
 def save(name,data):
  path=directory/name
  if hasattr(data,'save'):data.save(path);record={'size':list(data.size)}
  else:path.write_text(json.dumps(data,separators=(',',':'))+'\n');record={}
  assets[name]={**record,'sha256':digest(path)}
 palettes=[court+1]+([9,10] if court==4 else [11,12,13,14,15] if court==6 else [])
 for palette in palettes:
  for name,build in [('court',build_court),('court-tiles',build_court_tiles),('hud-glyphs',build_hud_glyphs),('hud-template',build_hud_template)]:save(f'{name}-{palette}.png',build(rom,colors,court=court,palette_id=palette))
 ring=initial_court_ring(rom,court=court);ring['tiles'][0][:192]=[0]*192;save('initial-ring.json',ring);packets={}
 for phase,label in [(0,'initialization'),(1,'gameplay')]:
  packets[label]=[]
  for column in range(64):
   state=initial_court_ring(rom,court=court);stream_court_column(rom,state,column,phase=phase,court=court);slot=column&31;indices=[r*8+slot//4 for r in range(8)];data=[[state['tiles'][layer][row*32+slot] for row in range(30)] for layer in range(2)];data[0][:6]=[0]*6
   packets[label].append({'column':column,'slot':slot,'tiles':data,'attribute_indices':indices,'attributes':[[state['attributes'][layer][i] for i in indices] for layer in range(2)]})
 save('stream-columns.json',packets)
 manifest={'court':court,'layout':1,'palettes':palettes,'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'chr_first_page':rom.prg[16384+0x3ec5+court],'palette_preset':court+1,'map_pointer':hex(int.from_bytes(rom.prg[16384+12+court*2:16384+14+court*2],'little')),'attributes_pointer':hex(int.from_bytes(rom.prg[16384+28+court*2:16384+30+court*2],'little'))},'assets':assets}
 path=directory/'manifest.json';path.write_text(json.dumps(manifest,indent=2)+'\n');records.append({'court':court,'manifest':f'{court}/manifest.json','sha256':digest(path)})
manifest={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'rgb_sha256':sha256(colors).hexdigest(),'tables':'bank1 $800C/$801C court-indexed rows and attributes; $BEC5 CHR; bank4 $962F[court+1] palette'},'courts':records,'verification':{'log_sha256':digest(log),'status':pixel_report['status'],'frames':48,'exact_frames':sum(r['mismatches']==0 for r in pixel_report.get('courtFramesDetail',[])),'residual_pixels':[{k:r[k] for k in ('court','frame','mismatches')} for r in pixel_report.get('courtFramesDetail',[]) if r['mismatches']],'scope':'isolated original court configurations, not natural selection or Go gameplay'},'limitations':['Dynamic background patches require original scene-animation commands and ordered video commit timing.','Exact original IRQ boundary pixels may use explicitly observed CPU-derived fetch timing; Go does not emulate original CPU cycle phase.','No OSS upload or full-game release acceptance.']};(out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps({'courts':8,'images':60,'accepted_for_release':False}))
