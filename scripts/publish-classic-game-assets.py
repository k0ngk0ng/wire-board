#!/usr/bin/env python3
"""Preflight/publish the four verified classic-game v4 manifests immutably.

Default is offline preflight. --publish binds to a known bucket object before
any writes. Every PUT forbids overwrite; OSS and CDN are read back. Credentials
are read only from config/secrets and never included in reports or diagnostics.
Failure inventory/tests: scripts/test_publish_classic_game_assets.py.
"""
from __future__ import annotations
import argparse
import base64
from concurrent.futures import ThreadPoolExecutor, as_completed, CancelledError
from email.utils import formatdate
import hashlib
import hmac
import json
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
import zlib

ROOT=Path(__file__).resolve().parents[1]
BUCKET='stoneage-assets'
ENDPOINT='https://stoneage-assets.oss-cn-shanghai.aliyuncs.com'
PREFIX='bga/v20261002-1'
CDN='https://cdn.ichenj.com/'+PREFIX
BUBBLE_MANIFESTS=[
 ('games/bubbleparty/assets/manifest-classic-v4.json','bubbleparty','/bubbleparty/v4/classic/',478),
 ('games/bubbleparty/assets/manifest-characters-v4.json','bubbleparty','/bubbleparty/v4/characters/',120),
 ('games/bubbleparty/assets/manifest-mounts-v4.json','bubbleparty','/bubbleparty/v4/mounts/',120),
]
MANIFESTS=BUBBLE_MANIFESTS+[
 ('games/fortressblue/assets/manifest-v4.json','fortressblue','/fortressblue/v4/',2),
]
sha=lambda data:hashlib.sha256(data).hexdigest()


def check(condition,message):
 if not condition:raise ValueError(message)


def media_dimensions(data,mime):
 if mime=='image/webp':
  check(len(data)>=16 and data[:4]==b'RIFF' and data[8:12]==b'WEBP' and struct.unpack_from('<I',data,4)[0]+8==len(data),'invalid WebP container')
  return None
 check(data[:8]==b'\x89PNG\r\n\x1a\n','invalid PNG signature')
 offset=8;dimensions=None;ended=False
 while offset<len(data):
  check(offset+12<=len(data),'truncated PNG chunk')
  size=struct.unpack_from('>I',data,offset)[0];end=offset+12+size
  check(end<=len(data),'truncated PNG payload')
  kind=data[offset+4:offset+8];payload=data[offset+8:end-4]
  check(zlib.crc32(kind+payload)&0xffffffff==struct.unpack_from('>I',data,end-4)[0],'PNG CRC mismatch')
  if kind==b'IHDR':
   check(offset==8 and size==13,'invalid PNG IHDR')
   dimensions=struct.unpack_from('>II',payload);check(min(dimensions)>0,'empty PNG dimensions')
  if kind==b'IEND':
   check(size==0 and end==len(data),'invalid PNG end');ended=True
  offset=end
 check(dimensions is not None and ended,'incomplete PNG')
 return dimensions


def validate_entry(root,game,entry,dist=False):
 root=Path(root).resolve();url=entry.get('urlPath','')
 check(isinstance(url,str) and url.startswith(f'/{game}/v4/'),'unexpected immutable object path')
 check(not any(char in url for char in ['\\','?','#','%']) and '..' not in PurePosixPath(url).parts and '//' not in url,'unsafe object path')
 check(isinstance(entry.get('bytes'),int) and entry['bytes']>0,'invalid expected bytes: '+url)
 check(bool(re.fullmatch('[0-9a-f]{64}',str(entry.get('sha256','')))),'invalid expected SHA: '+url)
 public=(root/'games'/game/'assets/public').resolve();file=public/url.lstrip('/')
 check(file.resolve().is_relative_to(public),'asset symlink escapes public tree: '+url)
 if 'output' in entry:check((root/entry['output']).resolve()==file.resolve(),'output disagrees with public URL: '+url)
 mime={'.png':'image/png','.webp':'image/webp'}.get(file.suffix)
 check(mime is not None and entry.get('mime',mime)==mime,'unsupported or mismatched MIME: '+url)
 data=file.read_bytes();verify_bytes(data,entry,url)
 size=media_dimensions(data,mime)
 if size and 'width' in entry:check(tuple(size)==(entry['width'],entry['height']),'dimensions differ: '+url)
 if size:
  for frame in entry.get('framesRaw',[]):
   x,y,right,bottom=frame[:4];check(0<=x<right<=size[0] and 0<=y<bottom<=size[1],'frame outside sheet: '+url)
 if dist:
  packaged=(root/'web/dist'/url.lstrip('/')).resolve()
  check(packaged.is_relative_to(root/'web/dist'),'built fallback escapes dist: '+url)
  verify_bytes(packaged.read_bytes(),entry,'built fallback '+url)
 return {**entry,'mime':mime,'data':data}


