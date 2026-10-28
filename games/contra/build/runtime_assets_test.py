"""Failure inventory: nested audio bytes are corrupt/missing or escape the
manifest root while the audio catalog itself still passes its checksum.
"""
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from identity import verify_asset_files

ROOT = Path(__file__).resolve().parents[3]
class RuntimeAssetsTest(unittest.TestCase):
    def test_nested_audio_is_verified(self):
        parent=ROOT/'.local/contra-runtime-assets';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent) as name:
            root=Path(name)
            def save(name,data):
                (root/name).write_bytes(data)
                return {'url':'/'+name,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}
            audio=save('sound.wav',b'original sound')
            catalog=save('audio.json',json.dumps({'paths':{'reference':audio}}).encode())
            other=save('payload.bin',b'payload')
            manifest={key:other for key in ['wasm','loader','rgb','resources']}
            manifest.update(audioCatalog=catalog,pages={},stages={})
            verify_asset_files(root,manifest)
            (root/'sound.wav').write_bytes(b'corrupt sound')
            with self.assertRaises(ValueError):verify_asset_files(root,manifest)
            (root/'sound.wav').unlink()
            with self.assertRaises(FileNotFoundError):verify_asset_files(root,manifest)
            audio['url']='/../escape.wav'
            manifest['audioCatalog']=save('audio.json',json.dumps({'paths':{'reference':audio}}).encode())
            with self.assertRaises(ValueError):verify_asset_files(root,manifest)

if __name__=='__main__':unittest.main()
