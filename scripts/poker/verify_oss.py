"""Read back every published poker object; check bytes, MIME and media signature."""
import concurrent.futures,hashlib,json,urllib.request
from pathlib import Path
import sys
root=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(root/"scripts"))
from resource_store import configuration, published_url

manifest=json.loads((root/'assets/poker/manifest.json').read_text());base=configuration()['CDN_ORIGIN']
def check(e):
 path=e['output'].removeprefix('web/public/');url=published_url(e,'_common',base)
 with urllib.request.urlopen(url,timeout=45) as r:data=r.read();mime=r.headers.get_content_type()
 digest=hashlib.sha256(data).hexdigest();assert digest==e['sha256'],url
 expected={'.ogg':'audio/ogg','.mp3':'audio/mpeg','.wav':'audio/wav'}.get(Path(path).suffix,'image/webp');assert mime==expected,(url,mime)
 if path.endswith('.mp3'):assert data[:3]==b'ID3' or (data[0]==255 and data[1]&224==224),url
 elif path.endswith('.ogg'):assert data[:4]==b'OggS',url
 else:assert data[:4]==b'RIFF' and data[8:12]==b'WEBP',url
 return dict(url=url,sha256=digest,mime=mime,bytes=len(data),passed=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:results=list(pool.map(check,manifest['entries']))
out=root/'.local/_common/poker/oss-readback.json';out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps({'passed':True,'objects':results},indent=2)+'\n');print('Verified',len(results),'objects:',out)
