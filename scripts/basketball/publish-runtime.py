#!/usr/bin/env python3
"""Publish a locally verified Wasm to immutable OSS; never replace an object."""
import argparse,hashlib,importlib.util,json,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--execute',action='store_true');args=ap.parse_args()
 public=Path(subprocess.check_output(['node','--input-type=module','-e','import {prepareBasketballRuntime} from "./games/basketball/build/runtime.mjs";console.log(prepareBasketballRuntime(process.cwd()))'],cwd=ROOT,text=True).strip())
 runtime=public/'basketball/v1/runtime';m=json.loads((runtime/'build.json').read_text());b=(runtime/'basketball.wasm').read_bytes();f=m['files']['basketball.wasm']
 assert hashlib.sha256(b).hexdigest()==f['sha256'] and len(b)==f['bytes']
 key='bga/basketball/runtime-'+f['sha256']+'/basketball.wasm';pin=dict(sourceSHA256=m['sourceSHA256'],**f,url='https://cdn.ichenj.com/'+key)
 if not args.execute:print(json.dumps(pin));return
 spec=importlib.util.spec_from_file_location('publish',Path(__file__).with_name('publish-assets.py'));module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
 oss=module.OSS(ROOT);oss.verify_bucket(ROOT);old,mime=oss.get(key)
 if old is None:oss.put(key,b,'application/wasm')
 else:assert old==b and mime=='application/wasm','immutable collision'
 for cdn in (False,True):
  data,mime=oss.get(key,cdn=cdn);assert data==b and mime=='application/wasm','readback differs'
 (ROOT/'games/basketball/assets/runtime-manifest.json').write_text(json.dumps(pin,indent=2)+'\n')
 out=ROOT/'.local/basketball-loading';out.mkdir(parents=True,exist_ok=True)
 (out/'runtime-oss.json').write_text(json.dumps(dict(status='verified-published',**pin),indent=2)+'\n')
 print(json.dumps(dict(status='verified-published',**pin)))
if __name__=='__main__':main()
