"""Original phase6 assets from the result presets, not a gameplay screenshot."""
from pathlib import Path
from hashlib import sha256
import json
from PIL import Image
from static_sprites import SpriteROM,DIGEST,signed
from static_court import paint_tile
from result_art import build_result_images,palette_for_court
ROOT=Path(__file__).resolve().parents[2];out=ROOT/'.local/basketball/results';out.mkdir(parents=True,exist_ok=True)
rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
manifest={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'decompressor':'fixed DBF8 / DCC3 / DCD4 / DCE5 / DD05','presets':'fixed D8C4 descriptors; compressed streams in CHR pages116/120/124','large_digits':'bank4 BCF0 / BD82','result_actor_kind':17,'actor_layout':'bank2 8000 kind17 -> 863A'},'actorProfiles':[[{'tile':t,'attributes':a,'x':signed(x),'y':signed(y)-23} for t,a,x,y in rom.pieces(17,p)] for p in range(rom.pose_count(17))],'courts':[],'layout':{'camera':{'x':0,'y':296},'field_source':[0,40,256,194],'hud_source':[0,3,256,45],'hud_destination':[0,194],'digit_size':[16,24],'digit_destinations':[[64,199],[144,199]],'hud_last_scanline':'Original preset99 writes only six rows; canvas row239 reads retained upper court-ring row6 using result HUD tiles.'},'limitations':['Stable original pixel verification covers New York and configured Court4 results with retained palette9.','Other result courts, Court4 palette10 and all120 actor poses have static coverage; not full natural-game acceptance.','Fade-in/fade-out palette transitions are outside the selected stable result frames.']}
def export_palette(court,palette_id,variant=False):
 images=build_result_images(rom,court,colors,palette_id=palette_id);entry={}
 images['hud']=images['hud'].crop((0,0,256,48))
 pattern=rom.chr[96*1024:98*1024]+rom.chr[14*1024:16*1024];palette=palette_for_court(rom,court,palette_id=palette_id);tiles=Image.new('RGBA',(128,512))
 for group in range(4):
  for tile in range(256):paint_tile(tiles,tile%16*8,(group*16+tile//16)*8,pattern,tile,palette,colors,group)
 images['hudTiles']=tiles
 for role in ['background','hud','digits','hudTiles']:
  name=f'court-{court}-palette-{palette_id}-{role}.png' if variant else f'court-{court}-{role}.png'
  file=out/name;images[role].save(file);entry[role]={'file':file.name,'sha256':sha256(file.read_bytes()).hexdigest()}
 return entry
for court in range(8):
 entry={'court':court,'presets':[court+101,99],**export_palette(court,court+1)}
 if court==4:entry['paletteVariants']=[{'paletteID':palette,**export_palette(court,palette,variant=True)} for palette in [9,10]]
 manifest['courts'].append(entry)
(out/'manifest.json').write_text(json.dumps(manifest,separators=(',',':'))+'\n');print(json.dumps({'courts':8,'actor_poses':120,'assets':40}))
