#!/usr/bin/env python3
"""Publish only verified tabletennis/v1 media; never overwrite immutable bytes."""
from pathlib import Path
import argparse,base64,hashlib,hmac,json,os,subprocess,urllib.request,urllib.error
from email.utils import formatdate
ROOT=Path(__file__).resolve().parents[2];parser=argparse.ArgumentParser();parser.add_argument('--credentials-root',type=Path,default=ROOT);args=parser.parse_args()
subprocess.run(['python3','-B',str(ROOT/'scripts/tabletennis/verify-assets.py'),'--decode-audio'],check=True)
m=json.loads((ROOT/'games/tabletennis/assets/manifest.json').read_text());prefix='bga/v20261002-1';bucket='stoneage-assets';endpoint='https://stoneage-assets.oss-cn-shanghai.aliyuncs.com';assert m['assetsBaseURL']=='https://cdn.ichenj.com/'+prefix
key=os.environ.get('OSS_ACCESS_KEY_ID') or (args.credentials_root/'config/secrets/oss-access-key-id').read_text().strip();secret=os.environ.get('OSS_ACCESS_KEY_SECRET') or (args.credentials_root/'config/secrets/oss-access-key-secret').read_text().strip();sha=lambda b:hashlib.sha256(b).hexdigest()
def request(method,path,body=None,mime=''):
 date=formatdate(usegmt=True);md5=base64.b64encode(hashlib.md5(body).digest()).decode() if body else '';extra='x-oss-forbid-overwrite:true\n' if method=='PUT' else '';canonical=f'{method}\n{md5}\n{mime}\n{date}\n{extra}/{bucket}/{path}';signature=base64.b64encode(hmac.new(secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode();headers={'Date':date,'Authorization':f'OSS {key}:{signature}'}
 if method=='PUT':headers.update({'Content-MD5':md5,'Content-Type':mime,'x-oss-forbid-overwrite':'true','Cache-Control':'public, max-age=31536000, immutable'})
 return urllib.request.urlopen(urllib.request.Request(endpoint+'/'+path,data=body,method=method,headers=headers),timeout=40)
known=json.loads((ROOT/'games/snooker/assets/manifest.json').read_text())['entries'][0]
with request('GET',prefix+known['urlPath']) as r:assert sha(r.read())==known['sha256'],'platform asset bucket does not match'
results=[]
for row in m['entries']:
 assert row['urlPath'].startswith('/tabletennis/v1/') and '..' not in row['urlPath'];path=prefix+row['urlPath'];data=(ROOT/row['output']).read_bytes();assert sha(data)==row['sha256']
 try:
  with request('GET',path) as r:assert sha(r.read())==row['sha256'],'existing immutable object differs'
 except urllib.error.HTTPError as e:
  if e.code!=404:raise
  with request('PUT',path,data,row['mime']) as r:assert r.status==200
 with request('GET',path) as r:assert sha(r.read())==row['sha256']
 url=m['assetsBaseURL']+row['urlPath']
 with urllib.request.urlopen(urllib.request.Request(url,headers={'Origin':'https://bga.ichenj.com'}),timeout=40) as r:
  assert sha(r.read())==row['sha256'];assert r.headers.get_content_type()==row['mime'];assert r.headers.get('Access-Control-Allow-Origin') in ('*','https://bga.ichenj.com');assert 'max-age=31536000' in r.headers.get('Cache-Control','');headers={k:r.headers.get(k) for k in ['Content-Type','Cache-Control','Access-Control-Allow-Origin']}
 results.append({'url':url,'sha256':row['sha256'],'bytes':len(data),'headers':headers});print('Verified',row['urlPath'],flush=True)
out=ROOT/'.local/tabletennis/assets';out.mkdir(parents=True,exist_ok=True);(out/'oss-publish.json').write_text(json.dumps({'passed':True,'objects':results},indent=2)+'\n')
