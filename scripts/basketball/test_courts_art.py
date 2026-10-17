"""Before implementation: eight courts must use independent original maps,
attributes, CHR and palettes, stream every column identically to original
BC74/BCFF, and match configured original gameplay pixels. Court IDs must not
select projection layouts; invalid IDs fail rather than substitute New York.
"""
from pathlib import Path
import json,unittest
from PIL import Image
from static_sprites import SpriteROM
from raster_boundary import render_hud_boundary
from derive_irq_timing import derive
from graphics import render_sprites
from static_court import build_court,build_court_tiles,build_hud_glyphs,render_hud_cells,render_court_ring,initial_court_ring,stream_court_column
ROOT=Path(__file__).resolve().parents[2]
class CourtsArtTests(unittest.TestCase):
 def setUp(self):
  self.rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());self.colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
 def test_distinct_sources_and_invalid_ids(self):
  images=[build_court(self.rom,self.colors,court=c) for c in range(8)]
  self.assertEqual(len({im.tobytes() for im in images}),8)
  for c in [-1,8]:
   with self.assertRaises(ValueError):build_court(self.rom,self.colors,court=c)
 def test_original_configured_frames(self):
  capture=json.loads((ROOT/'.local/basketball/courts/capture.json').read_text());timings=derive()['cases'];reports=[];pixels=0
  for entry in capture['records']:
   court=entry['court'];directory=ROOT/f'.local/nes-basket/runs/court-{court}-observed';glyphs=build_court_tiles(self.rom,self.colors,court=court);hudglyphs=build_hud_glyphs(self.rom,self.colors,court=court)
   for record in entry['frames']:
    n=record['frame'];palette_id=(directory/'ram.bin').read_bytes()[(n-1)*2048+0x304];glyphs=build_court_tiles(self.rom,self.colors,court=court,palette_id=palette_id);hudglyphs=build_hud_glyphs(self.rom,self.colors,court=court,palette_id=palette_id);stem=f"ppu-{record['emulator_frame']:05d}";state=json.loads((directory/(stem+'-semantic.json')).read_text());ring=initial_court_ring(self.rom,court=court)
    for event in state['courtEvents']:
     if 'patch' in event:
      p=event['patch'];start=p['row']*32+p['column'];ring['tiles'][p['plane']][start:start+len(p['tiles'])]=p['tiles']
     else:stream_court_column(self.rom,ring,event['column'],phase=event['phase'],court=court)
    field=(directory/(stem+'-field.bin')).read_bytes();self.assertEqual(ring['tiles'][1],list(field[0x2800:0x2bc0]));self.assertEqual(ring['attributes'][1],list(field[0x2bc0:0x2c00]))
    bg=render_court_ring(glyphs,ring,state['camera']['x'],state['camera']['y']);ppu=(directory/(stem+'-ppu.bin')).read_bytes();hud=render_hud_cells(hudglyphs,list(ppu[0x2000:0x20e0]),list(ppu[0x23c0:0x2400]));sprite=render_sprites(ppu,(directory/(stem+'-oam.bin')).read_bytes(),(directory/(stem+'-ctrl.bin')).read_bytes()[0],self.colors);original=Image.open(directory/f'frame-{n:05d}.png').convert('RGB');mismatch=[];boundary=render_hud_boundary(hudglyphs,ring,state['camera'],hud,timings.get(f'{court}:{n}',{}).get('timing'))
    for y in range(240):
     for x in range(256):
      if sprite.getpixel((x,y))[3]:continue
      actual=((0,0,0) if x<8 else bg.getpixel((x,y))[:3]) if y<193 else boundary.getpixel((x,0))[:3] if y==193 else hud.getpixel((x,y-191))[:3];pixels+=1
      if actual!=original.getpixel((x,y)):mismatch.append((x,y))
    self.assertEqual(mismatch,[],(court,n,len(mismatch),mismatch[:10]));reports.append({'court':court,'frame':n,'mismatches':0})
  self.assertEqual(len(reports),48);print(json.dumps({'status':'passed','frames':len(reports),'non_sprite_pixels':pixels,'reports':reports,'scope':capture['scope']+'; original CPU-derived fetch boundaries; no image-derived hint, not Go CPU-cycle reproduction'}))
if __name__=='__main__':unittest.main()
