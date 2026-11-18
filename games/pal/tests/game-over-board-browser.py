import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pal/game-over-board-browser'
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
BASE = ''
for attempt in range(300):
    try:
        candidate = (OUT / 'url').read_text().strip()
        with urllib.request.urlopen(candidate, timeout=1) as response:
            if response.status == 200:
                BASE = candidate
                break
    except (OSError, ValueError):
        pass
    time.sleep(0.1)
assert BASE, 'earned original game-over fixture is not ready'
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-failure-board'], cwd=ROOT, env=ENV, text=True).strip()

class Browser:
    def __init__(self, index):
        self.session = SESSION + '-' + str(index)
        self.profile = OUT / ('profile-' + str(index))
        self.env = dict(ENV, AGENT_BROWSER_PROFILE=str(self.profile))

    def run(self, *args):
        result = subprocess.run(['agent-browser', '--session', self.session, '--json', *map(str, args)], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=50)
        assert result.returncode == 0, result.stdout + result.stderr
        value = json.loads(result.stdout)
        assert value['success'], value
        return value['data']

    def js(self, code):
        return self.run('eval', code)['result']

    def request(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        return self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')

    def api(self, path, body=None):
        value = self.request(path, body)
        assert 200 <= value['status'] < 300, value
        return value['body']

    def room(self):
        return self.api('/state').get('room')

    def data(self):
        return self.room()['game']['data']

    def command(self, kind):
        room = self.room()
        return self.api('/rooms/' + room['id'], {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex})

    def touch(self, selector=None, point=None):
        if point is None:
            point = self.js('(()=>{const r=document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        subprocess.run(['node', 'scripts/browser_touch.mjs', self.run('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, point + point)], cwd=ROOT, env=self.env, check=True)

    def canvas_touch(self, x, y):
        point = self.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+r.width*' + str(x / 320) + ',r.y+r.height*' + str(y / 200) + ']})()')
        self.touch(point=point)

def wait(check, label, seconds=50):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if check():
            return
        time.sleep(0.12)
    raise AssertionError(label)

def glyphs(view):
    if not view.get('raster'):
        return view.get('overlay', [])
    graph = view['raster']
    seen, rows = set(), []
    def visit(index):
        if not index or index in seen:
            return
        seen.add(index)
        n = graph['nodes'][index - 1]
        visit(n[1])
        visit(n[2])
        if n[0] == 2:
            rows.append({'kind': 0, 'id': n[5], 'x': n[6], 'y': n[7], 'color': n[8]})
    visit(graph['root'])
    return rows

def phone_defend(owner):
    b = owner.data()['battle']
    if b['menu'] == 0:
        owner.canvas_touch(39, 177)
        if b.get('selected') != 3:
            wait(lambda: owner.data()['battle'].get('selected') == 3, 'phone original misc icon was not selected')
            owner.canvas_touch(39, 177)
        wait(lambda: owner.data()['battle'].get('menu') == 4, 'phone original misc menu did not open')
    def defense():
        return next((op for op in reversed(glyphs(owner.data())) if op['kind'] == 0 and op['id'] == ord('防') and op['x'] == 16 and 32 <= op['y'] <= 104 and op['color']), None)
    wait(lambda: defense() is not None, 'original defend entry did not render')
    op = defense()
    owner.canvas_touch(op['x'] + 8, op['y'] + 8)
    if not 249 <= op['color'] <= 254:
        wait(lambda: defense() and 249 <= defense()['color'] <= 254, 'phone defend entry was not selected')
        owner.canvas_touch(op['x'] + 8, op['y'] + 8)

