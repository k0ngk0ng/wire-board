import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[3]
CDN = os.environ.get('PAL_ENDING_CDN') == '1'
OUT = ROOT / ('.local/pal/cdn-ending-browser' if CDN else '.local/pal/ending-browser')
MEDIA = ROOT / '.local/pal/ending-media-e2e'
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), AGENT_BROWSER_PROFILE=str(OUT / 'profile'), AGENT_BROWSER_SCREENSHOT_DIR=str(OUT), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-ending'], cwd=ROOT, env=ENV, text=True).strip()

def browser(*args):
    result = subprocess.run(['agent-browser', '--session', SESSION, '--json', *map(str, args)], cwd=ROOT, env=ENV, check=True, capture_output=True, text=True)
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']

class Handler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self, path):
        if path.startswith('/pal/ending-v1/'):
            base, relative = (OUT, 'not-found') if CDN else (MEDIA, path[len('/pal/ending-v1/'):])
        else:
            base, relative = OUT, path.split('?', 1)[0].lstrip('/') or 'index.html'
        target = (base / relative).resolve()
        return str(target if target.is_relative_to(base) else OUT / 'not-found')

    def log_message(self, *args):
        pass

subprocess.run(['node', 'games/pal/tests/ending-render-fixture.mjs'], cwd=ROOT, env=ENV, check=True)
server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
report = {'passed': False, 'scope': 'lossless original AVI RGB rendering in actual PalStage; desktop and phone aspect sizing; independent FFmpeg decoded source reference; not the complete native story or platform audio', 'cdnOnlyFixture': CDN, 'cases': []}
device = None
try:
    browser('open', f'http://127.0.0.1:{server.server_port}')
    browser('wait', '--fn', 'window.endingReady===true')
    for width, height in [(1280, 900), (320, 820), (820, 320)]:
        if device:
            device.terminate()
            device.wait(timeout=5)
            device = None
        browser('set', 'viewport', width, height)
        if width != 1280:
            device = subprocess.Popen(['node', 'scripts/browser_device.mjs', browser('get', 'cdp-url')['cdpUrl'], f'http://127.0.0.1:{server.server_port}', str(width), str(height), '--keep'], cwd=ROOT, env=ENV, text=True, stdout=subprocess.PIPE)
            assert device.stdout.readline().strip() == 'ready'
        result = browser('eval', '''(async()=>{const pages=(await(await fetch('/reference.json')).json()).pages,c=document.querySelector('canvas'),hashes=[];for(const p of pages){await window.paintEnding(p.clip,p.frame);const pixels=c.getContext('2d').getImageData(0,0,320,200).data,hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',pixels))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==p.rgbaSHA256)throw Error(`mismatch ${p.clip}/${p.frame} ${hash}`);hashes.push(hash)}const b=c.getBoundingClientRect();return {frames:hashes.length,hashes,canvas:[b.width,b.height],center:[b.x+b.width/2,b.y+b.height/2],viewport:[innerWidth,innerHeight],touch:matchMedia('(pointer:coarse)').matches,overflow:document.documentElement.scrollWidth>innerWidth}})()''')['result']
        assert result['frames'] == 21 and not result['overflow'], result
        assert abs(result['canvas'][0] - min(width, height * 1.6)) <= 1, result
        assert abs(result['canvas'][0] / result['canvas'][1] - 1.6) < 0.001, result
        assert result['touch'] == (width != 1280), result
        assert abs(result['center'][0] - width / 2) < 1 and abs(result['center'][1] - height / 2) < 1, result
        browser('eval', 'window.paintEnding(5,900)')
        browser('screenshot', OUT / f'ending-{width}.png')
        report['cases'].append(result)
    report['passed'] = True
finally:
    if device:
        device.terminate()
        device.wait(timeout=5)
    try:
        browser('close')
    except Exception:
        pass
    server.shutdown()
    server.server_close()
    shutil.rmtree(OUT / 'profile', ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'passed': report['passed'], 'pixelComparisons': 63}))
