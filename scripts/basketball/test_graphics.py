"""Before-implementation checks for the offline NES graphics decoder.

Failure cases: swapped bitplanes, incorrect pixel order, lost transparency,
incorrect 8x16 addressing/flip, or silent acceptance of truncated inputs.
The renderer must additionally pass original-frame pixel comparisons.
"""
import unittest
from graphics import decode_tile, sprite_pixels,render_sprites


class GraphicsTests(unittest.TestCase):
    def test_bitplanes_and_orientation(self):
        raw=bytes([0x80,0,0,0,0,0,0,1,0x40,0,0,0,0,0,0,1])
        pixels=decode_tile(raw)
        self.assertEqual(pixels[0], [1,2,0,0,0,0,0,0])
        self.assertEqual(pixels[7], [0,0,0,0,0,0,0,3])
        self.assertTrue(all(row==[0]*8 for row in pixels[1:7]))

    def test_8x16_bank_and_flips(self):
        pattern=bytearray(8192)
        pattern[0x1020]=0x80  # tile 2, bank 1, top-left color 1
        pattern[0x1037]=0x01  # tile 3, bank 1, last-row right color 1
        normal=sprite_pixels(pattern,3,0,True,0)
        self.assertEqual((len(normal),len(normal[0])),(16,8))
        self.assertEqual(normal[0][0],1)
        self.assertEqual(normal[15][7],1)
        flipped=sprite_pixels(pattern,3,0xc0,True,0)
        self.assertEqual(flipped,[row[::-1] for row in normal[::-1]])

    def test_truncated_data(self):
        with self.assertRaises(ValueError):decode_tile(bytes(15))
        with self.assertRaises(ValueError):sprite_pixels(bytes(4096),3,0,True,0)

    def test_left_edge_sprite_mask(self):
        ppu=bytearray(0x4000);ppu[0]=0xff;ppu[0x3f11]=1
        oam=bytearray([255]*256);oam[:4]=bytes([10,0,0,4])
        colors=bytes([0,0,0,255,255,255]+[0]*186)
        im=render_sprites(ppu,oam,0,colors)
        self.assertEqual(im.getpixel((7,11))[3],0)
        self.assertEqual(im.getpixel((8,11)),(255,255,255,255))


if __name__=='__main__':unittest.main()
