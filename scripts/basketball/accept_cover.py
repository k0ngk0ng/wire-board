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
OUT = ROOT / '.local/basketball/verification/cover-browser'
PUBLIC = OUT / 'public'
TEMP = ROOT / '.local/basketball/tmp/cover-browser'
SOCKETS = ROOT / '.local/basketball/tmp/c'
for directory in (PUBLIC, TEMP, SOCKETS):
    directory.mkdir(parents=True, exist_ok=True)

(PUBLIC / 'index.html').write_text('<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>热血篮球封面验收</title><style>body{margin:0;background:#f7f3e7;font-family:sans-serif}main{display:flex;gap:16px;flex-wrap:wrap;padding:8px}figure{margin:0}figure>div{width:304px;aspect-ratio:640/380}svg{display:block;width:100%;height:100%}figcaption{font-size:13px;padding:8px;color:#354839}</style><div id="root"></div><script type="module" src="/cover.js"></script></html>')
subprocess.run([
    str(ROOT / 'web/node_modules/.bin/esbuild'),
    str(ROOT / 'scripts/basketball/cover_harness.tsx'),
    '--bundle', '--format=esm', '--jsx=automatic',
    '--outfile=' + str(PUBLIC / 'cover.js'),
], cwd=ROOT, env=dict(os.environ, NODE_PATH=str(ROOT / 'web/node_modules')), check=True, capture_output=True)

requests = []


class Handler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *args):
        requests.append(format % args)


server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Handler, directory=str(PUBLIC)))
threading.Thread(target=server.serve_forever, daemon=True).start()
session = 'bc' + uuid.uuid4().hex[:4]
profile = TEMP / session
env = dict(os.environ, AGENT_BROWSER_SESSION=session,
           AGENT_BROWSER_SOCKET_DIR=str(SOCKETS), TMPDIR=str(TEMP),
           AGENT_BROWSER_NO_VERSION_CHECK='1')


def browser(*args):
    result = subprocess.run(['agent-browser', '--session', session, *args, '--json'],
                            cwd=ROOT, env=env, capture_output=True, text=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    result = json.loads(result.stdout)
    assert result['success'], result
    return result['data']


def js(code):
    return browser('eval', code)['result']


opened = False
report = {
    'passed': False,
    'checks': [],
    'sourceSHA256': hashlib.sha256((ROOT / 'games/basketball/web/Cover.tsx').read_bytes()).hexdigest(),
    'command': 'python3 -B scripts/basketball/accept_cover.py',
}
try:
    browser('--profile', str(profile), 'open', 'http://127.0.0.1:' + str(server.server_port))
    opened = True
    browser('wait', '[data-cover-case] svg')
    browser('set', 'viewport', '1000', '280')
    report['covers'] = js('Array.from(document.querySelectorAll("[data-cover-case]"), e => ({name:e.dataset.coverCase,label:e.querySelector("svg").getAttribute("aria-label"),viewBox:e.querySelector("svg").getAttribute("viewBox"),images:e.querySelectorAll("image,img,foreignObject").length,hrefs:e.querySelectorAll("[href],[src]").length}))')
    assert len(report['covers']) == 3, report
    assert all(cover['images'] == 0 and cover['hrefs'] == 0 for cover in report['covers']), report
    assert all(cover['label'].startswith('热血篮球') and cover['viewBox'] == '0 0 640 380' for cover in report['covers']), report
    report['checks'].append('all asset configurations render accessible SVG artwork without image dependencies')

    assert js('(()=>{const ids=Array.from(document.querySelectorAll("svg [id]"),e=>e.id);return new Set(ids).size===ids.length&&Array.from(document.querySelectorAll("svg [fill],svg [clip-path]")).every(e=>Array.from((e.getAttribute("fill")||e.getAttribute("clip-path")||"").matchAll(/url\\(#([^)]*)\\)/g)).every(m=>e.closest("svg").querySelector(`[id="${CSS.escape(m[1])}"]`)))})()')
    report['checks'].append('repeated covers have unique, locally resolved gradients and clip paths')
    rects = js('Object.fromEntries(Array.from(document.querySelectorAll("[data-cover-case]"),e=>[e.dataset.coverCase,e.getBoundingClientRect().toJSON()]))')
    browser('screenshot', str(OUT / 'desktop.png'))
    screenshot = Image.open(OUT / 'desktop.png').convert('RGB')
    crops = {name: screenshot.crop((round(r['x']), round(r['y']), round(r['right']), round(r['bottom']))) for name, r in rects.items()}
    differences = {}
    for name, image in crops.items():
        delta = ImageChops.difference(crops['local'], image)
        channels = delta.tobytes()
        changed = sum(any(channels[offset:offset + 3]) for offset in range(0, len(channels), 3))
        maximum = max(high for low, high in delta.getextrema())
        pixels = image.width * image.height
        differences[name] = {'changedPixels': changed, 'maximumChannelDelta': maximum, 'pixels': pixels}
        assert maximum <= 2 and changed * 1000 <= pixels, differences
    report['renderDifferences'] = differences
    report['checks'].append('default, missing-media and CDN configurations render identical artwork')

    browser('set', 'viewport', '320', '720')
    assert js('document.documentElement.scrollWidth<=innerWidth&&Array.from(document.querySelectorAll("[data-cover-case]")).every(e=>{const r=e.getBoundingClientRect();return r.width>0&&r.left>=0&&r.right<=innerWidth})')
    browser('screenshot', str(OUT / 'mobile-320.png'))
    report['checks'].append('all covers remain visible without horizontal overflow at 320px')
    browser('set', 'offline', 'on')
    assert js('document.querySelectorAll("[data-cover-case] svg").length===3')
    browser('screenshot', str(OUT / 'offline.png'))
    assert ImageChops.difference(Image.open(OUT / 'mobile-320.png'), Image.open(OUT / 'offline.png')).getbbox() is None
    report['checks'].append('rendered artwork remains unchanged offline')
    report['mediaRequests'] = js('performance.getEntriesByType("resource").filter(e=>e.initiatorType!=="script"&&!(new URL(e.name)).pathname.endsWith("favicon.ico")).map(e=>e.name)')
    assert report['mediaRequests'] == [], report['mediaRequests']
    assert not any('/missing-cover/' in row or '/heroes' in row for row in requests), requests
    report['checks'].append('no cover media requests or missing-image requests')
    report['passed'] = True
except BaseException as error:
    report['error'] = str(error)
    raise
finally:
    (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    if opened:
        browser('close')
        (SOCKETS / (session + '.target')).unlink(missing_ok=True)
    server.shutdown()
    server.server_close()
    shutil.rmtree(profile, ignore_errors=True)
    shutil.rmtree(PUBLIC, ignore_errors=True)

print(json.dumps(report, ensure_ascii=False))
