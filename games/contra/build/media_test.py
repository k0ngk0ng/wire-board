"""Failure inventory: absent media, corrupt bytes, path escape, stale descriptor,
duplicate references and incomplete packs. No network or implicit cache fallback.
"""
import hashlib
import tempfile
import unittest
from pathlib import Path
from media import read_media, collect_media

ROOT = Path(__file__).resolve().parents[3]

class MediaTest(unittest.TestCase):
    def test_explicit_pack_rejects_corrupt_missing_and_escaping_media(self):
        parent = ROOT / '.local/contra-media-tests'
        parent.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent) as name:
            root = Path(name)
            payload = b'RIFF-test-audio'
            digest = hashlib.sha256(payload).hexdigest()
            stem = {'file': 'source.wav', 'sha256': digest}
            (root / (digest + '.wav')).write_bytes(payload)
            self.assertEqual(read_media(root, stem, root), payload)
            (root / (digest + '.wav')).write_bytes(b'corrupt')
            with self.assertRaises(ValueError): read_media(root, stem, root)
            (root / (digest + '.wav')).unlink()
            with self.assertRaises(FileNotFoundError): read_media(root, stem, root)
            with self.assertRaises(ValueError): read_media(root, dict(stem, file='../outside.wav'))

    def test_loops_replace_short_music_and_references_deduplicate(self):
        a = {'file':'short.wav','sha256':'a'*64}
        b = {'file':'loop.wav','sha256':'b'*64}
        source = {'sounds':[{'id':70,'stems':[a]}, {'id':2,'stems':[b,b]}]}
        loops = {'songs':[{'id':70,'stems':[b]}]}
        self.assertEqual(collect_media(source, loops), {'loop.wav':b})

if __name__ == '__main__': unittest.main()
