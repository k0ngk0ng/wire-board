#!/usr/bin/env python3
"""Build and exercise an independently registered game plus all six migrated UIs.

Uses only test accounts, normal HTTP commands and real browser interactions.
The temporary game is never shipped. All caches/profiles/results stay in .local.
Failure cases: discovery/creation failures, lost custom action fields, stale UI,
missing privacy projection, broken asset paths, JS errors, mobile overflow, or
result changes after reload. Run from the repository root with agent-browser.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

root = Path(__file__).resolve().parents[1]
out = root / '.local/module-refactor/browser'
out.mkdir(parents=True, exist_ok=True)
fixture = root / 'games/_module_acceptance'
if fixture.exists():
    raise SystemExit(f'Refusing to overwrite {fixture}')
env = dict(os.environ)
for key, value in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/abmodules', 'AGENT_BROWSER_PROFILE': '.local/module-refactor/browser/profile'}.items():
    location = root / value
    location.mkdir(parents=True, exist_ok=True)
    env[key] = str(location)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_MODULE_BROWSER='1', AGENT_BROWSER_SESSION='module-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'command': 'python3 -B scripts/accept_game_modules.py'}
server = None
browser_open = False
log_handles = []


def run(command, log):
    with (out / log).open('w') as handle:
        result = subprocess.run(command, cwd=root, env=env, stdout=handle, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f'{command} failed; see {out / log}')


def b(*command):
    result = subprocess.run(['agent-browser', *command, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=35)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value['success']:
        raise RuntimeError(str(value))
    return value['data']


def js(code):
    return b('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('fetch(' + json.dumps(path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= value['status'] < 300, value
    return value['body']


def room():
    return api('/api/state')['room']


def command(kind, **extra):
    for _ in range(10):
        r = room()
        body = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        response = js('fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}).then(async r=>({status:r.status,body:await r.json()}))')
        if response['status'] == 409:
            continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('version conflict did not settle')


def checkpoint(name):
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth <= innerWidth + 2'), 'page overflow: ' + name
    assert js('[...document.images].every(i=>!i.complete || i.naturalWidth>0)'), 'broken image: ' + name
    b('screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(b('snapshot', '-i'), ensure_ascii=False, indent=2))
    report['checks'].append(name)
    print('PASS', name, flush=True)

try:
    for name in ('url', 'stop'):
        (out / name).unlink(missing_ok=True)
    (fixture / 'web').mkdir(parents=True)
    shutil.copyfile(root / 'internal/server/testdata/module-ui.tsx', fixture / 'web/module.tsx')
    run(['npm', '--prefix', 'web', 'run', 'build'], 'build-fixture.log')
    handle = (out / 'server.log').open('w'); log_handles.append(handle)
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=11m', '-run', '^TestModuleBrowserFixture$', './internal/server'], cwd=root, env=env, stdout=handle, stderr=subprocess.STDOUT)
    started = time.monotonic()
    while not (out / 'url').exists():
        if server.poll() is not None or time.monotonic() - started > 90:
            raise RuntimeError('fixture failed to start; see server.log')
        time.sleep(.1)
    base = (out / 'url').read_text()
    b('open', base); browser_open = True
    api('/api/login', {'name': 'ModuleRoot', 'password': 'module-root-test-123'})
    api('/api/admin/games/acceptance', {'enabled': True})
    api('/api/logout', {})
    api('/api/register', {'name': '模块浏览器验收', 'password': 'module-browser-test-only', 'invite': 'test-invite'})
    b('reload')
    b('wait', '.game-selection')
    b('set', 'viewport', '1280', '900')
    assert js('[...document.querySelectorAll(".filter-tabs button")].some(b=>b.textContent==="接入验收")'), 'module missing from lobby filters'
    checkpoint('lobby-desktop')
    index = js('[...document.querySelectorAll(".game-feature")].findIndex(e=>e.textContent.includes("接入验收"))+1')
    assert index > 0
    b('click', f'.game-feature:nth-child({index}) button')
    b('wait', '[role=dialog]')
    b('select', 'select[aria-label=游戏]', 'splendor')
    b('wait', '.choose-games')
    b('click', '.choose-games button:last-child')
    b('wait', 'select[aria-label=游戏]')
    b('fill', 'input[name=name]', '模块浏览器牌桌')
    b('click', '[role=dialog] button[type=submit]')
    b('wait', '--text', '添加电脑')
    b('find', 'text', '添加电脑', 'click', '--exact')
    b('wait', '--fn', '!document.querySelector(".waiting-actions button")?.disabled')
    b('click', '.waiting-actions > button:first-child')
    b('wait', '--fn', '[...document.querySelectorAll(".waiting-actions button")].some(b=>b.textContent==="开始游戏" && !b.disabled)')
    b('click', '.waiting-actions > button:last-child')
    b('wait', '.acceptance-board')
    assert js('document.querySelector("[data-testid=private-card]").textContent') == 'private-test-card'
    assert js('Number(document.querySelector(".turn-info p").textContent.match(/[0-9]+/)[0])') > 0, 'deadline display uses the wrong clock'
    checkpoint('module-board-desktop')
    b('set', 'viewport', '390', '844')
    checkpoint('module-board-mobile')
    b('find', 'role', 'button', 'click', '--name', '对局日志')
    b('wait', '.journal-dialog')
    assert js('document.querySelector(".journal-dialog").textContent.includes("牌已洗好")'), 'empty module log is not renderable'
    b('click', '[role=dialog] button[aria-label=关闭]')
    b('find', 'text', '增加一次', 'click', '--exact')
    b('wait', '--fn', 'document.querySelector(".acceptance-board h2")?.textContent.includes("2")')
    b('find', 'text', '增加一次', 'click', '--exact')
    b('wait', '.results')
    result = room()['result']
    b('reload'); b('wait', '.results')
    assert room()['result'] == result
    checkpoint('module-finished-reloaded')
    command('leave')
    for kind, players, selector in [('splendor', 2, '.splendor-board'), ('rail', 2, '.rail-board'), ('catan', 3, '.catan-board'), ('carcassonne', 2, '.car-board'), ('sanguosha', 4, '.sg-board'), ('dota', 2, '.dota-table')]:
        api('/api/rooms', {'kind': kind, 'name': kind + ' 模块验收', 'capacity': players})
        for _ in range(players - 1):
            command('add_bot')
        command('ready'); command('start')
        b('wait', selector)
        b('set', 'viewport', '1280', '900'); checkpoint(kind + '-desktop')
        b('set', 'viewport', '390', '844'); checkpoint(kind + '-mobile')
        assert room()['game']['kind'] == kind
        command('close'); command('leave')
    report['passed'] = True
except Exception as error:
    import traceback
    report['error'] = repr(error)
    traceback.print_exc()
    if browser_open:
        try:
            report['browserErrors'] = b('errors')
            report['failureSnapshot'] = b('snapshot', '-i')
            report['failureState'] = api('/api/state')
            b('screenshot', str(out / 'failure.png'), '--full')
        except Exception as diagnostic_error:
            report['diagnosticError'] = str(diagnostic_error)
finally:
    if browser_open:
        try: b('close')
        except Exception as error: report['browserCloseError'] = str(error)
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    for handle in log_handles: handle.close()
    shutil.rmtree(fixture)
    # Always restore a production build without the acceptance-only module.
    try: run(['npm', '--prefix', 'web', 'run', 'build'], 'build-production.log')
    except Exception as error:
        report['passed'] = False; report['productionBuildError'] = str(error)
    report['commit'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