def ensure_unique(entries):
 paths=[entry['urlPath']for entry in entries]
 check(len(paths)==len(set(paths)),'duplicate immutable object paths')


def load_entries(root=ROOT,manifests=MANIFESTS,dist=False):
 entries=[];bindings=[]
 for relative,game,prefix,count in manifests:
  path=root/relative;raw=path.read_bytes();manifest=json.loads(raw)
  check(manifest.get('assetsBaseURL',CDN)==CDN,'unexpected CDN base in '+relative)
  check(len(manifest['entries'])==count,'unexpected entry count in '+relative)
  generator=manifest.get('generator');expected=manifest.get('generatorSha256',manifest.get('generatorSHA256'))
  if generator and expected:check(sha((root/generator).read_bytes())==expected,'generator SHA differs: '+relative)
  for entry in manifest['entries']:
   check(entry['urlPath'].startswith(prefix),'entry outside manifest scope: '+relative)
   entries.append(validate_entry(root,game,entry,dist))
  bindings.append({'path':relative,'sha256':sha(raw),'entries':count})
 ensure_unique(entries)
 return entries,bindings


def verify_bytes(data,entry,label):
 check(len(data)==entry['bytes'],'bytes mismatch: '+label)
 check(sha(data)==entry['sha256'],'SHA mismatch: '+label)


