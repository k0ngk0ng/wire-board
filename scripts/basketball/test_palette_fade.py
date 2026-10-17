"""Tests declared before the renderer LUT: exact index clamp, zero, aliases,
all32 BG/sprite entries, two teams, base/9/10, in/out and range rejection."""
from pathlib import Path
import json,unittest
from palette_fade import fade_nes_index,build_palette_fade_lut
ROOT=Path(__file__).resolve().parents[2]
class PaletteFadeTests(unittest.TestCase):
 def setUp(self):self.colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
 def test_original_function_buffers(self):
  cases=json.loads((ROOT/'.local/basketball/fade/index-oracle.json').read_text())['cases'];full={(c['paletteID'],tuple(c['teams']),c['flags']):c['colors'] for c in cases if c['level']==4}
  for case in cases:self.assertEqual([fade_nes_index(v,case['level']) for v in full[case['paletteID'],tuple(case['teams']),case['flags']]],case['colors'])
  self.assertEqual(len(cases),80)
 def test_rgb_alias_classes_preserve_index_results(self):
  lut=build_palette_fade_lut(self.colors);self.assertEqual(len(lut),54)
  for level in range(5):
   for index in range(64):
    source=list(self.colors[index*3:index*3+3]);entry=next(e for e in lut if e['rgb']==source);mapped=fade_nes_index(index,level)
    self.assertEqual(entry['levels'][level],list(self.colors[mapped*3:mapped*3+3]))
 def test_zero_identity_and_invalid_values(self):
  for index in range(64):self.assertEqual(fade_nes_index(index,0),15);self.assertEqual(fade_nes_index(index,4),index)
  for level in [-1,5,1.5]:
   with self.assertRaises(ValueError):fade_nes_index(32,level)
  for index in [-1,64]:
   with self.assertRaises(ValueError):fade_nes_index(index,2)
if __name__=='__main__':unittest.main()
