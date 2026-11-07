"""Pre-implementation failures: real FLAC headers disagree with the published
PCM metadata; missing/non-STREAMINFO/short first blocks or impossible sample
rate, precision and sample-count fields pass without a real probe binary.
Full PCM decoding and its SHA-256 remain verified by verify_music.main.
"""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("verify_music", Path(__file__).with_name("verify_music.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
ROOT = Path(__file__).resolve().parents[3]
ASSETS = ROOT / "games/pvz/assets/public/pvz/classic-v2"


class FlacStreamInfoTests(unittest.TestCase):
    def example(self):
        manifest = json.loads((ASSETS / "manifest.json").read_text())
        track = manifest["music"]["day"]["tracks"][0]
        return bytearray((ASSETS / track["src"]).read_bytes()[:42])

    def test_all_committed_stems_match_independent_published_pcm_metadata(self):
        manifest = json.loads((ASSETS / "manifest.json").read_text())
        tracks = {track["src"]: track for theme in manifest["music"].values() for track in theme["tracks"]}
        self.assertEqual(len(tracks), 19)
        for name, track in tracks.items():
            with self.subTest(stem=name):
                info = module.read_streaminfo((ASSETS / name).read_bytes())
                self.assertEqual(info, {"sampleRate": track["sampleRate"], "channels": track["channels"],
                                        "bitsPerSample": track["bitsPerSample"], "totalSamples": track["pcmFrames"]})

    def test_marker_first_block_and_truncation_fail_closed(self):
        original = self.example()
        variants = {"empty": b"", "truncated": original[:41], "marker": b"RIFF" + original[4:]}
        wrong_type = original.copy();wrong_type[4] = (wrong_type[4] & 0x80) | 4
        wrong_length = original.copy();wrong_length[5:8] = b"\x00\x00\x21"
        variants.update(type=wrong_type, length=wrong_length)
        for name, body in variants.items():
            with self.subTest(case=name), self.assertRaises(ValueError):
                module.read_streaminfo(body)

    def test_zero_rate_invalid_precision_and_unknown_sample_count_are_rejected(self):
        original = self.example()
        rate = original.copy();rate[18:20] = b"\x00\x00";rate[20] &= 0x0f
        precision = original.copy();precision[20] &= 0xfe;precision[21] = (precision[21] & 0x0f) | 0x20
        samples = original.copy();samples[21] &= 0xf0;samples[22:26] = b"\x00" * 4
        blocks = original.copy();blocks[8:10] = b"\x00\x00"
        for name, body in {"rate": rate, "precision": precision, "samples": samples, "blocks": blocks}.items():
            with self.subTest(case=name), self.assertRaises(ValueError):
                module.read_streaminfo(body)

    def test_channel_precision_and_sample_count_bit_boundaries_do_not_overlap(self):
        body = self.example()
        # STREAMINFO stores channel count minus one in byte 20 bits 1..3 and
        # precision minus one across bytes 20/21. Sample count is independent.
        body[20] = (body[20] & 0xf0) | 0x0f
        body[21] = 0xff;body[22:26] = b"\xff" * 4
        self.assertEqual(module.read_streaminfo(body), {"sampleRate": 44100, "channels": 8,
                                                       "bitsPerSample": 32, "totalSamples": 68719476735})


if __name__ == "__main__":
    unittest.main()