def oss_headers(method,path,body,mime,key,secret,date):
 md5=base64.b64encode(hashlib.md5(body).digest()).decode() if body is not None else ''
 extra='x-oss-forbid-overwrite:true\n' if method=='PUT' else ''
 canonical=f'{method}\n{md5}\n{mime}\n{date}\n{extra}/{BUCKET}/{path}'
 signature=base64.b64encode(hmac.new(secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode()
 headers={'Date':date,'Authorization':f'OSS {key}:{signature}'}
 if method=='PUT':headers.update({'Content-MD5':md5,'Content-Type':mime,'x-oss-forbid-overwrite':'true','Cache-Control':'public, max-age=31536000, immutable'})
 return headers


class OSSClient:
 def __init__(self,key,secret):self.key=key;self.secret=secret
 def _request(self,method,path,body=None,mime='',cdn=False):
  for attempt in range(5):
   try:
    url=path if cdn else ENDPOINT+'/'+path
    headers={} if cdn else oss_headers(method,path,body,mime,self.key,self.secret,formatdate(usegmt=True))
    with urllib.request.urlopen(urllib.request.Request(url,data=body,method=method,headers=headers),timeout=40)as response:
     return response.read(),response.headers.get_content_type()
   except urllib.error.HTTPError as error:
    error.close()
    if error.code not in ({404,429,500,502,503,504}if cdn else {429,500,502,503,504})or attempt==4:raise
   except (urllib.error.URLError,TimeoutError,OSError):
    if attempt==4:raise
   time.sleep(min(2**attempt,8))
 def read_oss(self,path):return self._request('GET',path)
 def create(self,path,data,mime):self._request('PUT',path,data,mime)
 def read_cdn(self,url):return self._request('GET',url,cdn=True)


def verify_anchor(client,path,expected):
 data,_=client.read_oss(path)
 check(sha(data)==expected,'known platform bucket/prefix anchor differs')
 return {'path':path,'sha256':expected,'verified':True}


def publish_one(client,entry):
 path=PREFIX+entry['urlPath'];status='existing'
 try:
  existing,mime=client.read_oss(path);verify_bytes(existing,entry,'existing '+path)
  check(mime==entry['mime'],'existing OSS MIME mismatch: '+path)
 except urllib.error.HTTPError as error:
  error.close()
  if error.code!=404:raise
  try:client.create(path,entry['data'],entry['mime']);status='created'
  except urllib.error.HTTPError as conflict:
   conflict.close()
   if conflict.code!=409:raise
   status='existing-after-create-race'
 # A successful PUT or a concurrent creation is never trusted without readback.
 data,mime=client.read_oss(path);verify_bytes(data,entry,'OSS '+path)
 check(mime==entry['mime'],'OSS MIME mismatch: '+path)
 data,mime=client.read_cdn(CDN+entry['urlPath']);verify_bytes(data,entry,'CDN '+path)
 check(mime==entry['mime'],'CDN MIME mismatch: '+path)
 return {'path':path,'urlPath':entry['urlPath'],'sha256':entry['sha256'],'bytes':entry['bytes'],'mime':entry['mime'],'status':status,'ossVerified':True,'cdnVerified':True}


def safe_error(error):
 if isinstance(error,urllib.error.HTTPError):return 'HTTP '+str(error.code)
 if isinstance(error,ValueError):return str(error)
 return type(error).__name__


def publish_entries(client,entries,workers=4,progress=None):
 check(1<=workers<=8,'workers must be 1..8')
 results=[];errors=[];stop=threading.Event()
 def run(entry):
  if stop.is_set():raise CancelledError()
  return publish_one(client,entry)
 with ThreadPoolExecutor(max_workers=workers)as pool:
  pending={pool.submit(run,entry):entry for entry in entries}
  for future in as_completed(pending):
   try:results.append(future.result())
   except CancelledError:continue
   except Exception as error:
    errors.append({'urlPath':pending[future]['urlPath'],'error':safe_error(error)})
    stop.set()
    for queued in pending:queued.cancel()
   if progress:progress(results,errors)
 return sorted(results,key=lambda item:item['path']),errors


def write_report(path,report):
 path.parent.mkdir(parents=True,exist_ok=True)
 temporary=path.with_suffix('.json.tmp');temporary.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');temporary.replace(path)


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--publish',action='store_true');parser.add_argument('--dist',action='store_true');parser.add_argument('--workers',type=int,default=4);args=parser.parse_args()
 target=ROOT/'.local/classic-game-assets'/('oss-publish.json'if args.publish else 'preflight.json')
 report={'passed':False,'mode':'publish'if args.publish else 'preflight','bucket':BUCKET,'prefix':PREFIX,'assetsBaseURL':CDN,'distVerified':args.dist,'workers':args.workers,'objects':[],'errors':[]}
 try:
  check(1<=args.workers<=8,'workers must be 1..8')
  entries,bindings=load_entries(dist=args.dist)
  report.update(manifests=bindings,expectedObjects=len(entries),expectedBytes=sum(e['bytes']for e in entries))
  # Existing specialized verifiers also check native sprite frames/catalogs.
  subprocess.run([sys.executable,'-B',str(ROOT/'scripts/bubbleparty/verify-assets.py')]+(['--dist']if args.dist else []),cwd=ROOT,check=True)
  subprocess.run([sys.executable,'-B',str(ROOT/'scripts/fortressblue/verify-supply-assets.py')]+(['--dist']if args.dist else []),cwd=ROOT,check=True)
  report['localVerified']=True;write_report(target,report)
  print(f'Preflight verified {len(entries)} objects / {report["expectedBytes"]} bytes',flush=True)
  if args.publish:
   key=(ROOT/'config/secrets/oss-access-key-id').read_text().strip();secret=(ROOT/'config/secrets/oss-access-key-secret').read_text().strip()
   check(bool(key and secret),'missing OSS credential material')
   client=OSSClient(key,secret)
   known=json.loads((ROOT/'games/snooker/assets/manifest.json').read_text())['entries'][0]
   report['anchor']=verify_anchor(client,PREFIX+known['urlPath'],known['sha256'])
   def progress(results,errors):
    report.update(objects=results,errors=errors);write_report(target,report)
    if len(results)%25==0 or errors:print(f'Verified {len(results)}/{len(entries)} objects; errors={len(errors)}',flush=True)
   results,errors=publish_entries(client,entries,args.workers,progress)
   report.update(objects=results,errors=errors)
   check(not errors and len(results)==len(entries),'publication incomplete; see per-object report')
  report['passed']=True
 except Exception as error:
  report['failure']=safe_error(error)
  print('FAILED:',report['failure'],file=sys.stderr)
 finally:write_report(target,report)
 print(target,flush=True)
 return 0 if report['passed']else 1

if __name__=='__main__':raise SystemExit(main())
