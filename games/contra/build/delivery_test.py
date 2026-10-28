"""Failure inventory: private HTML enters delivery, nested audio is omitted,
duplicate asset paths conflict, unknown types pass, and manifests are rewritten.
"""
import unittest
from delivery import delivery_entries

class DeliveryTest(unittest.TestCase):
    def test_exact_referenced_payloads_and_mime(self):
        def asset(name):return {'url':'/'+name,'bytes':1,'sha256':'a'*64}
        m={k:asset(v) for k,v in {'wasm':'contra.wasm','loader':'wasm_exec.js','audioCatalog':'audio.json','rgb':'colors.rgb','resources':'stage-1.json'}.items()}
        m.update(pages={'1':asset('page.idx')},stages={'0':m['resources']})
        audio={'paths':{'source':asset('sound.wav')}}
        rows=delivery_entries(m,audio)
        self.assertEqual(len(rows),7)
        self.assertEqual(rows['sound.wav']['mime'],'audio/wav')
        self.assertTrue(rows['sound.wav']['requiresRange'])
        self.assertNotIn('index.html',rows)
        m['loader']=asset('index.html')
        with self.assertRaises(ValueError):delivery_entries(m,audio)
        m['loader']=dict(m['wasm'],sha256='b'*64)
        with self.assertRaises(ValueError):delivery_entries(m,audio)
        # Aliases must not overwrite another staged file after normalization.
        for bad in ['//wasm_exec.js','/./wasm_exec.js','/a//wasm_exec.js','/a/../wasm_exec.js','/a\\wasm_exec.js','wasm_exec.js']:
            m['loader']=dict(asset('wasm_exec.js'),url=bad)
            with self.subTest(path=bad),self.assertRaises(ValueError):delivery_entries(m,audio)

if __name__=='__main__':unittest.main()
