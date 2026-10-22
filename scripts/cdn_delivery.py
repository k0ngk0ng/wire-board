#!/usr/bin/env python3
"""Publish immutable compiled assets, verify CDN responses, then trim packaging.
Only main's verified source uses credentials. Tags reuse the unchanged binary.
"""
import argparse,base64,concurrent.futures,gzip,hashlib,hmac,json,os,re,shutil
from email.utils import formatdate
from pathlib import Path
from urllib.request import Request,urlopen
from urllib.error import HTTPError
BASE=re.compile(r'https://cdn\.ichenj\.com/bga/web/[a-f0-9]{40}/\Z')
MIME={'.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.webp':'image/webp','.jpg':'image/jpeg','.woff':'font/woff','.woff2':'font/woff2','.ttf':'font/ttf','.wasm':'application/wasm'}
def sha(data):return hashlib.sha256(data).hexdigest()
def inventory(dist,base):
 if not BASE.fullmatch(base):raise ValueError('Expected immutable commit-bound CDN base')
 folder=dist/'assets'
 if folder.is_symlink():raise ValueError('Symlink asset directory')
 rows=[]
 candidates=sorted(folder.rglob('*'))
 if (dist/'favicon.svg').exists():candidates.append(dist/'favicon.svg')
 for p in candidates:
  if p.is_symlink():raise ValueError('Symlink in asset directory')
  if not p.is_file():continue
  if p.suffix=='.gz':continue # CDN negotiates compression; publish canonical bytes.
  if p.suffix not in MIME:raise ValueError('Unexpected public asset type: '+p.name)
  data=p.read_bytes();rel=p.relative_to(dist).as_posix()
  encoding='gzip' if p.suffix in ('.js','.css','.svg') else ''
  rows.append({'url':base+rel,'path':rel,'bytes':len(data),'sha256':sha(data),'mime':MIME[p.suffix],'encoding':encoding,'wireBytes':len(gzip.compress(data,mtime=0)) if encoding else len(data)})
 if not rows:raise ValueError('No compiled assets')
 return rows

def decoded(body,headers):
 if headers.get('Content-Encoding')=='gzip':
  try:return gzip.decompress(body)
  except (OSError,EOFError) as e:raise ValueError('Invalid gzip response') from e
 return body

def validate_response(row,body,headers):
 if row.get('encoding') and headers.get('Content-Encoding')!=row['encoding']:raise ValueError('CDN compression missing: '+row['url'])
 body=decoded(body,headers)
 mime=headers.get('Content-Type','').split(';')[0].lower()
 compatible={row['mime']}
 if row['mime']=='text/javascript':compatible.add('application/javascript')
 if len(body)!=row['bytes'] or sha(body)!=row['sha256']:raise ValueError('CDN content mismatch: '+row['url'])
 if mime not in compatible:raise ValueError('CDN MIME mismatch: '+row['url'])
 if headers.get('Access-Control-Allow-Origin') not in ('*','https://bga.ichenj.com'):raise ValueError('CDN CORS missing: '+row['url'])

def trim(dist,report):
 rows=report.get('files',[])
 if not report.get('passed') or not rows:raise ValueError('No verified CDN inventory')
 base=rows[0]['url'].split('/assets/')[0]+'/'
 if inventory(dist,base)!=rows:raise ValueError('Verified assets differ from packaging input')
 if 'manifestSHA256' in report and sha((dist/'game-load-manifest.json').read_bytes())!=report['manifestSHA256']:raise ValueError('Manifest changed after verification')
 if 'indexSHA256' in report and sha((dist/'index.html').read_bytes())!=report['indexSHA256']:raise ValueError('Entry changed after verification')
 shutil.rmtree(dist/'assets')

def credentials():
 def read(env,file):
  if os.environ.get(env):return os.environ[env]
  return (Path('config/secrets')/file).read_text().strip()
 return read('OSS_ACCESS_KEY_ID','oss-access-key-id'),read('OSS_ACCESS_KEY_SECRET','oss-access-key-secret')

