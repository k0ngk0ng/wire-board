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
CONTRA_MIME={'.js':'text/javascript','.wasm':'application/wasm','.json':'application/json','.idx':'application/octet-stream','.rgb':'application/octet-stream','.wav':'audio/wav'}
SHA256=re.compile(r'[a-f0-9]{64}\Z')
def sha(data):return hashlib.sha256(data).hexdigest()
def contra_inventory(dist,base):
 folder,receipt_path=dist/'contra',dist/'contra-delivery.json'
 if folder.is_symlink() or receipt_path.is_symlink():raise ValueError('Symlink Contra delivery root or receipt')
 if not folder.exists() and not receipt_path.exists():return []
 if not folder.is_dir() or not receipt_path.is_file():raise ValueError('Contra runtime requires its delivery directory and receipt')
 try:receipt=json.loads(receipt_path.read_bytes())
 except (ValueError,OSError) as e:raise ValueError('Invalid Contra delivery receipt') from e
 if not isinstance(receipt,dict):raise ValueError('Invalid Contra delivery receipt')
 runtime,rules=receipt.get('runtimeBuildID'),receipt.get('rulesBuildID')
 if not isinstance(runtime,str) or not SHA256.fullmatch(runtime) or not isinstance(rules,str) or not SHA256.fullmatch(rules):raise ValueError('Missing canonical Contra build identities')
 prefix='contra/runtime-'+runtime+'/'
 if receipt.get('prefix')!=prefix:raise ValueError('Contra prefix differs from runtime identity')
 files=receipt.get('files')
 if not isinstance(files,dict) or not files or 'manifest.json' not in files:raise ValueError('Incomplete Contra delivery receipt')
 expected={}
 for name,item in files.items():
  path=Path(name)
  if not re.fullmatch(r'[A-Za-z0-9._/-]+',name) or path.is_absolute() or path.as_posix()!=name or '..' in path.parts or path.suffix not in CONTRA_MIME:raise ValueError('Noncanonical Contra payload path: '+name)
  if not isinstance(item,dict) or type(item.get('bytes')) is not int or item['bytes']<=0 or not isinstance(item.get('sha256'),str) or not SHA256.fullmatch(item['sha256']):raise ValueError('Invalid Contra payload identity: '+name)
  if item.get('mime')!=CONTRA_MIME[path.suffix] or type(item.get('requiresRange')) is not bool or item['requiresRange']!=(path.suffix=='.wav'):raise ValueError('Invalid Contra MIME or Range contract: '+name)
  expected[prefix+name]=item
 actual=set()
 for path in folder.rglob('*'):
  if path.is_symlink():raise ValueError('Symlink in Contra delivery directory')
  if path.is_dir():continue
  if not path.is_file():raise ValueError('Contra payload must be a regular file')
  actual.add(path.relative_to(dist).as_posix())
 if actual!=set(expected):raise ValueError('Contra files do not exactly match the delivery receipt')
 rows=[]
 for rel,item in sorted(expected.items()):
  body=(dist/rel).read_bytes()
  if len(body)!=item['bytes'] or sha(body)!=item['sha256']:raise ValueError('Contra payload checksum differs: '+rel)
  encoding='gzip' if Path(rel).suffix=='.js' else ''
  rows.append({'url':base+rel,'path':rel,'bytes':len(body),'sha256':item['sha256'],'mime':item['mime'],'encoding':encoding,'wireBytes':len(gzip.compress(body,mtime=0)) if encoding else len(body),'requiresRange':item['requiresRange']})
 if files['manifest.json']['sha256']!=runtime:raise ValueError('Contra manifest checksum differs from runtime identity')
 try:manifest=json.loads((dist/prefix/'manifest.json').read_bytes())
 except ValueError as e:raise ValueError('Invalid Contra runtime manifest') from e
 if not isinstance(manifest,dict) or manifest.get('version')!=1 or manifest.get('rulesBuildID')!=rules:raise ValueError('Contra runtime rules identity differs from receipt')
 return rows

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
 rows.extend(contra_inventory(dist,base))
 if not rows:raise ValueError('No compiled assets')
 return rows

def decoded(body,headers):
 if headers.get('Content-Encoding')=='gzip':
  try:return gzip.decompress(body)
  except (OSError,EOFError) as e:raise ValueError('Invalid gzip response') from e
 return body

