#!/usr/bin/env python3
"""Real room checks for staged attackers, native anti-air play and settlement.

Use the normal opt-in browser server plus TestPVZFollowupResultBrowserFixture.
The result fixture controls its initial state; this script never injects results.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/followup-ui'
OUT.mkdir(parents=True, exist_ok=True)
REPORT = {'passed': False, 'scope': 'authenticated production rooms, real 90-second unlock and native balloon/blover actions; engine-resolved controlled settlement fixture',
          'command': 'PVZ_BROWSER_URL=<normal fixture> PVZ_RESULT_URL=<result fixture> python3 -B games/pvz/tests/followup-ui-browser.py'}
CLIENTS = []


class Browser:
    def __init__(self, base, role):
        self.env = dict(os.environ, AGENT_BROWSER_SESSION='pui' + uuid.uuid4().hex[:6],
                        AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ab'),
                        AGENT_BROWSER_PROFILE=str(OUT / ('profile-' + role)), TMPDIR=str(OUT / 'tmp'),
                        AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
        for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']:
            Path(self.env[key]).mkdir(parents=True, exist_ok=True)
        CLIENTS.append(self)
        self.b('open', base)
        self.api('/register', {'name': '对战回归' + uuid.uuid4().hex[:6], 'password': 'pvz-browser-local', 'invite': 'pvz-browser'})
        self.b('reload')
        self.b('set', 'viewport', 1440, 960)

    def b(self, *args):
        r = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=60)
        assert r.returncode == 0, r.stdout + r.stderr
        v = json.loads(r.stdout)
        assert v['success'], v
        return v['data']

    def js(self, code):
        return self.b('eval', code)['result']

    def api(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        r = self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
        assert 200 <= r['status'] < 300, r
        return r['body']

    def room(self):
        return self.api('/state')['room']

    def command(self, kind, **extra):
        r = self.room()
        return self.api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))

    def select(self, kind):
        selector = '[data-pvz-card=' + json.dumps(kind) + ']'
        if not self.js('document.querySelector(' + json.dumps(selector) + ').getAttribute("aria-pressed")==="true"'):
            self.b('click', selector)
        assert self.js('document.querySelector(' + json.dumps(selector) + ').getAttribute("aria-pressed")==="true"')

    def lawn_click(self, x, y):
        self.b('scrollintoview', '[data-pvz-canvas]')
        rect = self.js('document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON()')
        self.b('mouse', 'move', round(rect['left'] + rect['width'] * x / 800), round(rect['top'] + rect['height'] * y / 600))
        self.b('mouse', 'down')
        self.b('mouse', 'up')


def wait(fn, seconds=45):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn():
            return
        time.sleep(.2)
    raise AssertionError('condition timed out')


def audit_result(browser, name):
    detail = browser.js('''(()=>{const root=document.querySelector('[data-pvz-outcome]'),modal=root?.closest('[role=dialog]');return {reason:root?.querySelector('[data-pvz-outcome-reason]')?.innerText,rows:[...root.querySelectorAll('tbody tr')].map(e=>e.innerText),overflow:document.documentElement.scrollWidth>innerWidth+1,cells:[...root.querySelectorAll('th,td,p')].map(e=>{const r=e.getBoundingClientRect();return {text:e.innerText,left:r.left,right:r.right,width:r.width,height:r.height}}),inFullscreen:!!document.querySelector('[data-room-fullscreen]')?.contains(modal)}})()''')
    assert detail['reason'] == 'A队获胜：双方均攻破，A队用时 1:20.00，比B队快 0.05 秒。', detail
    assert len(detail['rows']) == 2 and '1:20.05' in detail['rows'][0] and '1:20.00' in detail['rows'][1], detail
    assert not detail['overflow'], detail
    width = browser.js('innerWidth')
    assert all(c['left'] >= 0 and c['right'] <= width + 1 and c['height'] > 0 for c in detail['cells']), detail
    browser.b('screenshot', OUT / (name + '.png'), '--full')
    REPORT[name] = detail


try:
    normal, terminal = os.environ['PVZ_BROWSER_URL'], os.environ['PVZ_RESULT_URL']
    defense, attack = Browser(normal, 'defense'), Browser(normal, 'attack')
    created = defense.api('/rooms', {'kind': 'pvz', 'name': '特殊僵尸解锁与对空回归', 'capacity': 2,
                                    'options': {'mode': 'duel', 'teams': [0, 1], 'scene': 'day', 'roundSeconds': 180, 'prepSeconds': 10}})
    attack.api('/rooms/' + created['id'], {'type': 'join', 'version': defense.room()['version'], 'nonce': uuid.uuid4().hex})
    defense.command('ready')
    attack.command('ready')
    defense.command('start')
    defense.b('wait', '[data-pvz-board][data-ready=true]')
    attack.b('wait', '[data-pvz-board][data-ready=true]')
    wait(lambda: not defense.room().get('loading'))
    opening = defense.room()['game']['data']
    assert opening['players'][0]['money'] == 500 and 'blover' in opening['players'][0]['deck'], opening['players'][0]
    wait(lambda: attack.room()['game']['data']['phase'] == 'battle')
    locks = attack.js('Object.fromEntries(["balloon","digger","gargantuar"].map(k=>[k,document.querySelector(`[data-pvz-card=${k}]`).innerText]))')
    assert all('解锁' in text for text in locks.values()), locks
    REPORT['openingLocks'] = locks
    attack.b('screenshot', OUT / 'special-units-locked.png', '--full')
    attack.select('balloon')
    attack.lawn_click(780, 130)
    wait(lambda: attack.js('document.querySelector(".pvz-action-error").innerText.includes("90 秒解锁")'))
    assert not attack.room()['game']['data']['zombies'], 'locked attacker was deployed'
    attack.b('set', 'viewport', 320, 760)
    assert not attack.js('document.documentElement.scrollWidth>innerWidth+1')
    attack.b('screenshot', OUT / 'special-units-locked-320.png', '--full')
    attack.b('set', 'viewport', 1440, 960)

    # Use the unlock waiting period to verify the separate real result overlay.
    result = Browser(terminal, 'result')
    result.api('/rooms', {'kind': 'pvz', 'name': '换边胜负原因回归', 'capacity': 2,
                          'options': {'mode': 'duel', 'teams': [0, 1], 'scene': 'day', 'roundSeconds': 180, 'prepSeconds': 10}})
    result.command('add_bot')
    result.command('ready')
    result.command('start')
    result.b('wait', '[data-pvz-outcome]')
    resolved = result.room()
    assert resolved['game']['data']['winnerTeam'] == 0 and resolved['game']['winners'] == [0], resolved['game']
    REPORT['authoritativeResult'] = {'winners': resolved['game']['winners'], 'rounds': resolved['game']['data']['rounds']}
    audit_result(result, 'result-desktop')
    result.b('set', 'viewport', 320, 760)
    audit_result(result, 'result-320')
    result.b('press', 'Escape')
    result.b('click', 'button[aria-label="全屏"]')
    result.b('wait', '[data-room-fullscreen]')
    result.b('click', '.result-reopen')
    audit_result(result, 'result-fullscreen-320')
    assert REPORT['result-fullscreen-320']['inFullscreen']
    result.b('reload')
    result.b('wait', '[data-pvz-outcome]')
    audit_result(result, 'result-after-reload-320')

    # No state/time injection: let the normal authoritative battle clock reach 90s.
    wait(lambda: not attack.js('document.querySelector("[data-pvz-card=balloon]").innerText.includes("解锁")'), seconds=100)
    attack.select('balloon')
    attack.lawn_click(780, 130)
    wait(lambda: any(z['kind'] == 'balloon' and z.get('altitude', 0) > 0 for z in defense.room()['game']['data']['zombies']))
    balloon = defense.room()['game']['data']
    REPORT['balloonDeployment'] = {'tick': balloon['tick'], 'zombies': balloon['zombies'], 'money': balloon['players'][1]['money']}
    defense.b('screenshot', OUT / 'balloon-before-blover.png', '--full')
    defense.select('blover')
    defense.lawn_click(80, 130)
    wait(lambda: not any(z['kind'] == 'balloon' for z in defense.room()['game']['data']['zombies']))
    cleared = defense.room()['game']['data']
    assert cleared['phase'] == 'battle' and cleared['mowers'][0]['state'] == 'ready', cleared
    REPORT['antiAirResolved'] = {'tick': cleared['tick'], 'phase': cleared['phase'], 'mower': cleared['mowers'][0], 'events': cleared['events']}
    defense.b('screenshot', OUT / 'balloon-cleared-by-blover.png', '--full')
    REPORT['passed'] = True
finally:
    for client in CLIENTS:
        try:
            client.command('close')
            client.command('leave')
        except Exception:
            pass
        try:
            client.b('close')
        except Exception:
            pass
        shutil.rmtree(client.env['AGENT_BROWSER_PROFILE'], ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed']}))
