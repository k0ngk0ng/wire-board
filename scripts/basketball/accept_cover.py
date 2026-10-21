#!/usr/bin/env python3
"""Real React cover: CDN, local fallback, actual 404 recovery and rendered comparison."""
from pathlib import Path
import functools
import hashlib
import http.server
import json
import os
import shutil
import subprocess
import threading
import uuid
from PIL import Image, ImageChops
ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/basketball-cover-browser'
PUBLIC = OUT / 'public'
PUBLIC.mkdir(parents=True, exist_ok=True)
pin = json.loads((ROOT / 'games/basketball/assets/cover-manifest.json').read_text())
asset = ROOT / 'web/dist' / pin['localPath'].lstrip('/')
assert hashlib.sha256(asset.read_bytes()).hexdigest() == pin['sha256']
target = PUBLIC / pin['localPath'].lstrip('/')
target.parent.mkdir(parents=True, exist_ok=True)
shutil.copy2(asset, target)
(PUBLIC / 'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>热血篮球封面验收</title><style>body{margin:0;background:#f7f3e7;font-family:sans-serif}main{display:flex;gap:16px;flex-wrap:wrap;padding:8px}figure{margin:0}figure>div{width:304px;aspect-ratio:640/380}svg{display:block;width:100%;height:100%}figcaption{font-size:13px;padding:8px;color:#354839}</style><div id="root"></div><script type="module" src="/cover.js"></script>')
subprocess.run([str(ROOT / 'web/node_modules/.bin/esbuild'), str(ROOT / 'scripts/basketball/cover_harness.tsx'), '--bundle', '--format=esm', '--jsx=automatic', '--outfile='+str(PUBLIC/'cover.js')], cwd=ROOT, env=dict(os.environ, NODE_PATH=str(ROOT/'web/node_modules')), check=True, capture_output=True)
requests = []
class Handler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *args): requests.append(format % args)
server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Handler, directory=str(PUBLIC)))
threading.Thread(target=server.serve_forever, daemon=True).start()
env = dict(os.environ, AGENT_BROWSER_SESSION='bc'+uuid.uuid4().hex[:4], AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/cs'), TMPDIR=str(ROOT/'.local/tmp'))
def browser(*args):
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=ROOT, env=env, capture_output=True, text=True, timeout=45)
    assert result.returncode == 0, result.stdout+result.stderr
    result = json.loads(result.stdout); assert result['success'], result
    return result['data']
def js(code): return browser('eval', code)['result']
opened = False
report = {'passed':False,'checks':[],'sourceSHA256':hashlib.sha256((ROOT/'games/basketball/web/Cover.tsx').read_bytes()).hexdigest(),'imageSHA256':pin['sha256']}
try:
    browser('--profile', str(OUT/'profile'), 'open', 'http://127.0.0.1:'+str(server.server_port)); opened=True
    browser('set','viewport','1000','300')
    expected = {'local':pin['localPath'],'fallback':pin['localPath'],'cdn':pin['url']}
    ready = js('(async()=>{const expected='+json.dumps(expected)+';for(let n=0;n<200;n++){const ready=Object.entries(expected).every(([name,url])=>document.querySelector(`[data-cover-case="${name}"] image`)?.getAttribute("href")===url);if(ready){await Promise.all(Object.values(expected).map(src=>{const i=new Image();i.src=src;return i.decode()}));return true}await new Promise(r=>setTimeout(r,50))}throw Error("cover did not load")})()')
    assert ready
    assert any('/missing-cover/' in row and '404' in row for row in requests), requests
    report['checks'].append('actual missing remote image recovers to verified local bytes')
    rects = js('Object.fromEntries([...document.querySelectorAll("[data-cover-case]")].map(e=>[e.dataset.coverCase,e.getBoundingClientRect().toJSON()]))')
    browser('screenshot',str(OUT/'desktop.png'))
    screenshot=Image.open(OUT/'desktop.png').convert('RGB')
    crops={name:screenshot.crop((round(r['x']),round(r['y']),round(r['right']),round(r['bottom']))) for name,r in rects.items()}
    differences={}
    for name,image in crops.items():
        delta=ImageChops.difference(crops['local'],image)
        changed=sum(pixel!=(0,0,0) for pixel in delta.getdata())
        maximum=max(high for low,high in delta.getextrema())
        differences[name]={'changedPixels':changed,'maximumChannelDelta':maximum,'pixels':image.width*image.height}
        # Distinct image decode surfaces may round a few fractional edge pixels
        # differently. Record the actual difference; reject changed artwork.
        assert maximum<=2 and changed<=image.width*image.height*.001,differences
    report['renderDifferences']=differences
    report['checks'].append('CDN and fallback render the same artwork within recorded subpixel tolerance')
    browser('set','viewport','320','720')
    assert js('document.documentElement.scrollWidth<=innerWidth')
    browser('screenshot',str(OUT/'mobile-320.png'))
    report['checks'].append('all cover variants fit 320px')
    report['passed']=True
finally:
    (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    if opened: browser('close')
    server.shutdown();server.server_close()
print(json.dumps(report,ensure_ascii=False))