def validate_headers(row,headers):
 mime=headers.get('Content-Type','').split(';')[0].lower()
 compatible={row['mime']}
 if row['mime']=='text/javascript':compatible.add('application/javascript')
 if mime not in compatible:raise ValueError('CDN MIME mismatch: '+row['url'])
 if headers.get('Access-Control-Allow-Origin') not in ('*','https://bga.ichenj.com'):raise ValueError('CDN CORS missing: '+row['url'])

def validate_response(row,body,headers):
 if row.get('encoding') and headers.get('Content-Encoding')!=row['encoding']:raise ValueError('CDN compression missing: '+row['url'])
 if row.get('requiresRange') and headers.get('Content-Encoding') not in (None,'','identity'):raise ValueError('Seekable audio must use identity encoding: '+row['url'])
 body=decoded(body,headers)
 if len(body)!=row['bytes'] or sha(body)!=row['sha256']:raise ValueError('CDN content mismatch: '+row['url'])
 validate_headers(row,headers)

def validate_range_response(row,expected,body,headers,status):
 end=len(expected)-1
 if status!=206 or headers.get('Content-Range')!=f'bytes 0-{end}/{row["bytes"]}':raise ValueError('CDN did not honor the audio byte range: '+row['url'])
 if headers.get('Content-Encoding') not in (None,'','identity') or body!=expected:raise ValueError('CDN audio range bytes differ: '+row['url'])
 validate_headers(row,headers)

def trim(dist,report):
 rows=report.get('files',[])
 if not report.get('passed') or not rows:raise ValueError('No verified CDN inventory')
 first=rows[0]
 if not first.get('path') or not first['url'].endswith(first['path']):raise ValueError('Invalid verified asset URL')
 base=first['url'][:-len(first['path'])]
 if inventory(dist,base)!=rows:raise ValueError('Verified assets differ from packaging input')
 if 'manifestSHA256' in report and sha((dist/'game-load-manifest.json').read_bytes())!=report['manifestSHA256']:raise ValueError('Manifest changed after verification')
 if 'indexSHA256' in report and sha((dist/'index.html').read_bytes())!=report['indexSHA256']:raise ValueError('Entry changed after verification')
 contra_prefix=None
 receipt_path=dist/'contra-delivery.json'
 if receipt_path.exists():
  raw=receipt_path.read_bytes()
  if report.get('contraDeliverySHA256')!=sha(raw):raise ValueError('Contra delivery receipt changed or was not verified')
  contra_prefix=json.loads(raw)['prefix']
 elif report.get('contraDeliverySHA256'):raise ValueError('Verified Contra delivery receipt is missing')
 if (dist/'assets').exists():shutil.rmtree(dist/'assets')
 if contra_prefix:shutil.rmtree(dist/contra_prefix)

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
 with urlopen(Request(row['url'],headers={'Origin':'https://bga.ichenj.com','Accept-Encoding':'identity' if row.get('requiresRange') else 'gzip'}),timeout=60) as r:
  body=r.read();validate_response(row,body,r.headers)
  if row.get('requiresRange'):expected=body[:64]
 if row.get('requiresRange'):
  with urlopen(Request(row['url'],headers={'Origin':'https://bga.ichenj.com','Accept-Encoding':'identity','Range':f'bytes=0-{len(expected)-1}'}),timeout=60) as r:
   validate_range_response(row,expected,r.read(),r.headers,r.status)
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
 args.report.parent.mkdir(parents=True,exist_ok=True);report={'passed':False,'files':rows,'indexSHA256':sha((args.dist/'index.html').read_bytes()),'manifestSHA256':sha((args.dist/'game-load-manifest.json').read_bytes())}
 receipt=args.dist/'contra-delivery.json'
 if receipt.exists():report['contraDeliverySHA256']=sha(receipt.read_bytes())
 args.report.write_text(json.dumps(report,indent=2)+'\n')
 if args.action=='publish':publish(args.dist,rows,'stoneage-assets','stoneage-assets.oss-cn-shanghai.aliyuncs.com')
 with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:list(pool.map(verify,rows))
 if inventory(args.dist,base)!=rows or receipt.exists() and sha(receipt.read_bytes())!=report.get('contraDeliverySHA256'):raise ValueError('Local assets changed during CDN verification')
 report['passed']=True;args.report.write_text(json.dumps(report,indent=2)+'\n');print(f'Verified {len(rows)} CDN assets; MIME, CORS and SHA-256 passed')
if __name__=='__main__':main()
