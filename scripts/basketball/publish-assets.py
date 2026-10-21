#!/usr/bin/env python3
"""Plan by default; --execute publishes only verified immutable package bytes."""
import argparse,base64,hashlib,hmac,importlib.util,json,urllib.error,urllib.request
from email.utils import formatdate
from pathlib import Path
spec=importlib.util.spec_from_file_location('basketball_package',Path(__file__).with_name('package-assets.py'));pack=importlib.util.module_from_spec(spec);spec.loader.exec_module(pack)
ROOT=pack.ROOT
def publication_plan(package,root=ROOT):
 m=pack.verify_package(package,root);return {'status':'verified-plan','bucket':m['ossBucket'],'prefix':m['ossPrefix'],'assetsBaseURL':m['assetsBaseURL'],'objects':len(m['entries']),'bytes':sum(e['bytes'] for e in m['entries']),'contentSHA256':m['contentSHA256'],'writesPerformed':0}
def publish_verified(package,transport,root=ROOT):
 m=pack.verify_package(package,root);package=pack.inside_output(Path(root),package);pending=[]
 # Check every pre-existing object before the first PUT; no overwrite fallback.
 for e in m['entries']:
  path=m['ossPrefix']+e['urlPath'];data,mime=transport.get(path)
  if data is None:pending.append(e)
  else:pack.require(pack.sha(data)==e['sha256'] and mime==e['mime'],'immutable OSS collision: '+e['urlPath'])
 for e in pending:
  data=pack.read_under(package,e['output']);pack.require(pack.sha(data)==e['sha256'],'payload changed during publication')
  transport.put(m['ossPrefix']+e['urlPath'],data,e['mime'])
 results=[]
 for e in m['entries']:
  for cdn in (False,True):
   data,mime=transport.get(m['ossPrefix']+e['urlPath'],cdn=cdn);pack.require(data is not None and pack.sha(data)==e['sha256'] and mime==e['mime'],'OSS/CDN readback differs: '+e['urlPath'])
  results.append({'urlPath':e['urlPath'],'sha256':e['sha256'],'bytes':e['bytes'],'cdnVerified':True})
 return {'status':'verified-published','contentSHA256':m['contentSHA256'],'writesPerformed':len(pending),'objects':results}
class OSS:
 def __init__(self,root):
  self.bucket='stoneage-assets';self.endpoint='https://stoneage-assets.oss-cn-shanghai.aliyuncs.com';self.key=(root/'config/secrets/oss-access-key-id').read_text().strip();self.secret=(root/'config/secrets/oss-access-key-secret').read_text().strip()
  pack.require(bool(self.key and self.secret),'OSS credentials missing')
 def request(self,method,path,body=None,mime=''):
  pack.safe_relative(path);date=formatdate(usegmt=True);md5=base64.b64encode(hashlib.md5(body).digest()).decode() if body is not None else ''
  extra='x-oss-forbid-overwrite:true\n' if method=='PUT' else '';canonical=f'{method}\n{md5}\n{mime}\n{date}\n{extra}/{self.bucket}/{path}'
  signature=base64.b64encode(hmac.new(self.secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode();headers={'Date':date,'Authorization':f'OSS {self.key}:{signature}'}
  if method=='PUT':headers.update({'Content-MD5':md5,'Content-Type':mime,'x-oss-forbid-overwrite':'true','Cache-Control':'public, max-age=31536000, immutable'})
  return urllib.request.urlopen(urllib.request.Request(self.endpoint+'/'+path,data=body,method=method,headers=headers),timeout=40)
 def get(self,path,cdn=False):
  pack.safe_relative(path)
  try:
   response=urllib.request.urlopen('https://cdn.ichenj.com/'+path,timeout=40) if cdn else self.request('GET',path)
   with response:return response.read(),response.headers.get_content_type()
  except urllib.error.HTTPError as exc:
   if exc.code==404:return None,None
   raise RuntimeError(f'asset read failed HTTP {exc.code}') from None
 def put(self,path,data,mime):
  try:
   with self.request('PUT',path,data,mime) as r:pack.require(r.status==200,'OSS PUT failed')
  except urllib.error.HTTPError as exc:raise RuntimeError(f'immutable asset write failed HTTP {exc.code}') from None
 def verify_bucket(self,root):
  known=json.loads((root/'games/snooker/assets/manifest.json').read_text());e=known['entries'][0];data,_=self.get('bga/v20261002-1'+e['urlPath']);pack.require(data is not None and pack.sha(data)==e['sha256'],'platform bucket anchor differs')
def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--package',required=True,type=Path);ap.add_argument('--execute',action='store_true');ap.add_argument('--report',type=Path);args=ap.parse_args()
 try:
  plan=publication_plan(args.package)
  if not args.execute:print(json.dumps(plan));return
  pack.require(args.report is not None,'--execute requires a worktree-local --report path')
  report=pack.inside_output(ROOT,args.report,new=True);client=OSS(ROOT);client.verify_bucket(ROOT)
  result=publish_verified(args.package,client);report.parent.mkdir(parents=True,exist_ok=True);report.write_bytes(pack.encoded(result));print(json.dumps({'status':result['status'],'writesPerformed':result['writesPerformed'],'report':str(report)}))
 except Exception as exc:
  # Never print request headers, credentials, or remote response bodies.
  ap.exit(1,'Publication failed: '+(str(exc) if isinstance(exc,(ValueError,RuntimeError)) else type(exc).__name__)+'\n')
if __name__=='__main__':main()
