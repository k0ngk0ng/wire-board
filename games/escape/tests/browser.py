#!/usr/bin/env python3
"""Written before UI: production rooms, real pointer/keyboard and screenshots."""
import json
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
COMMON=Path(subprocess.check_output(['git','rev-parse','--git-common-dir'],cwd=ROOT,text=True).strip())
SOCKETS=(COMMON if COMMON.is_absolute() else ROOT/COMMON).resolve().parent/'.local/escape-ab'
OUT = ROOT / '.local/escape/browser'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ.get('ESCAPE_BROWSER_URL', '').rstrip('/') or (OUT / 'url').read_text().strip()
ENV = dict(os.environ, AGENT_BROWSER_SESSION='esc' + uuid.uuid4().hex[:4],
           AGENT_BROWSER_SOCKET_DIR=str(SOCKETS),
           AGENT_BROWSER_PROFILE=str(OUT / 'profile'), TMPDIR=str(OUT / 'tmp'),
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
REPORT = {'passed': False, 'command': 'python3 -B games/escape/tests/browser.py',
          'scope': 'production module and authenticated room; native browser inputs; simulated mobile viewport', 'cases': []}

def b(*args):
    p = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=ENV,
                       text=True, capture_output=True, timeout=45)
    assert p.returncode == 0, p.stdout + p.stderr
    r = json.loads(p.stdout); assert r['success'], r
    return r['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= r['status'] < 300, r
    return r['body']

def room(): return api('/state')['room']

def command(kind, **extra):
    r = room()
    return api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))

def wait(fn, seconds=25):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn(): return
        time.sleep(.1)
    raise AssertionError('condition timed out')

REPORT['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [*(ROOT/'games/escape/web').glob('*'),ROOT/'games/escape/controls.json',ROOT/'games/escape/assets/manifest.json'] if p.is_file()}
REPORT['distIndexSHA256']=hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest()
opened = False
try:
    b('open', BASE); opened = True
    api('/login', {'name': '密室体验', 'password': 'escape-browser-password'})
    existing = api('/state').get('room')
    if existing:
        assert existing['kind'] == 'escape' and existing['name'] in ('密室视觉与交互验收','密室机关与合作验收')
        if existing['status'] == 'playing': command('close')
        command('leave')
    for scenario, width, height in [('station', 1440, 1000), ('archive', 1440, 1000), ('station', 320, 820), ('archive', 320, 820)]:
        b('set', 'viewport', width, height)
        api('/rooms', {'kind': 'escape', 'name': '密室视觉与交互验收', 'capacity': 2,
                       'options': {'scenario': scenario, 'difficulty': 'easy', 'seed': 'visual-17'}})
        command('add_bot')
        command('ready'); command('start')
        b('reload'); b('wait', '[data-escape-board][data-ready="true"]')
        wait(lambda: not room().get('loading'))
        assert room()['game']['data']['options']['scenario'] == scenario
        state = room()['game']['data']
        assert state['phase'] == 'playing'
        overflow = js('document.documentElement.scrollWidth>innerWidth+1')
        assert not overflow, 'horizontal overflow'
        b('screenshot', OUT / f'{scenario}-{width}.png', '--full')
        b('click', '[data-escape-object="briefing"]')
        wait(lambda: len(room()['game']['data'].get('journal', [])) > 0)
        js('document.querySelector(\'button[aria-label="设施地图"]\').scrollIntoView({behavior:"instant",block:"center"})')
        b('click', 'button[aria-label="设施地图"]')
        wait(lambda: js('!!document.querySelector("[data-escape-map]")'))
        b('press','Tab')
        assert js('document.querySelector("[data-escape-dialog]").contains(document.activeElement)'), 'dialog keyboard focus escaped to the covered board'
        b('screenshot', OUT / f'{scenario}-map-{width}.png')
        b('click', 'button[aria-label="关闭设施地图"]')
        before = room()['game']['data']
        b('find', 'first', '[data-escape-door]', 'click')
        wait(lambda: room()['game']['data']['players'][0]['room'] != before['players'][0]['room'])
        after = room()['game']['data']
        b('screenshot', OUT / f'{scenario}-exploration-{width}.png')
        b('click', 'button[aria-label="全屏"]')
        wait(lambda: js('!!document.querySelector("[data-game-fullscreen-surface][data-room-fullscreen]")') or js('!!document.fullscreenElement'))
        assert js('document.querySelector("[data-escape-canvas]").getBoundingClientRect().width>200')
        if width == 320:
            assert js('''[...document.querySelectorAll('[data-escape-field-health]')].length===2 && [...document.querySelectorAll('[data-escape-field-health]')].every(e=>{const r=e.getBoundingClientRect();return r.width>20&&r.height>10&&r.top>=0&&r.bottom<=innerHeight})'''), 'fullscreen health is not actually visible'
        b('screenshot', OUT / f'{scenario}-fullscreen-{width}.png')
        b('click', 'button[aria-label="退出全屏"]')
        b('reload'); b('wait', '[data-escape-board][data-ready="true"]')
        assert room()['game']['data']['epoch'] == after['epoch'], 'reload reset room'
        REPORT['cases'].append({'scenario': scenario, 'viewport': [width, height], 'entry': state, 'afterDoor': after, 'overflow': overflow})
        command('close'); command('leave')
    REPORT['passed'] = True
finally:
    if opened:
        if not REPORT['passed']:
            try:
                REPORT['page'] = js('document.body.innerText')
                b('screenshot', OUT / 'failure.png', '--full')
            except Exception: pass
        try: b('close')
        except Exception: pass
    shutil.rmtree(OUT / 'profile', ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed'], 'cases': len(REPORT['cases'])}))
