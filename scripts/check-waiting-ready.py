#!/usr/bin/env python3
"""Waiting-room draft readiness E2E; real HTTP/SQLite and two browser sessions.

Before implementation failure inventory: draft retains stale human ready badge;
other human confirmations wrongly carry to a new map; bots lose ready; reverting
a draft loses prior ready; remote saved map changes prematurely; save vs ready
atomic update diverges; 320px overflows. No game-state injection.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import traceback
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
out = (root / args.out).resolve()
assert out.is_relative_to(root) and not out.exists()
out.mkdir(parents=True)
run = 'wr' + uuid.uuid4().hex[:6]
temp = out / 'temporary'
sockets = root / '.s' / run
env = dict(os.environ)
for key, path in {'TMPDIR': temp / 'tmp', 'GOTMPDIR': temp / 'tmp',
                  'GOCACHE': root / '.local/go-build', 'GOMODCACHE': root / '.local/go-mod',
                  'npm_config_cache': root / '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': sockets}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(WIRE_BOARD_BUBBLEPARTY_BROWSER='1', WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),
           GOTOOLCHAIN='local', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'screenshots': [],
          'distSHA256': hashlib.sha256((root / 'web/dist/index.html').read_bytes()).hexdigest()}
opened = set()
server = log = None


def browser(who, *args):
    profile = temp / who
    profile.mkdir(parents=True, exist_ok=True)
    e = dict(env, AGENT_BROWSER_SESSION=run + who, AGENT_BROWSER_PROFILE=str(profile))
    p = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=e,
                       text=True, capture_output=True, timeout=45)
    assert p.returncode == 0, p.stdout + p.stderr
    result = json.loads(p.stdout)
    assert result['success'], result
    return result['data']


def js(who, code):
    return browser(who, 'eval', code)['result']


def api(who, path, body=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js(who, '(async()=>{const r=await fetch(' + json.dumps('/api' + path) + opts + ');return {status:r.status,data:await r.json()}})()')
    assert result['status'] in (200, 201), result
    return result['data']


def room(who='host'):
    return api(who, '/state')['room']


def cmd(who, kind, **extra):
    r = room(who)
    return api(who, '/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))


def wait(fn, seconds=20):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if value := fn():
            return value
        time.sleep(.1)
    raise AssertionError('condition timed out')


def click(who, name):
    browser(who, 'find', 'role', 'button', 'click', '--name', name, '--exact')


def view(who):
    return js(who, '''(()=>{const map=document.querySelector('select[aria-label="水泡战场地图"]');return {map:map?.value,seats:[...document.querySelectorAll('.waiting-seat.filled')].map(e=>({bot:!!e.querySelector('.bot-badge'),ready:e.textContent.includes('已准备'),text:e.textContent})),buttons:[...document.querySelectorAll('.waiting-actions button')].map(e=>({text:e.textContent.trim(),disabled:e.disabled})),status:document.querySelector('.waiting-seats [role=status]')?.textContent}})()''')


def record(name, who='host'):
    data = view(who)
    report['checks'].append({'name': name, 'view': data})
    print('PASS', name, flush=True)
    return data


def ready(who, flags):
    return [s['ready'] for s in view(who)['seats']] == flags


def shot(name, who='host'):
    browser(who, 'screenshot', str(out / (name + '.png')), '--full')
    report['screenshots'].append(name + '.png')


try:
    log = (out / 'server.log').open('w')
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=12m', '-run', '^TestBubblepartyBrowserFixture$', './games/bubbleparty/tests'], cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT)
    wait(lambda: (out / 'url').exists(), 120)
    base = (out / 'url').read_text()
    for who in ['host', 'guest']:
        browser(who, 'open', base)
        opened.add(who)
        browser(who, 'set', 'viewport', '1280', '1000')
    api('host', '/login', {'name': '泡泡验收管理员', 'password': 'bubbleparty-browser-test-only'})
    api('guest', '/register', {'name': '换图准备验收', 'password': 'waiting-ready-test', 'invite': 'bubbleparty-browser'})
    api('host', '/rooms', {'kind': 'bubbleparty', 'name': '换图准备一致性', 'capacity': 4, 'options': {'map': 'community10', 'roundSeconds': 180}})
    api('guest', '/rooms/' + room()['id'], {'type': 'join', 'version': room()['version'], 'nonce': uuid.uuid4().hex})
    cmd('host', 'add_bot')
    cmd('host', 'add_bot')
    for who in ['host', 'guest']:
        browser(who, 'reload')
        browser(who, 'wait', '.waiting-layout')
        click(who, '我准备好了')
    wait(lambda: ready('host', [True, True, True, True]) and ready('guest', [True, True, True, True]))
    browser('host', 'select', 'select[aria-label="水泡战场地图"]', 'waterpark')
    assert ready('host', [False, False, True, True]), view('host')
    assert any(b['text'] == '保存并准备' for b in view('host')['buttons'])
    assert view('guest')['map'] == 'community10' and ready('guest', [True, True, True, True]), view('guest')
    record('draft invalidates human badges and preserves bot ready')
    record('remote remains on authoritative saved map', 'guest')
    shot('draft-desktop')
    browser('host', 'select', 'select[aria-label="水泡战场地图"]', 'community10')
    assert ready('host', [True, True, True, True])
    record('reverting unsaved map restores original readiness')
    browser('host', 'select', 'select[aria-label="水泡战场地图"]', 'waterpark')
    click('host', '保存设置')
    wait(lambda: view('guest')['map'] == 'waterpark' and ready('guest', [False, False, True, True]))
    assert ready('host', [False, False, True, True])
    record('save resets all human confirmations, bots remain ready')
    record('remote saved map and badges synchronize', 'guest')
    for who in ['host', 'guest']:
        click(who, '我准备好了')
    wait(lambda: ready('host', [True, True, True, True]))
    browser('host', 'set', 'viewport', '320', '844')
    browser('host', 'select', 'select[aria-label="水泡战场地图"]', 'forest')
    assert ready('host', [False, False, True, True])
    assert js('host', 'document.documentElement.scrollWidth<=innerWidth+2')
    shot('draft-320')
    click('host', '保存并准备')
    wait(lambda: ready('host', [True, False, True, True]) and view('guest')['map'] == 'forest')
    wait(lambda: ready('guest', [True, False, True, True]))
    assert next(b for b in view('host')['buttons'] if b['text'] == '开始游戏')['disabled']
    record('save-and-ready confirms only host, awaits other human')
    record('remote sees changed map and must confirm again', 'guest')
    shot('saved-320')
    click('guest', '我准备好了')
    wait(lambda: not next(b for b in view('host')['buttons'] if b['text'] == '开始游戏')['disabled'])
    click('host', '开始游戏')
    wait(lambda: room().get('game', {}).get('data', {}).get('phase') == 'playing')
    assert room()['game']['data']['mapId'] == 'forest'
    report['checks'].append({'name': 'actual game starts on selected map with two humans and two bots'})
    report['passed'] = True
except BaseException:
    report['error'] = traceback.format_exc()
    print(report['error'], flush=True)
finally:
    for who in opened:
        try:
            browser(who, 'close')
        except BaseException:
            report['passed'] = False
    if server:
        (out / 'stop').write_text('stop')
        try:
            server.wait(timeout=20)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
            report['passed'] = False
        if server.returncode:
            report['passed'] = False
    if log:
        log.close()
    for path in [temp, sockets, out / 'data']:
        if path.exists():
            shutil.rmtree(path)
    for name in ['url', 'stop']:
        (out / name).unlink(missing_ok=True)
    (out / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(out / 'report.json', flush=True)
    if not report['passed']:
        raise SystemExit(1)