def publish(dist,rows,bucket,endpoint):
 key_id,secret=credentials()
 def upload(row):
  key=row['url'].removeprefix('https://cdn.ichenj.com/')
  url='https://'+endpoint+'/'+key
  # Never replace an existing immutable object. Verify identical re-runs.
  date=formatdate(usegmt=True)
  canonical=f'GET\n\n\n{date}\n/{bucket}/{key}'
  signature=base64.b64encode(hmac.new(secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode()
  get=Request(url,headers={'Date':date,'Authorization':f'OSS {key_id}:{signature}'})
  try:
   with urlopen(get,timeout=45) as r:
    existing=decoded(r.read(),r.headers)
    if row.get('encoding') and r.headers.get('Content-Encoding')!=row['encoding']:raise ValueError('Immutable encoding differs: '+key)
   if sha(existing)!=row['sha256']:raise ValueError('Immutable object already differs: '+key)
   return
  except HTTPError as e:
   if e.code!=404:raise
  body=(dist/row['path']).read_bytes()
  if row['encoding']:body=gzip.compress(body,mtime=0)
  md5=base64.b64encode(hashlib.md5(body).digest()).decode();date=formatdate(usegmt=True)
  canonical=f'PUT\n{md5}\n{row["mime"]}\n{date}\nx-oss-forbid-overwrite:true\n/{bucket}/{key}'
  signature=base64.b64encode(hmac.new(secret.encode(),canonical.encode(),hashlib.sha1).digest()).decode()
  request=Request(url,data=body,method='PUT',headers={'Content-MD5':md5,'Content-Type':row['mime'],'Date':date,'x-oss-forbid-overwrite':'true','Cache-Control':'public, max-age=31536000, immutable','Authorization':f'OSS {key_id}:{signature}'})
  if row['encoding']:request.add_header('Content-Encoding',row['encoding'])
  with urlopen(request,timeout=45) as r:
   if r.status!=200:raise ValueError('Upload failed: '+key)
 with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:list(pool.map(upload,rows))

def verify(row):
 with urlopen(Request(row['url'],headers={'Origin':'https://bga.ichenj.com','Accept-Encoding':'gzip'}),timeout=60) as r:validate_response(row,r.read(),r.headers)
 return row

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('action',choices=['publish','verify','trim']);p.add_argument('--dist',type=Path,default=Path('web/dist'));p.add_argument('--report',type=Path,default=Path('.local/cdn-delivery/verified.json'));args=p.parse_args()
 if args.action=='trim':trim(args.dist,json.loads(args.report.read_text()));print('Removed verified compiled assets from server packaging');return
 manifest=json.loads((args.dist/'game-load-manifest.json').read_text());base=manifest.get('assetBase','');rows=inventory(args.dist,base)
 known={r['url']:r for r in rows}
 for files in manifest['games'].values():
  for f in files:
   if f['url'] not in known or f['bytes']!=known[f['url']]['bytes']:raise ValueError('Manifest does not match compiled assets')
 if manifest['entry'] not in known:raise ValueError('Entry is not bound to CDN inventory')
 args.report.parent.mkdir(parents=True,exist_ok=True);report={'passed':False,'files':rows,'indexSHA256':sha((args.dist/'index.html').read_bytes()),'manifestSHA256':sha((args.dist/'game-load-manifest.json').read_bytes())};args.report.write_text(json.dumps(report,indent=2)+'\n')
 if args.action=='publish':publish(args.dist,rows,'stoneage-assets','stoneage-assets.oss-cn-shanghai.aliyuncs.com')
 with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:list(pool.map(verify,rows))
 report['passed']=True;args.report.write_text(json.dumps(report,indent=2)+'\n');print(f'Verified {len(rows)} CDN assets; MIME, CORS and SHA-256 passed')
if __name__=='__main__':main()
