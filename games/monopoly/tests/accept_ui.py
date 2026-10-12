#!/usr/bin/env python3
"""Actual Chromium/HTTP/WebSocket Monopoly UI acceptance, written before the UI.

Failure scenarios: missing discovery/setup; state lost after reload; wrong seat
actions; desktop/mobile overflow; hidden or blocked controls; misleading auction
prompts; stale private proposals; mismatched building/mortgage buttons; debt not
settled through asset controls; bankrupt player cannot leave; spectator acts. Media failures: missing CDN images,
wrong audio MIME, muted/reloaded events replay, autoplay refusal or failed audio
blocks game actions, failed audio never recovers, global sound plays twice.
All fixture state is declared in browser_fixture_test.go, not production code.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.request
import uuid

root = Path(__file__).resolve().parents[3]
out = root / '.local/monopoly/acceptance/browser'
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
for key, relative in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/ab-monopoly-ui', 'AGENT_BROWSER_PROFILE': '.local/monopoly/acceptance/browser/profile'}.items():
    path = root / relative
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_MONOPOLY_BROWSER='1', AGENT_BROWSER_SESSION='monopoly-ui-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'command': 'python3 -B games/monopoly/tests/accept_ui.py', 'scope': 'Real production UI; deterministic natural setup and explicit economic fixtures; browser muted', 'checks': []}
server = None
browser_open = False
handles = []


def run(args, logfile):
    with (out / logfile).open('w') as h:
        result = subprocess.run(args, cwd=root, env=env, stdout=h, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f'{args} failed; see {logfile}')


def browser(*args):
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=40)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value['success']:
        raise RuntimeError(str(value))
    return value['data']


def js(code):
    return browser('eval', code)['result']


def api(path, body=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('fetch(' + json.dumps('/api' + path) + opts + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= value['status'] < 300, value
    return value['body']


def room():
    return api('/state')['room']


def command(kind, **extra):
    r = room()
    return api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))


class Peer:
    def __init__(self, base, name):
        self.base, self.name = base, name
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.api('/register', dict(name=name, password='monopoly-browser-test', invite='monopoly-browser'))
        self.id = self.api('/state')['user']['id']

    def api(self, path, body=None):
        raw = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(self.base + '/api' + path, data=raw, headers={'Content-Type': 'application/json'})
        with self.opener.open(req, timeout=10) as response:
            return json.load(response)

    def command(self, kind, r=None, **extra):
        r = r or self.api('/state')['room']
        return self.api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))

    def act(self, **action):
        r = self.api('/state')['room']
        action.setdefault('window', r['game']['data']['window'])
        return self.command('action', r, action=action)


def checkpoint(name):
    assert not browser('errors')['errors'], browser('errors')
    assert js('document.documentElement.scrollWidth <= innerWidth + 2'), 'horizontal overflow: ' + name
    browser('screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(browser('snapshot', '-i'), ensure_ascii=False, indent=2))
    report['checks'].append(name)
    print('PASS', name, flush=True)


def click(text):
    expected = json.dumps(text)
    wait_js(f'[...document.querySelectorAll("button")].some(b=>(b.textContent.trim()==={expected}||b.getAttribute("aria-label")==={expected})&&!b.matches(":disabled"))')
    browser('find', 'role', 'button', 'click', '--name', text, '--exact')


def wait_js(expression):
    browser('wait', '--fn', expression)


def wait_cash(amount, player=0):
    selector = json.dumps(f'[data-mono-cash="{player}"]')
    wait_js(f'document.querySelector({selector})?.textContent.includes("{amount}")')


def login(name):
    api('/logout', {})
    api('/login', {'name': name, 'password': 'monopoly-browser-test'})
    browser('reload')


def start(scenario, create_in_ui=False):
    if create_in_ui:
        index = js('[...document.querySelectorAll(".game-feature")].findIndex(e=>e.textContent.includes("强手棋"))+1')
        assert index > 0, 'Monopoly is absent from lobby'
        browser('click', f'.game-feature:nth-child({index}) button')
        browser('wait', '[role=dialog]')
        browser('select', '[role=dialog] select:not([aria-label])', '3')
        browser('fill', 'input[name=name]', '强手棋浏览器验收')
        browser('click', '[role=dialog] button[type=submit]')
        browser('wait', '.waiting')
    else:
        api('/rooms', dict(kind='monopoly', name='强手棋 ' + scenario, capacity=3, options={'scenario': scenario, 'turnSeconds': 300}))
    for p in peers:
        p.command('join', room())
    command('ready')
    if create_in_ui:
        browser('fill', 'input[aria-label="行动时限（秒）"]', '300')
        click('保存设置')
        wait_js('!document.querySelector(".waiting-actions > button:first-child")?.disabled')
        assert not room()['seats'][0]['ready'], 'settings did not clear readiness'
        click('准备')
    for p in peers:
        p.command('ready')
    if create_in_ui:
        wait_js('[...document.querySelectorAll(".waiting-actions button")].some(b=>b.textContent==="开始游戏"&&!b.disabled)')
        click('开始游戏')
    else:
        command('start')
    browser('wait', '[data-monopoly-board]')


def finish():
    r = api('/state').get('room')
    if r and r['status'] == 'playing':
        command('close')
        browser('wait', '--text', '这张牌桌已结束')
        assert js('[...document.querySelectorAll(".mono-game-action")].every(b=>b.disabled)'), 'closed game has active controls'
    if r:
        command('leave')
    for p in peers:
        r = p.api('/state').get('room')
        if r and r['status'] == 'playing' and r['host'] == p.id:
            p.command('close')
    for p in peers:
        if p.api('/state').get('room'):
            p.command('leave')
    browser('wait', '.game-selection')


try:
    for name in ['url', 'stop']:
        (out / name).unlink(missing_ok=True)
    run(['npm', '--prefix', 'web', 'run', 'build'], 'build.log')
    h = (out / 'server.log').open('w'); handles.append(h)
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=16m', '-run', '^TestMonopolyBrowserFixture$', './games/monopoly/tests'], cwd=root, env=env, stdout=h, stderr=subprocess.STDOUT)
    for _ in range(900):
        if (out / 'url').exists(): break
        if server.poll() is not None: raise RuntimeError('fixture server exited')
        time.sleep(.1)
    else: raise RuntimeError('fixture server did not start')
    base = (out / 'url').read_text()
    browser('open', base); browser_open = True
    main_name = '强手棋体验'
    api('/register', dict(name=main_name, password='monopoly-browser-test', invite='monopoly-browser'))
    peers = [Peer(base, '同桌甲'), Peer(base, '同桌乙')]
    spectator = Peer(base, '围观玩家')
    browser('reload'); browser('wait', '.game-selection')
    browser('set', 'viewport', '1440', '1000')
    assert 'monopoly' not in api('/state')['availableGames']
    assert not js('[...document.querySelectorAll(".game-feature")].some(e=>e.textContent.includes("强手棋"))'), 'unpublished game visible to player'
    checkpoint('default-unpublished-player-lobby')
    login('上架管理员')
    browser('wait', '--text', '管理后台')
    click('管理后台')
    browser('wait', '--text', '游戏上架')
    click('游戏上架')
    # Exercise the actual admin UI, not an automatic publication side channel.
    browser('snapshot', '-i')
    index = js('[...document.querySelectorAll(".admin-game-card")].findIndex(e=>e.textContent.includes("强手棋"))+1')
    assert index > 0
    browser('click', f'.admin-game-card:nth-child({index}) button')
    browser('wait', '[role=dialog]')
    click('确认操作')
    wait_js('[...document.querySelectorAll(".admin-game-card")].some(e=>e.textContent.includes("强手棋")&&e.textContent.includes("已上架"))')
    assert 'monopoly' in api('/state')['availableGames']
    checkpoint('admin-published-monopoly')
    login(main_name); browser('open', base); browser('wait', '.game-selection')
    start('natural', True)
    media = bool(env.get('WIRE_BOARD_MONOPOLY_ASSETS'))
    if media:
        wait_js('[...document.querySelectorAll("[data-monopoly-board] img")].length>=10&&[...document.querySelectorAll("[data-monopoly-board] img")].every(i=>i.complete&&i.naturalWidth>0)')
        js('window.__media=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return play.call(this).then(()=>window.__media.push(src))}')
        assert js('document.querySelector("audio[data-mono-sound]")?.paused === true'), 'initial snapshot replayed sound'
    click('掷骰')
    browser('wait', '[data-mono-purchase]')
    if media:
        wait_js('window.__media.some(s=>s.endsWith("/rolling.mp3"))')
        assert room()['game']['data']['rollSerial'] == 1
        click('关闭音效')
        wait_js('document.querySelector("audio[data-mono-sound]").paused')
        sounds_before = js('window.__media.length')
    click('购买 · 100')
    wait_cash(1400)
    if media:
        assert js('window.__media.length') == sounds_before, 'muted purchase played sound'
        click('开启音效')
        assert js('window.__media.length') == sounds_before, 'unmute replayed stale sound'
    before = room()['game']['data']
    browser('reload'); browser('wait', '[data-monopoly-board]')
    assert room()['game']['data'] == before
    if media:
        assert js('document.querySelector("audio[data-mono-sound]").paused && document.querySelector("audio[data-mono-sound]").currentTime === 0'), 'reload replayed old events'
        report['mediaBase'] = env['WIRE_BOARD_MONOPOLY_ASSETS']
        checkpoint('cdn-images-audio-mute-reload')
    checkpoint('natural-desktop-reloaded')
    for width in [390, 320]:
        browser('set', 'viewport', str(width), '844' if width == 390 else '740')
        checkpoint('natural-mobile-' + str(width))
    assert js('[...document.querySelectorAll("[data-mono-square]")].every(e=>{const r=e.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;return y<0||y>innerHeight||e.contains(document.elementFromPoint(x,y))})'), 'floating UI blocks visible board squares'
    click('放大棋盘')
    assert js('document.querySelector(".mono-board-scroll").scrollWidth>document.querySelector(".mono-board-scroll").clientWidth'), 'zoom did not enlarge the board'
    checkpoint('natural-mobile-zoom')
    click('适应屏幕')
    finish()

    start('management')
    browser('set', 'viewport', '1440', '1000')
    browser('click', '[data-mono-square="6"]')
    browser('wait', '[data-mono-property-dialog]')
    if media:
        js('window.__media=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return play.call(this).then(()=>window.__media.push(src))}')
    click('建房 · 50')
    wait_cash(1450)
    if media: wait_js('window.__media.some(s=>s.endsWith("/buying.mp3"))')
    assert room()['game']['data']['properties'][6]['buildings'] == 1
    click('出售一座 · 25')
    wait_cash(1475)
    click('关闭')
    browser('click', '[data-mono-square="5"]')
    if media: browser('network', 'route', '**/monopoly/v1/audio/moneyplus.mp3', '--abort')
    click('抵押 · 收入 100')
    wait_cash(1575)
    if media:
        wait_js('document.querySelector("audio[data-mono-sound]").error !== null')
        browser('network', 'unroute', '**/monopoly/v1/audio/moneyplus.mp3')
        js('window.__media=[]')
        checkpoint('audio-failure-does-not-block-mortgage')
    click('关闭')
    browser('click', '[data-mono-tab="trade"]')
    browser('select', 'select[aria-label="交易对象"]', '1')
    browser('check', '[data-trade-side="give"] input[data-property="5"]')
    browser('fill', 'input[aria-label="我获得的现金"]', '100')
    assert js('document.querySelector("[data-trade-side=give] input[data-property=\\"5\\"]").checked'), 'property selection did not persist'
    assert js('document.querySelector("input[aria-label=我获得的现金]").value') == '100', 'cash edit did not persist'
    click('发送报价')
    wait_js('document.querySelector("[data-mono-offer]")!==null')
    peer_room = peers[0].api('/state')['room']
    offer = peer_room['game']['data']['offers'][0]
    peers[0].act(type='trade_accept', tradeId=offer['id'])
    wait_cash(1675)
    if media: wait_js('window.__media.some(s=>s.endsWith("/buying.mp3"))')
    peers[0].act(type='trade_offer', offer={'target': 0, 'give': {'cash': 200, 'properties': [37]}, 'want': {'properties': [6]}})
    browser('wait', '[data-mono-offer]')
    click('接受交易')
    wait_cash(1857)
    assert room()['game']['data']['properties'][37]['owner'] == 0
    checkpoint('management-and-trade-desktop')
    spectator.api('/rooms/' + room()['id'] + '/watch', {})
    login(spectator.name); browser('wait', '[data-monopoly-board]')
    assert js('[...document.querySelectorAll(".mono-game-action")].every(b=>b.disabled)'), 'spectator can act'
    checkpoint('spectator-readonly')
    login(main_name); browser('wait', '[data-monopoly-board]')
    spectator.api('/rooms/' + room()['id'] + '/watch', {'leave': True})
    finish()

    start('auction')
    browser('set', 'viewport', '390', '844')
    click('转入拍卖')
    browser('wait', '[data-mono-auction]')
    browser('fill', 'input[aria-label="出价金额"]', '100')
    click('确认出价')
    peers[0].act(type='bid', amount=150)
    wait_js('document.querySelector("[data-mono-high-bid]")?.textContent.includes("150")')
    checkpoint('auction-mobile')
    login(peers[1].name); browser('wait', '[data-mono-auction]')
    assert not js('!!document.querySelector("input[aria-label=出价金额]")'), 'unfunded player prompted to bid'
    checkpoint('auction-no-unnecessary-prompt')
    login(main_name); browser('wait', '[data-mono-auction]')
    click('退出本场竞价')
    wait_js('!document.querySelector("[data-mono-auction]")')
    assert room()['game']['data']['properties'][6]['owner'] == 1
    assert all(not s.get('autoPlay') for s in room()['seats'])
    finish()

    start('debt')
    browser('set', 'viewport', '320', '740')
    browser('wait', '[data-mono-debt]')
    checkpoint('debt-mobile')
    browser('click', '[data-mono-tab="assets"]')
    browser('click', '[data-mono-asset="5"]')
    click('抵押 · 收入 100')
    click('关闭')
    browser('click', '[data-mono-tab="action"]')
    wait_js('!document.querySelector("[data-mono-debt]")')
    assert room()['game']['data']['players'][0]['cash'] == 35
    checkpoint('debt-funded-mobile')
    finish()

    start('bankrupt')
    click('宣告破产')
    browser('wait', '[data-mono-bankrupt-confirm]')
    click('确认破产')
    wait_js('document.body.textContent.includes("你已出局")')
    assert not js('[...document.querySelectorAll("button")].some(b=>b.textContent.includes("开启托管"))')
    checkpoint('bankrupt-can-leave')
    click('离开房间')
    browser('wait', '.game-selection')
    assert api('/state').get('room') is None
    assert len(peers[0].api('/state')['room']['seats']) == 3
    finish()

    start('finish')
    click('宣告破产'); click('确认破产')
    browser('wait', '.results')
    result = room()['result']
    browser('reload'); browser('wait', '.results')
    assert room()['result'] == result
    checkpoint('finished-result-restored')
    click('再来一局')
    browser('wait', '.waiting')
    assert room()['status'] == 'waiting' and not room().get('game')
    checkpoint('rematch-waiting')
    # Close waiting-room membership without starting the fixture again.
    command('leave')
    for p in peers:
        if p.api('/state').get('room'): p.command('leave')
    browser('wait', '.game-selection')

    if '--skip-full' not in sys.argv:
        api('/rooms', dict(kind='monopoly', name='强手棋完整托管局', capacity=2, options={'scenario': 'full-bots', 'turnSeconds': 300}))
        browser('wait', '.waiting')
        click('添加电脑'); click('准备'); click('开始游戏')
        browser('wait', '[data-monopoly-board]')
        initial = room()['game']['data']
        assert all(p['cash'] == 1500 and p['position'] == 0 for p in initial['players'])
        assert all(p['owner'] == -1 for p in initial['properties'])
        browser('set', 'viewport', '1440', '1000')
        click('开启托管')
        began = time.monotonic(); sample_at = 0
        while True:
            r = room()
            elapsed = time.monotonic() - began
            if r['status'] == 'finished': break
            assert elapsed < 720, 'live bot game failed to finish within 12 minutes'
            if elapsed >= sample_at:
                print('PLAY', round(elapsed), 'seconds', 'version', r['version'], 'phase', r['game']['data']['phase'], flush=True)
                assert not browser('errors')['errors']
                if sample_at in [0, 120, 240]: checkpoint('live-bots-' + str(sample_at))
                sample_at += 30
            time.sleep(5)
        browser('wait', '.results')
        assert len(r['game']['winners']) == 1 and not r['result']['rated']
        assert sum(p['bankrupt'] for p in r['game']['data']['players']) == 1
        result = r['result']; browser('reload'); browser('wait', '.results')
        assert room()['result'] == result
        report['liveGame'] = {'seconds': round(elapsed, 2), 'result': result, 'finalVersion': r['version']}
        checkpoint('live-bots-finished-restored')
        command('leave')
    report['passed'] = True
except Exception as error:
    import traceback
    report['error'] = repr(error)
    traceback.print_exc()
    if browser_open:
        try:
            report['failureSnapshot'] = browser('snapshot', '-i')
            report['failureState'] = api('/state')
            browser('screenshot', str(out / 'failure.png'), '--full')
        except Exception as diagnostic: report['diagnosticError'] = str(diagnostic)
finally:
    if browser_open:
        try: browser('close')
        except Exception as error: report['browserCloseError'] = str(error)
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    for handle in handles: handle.close()
    report['commit'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
