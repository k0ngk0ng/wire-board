"""Export original 8x8 fragments for precise Canvas sprite scanline limits.
The existing player/object sheets remain the presentation asset reference;
fragment layouts resolve their overlap and clipping without a runtime ROM.
"""
from pathlib import Path
from hashlib import sha256
import json
from PIL import Image
from graphics import decode_tile
from palette_fade import build_palette_fade_lut
from static_sprites import SpriteROM,DIGEST,signed
ROOT=Path(__file__).resolve().parents[2];out=ROOT/'.local/basketball/raster';out.mkdir(parents=True,exist_ok=True)
rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes();banks=list(range(9))+[13]+list(range(32,36));atlas=Image.new('RGBA',(512,len(banks)*16*8))
for bi,bank in enumerate(banks):
 for team in range(8):
  palette=rom.team_palette(team)
  for group in range(2):
   top=(bi*16+team*2+group)*8
   for tile in range(64):
    pixels=decode_tile(rom.chr[bank*1024+tile*16:bank*1024+(tile+1)*16])
    for y,row in enumerate(pixels):
     for x,index in enumerate(row):
      if index:
       color=palette[group*4+index]&63;atlas.putpixel((tile*8+x,top+y),tuple(colors[color*3:color*3+3])+(255,))
file=out/'sprite-tiles.png';atlas.save(file)
profiles={str(kind):[[{'tile':tile,'attributes':attr,'x':signed(x),'y':signed(y)-23} for tile,attr,x,y in rom.pieces(kind,pose)] for pose in range(rom.pose_count(kind))] for kind in range(15)}
head_tiles=[[[((tile+rom.read(rom.word(0xd12d+tile*2)+side*5+character))&255) for character in range(5)] for side in range(2)] for tile in range(4)]
manifest={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'layout':'bank2 $8000 / fixed $D079,$D0C6','head_mapping':'fixed $D12D','oam_rules':'fixed $CE09..$D078 / $F002','palette':'bank4 $97AB'},'atlas':{'file':'sprite-tiles.png','sha256':sha256(file.read_bytes()).hexdigest(),'size':list(atlas.size),'banks':banks,'tileSize':8,'rowFormula':'(banks.indexOf(bank)*16 + team*2 + paletteInTeam)*8','xFormula':'localTile*8'},'paletteFade':{'source':'bank4 $94E0/$9526, full BG and sprite groups; flags00/80','rgb_sha256':sha256(colors).hexdigest(),'indexCount':64,'entries':build_palette_fade_lut(colors)},'headTiles':head_tiles,'profiles':profiles,'origin':{'actorAnchorOffset':[0,24],'playerSheet':[64,96],'objectSheet':[64,96],'fragmentY':'original signed piece Y minus 23'},'limits':{'spriteCapacity':64,'perScanline':8,'highestStoredY':184,'leftMaskPixels':8},'shadow':{'tile':47,'attributes':0,'x':-5,'y':-7},'limitations':['Gameplay and initialized result scenes; sprite-behind-background priority remains an explicit renderer error.','Pixel verification is renderer-only; this file contains no game decisions or CPU instructions.']}
(out/'manifest.json').write_text(json.dumps(manifest,separators=(',',':'))+'\n');print(json.dumps({'sprite_tiles':len(banks)*16*64,'metasprite_kinds':len(profiles),'output':str(out)}))
