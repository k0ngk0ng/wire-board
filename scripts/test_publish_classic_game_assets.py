"""Preimplementation failure inventory: .local/classic-game-assets/SCENARIOS.md.
No real network or credentials are used by these targeted checks.
"""
import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import threading
import time
import unittest
import urllib.error

ROOT=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('classic_publisher',ROOT/'scripts/publish-classic-game-assets.py')
pub=importlib.util.module_from_spec(spec);spec.loader.exec_module(pub)

class FakeOSS:
 def __init__(self,stored=None,cdn_bad=False,race=False):
  self.stored={} if stored is None else dict(stored);self.puts=[];self.reads=[];self.cdn_bad=cdn_bad;self.race=race
  self.active=0;self.maximum=0;self.lock=threading.Lock()
 def read_oss(self,path):
  self.reads.append(path)
  if path not in self.stored:raise urllib.error.HTTPError('https://oss.test/'+path,404,'missing',{},None)
  return self.stored[path],'image/png'
 def create(self,path,data,mime):
  with self.lock:self.active+=1;self.maximum=max(self.maximum,self.active)
  time.sleep(.01);self.puts.append(path);self.stored[path]=data
  with self.lock:self.active-=1
  if self.race:raise urllib.error.HTTPError('https://oss.test/'+path,409,'exists',{},None)
 def read_cdn(self,url):
  path=url.removeprefix('https://cdn.ichenj.com/')
  return (b'bad' if self.cdn_bad else self.stored[path]),'image/png'

def entry(index=0):
 data=b'verified-content'
 return {'urlPath':f'/bubbleparty/v4/classic/test-{index}.png','sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'mime':'image/png','data':data}

class PublisherChecks(unittest.TestCase):
 def test_invalid_paths_sizes_hashes_and_duplicates_rejected_locally(self):
  parent=ROOT/'.local/classic-game-assets/test-tmp';parent.mkdir(parents=True,exist_ok=True)
  with tempfile.TemporaryDirectory(dir=parent) as d:
   root=Path(d);path=root/'games/bubbleparty/assets/public/bubbleparty/v4/classic/a.png';path.parent.mkdir(parents=True)
   source=next((ROOT/'games/bubbleparty/assets/public/bubbleparty/v4/classic/previews').glob('*.png')).read_bytes();path.write_bytes(source)
   e={'urlPath':'/bubbleparty/v4/classic/a.png','sha256':hashlib.sha256(source).hexdigest(),'bytes':len(source)}
   self.assertEqual(pub.validate_entry(root,'bubbleparty',e)['data'],source)
   for patch in [{'bytes':1},{'sha256':'0'*64},{'urlPath':'/bubbleparty/v2/a.png'},{'urlPath':'/bubbleparty/v4/../../secret.png'},{'urlPath':'https://example.test/a.png'},{'output':'config/secrets/private'}]:
    with self.subTest(patch=patch),self.assertRaises((ValueError,FileNotFoundError)):pub.validate_entry(root,'bubbleparty',dict(e,**patch))
   path.unlink();outside=root/'outside.png';outside.write_bytes(source);path.symlink_to(outside)
   with self.assertRaises(ValueError):pub.validate_entry(root,'bubbleparty',e)
  with self.assertRaises(ValueError):pub.ensure_unique([entry(),entry()])
 def test_put_signature_includes_no_overwrite_and_never_embeds_key_in_url(self):
  h=pub.oss_headers('PUT','bga/v20261002-1/bubbleparty/v4/test.png',b'x','image/png','test-key','test-secret','Mon, 01 Jan 2024 00:00:00 GMT')
  self.assertEqual(h['x-oss-forbid-overwrite'],'true');self.assertEqual(h['Cache-Control'],'public, max-age=31536000, immutable')
  self.assertTrue(h['Authorization'].startswith('OSS test-key:'));self.assertNotIn('test-secret',json.dumps(h))
 def test_matching_existing_is_reused_conflicting_existing_never_written(self):
  e=entry();key=pub.PREFIX+e['urlPath'];client=FakeOSS({key:e['data']})
  result=pub.publish_one(client,e);self.assertEqual(result['status'],'existing');self.assertEqual(client.puts,[])
  client=FakeOSS({key:b'wrong'})
  with self.assertRaises(ValueError):pub.publish_one(client,e)
  self.assertEqual(client.puts,[])
 def test_missing_upload_and_competing_creation_are_verified(self):
  for race in [False,True]:
   client=FakeOSS(race=race);result=pub.publish_one(client,entry())
   self.assertTrue(result['ossVerified'] and result['cdnVerified']);self.assertEqual(len(client.puts),1)
 def test_wrong_cdn_bytes_and_wrong_anchor_fail(self):
  client=FakeOSS(cdn_bad=True)
  with self.assertRaises(ValueError):pub.publish_one(client,entry())
  client=FakeOSS({'anchor':b'wrong'})
  with self.assertRaises(ValueError):pub.verify_anchor(client,'anchor','0'*64)
  self.assertEqual(client.puts,[])
 def test_parallel_publication_is_bounded_and_reports_every_item(self):
  client=FakeOSS();results,errors=pub.publish_entries(client,[entry(i)for i in range(12)],workers=3)
  self.assertEqual(len(results),12);self.assertEqual(errors,[]);self.assertLessEqual(client.maximum,3)

if __name__=='__main__':unittest.main()
