"""Before-implementation result-art tests.
Failure cases: ordinary court substituted for result preset, wrong compressed
CHR stream command, incomplete table output, large score tile layout/palette,
kind17 fragments missing, or stage6 rejected/painted with the gameplay HUD.
"""
from pathlib import Path
import json,unittest
from PIL import Image
from graphics import render_sprites
from static_sprites import SpriteROM
from result_art import decode_background,build_result_images,compose_result_background
ROOT=Path(__file__).resolve().parents[2]
class ResultArtTests(unittest.TestCase):
 def setUp(self):
  self.rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());self.directory=ROOT/'.local/nes-basket/runs/result-art-observed';self.colors=(self.directory/'nes-colors.rgb').read_bytes()
 def test_original_background_presets(self):
  r=decode_background(self.rom,102);self.assertEqual(r['target'],0x2800);self.assertEqual(len(r['data']),1024)
  item=json.loads((ROOT/'.local/basketball/result-art/capture.json').read_text())['frames'][0];p=self.directory/f"ppu-{item['emulator_frame']:05d}-field.bin";ppu=p.read_bytes()
  self.assertEqual(bytes(r['data']),ppu[0x2800:0x2c00])
  with self.assertRaises(ValueError):decode_background(self.rom,255)
 def test_result_original_pixels(self):
  observations=json.loads((ROOT/'.local/basketball/result-art/capture.json').read_text())['frames'];images=build_result_images(self.rom,1,self.colors);count=0
  for item in observations:
   stem='ppu-%05d'%item['emulator_frame'];s=json.loads((self.directory/(stem+'-semantic.json')).read_text());ppu=(self.directory/(stem+'-ppu.bin')).read_bytes();oam=(self.directory/(stem+'-oam.bin')).read_bytes();ctrl=(self.directory/(stem+'-ctrl.bin')).read_bytes()[0]
   original=Image.open(self.directory/('frame-%05d.png'%item['frame'])).convert('RGB');bg=compose_result_background(images,s['result']['digits'],s['camera']);sprites=render_sprites(ppu,oam,ctrl,self.colors);mismatch=[]
   for y in range(240):
    for x in range(256):
     if sprites.getpixel((x,y))[3]:continue
     count+=1
     if bg.getpixel((x,y))[:3]!=original.getpixel((x,y)):mismatch.append((x,y))
   self.assertEqual(len(mismatch),0,(item['frame'],len(mismatch),mismatch[:10]))
  self.assertEqual(len(observations),15);print(json.dumps({'frames':15,'background_pixels':count,'mismatches':0,'scope':'static result background and big-score glyphs; original sprites excluded here and tested in Canvas'}))
 def test_kind17_table(self):
  self.assertEqual(self.rom.pose_count(17),120)
  self.assertTrue(all(self.rom.pieces(17,p) for p in range(120)))
if __name__=='__main__':unittest.main()
