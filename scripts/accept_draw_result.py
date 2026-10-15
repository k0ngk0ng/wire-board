#!/usr/bin/env python3
"""Shared result UI E2E; no unrelated game suites.

Failure cases: blank/winner heading for a draw; zero rating omitted or labelled
as a loss; draw missing from profile/history; reload changes the result; mobile
overflow. Test module is temporary and never shipped. All writes stay in .local.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request
import uuid

root = Path(__file__).resolve().parents[1]
out = root / '.local/chess-family/draw/browser'
out.mkdir(parents=True, exist_ok=True)
fixture = root / 'games/_draw_acceptance'
env = dict(os.environ)
for key, rel in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/ab-draw', 'AGENT_BROWSER_PROFILE': '.local/chess-family/draw/browser/profile'}.items():
    p = root / rel; p.mkdir(parents=True, exist_ok=True); env[key] = str(p)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_DRAW_BROWSER='1', AGENT_BROWSER_SESSION='draw-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'command': '. scripts/env.sh && python3 -B scripts/accept_draw_result.py'}
server = None
opened = False
created = False

def run(cmd, name):
    with (out / name).open('w') as f: subprocess.run(cmd, cwd=root, env=env, stdout=f, stderr=subprocess.STDOUT, check=True)

def b(*args):
    r = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=40)
    if r.returncode: raise RuntimeError(r.stdout + r.stderr)
    v = json.loads(r.stdout); assert v['success'], v
    return v['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('fetch(' + json.dumps('/api' + path) + opts + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= r['status'] < 300, r
    return r['body']

def command(kind):
    r = api('/state')['room']
    return api('/rooms/' + r['id'], {'type': kind, 'version': r['version'], 'nonce': uuid.uuid4().hex})

def shot(name):
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth<=innerWidth+2'), 'horizontal overflow'
    b('screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(b('snapshot', '-i'), ensure_ascii=False, indent=2))
    report['checks'].append(name); print('PASS', name, flush=True)

try:
    if fixture.exists(): raise RuntimeError('Refusing to overwrite fixture')
    (fixture / 'web').mkdir(parents=True); created = True
    (fixture / 'web/module.tsx').write_text('''import {defineGame} from "../../../web/src/game-api/module";
export default defineGame<{offered:boolean}>({id:"drawcheck",name:"和棋接入验收",description:"公共和棋结果验收。",minPlayers:2,maxPlayers:2,supportsBots:true,
Board:({room,data,act,busy})=><section className="draw-acceptance"><p>{data.offered?"等待同意":"可以提议和棋"}</p><button className="primary" disabled={busy||room.spectating||room.you!==0||data.offered||room.game?.finished} onClick={()=>void act({type:"offer"})}>提议和棋</button></section>});
''')
    run(['npm', '--prefix', 'web', 'run', 'build'], 'build-fixture.log')
    for name in ['url', 'stop']: (out / name).unlink(missing_ok=True)
    with (out / 'server.log').open('w') as f:
        server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=6m', '-run', '^TestModuleDrawBrowserFixture$', './internal/server'], cwd=root, env=env, stdout=f, stderr=subprocess.STDOUT)
    deadline = time.monotonic() + 90
    while not (out / 'url').exists():
        if server.poll() is not None or time.monotonic() > deadline: raise RuntimeError('See server.log')
        time.sleep(.1)
    base = (out / 'url').read_text()
    b('open', base); opened = True
    b('tab', next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login', {'name': 'DrawRoot', 'password': 'draw-browser-password'})
    api('/admin/games/drawcheck', {'enabled': True})
    room = api('/rooms', {'kind': 'drawcheck', 'capacity': 2, 'name': '双方同意和棋'})
    peer = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def peer_api(path, body=None):
        data = None if body is None else json.dumps(body).encode()
        with peer.open(urllib.request.Request(base + '/api' + path, data=data, headers={'Content-Type': 'application/json'}), timeout=10) as r: return json.load(r)
    peer_api('/register', {'name': '和棋对手', 'password': 'draw-browser-password', 'invite': 'draw-browser'})
    peer_api('/rooms/' + room['id'], {'type': 'join', 'version': room['version'], 'nonce': uuid.uuid4().hex})
    command('ready')
    peer_api('/rooms/' + room['id'], {'type': 'ready', 'version': api('/state')['room']['version'], 'nonce': uuid.uuid4().hex})
    command('start'); b('reload'); b('wait', '.draw-acceptance')
    b('find', 'role', 'button', 'click', '--name', '提议和棋', '--exact')
    b('wait', '--text', '等待同意')
    peer_api('/rooms/' + room['id'], {'type': 'action', 'version': api('/state')['room']['version'], 'nonce': uuid.uuid4().hex, 'action': {'type': 'accept'}})
    b('wait', '.results')
    assert js('!!document.querySelector(".results").closest("[role=dialog]")'), 'result must be an overlay'
    assert js('document.querySelector(".results h2").textContent.trim()==="本局和棋"')
    assert js('[...document.querySelectorAll(".result-ratings b")].length===2 && [...document.querySelectorAll(".result-ratings b")].every(e=>e.textContent.trim()==="0 积分")')
    assert js('document.querySelectorAll(".result-ratings .rating-loss").length===0'), 'draw labelled a rating loss'
    result = api('/state')['room']['result']
    for width in [1280, 320]:
        b('set', 'viewport', str(width), '900' if width == 1280 else '740')
        b('scrollintoview', '.results'); shot('draw-result-' + str(width))
    b('reload'); b('wait', '.results')
    assert api('/state')['room']['result'] == result
    assert js('document.querySelector(".results h2").textContent.includes("和棋")')
    # Escape closes only the overlay; subsequent profile fetches must still finish.
    b('press', 'Escape')
    b('find', 'role', 'button', 'click', '--name', '我的战绩与好友', '--exact')
    b('wait', '.profile-history article')
    assert js('document.querySelector(".profile-history article header").textContent.includes("和棋")')
    assert js('document.querySelectorAll(".profile-history .winner").length===0')
    assert js('[...document.querySelectorAll(".profile-history .history-player b")].every(e=>e.textContent.includes("积分 0"))')
    b('scrollintoview', '.profile-history'); shot('draw-history-mobile')
    report['result'] = result; report['passed'] = True
except Exception as e:
    import traceback
    traceback.print_exc(); report['error'] = repr(e)
    if opened:
        try: report['snapshot'] = b('snapshot', '-i'); b('screenshot', str(out / 'failure.png'), '--full')
        except Exception: pass
finally:
    if opened:
        try: b('close')
        except Exception: pass
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    if created:
        shutil.rmtree(fixture)
        try: run(['npm', '--prefix', 'web', 'run', 'build'], 'build-production.log')
        except Exception as e: report['passed'] = False; report['cleanupError'] = str(e)
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
