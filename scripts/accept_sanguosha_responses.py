#!/usr/bin/env python3
"""Verify actual response controls for holder, non-holders and spectator.

Failure cases: non-holders asked to pass, missing legal response, stale controls
following a pass/reload, private hands in another view, JS errors or overflow.
All services, accounts, profiles and repeatable artifacts are workspace-local.
"""
import json
import os
from pathlib import Path
import subprocess
import time
import traceback
import uuid

root = Path(__file__).resolve().parents[1]
out = root / '.local/sanguosha-responses/browser'
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
for key, relative in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/absg'}.items():
    target = root / relative
    target.mkdir(parents=True, exist_ok=True)
    env[key] = str(target)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_SG_RESPONSE_BROWSER='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
run_id = uuid.uuid4().hex[:8]
opened = []
server = None
report = {'passed': False, 'command': 'python3 -B scripts/accept_sanguosha_responses.py', 'checks': []}
handle = None


def browser(player, *command):
    profile = out / ('profile-' + str(player))
    profile.mkdir(exist_ok=True)
    local = dict(env, AGENT_BROWSER_SESSION=f'sgr-{run_id}-{player}', AGENT_BROWSER_PROFILE=str(profile))
    result = subprocess.run(['agent-browser', *command, '--json'], cwd=root, env=local, text=True, capture_output=True, timeout=35)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value['success']:
        raise RuntimeError(str(value))
    return value['data']


def js(player, code):
    return browser(player, 'eval', code)['result']


def api(player, path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js(player, 'fetch(' + json.dumps(path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= value['status'] < 300, value
    return value['body']


def checkpoint(player, name, eligible):
    room = api(player, '/api/state')['room']
    game = room['game']['sanguosha']
    assert bool(game.get('pending', {}).get('canRespond')) == eligible, (name, game.get('pending'))
    for seat, item in enumerate(game['players']):
        if seat != room['you']:
            assert 'hand' not in item
    actual = js(player, '[...document.querySelectorAll(".sg-actions button")].some(b=>b.textContent.trim()==="放弃")')
    assert actual == eligible, (name, 'unexpected response control', actual)
    assert not browser(player, 'errors')['errors']
    assert js(player, 'document.documentElement.scrollWidth<=innerWidth+2'), name + ' overflow'
    browser(player, 'screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(browser(player, 'snapshot', '-i'), ensure_ascii=False, indent=2))
    report['checks'].append({'name': name, 'player': room['you'], 'canRespond': eligible, 'phase': room['game']['phase']})
    print('PASS', name, flush=True)

try:
    for name in ['fixture.json', 'stop']:
        (out / name).unlink(missing_ok=True)
    with (out / 'build.log').open('w') as build:
        subprocess.run(['npm', '--prefix', 'web', 'run', 'build'], cwd=root, env=env, stdout=build, stderr=subprocess.STDOUT, check=True)
    handle = (out / 'server.log').open('w')
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=6m', '-run', '^TestSanguoshaResponseBrowserFixture$', './internal/server'], cwd=root, env=env, stdout=handle, stderr=subprocess.STDOUT)
    started = time.monotonic()
    while not (out / 'fixture.json').exists():
        if server.poll() is not None or time.monotonic() - started > 60:
            raise RuntimeError('fixture failed to start; see server.log')
        time.sleep(.1)
    fixture = json.loads((out / 'fixture.json').read_text())
    for player in [0, 1, 2, 4]:
        browser(player, 'open', fixture['url']); opened.append(player)
        api(player, '/api/login', {'name': f'响应验收{player}', 'password': 'test-password-123'})
        browser(player, 'reload'); browser(player, 'wait', '.sg-board')
        browser(player, 'set', 'viewport', '1280' if player in [0, 1] else '390', '900')
    room = api(0, '/api/state')['room']
    api(0, '/api/rooms/' + room['id'], {'type': 'action', 'version': room['version'], 'nonce': uuid.uuid4().hex, 'action': {'type': 'sg_play', 'cards': [fixture['card']]}})
    browser(1, 'wait', '--fn', '[...document.querySelectorAll(".sg-actions button")].some(b=>b.textContent.trim()==="放弃")')
    for player, name in [(0, 'source-no-counter'), (1, 'counter-holder'), (2, 'other-no-counter-mobile'), (4, 'spectator-mobile')]:
        checkpoint(player, name, player == 1)
    browser(1, 'reload'); browser(1, 'wait', '.sg-board')
    checkpoint(1, 'holder-reloaded', True)
    browser(1, 'find', 'role', 'button', 'click', '--name', '放弃')
    browser(0, 'wait', '--fn', 'document.querySelector(".sg-actions strong")?.textContent.includes("轮到你出牌")')
    for player in [0, 1, 2, 4]:
        checkpoint(player, 'resolved-' + str(player), False)
    room = api(0, '/api/state')['room']
    assert len(room['game']['sanguosha']['players'][0]['hand']) == 2
    assert not room['game']['sanguosha'].get('pending')
    report['passed'] = True
except Exception as error:
    report['error'] = repr(error)
    traceback.print_exc()
finally:
    for player in opened:
        try: browser(player, 'close')
        except Exception as error: report.setdefault('cleanupErrors', []).append(str(error))
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    if handle: handle.close()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
