#!/usr/bin/env python3
"""Pin only verified public-object metadata; never copy media into Git."""
import argparse,importlib.util,json
from pathlib import Path
spec=importlib.util.spec_from_file_location('basketball_package',Path(__file__).with_name('package-assets.py'));pack=importlib.util.module_from_spec(spec);spec.loader.exec_module(pack)
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--package',required=True,type=Path);p.add_argument('--output',required=True,type=Path);args=p.parse_args()
 try:
  m=pack.verify_package(args.package);target=pack.inside_output(pack.ROOT,args.output)
  pin={key:m[key] for key in ['schema','game','sourceROM','contentSHA256','assetsBaseURL']};pin['entries']=[{key:e[key] for key in ['urlPath','sha256','bytes','mime']} for e in m['entries']]
  target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(pack.encoded(pin));print(json.dumps({'status':'pinned','objects':len(pin['entries']),'contentSHA256':pin['contentSHA256'],'bytes':target.stat().st_size}))
 except (ValueError,OSError,KeyError) as exc:p.exit(1,'Asset pin failed: '+str(exc)+'\n')
if __name__=='__main__':main()
