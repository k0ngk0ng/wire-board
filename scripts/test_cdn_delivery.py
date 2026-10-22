"""Pre-implementation safety failures: incomplete/hash/mime/CORS mismatches,
symlinks and traversal in uploads; trim before verification; secret publication.
"""
import unittest,tempfile,json,hashlib
from pathlib import Path
import cdn_delivery as cdn
class Delivery(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.dist=Path(self.temp.name);(self.dist/'assets').mkdir();(self.dist/'assets/index-abcdefgh.js').write_bytes(b'export const ready=true;');(self.dist/'index.html').write_text('entry')
  self.base='https://cdn.ichenj.com/bga/web/'+'a'*40+'/'
  self.entry={'url':self.base+'assets/index-abcdefgh.js','bytes':24,'sha256':hashlib.sha256(b'export const ready=true;').hexdigest(),'mime':'text/javascript'}
 def test_checks_remote_bytes_mime_and_cors(self):
  body=b'export const ready=true;';headers={'Content-Type':'text/javascript','Access-Control-Allow-Origin':'*'}
  cdn.validate_response(self.entry,body,headers)
  for b,h in [(body+b'!',headers),(body,{'Content-Type':'text/html','Access-Control-Allow-Origin':'*'}),(body,{'Content-Type':'text/javascript'})]:
   with self.assertRaises(ValueError):cdn.validate_response(self.entry,b,h)
 def test_inventory_is_bound_to_files_and_rejects_symlinks(self):
  rows=cdn.inventory(self.dist,self.base);self.assertEqual(rows[0]['url'],self.entry['url']);self.assertEqual(rows[0]['sha256'],self.entry['sha256'])
  (self.dist/'assets/leak.js').symlink_to(self.dist/'index.html')
  with self.assertRaises(ValueError):cdn.inventory(self.dist,self.base)
 def test_trim_requires_exact_verified_inventory(self):
  rows=cdn.inventory(self.dist,self.base)
  with self.assertRaises(ValueError):cdn.trim(self.dist,{'passed':False,'files':rows})
  altered=[dict(rows[0],sha256='0'*64)]
  with self.assertRaises(ValueError):cdn.trim(self.dist,{'passed':True,'files':altered})
  cdn.trim(self.dist,{'passed':True,'files':rows});self.assertFalse((self.dist/'assets').exists());self.assertTrue((self.dist/'index.html').exists())
if __name__=='__main__':unittest.main()
