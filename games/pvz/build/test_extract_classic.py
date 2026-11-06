import importlib.util
from pathlib import Path
import struct
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("extract_classic", Path(__file__).with_name("extract_classic.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def package(members):
    table = bytearray(struct.pack("<II", 0xBAC04AC0, 0))
    for name, data in members:
        name = name.encode("utf8")
        table += bytes([0, len(name)]) + name + struct.pack("<IQ", len(data), 0)
    table += b"\x80"
    plain = table + b"".join(data for _, data in members)
    return bytes(value ^ 0xf7 for value in plain)


class PakExtractionTests(unittest.TestCase):
    def test_valid_members_and_source_offsets(self):
        members = module.read_pak(package([("images\\one.png", b"abc"), ("empty", b"")]))
        self.assertEqual([(m["path"], m["data"]) for m in members], [("images/one.png", b"abc"), ("empty", b"")])
        self.assertGreater(members[0]["offset"], 8)

    def test_rejects_unsafe_and_duplicate_paths(self):
        for name in ["../bad", "/absolute", "C:\\bad", "a//b", "a/./b", "a/../b", "a\x00b"]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                module.read_pak(package([(name, b"x")]))
        with self.assertRaises(ValueError):
            module.read_pak(package([("A", b"1"), ("a", b"2")]))

    def test_rejects_truncation_and_unknown_data(self):
        data = package([("a", b"abc")])
        for invalid in [b"wrong", data[:7], data[:12], data[:-1], data + b"tail"]:
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                module.read_pak(invalid)

    def test_refuses_existing_output(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).parents[3] / ".local") as temp:
            output = Path(temp) / "output"
            output.mkdir()
            with self.assertRaises(FileExistsError):
                module.extract(package([("a", b"abc")]), output)


if __name__ == "__main__":
    unittest.main()