players = [Browser(0), Browser(1)]
device = None
report = {'passed': False, 'scope': 'actual earned original rescue battle on platform Board, ordinary keyboard/touch defense and natural defeat, original failure reading, failure save-exit preserving healthy manual point, private recovery and new-journey restart; not full campaign, music, performance or release acceptance', 'cases': []}
try:
    owner, stranger = players
    for player in players:
        player.run('open', BASE)
        player.run('set', 'viewport', 1280, 900)
        player.api('/register', {'name': '真实战败' + uuid.uuid4().hex[:6], 'password': 'native-failure-123', 'invite': 'test-invite'})
    for phone in [False, True]:
        if phone:
            owner.run('set', 'viewport', 320, 820)
            device = subprocess.Popen(['node', 'scripts/browser_device.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, '320', '820', '--keep'], cwd=ROOT, env=owner.env, text=True, stdout=subprocess.PIPE)
            assert device.stdout.readline().strip() == 'ready'
        room = owner.api('/rooms', {'kind': 'pal', 'name': '原版真实战败', 'capacity': 1})
        owner.command('ready')
        owner.command('start')
        owner.run('reload')
        wait(lambda: owner.room() and not owner.room().get('loading'), 'earned rescue battle did not load', 80)
        match_id = owner.room()['matchId']
        if phone:
            owner.run('click', '[data-save-game]')
            wait(lambda: not owner.room(), 'healthy manual save did not release the seat')
            healthy = json.loads((OUT / 'frozen-view.json').read_text())
            owner.run('click', '[data-resume-save]')
            wait(lambda: owner.room() and not owner.room().get('loading'), 'healthy original battle did not resume', 80)
        owner.run('click', '[data-fullscreen-enter]')
        owner.run('wait', '[data-room-fullscreen]')
        expected = [320, 200] if phone else [1280, 800]
        assert owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == expected
        initial = owner.data()
        assert initial['phase'] == 'battle' and [p['role'] for p in initial['party']] == [0, 1] and all(p['maxHP'] < 9999 and p['controller'] == 0 for p in initial['party'])
        commands, seen_reading, failure_saved = 0, False, False
        for step in range(1200):
            v = owner.data()
            assert not owner.room()['game'].get('finished') and v['cash'] == initial['cash']
            if v['phase'] == 'gameover':
                break
            if phone and v.get('gameOver') and v.get('paletteEffect', {}).get('redStep', 0) >= 16 and not failure_saved:
                owner.run('screenshot', OUT / 'phone-red-before-save.png')
                owner.touch('[data-room-fullscreen] [data-save-game]')
                wait(lambda: not owner.room(), 'failed red-animation save did not release the seat')
                failed_frozen = json.loads((OUT / 'frozen-view.json').read_text())
                assert failed_frozen.get('gameOver')
                assert stranger.api('/state')['saves'] == []
                owner.run('click', '[data-resume-save]')
                wait(lambda: owner.room() and not owner.room().get('loading'), 'saved failure animation did not resume', 80)
                owner.run('click', '[data-fullscreen-enter]')
                owner.run('wait', '[data-room-fullscreen]')
                failure_saved = True
                continue
            if v.get('dialogue'):
                text = v['dialogue']['text']
                assert '胜败乃兵家常事也' in text and '大侠请重新来过吧' in text
                seen_reading = True
                owner.run('screenshot', OUT / ('phone-failure-reading.png' if phone else 'desktop-failure-reading.png'))
                if phone:
                    owner.canvas_touch(160, 100)
                else:
                    owner.run('click', '[data-pal-board] canvas')
            elif v.get('battle') and v['battle']['phase'] == 0 and v['battle']['state'] == 1 and v['seq'] == v['appliedSeq']:
                if phone:
                    phone_defend(owner)
                else:
                    owner.run('press', 'g')
                commands += 1
            time.sleep(0.12)
        else:
            raise AssertionError('ordinary defense did not reach natural original defeat')
        assert seen_reading and commands > 0 and owner.room()['game'].get('restartable') is True
        owner.run('screenshot', OUT / ('phone-failure-ready.png' if phone else 'desktop-failure-ready.png'))
        if not phone:
            assert owner.js('document.querySelector("[data-room-fullscreen] [data-load-checkpoint]").disabled')
            owner.run('click', '[data-room-fullscreen] [data-restart-journey]')
            owner.run('wait', '[data-cancel-restart]')
            owner.run('click', '[data-cancel-restart]')
            assert owner.room()['matchId'] == match_id and owner.data()['phase'] == 'gameover'
            owner.run('click', '[data-room-fullscreen] [data-restart-journey]')
            owner.run('wait', '[data-confirm-restart]')
            owner.run('click', '[data-confirm-restart]')
            wait(lambda: owner.room()['matchId'] != match_id and not owner.room().get('loading'), 'failed journey did not restart through original new game', 80)
            assert owner.js('!!document.querySelector("[data-room-fullscreen]")'), 'new journey shrank the existing fullscreen game area'
            assert not owner.data().get('gameOver') and not owner.room().get('checkpointSavedAt')
            wait(lambda: owner.data().get('dialogue') is not None, 'new original journey did not reach an operable first page')
            first_prompt = owner.data()['dialogue']['id']
            owner.run('click', '[data-pal-board] canvas')
            wait(lambda: owner.data().get('dialogue', {}).get('id') != first_prompt, 'new journey retained old input ownership')
            report['cases'].append({'device': 'desktop', 'nativeKeyboardDefense': commands, 'actualFailureReading': True, 'noInventedManualSave': True, 'cancelRestart': True, 'newMatchRestart': True, 'newOriginalPageOperable': True, 'fullscreenRetained': True, 'canvas': expected})
            owner.command('close')
            owner.command('leave')
        else:
            assert failure_saved
            owner.touch('[data-room-fullscreen] [data-load-checkpoint]')
            owner.run('wait', '[data-confirm-checkpoint]')
            owner.touch('[data-confirm-checkpoint]')
            wait(lambda: not owner.room().get('loading') and owner.data()['phase'] == 'battle' and not owner.data().get('gameOver'), 'failed save replaced the healthy actual manual point', 80)
            loaded = json.loads((OUT / 'loaded-view.json').read_text())
            normalize = lambda v: {k: value for k, value in v.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
            assert normalize(loaded) == normalize(healthy)
            assert owner.room()['matchId'] == match_id
            assert owner.js('!!document.querySelector("[data-room-fullscreen]")')
            assert owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == expected
            owner.run('screenshot', OUT / 'phone-healthy-recovered.png')
            phone_defend(owner)
            wait(lambda: owner.data().get('battle', {}).get('phase') == 1 or any(p.get('state') == 2 and p.get('action') == 1 for p in owner.data().get('battle', {}).get('players', [])), 'restored original battle did not accept a fresh touch defense')
            report['cases'].append({'device': 'phone', 'nativeTouchDefense': commands, 'actualFailureReading': True, 'failedAnimationSaveExit': True, 'healthyManualBaselineRetained': True, 'exactLoadedBoundary': True, 'freshTouchDefenseAccepted': True, 'sameMatch': True, 'canvas': expected})
    report['passed'] = True
except Exception as error:
    report['error'] = str(error)
    try:
        report['failureRoom'] = players[0].room()
        players[0].run('screenshot', OUT / 'failure.png')
    except Exception:
        pass
    raise
finally:
    if device:
        device.terminate()
        device.wait(timeout=5)
    for player in players:
        try:
            player.run('close')
        except Exception:
            pass
        shutil.rmtree(player.profile, ignore_errors=True)
    (OUT / 'stop').write_text('done\n')
    (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(report, ensure_ascii=False))
