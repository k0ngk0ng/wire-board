#!/usr/bin/env python3
"""Targeted production-UI browser check, specified before the CSS change.

Failure cases: a wide resource HUD pushes action groups down; status text or
long vote buttons overlap/crop at 320px; normal/fullscreen resize shrinks the
Canvas; dialogs or pause/resume become unreachable after moving the controls.
Uses the existing real HTTP fixture and agent-browser. Build web/dist first.
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

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--baseline', action='store_true')
args = parser.parse_args()
OUT = (ROOT / args.out).resolve()
assert OUT.is_relative_to(ROOT) and not OUT.exists()
OUT.mkdir(parents=True)
TEMP = OUT / 'temporary'
SESSION = 'sfhud' + uuid.uuid4().hex[:6]
SOCKETS = ROOT / '.s' / SESSION
ENV = dict(os.environ)
for key, path in {
    'TMPDIR': TEMP / 'tmp', 'GOTMPDIR': TEMP / 'tmp',
    'GOCACHE': ROOT / '.local/go-build', 'GOMODCACHE': ROOT / '.local/go-mod',
    'AGENT_BROWSER_SOCKET_DIR': SOCKETS, 'AGENT_BROWSER_PROFILE': TEMP / 'profile',
}.items():
    path.mkdir(parents=True, exist_ok=True)
    ENV[key] = str(path)
ENV.update(GOTOOLCHAIN='local', AGENT_BROWSER_SESSION=SESSION,
           AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',
           WIRE_BOARD_STARFORT_BROWSER='1', WIRE_BOARD_STARFORT_BROWSER_OUT=str(OUT / 'fixture'))
REPORT = {'passed': False, 'baseline': args.baseline, 'checks': [], 'failures': [],
          'command': f'python3 -B games/starfort/tests/accept_mobile_hud.py --out {args.out}' + (' --baseline' if args.baseline else ''),
          'scope': 'Production UI, Canvas, real HTTP fixture and browser input; layout check, not a complete match or physical iPhone/Safari test.'}
server = log = None
opened = False


def b(*arguments):
    result = subprocess.run(['agent-browser', *map(str, arguments), '--json'], cwd=ROOT, env=ENV,
                            text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']


def js(code):
    return b('eval', code)['result']


def tap(selector):
    rect = js('document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect().toJSON()')
    point = {'x': rect['x'] + rect['width'] / 2, 'y': rect['y'] + rect['height'] / 2, 'id': 1}
    steps = [{'type': 'touchStart', 'touchPoints': [point], 'wait': 60},
             {'type': 'touchEnd', 'touchPoints': [], 'wait': 120}]
    result = subprocess.run(['node', 'scripts/controllers/gamepad/native-touch.mjs',
                             b('get', 'cdp-url')['cdpUrl'], base, json.dumps(steps), selector],
                            cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=20)
    assert result.returncode == 0, result.stderr
    REPORT.setdefault('nativeTouch', []).append({'selector': selector, 'events': json.loads(result.stdout)})


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert value['status'] in (200, 201), value
    return value['body']


def command(kind):
    for _ in range(12):
        room = api('/state')['room']
        body = {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex}
        value = js('(async()=>{const r=await fetch(' + json.dumps('/api/rooms/' + room['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')});return {status:r.status,body:await r.json()}})()')
        if value['status'] == 409:
            continue
        assert value['status'] == 200, value
        return
    raise AssertionError('stale room command')


def wait(fn, seconds=30):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        assert server is None or server.poll() is None, 'fixture exited'
        if value := fn():
            return value
        time.sleep(.1)
    raise AssertionError('wait expired')


def save(name, value):
    (OUT / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def verify(name, condition, detail):
    item = {'name': name, 'passed': bool(condition), 'detail': detail}
    REPORT['checks'].append(item)
    if not condition:
        REPORT['failures'].append(item)
        assert args.baseline, item


MEASURE = r'''(()=>{
 const rect=s=>document.querySelector(s)?.getBoundingClientRect().toJSON();
 const selectors=['.sf-hud','.sf-compact-stats > span','.sf-compact-countdown','.sf-field-actions > button','.sf-control-buttons > button'];
 const text=[...document.querySelectorAll(selectors.slice(1).join(','))].map(e=>{
   const range=document.createRange();range.selectNodeContents(e);const r=range.getBoundingClientRect();
   const p=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2),s=getComputedStyle(e);
   return {text:e.textContent.trim(),rect:r.toJSON(),fontSize:s.fontSize,color:s.color,background:s.backgroundColor,
     visible:r.width>0&&r.height>0&&r.x>=0&&r.right<=innerWidth&&r.y>=0&&r.bottom<=innerHeight&&!!p&&(e.contains(p)||p.contains(e))};
 });
 return {board:rect('.starfort-board'),hud:rect('.sf-hud'),actions:rect('.sf-field-actions'),wave:rect('.sf-wave-controls'),canvas:rect('[data-starfort-canvas]'),text,overflow:document.documentElement.scrollWidth>innerWidth+1};
})()'''


def measure(name, compact=True):
    value = js(MEASURE)
    save(name, value)
    b('screenshot', OUT / (name + '.png'))
    board, hud, actions, wave = (value[key] for key in ['board', 'hud', 'actions', 'wave'])
    if compact:
        verify(name + ' actions stay at top', actions['top'] - board['top'] <= 12, actions)
        verify(name + ' status leaves right column', hud['right'] + 6 <= actions['left'], value)
        verify(name + ' wave controls directly below actions', 0 <= wave['top'] - actions['bottom'] <= 10, value)
        verify(name + ' top controls use at most 126px', max(hud['bottom'], wave['bottom']) - board['top'] <= 126, value)
    verify(name + ' text is visible and unobscured', all(item['visible'] for item in value['text']), value['text'])
    verify(name + ' canvas still fills board', abs(value['canvas']['width'] - board['width']) <= 4 and abs(value['canvas']['height'] - board['height']) <= 4, value)
    verify(name + ' no horizontal overflow', not value['overflow'], value)
    return value


try:
    log = (OUT / 'server.log').open('w')
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=8m', '-run', '^TestStarfortBrowserFixture$', './games/starfort/tests'], cwd=ROOT, env=ENV, stdout=log, stderr=subprocess.STDOUT)
    wait(lambda: (OUT / 'fixture/url').exists(), 120)
    base = (OUT / 'fixture/url').read_text()
    b('open', base)
    opened = True
    api('/login', {'name': '上架管理员', 'password': 'starfort-browser-test'})
    b('reload')
    b('wait', '.room-list')
    api('/rooms', {'kind': 'starfort', 'name': '手机状态栏验收', 'capacity': 1, 'options': {'scenario': 'rebalance-tasks', 'mode': 'endless', 'waves': 20, 'difficulty': 'normal', 'prepSeconds': 45}})
    command('ready')
    command('start')
    b('wait', '.starfort-board')
    b('wait', '--fn', 'document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
    b('wait', '.sf-tasks-dialog')
    b('click', '.sf-tasks-dialog .modal-head button[aria-label="关闭"]')
    wait(lambda: not js('!!document.querySelector("[role=dialog]")'))
    for width, height in [(320, 740), (393, 852), (430, 932), (740, 320), (1024, 768)]:
        b('set', 'viewport', width, height)
        b('scrollintoview', '.starfort-board')
        # Ordinary mode has identical overlay geometry; measure with the board
        # aligned to the viewport so page scrolling cannot hide a valid panel.
        js('window.scrollBy(0,document.querySelector(".starfort-board").getBoundingClientRect().top-80);true')
        measure(f'normal-{width}x{height}')
        b('click', '[data-fullscreen-enter]')
        b('wait', '[data-room-fullscreen]')
        measure(f'fullscreen-{width}x{height}')
        b('press', 'Escape')
        wait(lambda: not js('!!document.querySelector("[data-room-fullscreen]")'))
    b('set', 'viewport', 320, 740)
    b('click', '[data-fullscreen-enter]')
    b('wait', '[data-room-fullscreen]')
    tap('[data-sf-open-info]')
    b('wait', '.sf-info-dialog')
    b('screenshot', OUT / 'mobile-info-dialog.png')
    b('click', '.sf-info-dialog .modal-head button[aria-label="关闭"]')
    wait(lambda: not js('!!document.querySelector(".sf-info-dialog")'))
    tap('[data-sf-pause]')
    b('wait', '[data-sf-resume]')
    measure('mobile-paused')
    tap('[data-sf-resume]')
    b('wait', '[data-sf-pause]')
    # Layout-only stress: long votes and late-game resource widths, no rules mocked.
    js('document.querySelector("[data-sf-ready]").textContent="取消准备";document.querySelector("[data-sf-pause]").textContent="取消暂停投票";document.querySelector("[data-sf-own-gold]").textContent="123456";document.querySelector("[data-sf-wave] strong").textContent="999";true')
    measure('mobile-long-labels')
    b('press', 'Escape')
    b('set', 'viewport', 1366, 900)
    b('click', '[data-fullscreen-enter]')
    b('wait', '[data-room-fullscreen]')
    measure('desktop', compact=False)
    b('press', 'Escape')
    verify('fullscreen exit restores page', js('!document.querySelector("[inert]") && document.documentElement.style.overflow!=="hidden"'), {})
    REPORT['passed'] = not REPORT['failures']
except BaseException:
    REPORT['error'] = traceback.format_exc()
    print(REPORT['error'], flush=True)
finally:
    if opened:
        try:
            b('close')
        except Exception:
            REPORT['passed'] = False
    if server:
        if (OUT / 'fixture').exists():
            (OUT / 'fixture/stop').write_text('stop')
        try:
            server.wait(timeout=20)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
        REPORT['serverExitCode'] = server.returncode
        if server.returncode != 0:
            REPORT['passed'] = False
    if log:
        log.close()
    for path in [TEMP, SOCKETS, OUT / 'fixture']:
        if path.exists():
            shutil.rmtree(path)
    REPORT['sources'] = {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in [Path(__file__), ROOT / 'games/starfort/web/viewport.css']}
    REPORT['buildIndexSHA256'] = hashlib.sha256((ROOT / 'web/dist/index.html').read_bytes()).hexdigest()
    save('report', REPORT)
raise SystemExit(0 if REPORT['passed'] or (args.baseline and 'error' not in REPORT) else 1)
