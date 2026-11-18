#!/usr/bin/env python3
"""Test-first shared-save acceptance using native browser clicks and touch.

Run the opt-in TestSavedGameBrowserFixture with SAVED_GAME_BROWSER=1 first.
This fixture validates the shared platform, not the unfinished PAL game.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.local/saved-games/browser'
OUT.mkdir(parents=True, exist_ok=True)
BASE = (OUT / 'url').read_text().strip()
BASE_ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/sg'),
                TMPDIR=str(OUT / 'tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1',
                AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'TMPDIR']:
    Path(BASE_ENV[key]).mkdir(parents=True, exist_ok=True)
session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'sv'], cwd=ROOT, env=BASE_ENV, text=True).strip()
REPORT = {'passed': False, 'scope': 'production platform UI; test-only Gomoku opt-in, not PAL gameplay',
          'command': 'python3 -B scripts/accept_saved_games.py', 'cases': []}


class Browser:
    def __init__(self, name):
        self.name = name
        self.env = dict(BASE_ENV, AGENT_BROWSER_PROFILE=str(OUT / ('profile-' + name)))
        Path(self.env['AGENT_BROWSER_PROFILE']).mkdir(parents=True, exist_ok=True)

    def b(self, *args):
        result = subprocess.run(['agent-browser', '--session', session + '-' + self.name, *map(str, args), '--json'],
                                cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=40)
        assert result.returncode == 0, result.stdout + result.stderr
        data = json.loads(result.stdout)
        assert data['success'], data
        return data['data']

    def js(self, code):
        return self.b('eval', code)['result']

    def api(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        result = self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
        assert 200 <= result['status'] < 300, result
        return result['body']

    def room(self):
        return self.api('/state').get('room')

    def command(self, kind, **extra):
        room = self.room()
        return self.api('/rooms/' + room['id'], dict(type=kind, version=room['version'], nonce=uuid.uuid4().hex, **extra))

    def tap(self, selector):
        self.js('document.querySelector(' + json.dumps(selector) + ').scrollIntoView({block:"center",behavior:"instant"})')
        x, y = self.js('(()=>{const r=document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        result = subprocess.run(['node', 'scripts/browser_touch.mjs', self.b('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, [x, y, x, y])], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=20)
        assert result.returncode == 0, result.stdout + result.stderr


def wait(check):
    deadline = time.monotonic() + 25
    while time.monotonic() < deadline:
        if check(): return
        time.sleep(.15)
    raise AssertionError('expected saved-game state did not arrive')


players = [Browser('a'), Browser('b')]
try:
    for i, p in enumerate(players):
        p.b('open', BASE)
        p.api('/register', {'name': '存档浏览器' + uuid.uuid4().hex[:6], 'password': 'browser-save-test-123', 'invite': 'test-invite'})
    a, b = players
    room = a.api('/rooms', {'kind': 'gomoku', 'name': '保存后继续的旅程', 'capacity': 2, 'options': {'turnSeconds': 300}})
    b.api('/rooms/' + room['id'], {'type': 'join', 'version': room['version'], 'nonce': uuid.uuid4().hex})
    for p in players: p.command('ready')
    a.command('start')
    for p in players: p.b('reload')
    wait(lambda: not a.room().get('loading'))
    for p in players: p.b('wait', '[data-save-game]')
    before = a.room()
    actor = players[before['game']['data']['turn']]
    state = actor.room()['game']['data']
    actor.command('action', action={'type': 'place', 'round': state['round'], 'ply': state['ply'], 'x': 7, 'y': 7})
    before = a.room()
    a.b('click', '[data-save-game]')
    wait(lambda: len(a.room().get('saveVotes', [])) == 1)
    wait(lambda: b.js('!!document.querySelector("[data-decline-save]")'))
    b.b('set', 'viewport', 320, 820)
    b.b('screenshot', OUT / 'decline-phone.png', '--full')
    b.tap('[data-decline-save]')
    wait(lambda: not a.room().get('saveVotes') and not a.room()['game'].get('paused'))
    assert before['game']['data']['board'] == a.room()['game']['data']['board'], 'declining save changed progress'
    REPORT['cases'].append({'check': 'non-approving teammate can decline frozen save using native 320px touch', 'passed': True})
    b.b('set', 'viewport', 1280, 900)
    a.b('click', '[data-save-game]')
    wait(lambda: len(a.room().get('saveVotes', [])) == 1)
    a.b('screenshot', OUT / 'approval-desktop.png', '--full')
    b.b('click', '[data-save-game]')
    wait(lambda: not a.room() and not b.room())
    for p in players: p.b('wait', '[data-resume-save]')
    a.b('screenshot', OUT / 'saves-desktop.png', '--full')
    a.b('set', 'viewport', 320, 820)
    assert not a.js('document.documentElement.scrollWidth>innerWidth+1'), 'saved list overflows phone width'
    a.b('screenshot', OUT / 'saves-phone.png', '--full')
    # Saving must also leave the ordinary create-room action reachable.
    a.tap('.lobby-create')
    a.b('wait', '[role="dialog"]')
    a.b('click', 'button[aria-label="关闭"]')
    a.tap('[data-resume-save]')
    a.b('wait', '[data-saved-restoring]')
    assert a.room()['status'] == 'restoring' and not b.room(), 'absent player was occupied or play resumed early'
    a.b('screenshot', OUT / 'restoring-phone.png', '--full')
    b.b('click', '[data-resume-save]')
    wait(lambda: a.room()['status'] == 'playing' and not a.room().get('loading'))
    after = a.room()
    assert before['matchId'] == after['matchId'] and before['game']['data']['board'] == after['game']['data']['board'], 'resume changed identity or board'
    for p in players:
        wait(lambda p=p: p.js('!!document.querySelector("[data-save-game]") && ![...document.querySelectorAll(".toast")].some(e=>/已离开|已保存/.test(e.textContent))'))
    a.b('screenshot', OUT / 'resumed-phone.png', '--full')
    REPORT['cases'].append({'check': 'two players save, release seats and resume through desktop and native 320px touch', 'passed': True,
                            'matchIDPreserved': True, 'boardPreserved': True})
    a.b('click', 'button[aria-label="全屏"]')
    a.b('wait', '[data-room-fullscreen]')
    assert a.js('!!document.querySelector("[data-room-fullscreen] [data-save-game]")'), 'save control is missing inside fullscreen'
    assert a.js('(()=>{const r=document.querySelector("[data-room-fullscreen] [data-save-game]").getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})()'), 'fullscreen save is outside the phone viewport'
    a.b('screenshot', OUT / 'save-fullscreen-phone.png', '--full')
    a.tap('[data-room-fullscreen] [data-save-game]')
    b.b('click', '[data-save-game]')
    wait(lambda: not a.room() and not b.room())
    REPORT['cases'].append({'check': 'save remains reachable through native phone touch inside shared fullscreen', 'passed': True})
    # Existing non-opted-in games keep their original controls.
    a.api('/rooms', {'kind': 'splendor', 'name': '普通对局', 'capacity': 2})
    a.command('add_bot'); a.command('ready'); a.command('start')
    a.b('reload')
    wait(lambda: not a.room().get('loading'))
    assert not a.js('!!document.querySelector("[data-save-game]")'), 'save enabled on a game without opt-in'
    REPORT['cases'].append({'check': 'save control absent for a non-opted-in game', 'passed': True})
    a.command('close'); a.command('leave')
    REPORT['passed'] = True
finally:
    REPORT['distIndexSHA256'] = hashlib.sha256((ROOT / 'web/dist/index.html').read_bytes()).hexdigest()
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    for p in players:
        try: p.b('close')
        except Exception: pass
        shutil.rmtree(p.env['AGENT_BROWSER_PROFILE'], ignore_errors=True)
    print(json.dumps({'passed': REPORT['passed'], 'result': str(OUT / 'result.json')}, ensure_ascii=False))
