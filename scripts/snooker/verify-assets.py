#!/usr/bin/env python3
"""Validate originals, generated assets and (optionally) every immutable OSS object."""
from pathlib import Path
import argparse,concurrent.futures,hashlib,json,urllib.request,subprocess,array,math
root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--oss',action='store_true');parser.add_argument('--decode-audio',action='store_true');parser.add_argument('--dist',action='store_true');args=parser.parse_args()
sha=lambda b:hashlib.sha256(b).hexdigest()
source=json.loads((root/'games/snooker/assets/source-manifest.json').read_text())
for e in source['entries']:assert sha((root/e['path']).read_bytes())==e['sha256'],e['path']
assert sha((root/source['licenseFile']).read_bytes())==source['licenseSHA256']
manifest=json.loads((root/'games/snooker/assets/manifest.json').read_text());base='https://cdn.ichenj.com/bga/v20261002-1'
def check(e):
 assert (root/e['output']).relative_to(root/'games/snooker/assets/public').as_posix()==e['urlPath'].lstrip('/'),'wrong public directory'
 data=(root/e['output']).read_bytes();assert sha(data)==e['sha256'];assert sha((root/e['source']).read_bytes())==e['sourceSHA256'];mime='image/webp' if e['type']=='image' else 'audio/mpeg'
 if mime=='image/webp':assert data[:4]==b'RIFF' and data[8:12]==b'WEBP'
 else:assert data[:3]==b'ID3' or (data[0]==255 and data[1]&224==224)
 result={'path':e['urlPath'],'sha256':sha(data),'bytes':len(data),'mime':mime,'passed':True}
 if args.dist:assert sha((root/'web/dist'/e['urlPath'].lstrip('/')).read_bytes())==e['sha256'],'asset missing or changed in built frontend'
 if args.decode_audio and e['type']=='audio':
  decoded=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(root/e['output']),'-f','f32le','-ac','1','-ar','44100','-'],check=True,capture_output=True)
  samples=array.array('f');samples.frombytes(decoded.stdout);peak=max(abs(v) for v in samples);rms=math.sqrt(sum(v*v for v in samples)/len(samples));assert .005<peak<.99 and rms>.0001,(e['name'],peak,rms)
  result.update(seconds=len(samples)/44100,peak=peak,rms=rms)
 if args.oss:
  url=base+e['urlPath']
  with urllib.request.urlopen(url,timeout=40) as r:download=r.read();content_type=r.headers.get_content_type()
  assert sha(download)==e['sha256'],url
  assert content_type==mime,(url,content_type)
  result['url']=url
 return result
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:results=list(pool.map(check,manifest['entries']))
out=root/'.local/snooker/verification';out.mkdir(parents=True,exist_ok=True)
file=out/('oss-readback.json' if args.oss else 'audio-levels.json' if args.decode_audio else 'local-assets.json');file.write_text(json.dumps({'passed':True,'sourceLicense':'CC0-1.0; project originals separately marked','objects':results},indent=2)+'\n');print('Verified',len(results),'objects:',file)
