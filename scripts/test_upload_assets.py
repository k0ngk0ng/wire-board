"""Failure cases fixed before extending uploads: audio silently omitted, wrong
MIME/signatures, invalid media partially uploaded, credentials/non-media included,
and symlinks escaping the prepared bundle. No real OSS writes in these checks.
"""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('uploader', Path(__file__).with_name('upload_assets.py'))
uploader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(uploader)


class MediaSelection(unittest.TestCase):
    def test_images_remain_default_and_audio_is_explicit(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / 'house.webp').write_bytes(b'RIFF\x20\x00\x00\x00WEBP' + b'VP8 ' + b'\x00' * 16)
            (root / 'dice.mp3').write_bytes(b'ID3\x04\x00\x00\x00\x00\x00\x00' + b'\x00' * 24)
            (root / 'keys.txt').write_text('must not upload')
            (root / 'manifest.json').write_text('{}')
            self.assertEqual([(p.name, mime) for p, mime in uploader.collect_assets(root)], [('house.webp', 'image/webp')])
            self.assertEqual([(p.name, mime) for p, mime in uploader.collect_assets(root, include_audio=True)], [('dice.mp3', 'audio/mpeg'), ('house.webp', 'image/webp')])

    def test_invalid_media_rejected_before_upload(self):
        for name, body in [('bad.webp', b'not a webp'), ('bad.mp3', b'<html>login page</html>'), ('bad.ogg', b'not ogg'), ('bad.wav', b'RIFF1234WEBP0000')]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as d:
                root = Path(d); (root / name).write_bytes(body)
                with self.assertRaises(ValueError): uploader.collect_assets(root, include_audio=True)

    def test_supported_audio_mime_types(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / 'sound.ogg').write_bytes(b'OggS' + bytes(32))
            (root / 'sound.wav').write_bytes(b'RIFF' + bytes(4) + b'WAVE' + bytes(24))
            (root / 'sound.mp3').write_bytes(b'\xff\xfb\x90\x64' + bytes(32))
            self.assertEqual([mime for _, mime in uploader.collect_assets(root, True)], ['audio/mpeg', 'audio/ogg', 'audio/wav'])

    def test_symlink_not_followed(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); (root / 'private.txt').write_bytes(b'ID3' + bytes(32))
            (root / 'clip.mp3').symlink_to(root / 'private.txt')
            with self.assertRaises(ValueError): uploader.collect_assets(root, True)


if __name__ == '__main__': unittest.main()
