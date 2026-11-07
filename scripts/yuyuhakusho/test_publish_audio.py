"""Before implementation: mutation, path escape, duplicate or lossy payloads
must fail before upload; publication must use an immutable catalog identity.
"""
from pathlib import Path
import importlib.util,json,hashlib,tempfile,unittest
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('yuyu_audio_publisher',Path(__file__).with_name('publish_audio.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class AudioPlan(unittest.TestCase):
 def test_exact_local_flac_and_immutable_catalog_prefix(self):
  parent=ROOT/'.local/md-yuyuhakusho/tmp';parent.mkdir(parents=True,exist_ok=True)
  with tempfile.TemporaryDirectory(dir=parent)as d:
   root=Path(d);body=b'fLaC-fixture';(root/'06.flac').write_bytes(body)
   row={'file':'06.flac','bytes':len(body),'sha256':hashlib.sha256(body).hexdigest(),'frames':1,'pcm_sha256':'a'*64,'loop':False}
   raw=json.dumps({'schema':1,'music':{},'effects':{'6':row}}).encode();pin,rows=m.plan_audio(raw,root)
   self.assertEqual(pin['catalogSHA256'],hashlib.sha256(raw).hexdigest());self.assertEqual(pin['files'],1)
   self.assertEqual(rows[0]['url'],pin['assetsBaseURL']+'/06.flac');self.assertEqual(rows[0]['encoding'],'');self.assertTrue(rows[0]['requiresRange'])
   (root/'06.flac').write_bytes(body+b'!')
   with self.assertRaises(ValueError):m.plan_audio(raw,root)
   row['file']='../06.flac'
   with self.assertRaises(ValueError):m.plan_audio(json.dumps({'schema':1,'music':{},'effects':{'6':row}}).encode(),root)
if __name__=='__main__':unittest.main()
