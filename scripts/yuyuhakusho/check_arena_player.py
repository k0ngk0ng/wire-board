"""Repeatable seven-arena browser E2E, specified before the selection UI."""
from pathlib import Path
import argparse, hashlib, json, os, subprocess, threading
from http.server import ThreadingHTTPServer
from serve_local_player import build, PlayerHandler, ROOT

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--existing-url')
parser.add_argument('--phase', default='final')
args = parser.parse_args()
OUT = ROOT / 'games/yuyuhakusho/verification/additional-arena-player' / args.phase
OUT.mkdir(parents=True, exist_ok=True)
TMP = ROOT / '.local/md-yuyuhakusho/arena-browser'
TMP.mkdir(parents=True, exist_ok=True)
env = dict(os.environ, TMPDIR=str(TMP), AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/yys'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True, exist_ok=True)
session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'yy-arena'], cwd=ROOT, env=env, text=True).strip()
env['AGENT_BROWSER_SESSION'] = 'ar' + session.rsplit('-', 1)[-1][:6]
server = None
if args.existing_url:
    url = args.existing_url
else:
    build()
    PlayerHandler.requested_paths = []
    server = ThreadingHTTPServer(('127.0.0.1', 0), PlayerHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    url = 'http://127.0.0.1:' + str(server.server_port) + '/'

def browser(*items):
    result = subprocess.run(['agent-browser', *items, '--json'], cwd=ROOT, env=env, text=True, capture_output=True, timeout=55)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value.get('success'):
        raise RuntimeError(value)
    return value.get('data')

def js(expression):
    return browser('eval', expression)['result']

def native(actions):
    endpoint = browser('get', 'cdp-url')
    endpoint = endpoint.get('url') or endpoint.get('cdpUrl') or endpoint.get('value')
    result = subprocess.run(['node', str(ROOT / 'scripts/yuyuhakusho/native_player_input.mjs'), endpoint, url, json.dumps(actions)], cwd=ROOT, env=env, text=True, capture_output=True, timeout=25)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    return json.loads(result.stdout)

def rect(selector):
    return js('(()=>{const r=document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')

def center(value):
    return {'x': value['x'] + value['w'] / 2, 'y': value['y'] + value['h'] / 2}

opened = False
report = {'pass': False, 'phase': args.phase, 'cases': []}
try:
    browser('--profile', str(TMP / 'profile'), 'open', url)
    opened = True
    browser('set', 'viewport', '1120', '960')
    browser('wait', '--fn', 'window.yyResearch?.assetsReady === true')
    assert js('document.querySelectorAll("#arena-select option").length') == 8, 'eight-arena selector is unavailable'
    assert js('window.yyResearch.view?.tick ?? 0') == 0
    if server:
        assert not any(path.startswith('/arena-') for path in PlayerHandler.requested_paths), PlayerHandler.requested_paths
    browser('select', '#player-character', '7')
    for arena in (2, 4, 6, 8, 10, 12, 14):
        browser('select', '#arena-select', str(arena))
        browser('click', '#start-match')
        browser('wait', '--fn', 'window.yyResearch.view?.arena === ' + str(arena) + ' && window.yyResearch.view.ready && window.yyResearch.view.tick > 280')
        view = js('window.yyResearch.view')
        assert view['window_complete'] and not view['players'][0]['computer'] and view['players'][0]['character_id'] == 7
        assert js('window.yyResearch.pixelSignature') != 0
        assert js('window.yyResearch.errors.length') == 0, js('window.yyResearch.errors')
        report['cases'].append({'arena': arena, 'view': view, 'pixel_signature': js('window.yyResearch.pixelSignature')})
        browser('screenshot', str(OUT / ('arena-' + format(arena, 'x') + '.png')))
    previous = js('window.yyResearch.view.tick')
    browser('select', '#arena-select', '2')
    assert js('window.yyResearch.view.arena') == 14, 'draft selection changed the running arena'
    browser('wait', '--fn', 'window.yyResearch.view.tick > ' + str(previous + 5))
    browser('select', '#arena-select', '10')
    browser('click', '#start-match')
    browser('wait', '--fn', 'window.yyResearch.view?.arena === 10 && window.yyResearch.view.tick > 280')
    before = js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})')
    native([{'method': 'Input.dispatchKeyEvent', 'type': 'keyDown', 'key': 'd', 'code': 'KeyD', 'windowsVirtualKeyCode': 68, 'expect': ['right']}])
    browser('wait', '--fn', 'window.yyResearch.view.players[0].X > ' + str(before['x'] + 8))
    native([{'method': 'Input.dispatchKeyEvent', 'type': 'keyUp', 'key': 'd', 'code': 'KeyD', 'windowsVirtualKeyCode': 68, 'expect': []}])
    report['keyboard'] = {'before': before, 'after': js('({x:window.yyResearch.view.players[0].X,pixel:window.yyResearch.pixelSignature})')}
    browser('screenshot', str(OUT / 'desktop.png'))
    browser('set', 'viewport', '320', '740')
    browser('scrollintoview', '[data-controller-layout="md"]')
    assert not js('document.documentElement.scrollWidth > innerWidth')
    canvas, pad = rect('canvas'), rect('[data-controller-layout="md"]')
    assert pad['y'] >= canvas['y'] + canvas['h'], (canvas, pad)
    left, z, c = [center(rect('[data-controller-button="' + name + '"]')) for name in ('left', 'z', 'c')]
    report['touch'] = native([{'type': 'touchStart', 'touchPoints': [{'id': 1, **left}, {'id': 2, **z}, {'id': 3, **c}], 'expect': ['c', 'left', 'z']}, {'type': 'touchCancel', 'touchPoints': [], 'expect': []}])
    assert js('window.yyResearch.trusted.length > 0 && window.yyResearch.trusted.every(Boolean)')
    assert js('window.yyResearch.errors.length') == 0
    browser('screenshot', str(OUT / 'mobile-320.png'))
    report.update(pass_=True, scope='Seven selected arenas in the independent research player; actual Go/Wasm, Canvas and trusted keyboard/touch; no platform room or network test', input_trusted=True, human_no_auto_takeover=True, asset_requests=PlayerHandler.requested_paths if server else None)
    report['pass'] = report.pop('pass_')
    if server:
        assert '/arena-a-aux.json.gz' in PlayerHandler.requested_paths
        assert not any(any(part in path for part in ('.bin', '.rom', 'libretro')) for path in PlayerHandler.requested_paths)
    report['source_sha256'] = {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in [Path(__file__), ROOT / 'scripts/yuyuhakusho/local_player.tsx', ROOT / 'games/yuyuhakusho/cmd/local-player-wasm/main_js.go', ROOT / 'games/yuyuhakusho/runtimebridge/session.go']}
    (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print('seven-arena browser E2E pass; screenshots and request log:', OUT)
except Exception as error:
    report['error'] = repr(error)
    (OUT / 'failure.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    if opened:
        try:
            browser('screenshot', str(OUT / 'failure.png'))
        except Exception:
            pass
    raise
finally:
    if opened:
        browser('close')
    if server:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
