import argparse
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[3]
PARSER = argparse.ArgumentParser()
PARSER.add_argument('--game-over', action='store_true')
ARGS = PARSER.parse_args()
PREFIX = 'game-over' if ARGS.game_over else 'screen-shake'
OUT = ROOT / ('.local/pal/' + PREFIX + '-browser')
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), AGENT_BROWSER_PROFILE=str(OUT / 'profile'), AGENT_BROWSER_SCREENSHOT_DIR=str(OUT), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-shake'], cwd=ROOT, env=ENV, text=True).strip()

def browser(*args):
    result = subprocess.run(['agent-browser', '--session', SESSION, '--json', *map(str, args)], cwd=ROOT, env=ENV, check=True, capture_output=True, text=True)
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']

class Handler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self, path):
        if path.startswith('/pal/v1/'):
            base, relative = ROOT / '.local/pal/visual/pal/v1', path[len('/pal/v1/'):]
        else:
            base, relative = OUT, path.split('?', 1)[0].lstrip('/') or 'index.html'
        target = (base / relative).resolve()
        return str(target if target.is_relative_to(base) else OUT / 'not-found')

    def log_message(self, *args):
        pass

subprocess.run(['node', 'games/pal/tests/screen-shake-render-fixture.mjs', *(['--game-over'] if ARGS.game_over else [])], cwd=ROOT, env=ENV, check=True)
for kind in ['authority', 'reference']:
    shutil.copyfile(ROOT / ('.local/pal/verification/' + PREFIX + '-' + kind + '.json'), OUT / (kind + '.json'))
server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
report = {'passed': False, 'scope': 'actual SceneRenderer and PalStage original failure red-palette/index replacement, original messages and fade compared with original C pixels; isolated failure-script fixture, not actual battle defeat or platform acceptance' if ARGS.game_over else 'actual SceneRenderer and PalStage physical shake graphs, compared against original C VIDEO_UpdateScreen physical indexed pixels; desktop and phone maximum canvas; isolated scene rendering, not real-story or platform input acceptance', 'cases': []}
device = None
try:
    browser('open', f'http://127.0.0.1:{server.server_port}')
    browser('wait', '--fn', 'window.shakeReady===true')
    for width, height in [(1280, 900), (320, 820), (820, 320)]:
        if device:
            device.terminate()
            device.wait(timeout=5)
            device = None
        browser('set', 'viewport', width, height)
        if width != 1280:
            device = subprocess.Popen(['node', 'scripts/browser_device.mjs', browser('get', 'cdp-url')['cdpUrl'], f'http://127.0.0.1:{server.server_port}', str(width), str(height), '--keep'], cwd=ROOT, env=ENV, text=True, stdout=subprocess.PIPE)
            assert device.stdout.readline().strip() == 'ready'
        result = browser('eval', '''(async()=>{const pages=(await(await fetch('/reference.json')).json()).pages,c=document.querySelector('canvas'),hashes=[];for(let i=0;i<pages.length;i++){await window.paintShake(i);const pixels=c.getContext('2d').getImageData(0,0,320,200).data,hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',pixels))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==pages[i].rgbaSHA256)throw Error(`mismatch ${pages[i].label} ${hash} expected ${pages[i].rgbaSHA256}`);hashes.push(hash)}const b=c.getBoundingClientRect();return {frames:hashes.length,hashes,canvas:[b.width,b.height],center:[b.x+b.width/2,b.y+b.height/2],viewport:[innerWidth,innerHeight],touch:matchMedia('(pointer:coarse)').matches,overflow:document.documentElement.scrollWidth>innerWidth}})()''')['result']
        assert result['frames'] >= 19 and not result['overflow'], result
        assert abs(result['canvas'][0] - min(width, height * 1.6)) <= 1, result
        assert abs(result['canvas'][0] / result['canvas'][1] - 1.6) < 0.001, result
        assert result['touch'] == (width != 1280), result
        assert abs(result['center'][0] - width / 2) < 1 and abs(result['center'][1] - height / 2) < 1, result
        browser('eval', 'window.paintShake(0)')
        browser('screenshot', OUT / f'shake-{width}.png')
        browser('eval', 'window.paintShake(50)')
        browser('screenshot', OUT / f'fade-{width}.png')
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
print(json.dumps({'passed': report['passed'], 'pixelComparisons': sum(case['frames'] for case in report['cases'])}))
