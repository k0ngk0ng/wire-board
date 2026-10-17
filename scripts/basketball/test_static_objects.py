"""Before-implementation failures: ball pose chooses wrong CHR page, shared page
changes prop pixels, palette ownership/attribute masks, facing/OAM reversal,
wrong origin, unsupported kind, truncated input, or mixed-team palette guessing.
"""
from pathlib import Path
import json,unittest
from PIL import Image
from static_sprites import SpriteROM
from static_objects import render_object,object_palette_side
ROOT=Path(__file__).resolve().parents[2]

class StaticObjectsTests(unittest.TestCase):
 def setUp(self):
  self.rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes())
  self.colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
 def test_ball_chr_selection(self):
  self.assertEqual([self.rom.ball_chr_bank(i) for i in range(40)],[32,32,32,32,35,35,32,32,32,32,32,34,34,32,32,32,32,33,33,33,33,33,33,33,33,33,33,34,34,34,34,34,34,34,34,34,34,34,35,35])
  self.assertEqual(self.rom.ball_chr_bank(127),32)
  self.assertEqual(self.rom.ball_chr_bank(128),32)
  with self.assertRaises(ValueError):self.rom.ball_chr_bank(40)
 def test_all_recorded_object_images(self):
  doc=json.loads((ROOT/'.local/basketball/atlas/atlas.json').read_text());checked=0
  for entry in doc['frames']:
   if entry['object']<4:continue
   source=entry['sources'][0];directory=ROOT/'.local/nes-basket/runs'/source['capture'];stem='ppu-%05d'%source['frame']
   ppu=(directory/(stem+'-ppu.bin')).read_bytes();pages=self.rom.identify_chr_pages(ppu[:8192]);state=source['state']
   got=render_object(self.rom,state['ram03d6'],state['ram005b'],pages,ppu[0x3f00:0x3f20],self.colors,reverse=source['priority']=='reverse')
   want=Image.open(ROOT/'.local/basketball/atlas'/entry['file']).convert('RGBA')
   self.assertEqual(got.tobytes(),want.tobytes(),entry['file']);checked+=1
  self.assertEqual(checked,7)
 def test_static_objects_never_clip_or_mix_teams(self):
  for kind in range(8,15):
   for pose in range(self.rom.pose_count(kind)):
    self.assertIn(object_palette_side(self.rom,kind,pose),(0,1))
    for bank in ([self.rom.ball_chr_bank(pose)] if kind==8 else range(32,36)):
     for facing in (0,128):
      got=render_object(self.rom,kind,pose|facing,[96,97,98,99,0,1,2,bank],bytes(16)+self.rom.team_palette(0)+self.rom.team_palette(1),self.colors,size=(128,128),origin=(64,96))
      self.assertTrue(got.getbbox(),(kind,pose,bank,facing))
 def test_palette_mask_uses_home_colors(self):
  pages=[96,97,98,99,0,1,2,35];palette=bytes(16)+self.rom.team_palette(0)+self.rom.team_palette(1)
  home=render_object(self.rom,8,4,pages,palette,self.colors,attribute_mask=0xfd)
  home_colors=bytes(16)+self.rom.team_palette(0)*2
  expected=render_object(self.rom,8,4,pages,home_colors,self.colors)
  self.assertEqual(home.tobytes(),expected.tobytes())
  with self.assertRaises(ValueError):render_object(self.rom,0,0,pages,palette,self.colors)
  with self.assertRaises(ValueError):render_object(self.rom,8,0,pages,palette,self.colors,attribute_mask=0)

if __name__=='__main__':unittest.main()
