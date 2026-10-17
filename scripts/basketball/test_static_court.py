"""Before-implementation checks: static ROM background is not a screenshot;
ring-buffer columns must resolve into world X; camera pages skip the 16-byte
Y gap; no dynamic HUD name/clock/score/marker may leak into the base template.
Original screenshots independently verify court pixels and rendered HUD glyphs.
"""
from pathlib import Path
import csv,json,unittest
from PIL import Image
from graphics import render_sprites
from static_sprites import SpriteROM
from static_court import build_court,build_hud_template,build_hud_glyphs,render_hud_cells,camera_source_y,hud_dynamic_cell,build_court_tiles,render_court_ring,initial_court_ring,stream_court_column
ROOT=Path(__file__).resolve().parents[2]
class StaticCourtTests(unittest.TestCase):
 def setUp(self):
  self.rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());self.colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
 def test_camera_pages_and_template_exclusions(self):
  self.assertEqual(camera_source_y(0xef),239)
  self.assertEqual(camera_source_y(0x100),240)
  self.assertEqual(camera_source_y(0x129),281)
  with self.assertRaises(ValueError):camera_source_y(0xf0)
  with self.assertRaises(ValueError):camera_source_y(0x1f0)
  base=build_hud_template(self.rom,self.colors)
  self.assertEqual(base.size,(256,49))
  for y in range(49):
   for x in range(256):self.assertEqual(base.getpixel((x,y))[3]==0,hud_dynamic_cell(x//8,y//8),(x,y))
 def test_initial_ring_and_column_stream(self):
  ring=initial_court_ring(self.rom)
  directory=ROOT/'.local/nes-basket/runs/graphics-right'
  ppu=sorted(directory.glob('ppu-*-field.bin'))[0].read_bytes()
  self.assertEqual(ring['tiles'][1],list(ppu[0x2800:0x2bc0]))
  self.assertEqual(ring['attributes'][1],list(ppu[0x2bc0:0x2c00]))
  stream_court_column(self.rom,ring,48,phase=1)
  self.assertEqual(ring['columns'][16],48)
  self.assertEqual(ring['columns'][15],47)
 def test_observed_background_and_hud_pixels(self):
  court=build_court(self.rom,self.colors);self.assertEqual(court.size,(512,432))
  glyphs=build_hud_glyphs(self.rom,self.colors);base=build_hud_template(self.rom,self.colors)
  observations=json.loads((ROOT/'.local/basketball/graphics/verification.json').read_text())['frames_detail'];count=0;continuous_differences=0;background_pixels=0;hud_pixels=0;reports=[];court_tiles=build_court_tiles(self.rom,self.colors)
  for item in observations:
   directory=ROOT/'.local/nes-basket/runs'/('graphics-'+item['scenario']);stem='ppu-%05d'%item['emulator_frame']
   original=Image.open(directory/('frame-%05d.png'%item['frame'])).convert('RGB');ppu=(directory/(stem+'-ppu.bin')).read_bytes();oam=(directory/(stem+'-oam.bin')).read_bytes();ctrl=(directory/(stem+'-ctrl.bin')).read_bytes()[0]
   sprites=render_sprites(ppu,oam,ctrl,self.colors);sx,sy=(directory/(stem+'-scroll.bin')).read_bytes()
   ram=(directory/'ram.bin').read_bytes()[(item['frame']-1)*2048:item['frame']*2048];self.assertEqual(ram[0x399],sy)
   encoded_y=sy+256*ram[0x39a];source_y=camera_source_y(encoded_y);field=(directory/(stem+'-field.bin')).read_bytes();ring={'tiles':[list(field[0x2000:0x23c0]),list(field[0x2800:0x2bc0])],'attributes':[list(field[0x23c0:0x2400]),list(field[0x2bc0:0x2c00])]};field_image=render_court_ring(court_tiles,ring,sx,encoded_y);tiles=list(ppu[0x2000:0x20e0]);attrs=list(ppu[0x23c0:0x2400]);hud=render_hud_cells(glyphs,tiles,attrs)
   mismatch=[];base_mismatch=[]
   for y in range(240):
    for x in range(256):
     if sprites.getpixel((x,y))[3]:continue
     if y<194:
      ideal=(0,0,0) if x<8 and y<192 else court.getpixel((sx+x,source_y+y-48))[:3]
      want=field_image.getpixel((x,y))[:3];background_pixels+=1
      continuous_differences+=ideal!=original.getpixel((x,y))
     else:
      hy=y-191;want=hud.getpixel((x,hy))[:3];hud_pixels+=1
      if base.getpixel((x,hy))[3] and base.getpixel((x,hy))[:3]!=want:base_mismatch.append((x,hy))
     if original.getpixel((x,y))!=want:mismatch.append((x,y))
   self.assertEqual(len(mismatch),0,(item['scenario'],item['frame'],len(mismatch),mismatch[:8]))
   self.assertEqual(len(base_mismatch),0,(item['scenario'],item['frame'],len(base_mismatch),base_mismatch[:8]));count+=1
   reports.append({'scenario':item['scenario'],'frame':item['frame'],'background_mismatches':len(mismatch),'static_hud_mismatches':len(base_mismatch)})
  self.assertEqual(count,83)
  print(json.dumps({'status':'passed','frames':count,'background_pixels':background_pixels,'hud_pixels':hud_pixels,'mismatches':0,'continuous_crop_edge_differences':continuous_differences,'scope':'static court/tiles and independent HUD glyphs against original frames; captured tile/attribute state is verification input, not runtime image data','frames_detail':reports}))
if __name__=='__main__':unittest.main()
