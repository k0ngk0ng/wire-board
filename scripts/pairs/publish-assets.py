#!/usr/bin/env python3
"""Publish immutable pairs media. Verify existing bytes; never overwrite objects."""
import base64,hashlib,hmac,json,urllib.request,urllib.error
from email.utils import formatdate
from pathlib import Path
root=Path(__file__).resolve().parents[2]
bucket='stoneage-assets';endpoint=f'https://{bucket}.oss-cn-shanghai.aliyuncs.com';prefix='bga/v20261002-1';cdn='https://cdn.ichenj.com/'+prefix
key=(root/'config/secrets/oss-access-key-id').read_text().strip();secret=(root/'config/secrets/oss-access-key-secret').read_text().strip()
manifest=json.loads((root/'games/pairs/assets/manifest.json').read_text());results=[]
def request(method,path,body=None,mime=''):
 date=formatdate(usegmt=True);md5=base64.b64encode(hashlib.md5(body).digest()).decode() if body else ''
 extra='x-oss-forbid-overwrite:true\n' if method=='PUT' else ''
 canonical=f'{method}\n{md5}\n{mime}\n{date}\n{extra}/{bucket}/{path}'
 signature=base64.b64encode(hmac.new(secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode()
 headers={'Date':date,'Authorization':f'OSS {key}:{signature}'}
 if method=='PUT':headers.update({'Content-MD5':md5,'Content-Type':mime,'x-oss-forbid-overwrite':'true','Cache-Control':'public, max-age=31536000, immutable'})
 return urllib.request.urlopen(urllib.request.Request(endpoint+'/'+path,data=body,method=method,headers=headers),timeout=40)
# Confirm the platform bucket using an existing asset before any mutation.
known=json.loads((root/'games/snooker/assets/manifest.json').read_text())['entries'][0]
with request('GET',prefix+known['urlPath']) as r:assert hashlib.sha256(r.read()).hexdigest()==known['sha256']
for e in manifest['entries']:
 path=prefix+e['urlPath'];data=(root/e['output']).read_bytes();assert hashlib.sha256(data).hexdigest()==e['sha256']
 mime={'.webp':'image/webp','.wav':'audio/wav','.mp3':'audio/mpeg'}[Path(e['output']).suffix]
 try:
  with request('GET',path) as r:assert hashlib.sha256(r.read()).hexdigest()==e['sha256'],'immutable collision '+path
 except urllib.error.HTTPError as error:
  if error.code!=404:raise
  with request('PUT',path,data,mime) as r:assert r.status==200
 with request('GET',path) as r:assert hashlib.sha256(r.read()).hexdigest()==e['sha256']
 with urllib.request.urlopen(cdn+e['urlPath'],timeout=40) as r:
  assert hashlib.sha256(r.read()).hexdigest()==e['sha256'];assert r.headers.get_content_type()==mime
 results.append({'path':path,'sha256':e['sha256'],'bytes':len(data),'cdnVerified':True});print('Verified',e['urlPath'],flush=True)
out=root/'.local/pairs/oss-readback.json';out.write_text(json.dumps({'passed':True,'objects':results},indent=2)+'\n');print('Verified',len(results),'immutable OSS objects')
