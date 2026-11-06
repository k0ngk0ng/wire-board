"""Local-readback gate failures are listed in CDN_LOCAL_FAILURE_SCENARIOS.md.
No real GitHub/CDN requests; the actual release retains its network report.
"""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

import cdn_delivery as cdn

ROOT = Path(__file__).resolve().parent


class LocalReady(unittest.TestCase):
 def setUp(self):
  spec=importlib.util.spec_from_file_location('cdn_local_ready',ROOT/'cdn_local_ready.py');self.module=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.module)
  self.sha='a'*40;self.run=123
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.dist=Path(self.temp.name);(self.dist/'assets').mkdir();(self.dist/'assets/app.js').write_bytes(b'compiled')
  self.manifest={'assetBase':'https://cdn.ichenj.com/bga/web/'+self.sha+'/'}
  (self.dist/'game-load-manifest.json').write_text(json.dumps(self.manifest));(self.dist/'index.html').write_text('entry')
  self.report={'passed':True,'stage':'verified','cdnVerified':True,'verificationLocation':'local','files':cdn.inventory(self.dist,self.manifest['assetBase']),'indexSHA256':cdn.sha((self.dist/'index.html').read_bytes()),'manifestSHA256':cdn.sha((self.dist/'game-load-manifest.json').read_bytes())}
 def test_exact_local_inventory_is_bound_to_source_and_build(self):
  proof=self.module.local_readback_proof(self.dist,self.report,self.sha,self.run)
  self.assertEqual(proof['sourceSHA'],self.sha);self.assertEqual(proof['sourceRun'],self.run);self.assertEqual(proof['files'],self.report['files'])
 def test_upload_missing_failed_foreign_or_changed_readback_is_rejected(self):
  for patch in [{'passed':False},{'cdnVerified':False},{'stage':'uploaded'},{'verificationLocation':'actions'},{'files':[]},{'indexSHA256':'0'*64},{'manifestSHA256':'0'*64}]:
   with self.subTest(patch=patch),self.assertRaises(ValueError):self.module.local_readback_proof(self.dist,dict(self.report,**patch),self.sha,self.run)
  with self.assertRaises(ValueError):self.module.local_readback_proof(self.dist,self.report,'b'*40,self.run)
  (self.dist/'assets/app.js').write_bytes(b'changed')
  with self.assertRaises(ValueError):self.module.local_readback_proof(self.dist,self.report,self.sha,self.run)
 def test_cancelled_wrong_source_and_foreign_workflow_builds_fail(self):
  run={'id':self.run,'head_sha':self.sha,'head_branch':'main','event':'push','path':'.github/workflows/build.yml','status':'completed','conclusion':'success'}
  jobs=[{'name':'Test and Linux amd64 binary','conclusion':'success'}]
  self.module.validate_build(run,jobs,self.sha,self.run)
  for patch in [{'head_sha':'b'*40},{'id':124},{'head_branch':'feature'},{'event':'workflow_dispatch'},{'path':'.github/workflows/other.yml'},{'conclusion':'cancelled'},{'status':'in_progress'}]:
   with self.subTest(patch=patch),self.assertRaises(ValueError):self.module.validate_build(dict(run,**patch),jobs,self.sha,self.run)
  with self.assertRaises(ValueError):self.module.validate_build(run,[{'name':'Test and Linux amd64 binary','conclusion':'failure'}],self.sha,self.run)


if __name__=='__main__':unittest.main()
