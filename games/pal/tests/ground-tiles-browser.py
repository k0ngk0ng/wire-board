import base64
import functools
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[3]
PAN = os.environ.get('PAL_GROUND_PAN') == '1'
OUT = ROOT / '.local/pal/verification' / ('ground-pan' if PAN else 'ground-tiles')
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pab'), AGENT_BROWSER_PROFILE=str(OUT / 'profile'), AGENT_BROWSER_SCREENSHOT_DIR=str(OUT), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-ground'], cwd=ROOT, env=ENV, text=True).strip()
REPORT = {'passed': False, 'scope': 'desktop and 320px actual Canvas map layers and RLE actor/cover composition versus pinned original C at genuine scene cameras, including same-frame field prediction; no wave or independent native draw-projection equivalence claim', 'cases': []}
if PAN:
    REPORT['scope'] = 'desktop and 320px actual Canvas map camera and boundary fixtures plus original C inn wave, prediction and immutable seed fixtures; no user issue localization or whole-story claim'

def browser(*values):
    result = subprocess.run(['agent-browser', '--session', SESSION, *map(str, values), '--json'], cwd=ROOT, env=ENV, capture_output=True, text=True, timeout=60)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']

def evaluate(code):
    return browser('eval', code)['result']

(OUT / 'entry.ts').write_text('''import {SceneRenderer} from "../../../../games/pal/web/scene-renderer";
const renderer=new SceneRenderer('/pal/v1');
const canvas=document.querySelector('canvas');
const reference=await(await fetch('/reference.json')).json();
const bytes=new Uint8Array(await(await fetch('/original-pages.rgba')).arrayBuffer());
window.paintGround=async(index)=>{
const page=reference.pages[index];
await renderer.paint(canvas,page.view,()=>true,page.predicted);
const actual=canvas.getContext('2d').getImageData(0,0,320,200).data;
const hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',actual))].map(x=>x.toString(16).padStart(2,'0')).join('');
const expected=bytes.subarray(index*256000,(index+1)*256000);
const different=[];
for(let p=0;p<64000;p++)if(actual.subarray(p*4,p*4+4).some((v,c)=>v!==expected[p*4+c]))different.push(p);
return {label:page.label,map:page.view.map,hash,expected:page.rgbaSHA256,differentPixels:different.length,bounds:different.length?[Math.min(...different.map(p=>p%320)),Math.min(...different.map(p=>p/320|0)),Math.max(...different.map(p=>p%320)),Math.max(...different.map(p=>p/320|0))]:null};
};
document.body.dataset.ready='true';
''')
subprocess.run(['node', '--input-type=module', '-e', "import {build} from 'esbuild';await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:'esm',target:'es2022'});", str(OUT / 'entry.ts'), str(OUT / 'entry.js')], cwd=ROOT / 'web', env=ENV, check=True)
(OUT / 'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body{margin:0;width:100%;height:100%;background:#101510;color:#eff4ec}body{display:grid;place-items:center}canvas{width:min(100vw,160dvh);height:auto;image-rendering:pixelated}</style><canvas width="320" height="200"></canvas><script type="module" src="/entry.js"></script>')

class Handler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self, name):
        if name.startswith('/pal/v1/'):
            return str(ROOT / '.local/pal/visual/pal/v1' / name.removeprefix('/pal/v1/').split('?')[0])
        return super().translate_path(name)

    def log_message(self, *values):
        pass

server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(Handler, directory=str(OUT)))
threading.Thread(target=server.serve_forever, daemon=True).start()
try:
    browser('open', 'http://127.0.0.1:' + str(server.server_port))
    browser('wait', '--fn', 'document.body.dataset.ready==="true"')
    reference = json.loads((OUT / 'reference.json').read_text())
    for width, height in [(1280, 900), (320, 820)]:
        browser('set', 'viewport', width, height)
        for index, page in enumerate(reference['pages']):
            result = evaluate('window.paintGround(' + str(index) + ')')
            result['viewport'] = [width, height]
            REPORT['cases'].append(result)
            if not PAN or index in [0, 13, 26, 55] or result['differentPixels']:
                encoded = evaluate('(()=>{const d=document.querySelector("canvas").getContext("2d").getImageData(0,0,320,200).data;let s="";for(let i=0;i<d.length;i+=8192)s+=String.fromCharCode(...d.subarray(i,i+8192));return btoa(s)})()')
                actual = base64.b64decode(encoded)
                assert hashlib.sha256(actual).hexdigest() == result['hash']
                (OUT / (page['label'] + '-' + str(width) + '-actual.rgba')).write_bytes(actual)
                browser('screenshot', OUT / (page['label'] + '-' + str(width) + '.png'))
    REPORT['passed'] = all(case['differentPixels'] == 0 and case['hash'] == case['expected'] for case in REPORT['cases'])
finally:
    try:
        browser('close')
    except Exception:
        pass
    server.shutdown()
    server.server_close()
    shutil.rmtree(OUT / 'profile', ignore_errors=True)
    for name in ['entry.ts', 'entry.js', 'index.html']:
        (OUT / name).unlink(missing_ok=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(REPORT, ensure_ascii=False))
assert REPORT['passed'], REPORT['cases']
