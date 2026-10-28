"""Failure inventory: changing media preparation or browser packaging silently
reuses runtime identity; missing build inputs are ignored instead of failing.
"""
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from identity import runtime_inputs

ROOT = Path(__file__).resolve().parents[3]
class BuilderInputsTest(unittest.TestCase):
    def test_runtime_binds_packaging_and_media_helpers(self):
        parent = ROOT/'.local/contra-builder-inputs'
        parent.mkdir(parents=True, exist_ok=True)
        names = ['go.mod','go.sum','games/contra/assets/runtime-stages.json',
                 'games/contra/assets/audio-source.json','games/contra/assets/audio-loops.json',
                 'games/contra/server/data/visual-catalog-v1.json.gz',
                 'games/contra/build/identity.py','games/contra/build/media.py',
                 'games/contra/build/media_archive.py','games/contra/assets/audio-pack.json',
                 'scripts/contra/browser_build.py']
        with tempfile.TemporaryDirectory(dir=parent) as name:
            root = Path(name)
            for path in names:
                file = root/path
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text('original')
            with patch('identity.go', return_value=''):
                before = runtime_inputs(root, {'test':'isolated'})
                self.assertEqual({x['path'] for x in before}, set(names))
                (root/'games/contra/build/media.py').write_text('changed')
                self.assertNotEqual(before, runtime_inputs(root, {'test':'isolated'}))
                (root/'scripts/contra/browser_build.py').unlink()
                with self.assertRaises(FileNotFoundError): runtime_inputs(root, {'test':'isolated'})

if __name__ == '__main__': unittest.main()
