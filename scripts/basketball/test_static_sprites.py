"""Static-asset failure cases, written before the decoder.

Wrong PRG bank, pose-count boundary, split-array order, sign conversion,
mirrored tile position/attributes, per-character head replacement, OAM order,
and CHR page identity must fail against independently recorded original pixels.
Unobserved team palettes must remain explicit, never silently guessed.
"""
from pathlib import Path
import json
import unittest
from PIL import Image
from static_sprites import SpriteROM, render_player

ROOT = Path(__file__).resolve().parents[2]
ROM = ROOT/'.local/nes-basket/basketball.nes'

class StaticSpriteTests(unittest.TestCase):
    def test_bounds_and_identity(self):
        data = ROM.read_bytes()
        with self.assertRaises(ValueError): SpriteROM(data[:-1])
        with self.assertRaises(ValueError): SpriteROM(bytes(len(data)))
        rom = SpriteROM(data)
        self.assertEqual(rom.pose_count(0),87)
        self.assertEqual(rom.pose_count(1),85)
        with self.assertRaises(ValueError): rom.pieces(0,87)
        with self.assertRaises(ValueError): rom.pieces(19,0)
        self.assertEqual(rom.pieces(0,0)[0],(49,0,0,16))

    def test_team_palette_and_chr_from_rom(self):
        rom=SpriteROM(ROM.read_bytes())
        directory=ROOT/'.local/nes-basket/runs/graphics-right'
        ppu=next(directory.glob('ppu-*-ppu.bin')).read_bytes()
        self.assertEqual(rom.team_palette(0)[1:4],ppu[0x3f11:0x3f14])
        self.assertEqual(rom.team_palette(0)[5:8],ppu[0x3f15:0x3f18])
        self.assertEqual(rom.team_palette(1)[1:4],ppu[0x3f19:0x3f1c])
        self.assertEqual(rom.team_palette(1)[5:8],ppu[0x3f1d:0x3f20])
        self.assertEqual(rom.player_chr_pages(0,1),rom.identify_chr_pages(ppu[:8192])[4:])
        with self.assertRaises(ValueError):rom.team_palette(8)

    def test_all_static_player_poses(self):
        rom=SpriteROM(ROM.read_bytes())
        colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
        for team in range(8):
            palette=bytes(16)+rom.team_palette(team)*2
            for pose in range(rom.pose_count(team)):
                for facing in (0,128):
                    with self.subTest(team=team,pose=pose,facing=facing):
                        image=render_player(rom,team,pose|facing,0,0,[96,97,98,99]+rom.player_chr_pages(team,team),palette,colors,size=(128,128),origin=(64,96))
                        self.assertTrue(image.getbbox())

    def test_recorded_original_frames(self):
        rom = SpriteROM(ROM.read_bytes())
        doc=json.loads((ROOT/'.local/basketball/atlas/atlas.json').read_text())
        checked=0
        for entry in doc['frames']:
            if entry['object'] >=4: continue
            source=entry['sources'][0]
            directory=ROOT/'.local/nes-basket/runs'/source['capture']
            stem='ppu-%05d'%source['frame']
            ppu=(directory/(stem+'-ppu.bin')).read_bytes()
            colors=(directory/'nes-colors.rgb').read_bytes()
            state=source['state'];owner=source['object']
            char=(directory/'initial-ram.bin').read_bytes()[0x3e3+owner]
            generated=render_player(rom,state['ram03d6'],state['ram005b'],char,owner,
                rom.identify_chr_pages(ppu[:8192]),ppu[0x3f00:0x3f20],colors,
                reverse=source['priority']=='reverse')
            expected=Image.open(ROOT/'.local/basketball/atlas'/entry['file']).convert('RGBA')
            self.assertEqual(generated.tobytes(),expected.tobytes(),entry['file'])
            checked+=1
        self.assertGreaterEqual(checked,30)

if __name__=='__main__': unittest.main()
