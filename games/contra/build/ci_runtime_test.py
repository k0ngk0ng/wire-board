"""Failure inventory: trim removes files before native binding; receipt names
another runtime; corrupt/extra assets or escaping output paths pass preparation.
"""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from identity import sha
from delivery import delivery_entries
from ci_runtime import prepare_runtime

ROOT=Path(__file__).resolve().parents[3]
class CIRuntimeTest(unittest.TestCase):
    def test_preserves_exact_runtime_before_trim_and_rejects_bad_inputs(self):
        parent=ROOT/'.local/contra-ci-runtime-tests';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent) as directory:
            root=Path(directory);dist=root/'dist';assets={}
            def entry(name,data):
                assets[name]=data
                return {'url':'/'+name,'bytes':len(data),'sha256':sha(data)}
            sound=entry('sound.wav',b'RIFF-sound')
            audio={'paths':{'reference':sound}}
            manifest={'rulesBuildID':'b'*64,'wasm':entry('code.wasm',b'wasm'),
                      'loader':entry('loader.js',b'js'),'rgb':entry('colors.rgb',b'rgb'),
                      'resources':entry('stage.json',b'{}'),'pages':{},'stages':{},
                      'audioCatalog':entry('audio.json',json.dumps(audio).encode())}
            raw=json.dumps(manifest).encode();identity=sha(raw);prefix=f'contra/runtime-{identity}/'
            origin=dist/prefix;origin.mkdir(parents=True)
            for name,data in assets.items():(origin/name).write_bytes(data)
            (origin/'manifest.json').write_bytes(raw)
            rows=delivery_entries(manifest,audio);rows['manifest.json']={'bytes':len(raw),'sha256':identity,'mime':'application/json','requiresRange':False}
            receipt={'prefix':prefix,'runtimeBuildID':identity,'rulesBuildID':'b'*64,'files':rows}
            receipt_file=dist/'contra-delivery.json';receipt_file.write_text(json.dumps(receipt))
            with patch('ci_runtime.verify_runtime_file',return_value=(manifest,identity,{})):
                out=root/'preserved';prepare_runtime(root,dist,out)
                self.assertEqual((out/'sound.wav').read_bytes(),assets['sound.wav'])
                self.assertEqual((out/'manifest.json').read_bytes(),raw)
                (origin/'private.html').write_text('private')
                with self.assertRaises(ValueError):prepare_runtime(root,dist,root/'bad-extra')
                (origin/'private.html').unlink();(origin/'sound.wav').write_bytes(b'broken')
                with self.assertRaises(ValueError):prepare_runtime(root,dist,root/'bad-content')
                (origin/'sound.wav').write_bytes(assets['sound.wav'])
                receipt['runtimeBuildID']='c'*64;receipt_file.write_text(json.dumps(receipt))
                with self.assertRaises(ValueError):prepare_runtime(root,dist,root/'bad-id')
                with self.assertRaises(ValueError):prepare_runtime(root,dist,root.parent/'escape')

if __name__=='__main__':unittest.main()
