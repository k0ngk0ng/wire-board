#!/usr/bin/env python3
"""Specified before implementation: real PvZ rooms and native browser input."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/frontend'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ.get('PVZ_BROWSER_URL', '').rstrip('/')
ENV = dict(os.environ, AGENT_BROWSER_SESSION='p' + uuid.uuid4().hex[:3],
           AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ab'),
           AGENT_BROWSER_PROFILE=str(OUT / 'profile'), TMPDIR=str(OUT / 'tmp'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
REPORT = {'passed': False, 'command': 'PVZ_BROWSER_URL=<local URL> python3 -B games/pvz/tests/frontend-browser.py',
          'scope': 'production Board, authenticated HTTP/WebSocket room, native mouse; mobile viewport simulation',
          'cases': [], 'sourceSHA256': {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
                                     for p in (ROOT / 'games/pvz/web').glob('*') if p.is_file()}}
OPENED = False

def b(*args):
    result = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=ENV,
                            text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    record = json.loads(result.stdout)
    assert record['success'], record
    return record['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= result['status'] < 300, result
    return result['body']

def room(): return api('/state')['room']

def command(kind, **extra):
    state = room()
    return api('/rooms/' + state['id'], dict(type=kind, version=state['version'], nonce=uuid.uuid4().hex, **extra))

def wait(fn, seconds=30):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn(): return
        time.sleep(.1)
    raise AssertionError('condition timed out')

def geometry():
    return js('''(()=>{const c=document.querySelector('[data-pvz-canvas]'),r=c.getBoundingClientRect();return {width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth+1,canvas:r.toJSON(),frame:c.__pvzFrame,cards:[...document.querySelectorAll('[data-pvz-card]')].map(e=>({kind:e.dataset.pvzCard,rect:e.getBoundingClientRect().toJSON(),disabled:e.disabled})),wallets:[...document.querySelectorAll('[data-pvz-money]')].map(e=>({seat:e.dataset.pvzMoney,text:e.textContent,rect:e.getBoundingClientRect().toJSON()}))}})()''')

try:
    assert BASE, 'PVZ_BROWSER_URL must point to the local production-module test server'
    OPENED = True; b('open', BASE)
    api('/register', {'name': '植物验收' + uuid.uuid4().hex[:6], 'password': 'pvz-browser-local', 'invite': 'pvz-browser'})
    b('reload'); b('wait', '--text', '创建房间')
    assert 'pvz' in api('/state')['availableGames'], 'PvZ must be available by default'
    for count, width, height in [(2, 1440, 960), (4, 320, 760)]:
        b('set', 'viewport', width, height)
        api('/rooms', {'kind': 'pvz', 'name': '植物大战僵尸验收', 'capacity': count,
                       'options': {'mode': 'duel' if count == 2 else 'team', 'teams': [i % 2 for i in range(count)],
                                   'roundSeconds': 360, 'prepSeconds': 20}})
        for _ in range(count - 1): command('add_bot')
        command('ready'); command('start')
        b('wait', '[data-pvz-board][data-ready="true"]')
        wait(lambda: not room().get('loading'))
        b('scrollintoview', '[data-pvz-board]')
        view = room()['game']['data']
        assert view['players'][0]['side'] == 'plants', view
        assert len(view['players']) == count
        info = geometry()
        assert not info['overflow'], info
        assert len(info['wallets']) == count, info
        assert all(w['rect']['width'] > 0 and w['rect']['left'] >= 0 and w['rect']['right'] <= width + 1 for w in info['wallets']), info
        assert 1 <= len(info['cards']) <= 10, info
        assert js('document.querySelectorAll("[data-pvz-cell]").length===45'), 'missing geometric navigation grid'
        assert js('document.querySelector("[data-controller-group=board] button").getBoundingClientRect().width>20'), 'navigation targets are visually collapsed'
        b('click', '[data-pvz-card="sunflower"]')
        # Real native pointer input, mapping the same 1000-unit grid as production.
        rect = geometry()['canvas']
        x = rect['left'] + rect['width'] * (100 + 40) / 1000
        y = rect['top'] + rect['height'] * (100 + 45) / 600
        b('mouse', 'move', round(x), round(y)); b('mouse', 'down'); b('mouse', 'up')
        wait(lambda: any(p['kind'] == 'sunflower' and p['owner'] == 0 for p in room()['game']['data']['plants']))
        after = room()['game']['data']
        assert after['players'][0]['money'] < view['players'][0]['money']
        assert all(after['players'][i]['money'] >= view['players'][i]['money'] or after['players'][i]['seq'] > view['players'][i]['seq'] for i in range(2, count, 2)), 'teammate wallet debited without its own action'
        b('screenshot', OUT / f'plants-{count}-{width}.png')
        b('click', 'button[aria-label="铲子"]')
        b('press', 'PageDown'); b('press', 'ArrowRight'); b('press', 'ArrowLeft')
        focused = js('({label:document.activeElement.getAttribute("aria-label"),rect:document.activeElement.getBoundingClientRect().toJSON()})')
        assert focused['label'] == '第 1 行第 1 列', focused
        b('press', 'Enter')
        wait(lambda: not any(p['kind'] == 'sunflower' and p['owner'] == 0 for p in room()['game']['data']['plants']))
        js('window.__pvzCanvasBeforeFullscreen=document.querySelector("[data-pvz-canvas]");true')
        b('click', 'button[aria-label="全屏"]')
        b('wait', '[data-room-fullscreen]')
        assert js('window.__pvzCanvasBeforeFullscreen===document.querySelector("[data-pvz-canvas]")')
        assert js('document.querySelector("[data-room-fullscreen]").contains(document.querySelector("[data-pvz-card]"))')
        if width < 650:
            assert js('(()=>{const s=document.querySelector("[data-pvz-canvas]").getBoundingClientRect(),t=document.querySelector("button[aria-label=铲子]").getBoundingClientRect();return t.top>=s.top&&t.bottom<=s.bottom+2})()'), 'fullscreen shovel detached from playable field'
        b('screenshot', OUT / f'fullscreen-{count}-{width}.png')
        b('click', 'button[aria-label="退出全屏"]')
        b('reload'); b('wait', '[data-pvz-board][data-ready="true"]')
        assert room()['game']['data']['epoch'] == after['epoch'], 'reload reset match'
        REPORT['cases'].append({'players': count, 'viewport': [width, height], 'before': info, 'afterPlant': after})
        command('close'); command('leave')
    REPORT['passed'] = True
finally:
    if OPENED:
        if not REPORT['passed']:
            try:
                REPORT['page'] = js('document.body.innerText')
                b('screenshot', OUT / 'failure.png', '--full')
            except Exception: pass
        try:
            b('close')
            for suffix in ['config', 'target']:
                (Path(ENV['AGENT_BROWSER_SOCKET_DIR']) / (ENV['AGENT_BROWSER_SESSION'] + '.' + suffix)).unlink(missing_ok=True)
        except Exception: pass
    shutil.rmtree(OUT / 'profile', ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed'], 'cases': len(REPORT['cases'])}))
