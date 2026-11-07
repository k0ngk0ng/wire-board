import importlib.util
from pathlib import Path
import struct
import unittest
import zlib

spec = importlib.util.spec_from_file_location("reanim", Path(__file__).with_name("reanim.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def string(value):
    value = value.encode()
    return struct.pack("<I", len(value)) + value


def fixture():
    data = struct.pack("<IIIfII", 0xB393B4C0, 0, 1, 12., 0, 12)
    data += struct.pack("<III", 0, 0, 2)
    data += string("body") + struct.pack("<I", 44)
    data += struct.pack("<8fIII", 5, 10, 0, 0, 1, 1, 0, 1, 0, 0, 0)
    data += struct.pack("<8fIII", -10000, 12, -10000, -10000, -10000, -10000, -10000, -10000, 0, 0, 0)
    data += string("IMAGE_REANIM_BODY") + string("") + string("")
    data += string("") + string("") + string("")
    return struct.pack("<II", 0xDEADFED4, len(data)) + zlib.compress(data)


class ReanimationTests(unittest.TestCase):
    def test_initial_frame_sentinel_inherits_zero(self):
        encoded = fixture()
        data = bytearray(zlib.decompress(encoded[8:]))
        # header, one track, counted name, transform size, sixth float.
        struct.pack_into("<f", data, 24 + 12 + 4 + 4 + 4 + 6 * 4, -10000)
        encoded = struct.pack("<II", 0xDEADFED4, len(data)) + zlib.compress(data)
        self.assertEqual(module.decode(encoded)["tracks"][0]["frames"][0]["f"], 0)

    def test_decode_and_inherit_transform(self):
        out = module.decode(fixture())
        self.assertEqual(out["fps"], 12)
        self.assertEqual(out["tracks"][0]["name"], "body")
        frame = out["tracks"][0]["frames"][1]
        self.assertEqual((frame["x"], frame["y"], frame["image"]), (5, 12, "IMAGE_REANIM_BODY"))
        self.assertEqual(frame["f"], 0)

    def test_rejects_truncation_and_wrong_header(self):
        for data in [b"wrong", fixture()[:-10], b"abcdefgh" + fixture()[8:]]:
            with self.assertRaises(ValueError):
                module.decode(data)


if __name__ == "__main__":
    unittest.main()
