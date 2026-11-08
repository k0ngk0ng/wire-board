#!/usr/bin/env python3
"""Normal authenticated two-player battle; native inputs, natural wallets.

Failure scenarios are specified in open_roster_FAILURE_SCENARIOS.md. Only one
asset fetch is delayed to verify cancellation; no game state is injected.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/open-roster/browser'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ['PVZ_BROWSER_URL'].rstrip('/')
SESSIONS = []
REPORT = {'passed': False, 'scope': 'production HTTP room, two manual humans, natural wallets and native mouse/keyboard; 320px Chromium viewport simulation',
          'command': 'PVZ_BROWSER_URL=<local URL> python3 -B games/pvz/tests/open-roster-browser.py', 'cases': [],
          'sourceSHA256': {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT / 'games/pvz/web').glob('*') if p.is_file()}}


def session(label):
    env = dict(os.environ, AGENT_BROWSER_SESSION='por' + label + uuid.uuid4().hex[:4],
               AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ab'),
               AGENT_BROWSER_PROFILE=str(OUT / ('profile-' + label)), TMPDIR=str(OUT / 'tmp'),
               AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
    for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    SESSIONS.append(env)
    return env


def b(env, *args):
    result = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=env,
                            text=True, capture_output=True, timeout=60)
    assert result.returncode == 0, result.stdout + result.stderr
    record = json.loads(result.stdout)
    assert record['success'], record
    return record['data']


def js(env, code): return b(env, 'eval', code)['result']


def api(env, path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js(env, '(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= result['status'] < 300, result
    return result['body']


def room(env): return api(env, '/state')['room']


def command(env, kind, **extra):
    for _ in range(8):
        r = room(env)
        body = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        result = js(env, '(async()=>{const r=await fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')});return {status:r.status,body:await r.json()}})()')
        if result['status'] == 409: continue
        assert result['status'] == 200, result
        return result['body']
    raise AssertionError('room version did not converge')


def wait(fn, seconds=30):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn(): return
        time.sleep(.15)
    raise AssertionError('condition timed out')


def book(env):
    b(env, 'click', 'button[aria-label="卡册"]')
    b(env, 'wait', '[role="dialog"][aria-label="植物大战僵尸卡册"]')


def use(env, kind, imitate=None):
    book(env)
    if imitate: b(env, 'select', 'select[aria-label="模仿目标"]', imitate)
    b(env, 'click', '[data-pvz-book-card=' + json.dumps(kind) + ']')
    wait(lambda: not js(env, '!!document.querySelector(".pvz-card-book")'))
    assert js(env, 'document.querySelector(' + json.dumps('[data-pvz-card="' + kind + '"]') + ')?.getAttribute("aria-pressed")==="true"')


def cell(env, row, col, lane=False):
    selector = 'button[aria-label=' + json.dumps('向第 ' + str(row + 1) + ' 行出兵', ensure_ascii=False) + ']' if lane else '[data-pvz-cell=' + json.dumps(str(row) + ':' + str(col)) + ']'
    b(env, 'scrollintoview', selector)
    rect = js(env, 'document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect().toJSON()')
    b(env, 'mouse', 'move', round(rect['left'] + rect['width']/2), round(rect['top'] + rect['height']/2))
    b(env, 'mouse', 'down'); b(env, 'mouse', 'up')


try:
    plants, zombies = session('p'), session('z')
    for env, label in [(plants, '植物'), (zombies, '僵尸')]:
        b(env, 'open', BASE)
        api(env, '/register', {'name': '全卡池' + label + uuid.uuid4().hex[:5], 'password': 'pvz-open-local', 'invite': 'pvz-browser'})
        b(env, 'reload'); b(env, 'set', 'viewport', 1440, 1000)
    api(plants, '/rooms', {'kind': 'pvz', 'name': '完整卡池实战', 'capacity': 2,
                          'options': {'scene': 'day', 'mode': 'duel', 'teams': [0, 1], 'roundSeconds': 180, 'prepSeconds': 10}})
    r = room(plants)
    api(zombies, '/rooms/' + r['id'], {'type': 'join', 'version': r['version'], 'nonce': uuid.uuid4().hex})
    command(zombies, 'ready'); command(plants, 'ready'); command(plants, 'start')
    for env in [plants, zombies]: b(env, 'wait', '[data-pvz-board][data-ready="true"]')
    wait(lambda: not room(plants).get('loading'))
    wait(lambda: room(plants)['game']['data']['phase'] == 'battle')
    initial = room(plants)['game']['data']
    assert initial['rulesVersion'] == 4
    assert 'starfruit' not in initial['players'][0]['deck'] and 'flag' not in initial['players'][1]['deck']

    # Opening the full catalog loads previews, without downloading every native
    # animation. Delay one uncached selection, close it, and observe late load.
    book(plants)
    assert js(plants, 'document.querySelectorAll("[data-pvz-book-card]").length===49')
    manifest = js(plants, '(async()=>await(await fetch("/pvz/classic-v2/manifest.json")).json())()')
    path = manifest['sprites']['starfruit']['native']['src']
    js(plants, 'window.__pvzFetch=window.fetch;window.fetch=async function(input,init){if(String(input).endsWith(' + json.dumps(path) + '))await new Promise(resolve=>setTimeout(resolve,6000));return window.__pvzFetch.call(this,input,init)};true')
    b(plants, 'click', '[data-pvz-book-card="starfruit"]')
    b(plants, 'press', 'Escape')
    wait(lambda: not js(plants, '!!document.querySelector(".pvz-card-book")'))
    time.sleep(6)
    assert not js(plants, '!!document.querySelector("[data-pvz-card=starfruit][aria-pressed=true]")'), 'closed catalog changed selection after loading'
    js(plants, 'window.fetch=window.__pvzFetch;true')
    use(plants, 'starfruit')
    before = room(plants)['game']['data']
    cell(plants, 0, 2)
    wait(lambda: any(p['kind'] == 'starfruit' and p['owner'] == 0 for p in room(plants)['game']['data']['plants']))
    placed = room(plants)['game']['data']
    assert placed['players'][0]['money'] <= before['players'][0]['money'] - 100
    assert placed['players'][0]['deck'] == initial['players'][0]['deck']
    assert js(plants, 'document.querySelectorAll("[data-pvz-card]").length===10')
    REPORT['cases'].append({'case': 'battle non-quickbar plant and cancelled async selection', 'walletBefore': before['players'][0]['money'], 'walletAfter': placed['players'][0]['money'], 'plant': next(p for p in placed['plants'] if p['kind'] == 'starfruit')})
    b(plants, 'screenshot', OUT / 'desktop-battle.png', '--full')

    # A battle quickbar edit retains the real cooldown and personal wallet.
    book(plants)
    b(plants, 'click', '.pvz-book-modes button:nth-child(2)')
    b(plants, 'click', '[data-pvz-book-card="sunflower"]')
    b(plants, 'click', '[data-pvz-book-card="cabbagepult"]')
    b(plants, 'click', 'button[aria-label="保存快捷栏"]')
    wait(lambda: 'cabbagepult' in room(plants)['game']['data']['players'][0]['deck'])
    edited = room(plants)['game']['data']
    assert edited['players'][0]['cooldowns']['starfruit'] == placed['players'][0]['cooldowns']['starfruit']
    assert len(edited['players'][0]['deck']) == 10

    # Modal and full catalog remain usable in a narrow fullscreen game area.
    b(plants, 'set', 'viewport', 320, 760)
    b(plants, 'click', 'button[aria-label="全屏"]')
    b(plants, 'wait', '[data-room-fullscreen]'); book(plants)
    assert js(plants, 'document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[role=dialog]"))')
    assert not js(plants, 'document.documentElement.scrollWidth>innerWidth+1')
    bounds = js(plants, 'document.querySelector(".pvz-card-book").getBoundingClientRect().toJSON()')
    assert bounds['left'] >= 0 and bounds['right'] <= 321 and bounds['top'] >= 0 and bounds['bottom'] <= 761, bounds
    b(plants, 'screenshot', OUT / 'catalog-320-fullscreen.png')
    b(plants, 'click', '[role="dialog"] button[aria-label="关闭"]')
    wait(lambda: not js(plants, '!!document.querySelector(".pvz-card-book")'))
    if js(plants, '!!document.querySelector("[data-room-fullscreen]")'): b(plants, 'click', 'button[aria-label="退出全屏"]')
    b(plants, 'screenshot', OUT / 'battle-320.png', '--full')
    REPORT['cases'].append({'case': 'battle quickbar edit and mobile fullscreen', 'cooldownPreserved': edited['players'][0]['cooldowns']['starfruit'], 'quickbar': edited['players'][0]['deck'], 'modalBounds': bounds})

    b(plants, 'set', 'viewport', 1440, 1000)
    use(plants, 'imitater', imitate='squash')
    imitating = room(plants)['game']['data']
    assert imitating['players'][0]['imitaterKind'] == 'squash'
    assert imitating['players'][0]['deck'] == edited['players'][0]['deck']
    cell(plants, 1, 2)
    wait(lambda: any(p['row'] == 1 and p['col'] == 2 and p.get('imitated') for p in room(plants)['game']['data']['plants']))
    REPORT['cases'].append({'case': 'non-quickbar imitater, ordinary target update and placement', 'target': imitating['players'][0]['imitaterKind'], 'quickbarUnchanged': True})

    book(zombies)
    assert js(zombies, 'document.querySelectorAll("[data-pvz-book-card]").length===33')
    locked = js(zombies, 'document.querySelector("[data-pvz-book-card=balloon]").getAttribute("aria-disabled")')
    assert locked == 'true', 'flying units opened before normal 90s unlock'
    b(zombies, 'click', '[data-pvz-book-card="balloon"]')
    assert js(zombies, '!!document.querySelector(".pvz-card-book")')
    b(zombies, 'click', '[data-pvz-book-card="flag"]')
    wait(lambda: not js(zombies, '!!document.querySelector(".pvz-card-book")'))
    before = room(zombies)['game']['data']
    cell(zombies, 3, 8, lane=True)
    wait(lambda: any(z['kind'] == 'flag' and z['owner'] == 1 for z in room(zombies)['game']['data']['zombies']))
    deployed = room(zombies)['game']['data']
    assert deployed['players'][1]['money'] <= before['players'][1]['money'] - 35
    assert deployed['players'][1]['cooldowns']['backupdancer'] > deployed['tick']
    assert deployed['players'][1]['deck'] == initial['players'][1]['deck']
    requests = js(zombies, 'performance.getEntriesByType("resource").filter(e=>e.name.endsWith(".pvzr")).map(e=>e.name)')
    assert not any('zomboss' in url.lower() for url in requests)
    b(zombies, 'screenshot', OUT / 'zombies-desktop.png', '--full')
    REPORT['cases'].append({'case': 'non-quickbar zombie, shared cooldown, early air lock and lazy loading', 'walletBefore': before['players'][1]['money'], 'walletAfter': deployed['players'][1]['money'], 'nativeRequests': requests, 'cooldowns': deployed['players'][1]['cooldowns']})
    REPORT['passed'] = True
finally:
    if not REPORT['passed']:
        for index, env in enumerate(SESSIONS):
            try:
                b(env, 'screenshot', OUT / ('failure-' + str(index) + '.png'), '--full')
                REPORT['page' + str(index)] = js(env, 'document.body.innerText')
            except Exception: pass
    try:
        if SESSIONS and room(SESSIONS[0]): command(SESSIONS[0], 'close')
    except Exception: pass
    for env in SESSIONS:
        try: b(env, 'close')
        except Exception: pass
        shutil.rmtree(env['AGENT_BROWSER_PROFILE'], ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed'], 'cases': len(REPORT['cases'])}))
