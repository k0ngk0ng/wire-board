from pathlib import Path
import csv
import hashlib
import json
import os
import re
import shutil
import subprocess
import time
import uuid
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent
BASE = ROOT / '.local/hotlinemiami/browser'
REPORT_NAME = os.environ.get('HOTLINE_BROWSER_REPORT', 'current')
assert re.fullmatch(r'[a-zA-Z0-9_-]{1,40}', REPORT_NAME), 'invalid report directory name'
OUT = BASE / 'reports' / REPORT_NAME
for directory in [BASE, OUT, ROOT / '.local/hotlinemiami/tmp', ROOT / '.local/hotlinemiami/go-cache', ROOT / '.local/hotlinemiami/go-mod', ROOT / '.local/hotlinemiami/s', BASE / 'cache', BASE / 'main-profile', BASE / 'peer-profile']:
    directory.mkdir(parents=True, exist_ok=True)
ENV = dict(os.environ, TMPDIR=str(ROOT / '.local/hotlinemiami/tmp'), GOTMPDIR=str(ROOT / '.local/hotlinemiami/tmp'), GOCACHE=str(ROOT / '.local/hotlinemiami/go-cache'), GOMODCACHE=str(ROOT / '.local/hotlinemiami/go-mod'), GOTOOLCHAIN='local', AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/hotlinemiami/s'), XDG_CACHE_HOME=str(BASE / 'cache'), WIRE_BOARD_HOTLINE_BROWSER='1')
REPORT = {'passed': False, 'scope': 'production HTTP/SQLite/WS rooms, native Chromium keyboard/mouse and simulated touch; no rule/state injection', 'cases': [], 'failures': [], 'diagnostic': os.environ.get('HOTLINE_BROWSER_DIAGNOSTIC') == '1', 'budgets': {'keyboardP95MS': 33, 'touchP95MS': 50, 'frameP95MS': 20, 'frameP99MS': 33, 'minimumVisibleFPS': 55}}
SERVER = None
SESSIONS = {}
URLS = {}


def save():
    (OUT / 'report.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')


def wait_for(predicate, seconds=30):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        if SERVER is not None and SERVER.poll() is not None:
            raise AssertionError((OUT / 'server.log').read_text()[-6000:])
        try:
            result = predicate()
            if result:
                return result
        except (AssertionError, KeyError, TypeError) as error:
            last = str(error)
        time.sleep(0.1)
    raise AssertionError('condition timed out: ' + str(last))


def browser(client, *arguments, input_source=None):
    command = ['agent-browser', '--session', SESSIONS[client], '--profile', str(BASE / (client + '-profile')), *map(str, arguments), '--json']
    result = subprocess.run(command, cwd=ROOT, env=ENV, input=input_source, text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    response = json.loads(result.stdout)
    assert response['success'], response
    return response['data']


def evaluate(client, source):
    return browser(client, 'eval', '--stdin', input_source=source)['result']


def request(client, path, body=None, expected=(200, 201)):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body, ensure_ascii=False) + ')}'
    response = evaluate(client, '(async()=>{const response=await fetch(' + json.dumps('/api' + path) + options + ');return {status:response.status,body:await response.json()}})()')
    assert response['status'] in expected, response
    return response['body']


def room(client='main'):
    return request(client, '/state')['room']


def command(client, kind, **extra):
    nonce = uuid.uuid4().hex
    for attempt in range(8):
        current = room(client)
        response = evaluate(client, '(async()=>{const response=await fetch(' + json.dumps('/api/rooms/' + current['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(dict(type=kind, version=current['version'], nonce=nonce, **extra), ensure_ascii=False) + ')});return {status:response.status,body:await response.json()}})()')
        if response['status'] == 409:
            continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('room command never acquired current version: ' + kind)


def set_network(client, rtt=0, jitter=0, offline=False):
    (BASE / ('network-' + client + '.json')).write_text(json.dumps({'rtt': rtt, 'jitter': jitter, 'offline': offline}))


def input_ready(client='main'):
    return evaluate(client, '!!document.querySelector("[data-hotlinemiami-board][data-ready=true]")&&!!document.querySelector("[data-hotlinemiami-canvas]")?.__hotlineFrame?.inputEnabled')


def enabled(client, selector):
    return evaluate(client, '(()=>{const element=document.querySelector(' + json.dumps(selector) + ');return !!element&&!element.disabled})()')


def loaded():
    for client in ['main', 'peer']:
        error = evaluate(client, 'document.querySelector(".game-loading [role=alert] p")?.textContent')
        if error:
            raise RuntimeError(client + ' failed loading: ' + error)
    return not room().get('loading')


def endpoint(client):
    value = browser(client, 'get', 'cdp-url')
    return value if isinstance(value, str) else value.get('url', value.get('cdpUrl', value.get('value')))


def native(client, name, touch=False, **extra):
    path = OUT / (name + '-native.json')
    path.unlink(missing_ok=True)
    inputs = {'endpoint': endpoint(client), 'pageURL': URLS[client], 'options': dict(touch=touch, output=str(path), **extra)}
    result = subprocess.run(['node', str(HERE / 'native-input.mjs'), '--stdin'], cwd=ROOT, env=ENV, input=json.dumps(inputs), text=True, capture_output=True, timeout=45)
    (OUT / (name + '-native.log')).write_text(result.stderr)
    observation = json.loads(path.read_text()) if path.exists() else {}
    if result.returncode != 0:
        failure = {'case': name, 'error': result.stderr, 'summary': observation.get('summary', {})}
        REPORT['failures'].append(failure)
        if not REPORT['diagnostic'] or not observation:
            raise AssertionError(result.stderr + json.dumps(observation.get('summary', {})))
    with (OUT / (name + '-responses.csv')).open('w', newline='') as output:
        writer = csv.DictWriter(output, fieldnames=['kind', 'inputAt', 'visibleAt', 'milliseconds', 'pixelsChanged'])
        writer.writeheader()
        writer.writerows({key: item[key] for key in writer.fieldnames} for item in observation['responses'])
    with (OUT / (name + '-frames.csv')).open('w', newline='') as output:
        writer = csv.DictWriter(output, fieldnames=['at', 'drawCount', 'x', 'y', 'serverFrame', 'predictedFrame', 'receivedSeq', 'appliedSeq', 'inputWindow', 'pending'])
        writer.writeheader()
        writer.writerows(observation['frames'])
    return observation['summary']


def screenshot(client, name):
    browser(client, 'screenshot', OUT / (name + '.png'))


def image_audit(client):
    return evaluate(client, '([...document.querySelectorAll(".hm-preparation img,.hm-options img")].map(image=>({src:image.getAttribute("src"),loaded:image.complete&&image.naturalWidth>0,alt:image.alt,rect:image.getBoundingClientRect().toJSON()})))')


def select_loadout(client, seat, character=None, weapon=None):
    for kind, value in [('character', character), ('weapon', weapon)]:
        if value is None:
            continue
        summary = '.hm-seat-choices:nth-child(' + str(seat + 1) + ') ' + ('.hm-picker:not(.hm-weapon-picker) summary' if kind == 'character' else '.hm-weapon-picker summary')
        selector = '[data-hotlinemiami-' + kind + '="' + value + '"][data-seat="' + str(seat) + '"]'
        wait_for(lambda: enabled(client, selector))
        browser(client, 'scrollintoview', summary)
        browser(client, 'click', summary)
        browser(client, 'scrollintoview', selector)
        browser(client, 'click', selector)
        field = 'characters' if kind == 'character' else 'weapons'
        wait_for(lambda: room(client)['options'][field][seat] == value and enabled(client, selector))


def viewport_audit(client):
    return evaluate(client, '({width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth,canvas:document.querySelector("[data-hotlinemiami-canvas]")?.getBoundingClientRect().toJSON(),sticks:[...document.querySelectorAll("[data-hotlinemiami-stick]")].map(element=>({kind:element.dataset.hotlinemiamiStick,rect:element.getBoundingClientRect().toJSON()}))})')


def visual_audit(client):
    return evaluate(client, '''(() => {
      const rgb = value => (value.match(/[\\d.]+/g) || []).slice(0, 3).map(Number);
      const luminance = color => color.map(value => { value /= 255; return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4; }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
      return [...document.querySelectorAll('.hm-battle-heading strong,.hm-ammo span,.hm-timer,.hm-score strong,.hm-bottom-line span,.hm-touch-actions>button')].filter(element => element.getBoundingClientRect().width > 0).map(element => {
        let parent = element, background;
        while (parent) { background = getComputedStyle(parent).backgroundColor; if (background !== 'rgba(0, 0, 0, 0)' && background !== 'transparent') break; parent = parent.parentElement; }
        const color = getComputedStyle(element).color, foregroundLuminance = luminance(rgb(color)), backgroundLuminance = luminance(rgb(background));
        const range = document.createRange(); range.selectNodeContents(element);
        return { text: element.textContent, color, background, contrast: (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) / (Math.min(foregroundLuminance, backgroundLuminance) + 0.05), rect: range.getBoundingClientRect().toJSON() };
      });
    })()''')


def assert_no_errors(client):
    error = evaluate(client, 'document.querySelector(".hm-error[role=alert]")?.textContent||document.querySelector(".toast.error")?.textContent')
    assert not error, client + ' has a visible action error: ' + str(error)


def measured_http_round_trip(client):
    return evaluate(client, '(async()=>{const samples=[];for(let index=0;index<5;index++){const begin=performance.now();await fetch("/healthz?probe="+index,{cache:"no-store"});samples.push(performance.now()-begin);}return samples})()')


def cleanup_room():
    current = room()
    if current['status'] == 'playing':
        command('main', 'close')
    for client in ['peer', 'main']:
        if request(client, '/state').get('room'):
            command(client, 'leave')
    wait_for(lambda: not request('main', '/state').get('room'))


def production_run():
    assert os.environ.get('HOTLINE_BROWSER_ALLOW_PRODUCTION') == 'confirmed-deployed', 'production acceptance is not enabled'
    config_path = Path(os.environ['HOTLINE_BROWSER_PRODUCTION_CONFIG']).resolve()
    private_root = (ROOT / '.local/hotlinemiami/release').resolve()
    assert config_path.is_relative_to(private_root), 'production config must remain in the private game release directory'
    config = json.loads(config_path.read_text())
    origin = config['origin'].rstrip('/')
    parsed = urlsplit(origin)
    assert parsed.scheme == 'https' and parsed.netloc and not parsed.username and not parsed.password and not parsed.path and not parsed.query and not parsed.fragment
    identities = {'accounts': [], 'roomIds': [], 'origin': origin, 'complete': False}
    identity_path = private_root / 'browser-identities.private.json'

    def private_save():
        identity_path.write_text(json.dumps(identities, ensure_ascii=False, indent=2) + '\n')
        identity_path.chmod(0o600)

    REPORT['scope'] = 'published production UI; dedicated private test rooms and identities; native keyboard/mouse and simulated mobile touch; no network injection'
    REPORT['identityFile'] = str(identity_path.relative_to(ROOT))
    private_save()
    try:
        for client in ['main', 'peer']:
            result = subprocess.run(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'hm' if client == 'main' else 'hp'], cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=10)
            assert result.returncode == 0
            SESSIONS[client] = result.stdout.strip()
            URLS[client] = origin
            browser(client, 'open', origin)
            if client == 'main':
                version = request(client, '/version')
                assert version['version'] == config['expectedVersion'], 'production version differs from expected release'
                REPORT['version'] = version['version']
            account = {'client': client, 'name': '热线验收' + ('甲' if client == 'main' else '乙') + uuid.uuid4().hex[:6], 'password': uuid.uuid4().hex}
            identities['accounts'].append(account)
            private_save()
            request(client, '/register', {'name': account['name'], 'password': account['password'], 'invite': config['invite']})
            state = request(client, '/state')
            account['id'] = state['user']['id']
            assert 'hotlinemiami' in state['availableGames'], 'new game is not available by default'
            private_save()
            browser(client, 'reload')
            browser(client, 'wait', '--fn', 'document.body.innerText.includes("创建房间")')
            evaluate(client, '''(() => {window.__hotlineTrustedEvents=[];for(const type of ['keydown','keyup'])window.addEventListener(type,event=>{if(event.isTrusted)window.__hotlineTrustedEvents.push({type:event.type,at:performance.now(),trusted:true,key:event.code});},true);return true})()''')
        catalogue = evaluate('main', 'fetch("/hotlinemiami/manifest.json").then(response=>response.json()).then(manifest=>({maps:manifest.maps,characters:manifest.characters,weapons:manifest.weapons}))')
        assert [len(catalogue[key]) for key in ['maps', 'characters', 'weapons']] == [6, 8, 12], 'production catalogue differs from expanded release'
        REPORT['catalogue'] = {key: [entry['id'] for entry in catalogue[key]] for key in ['maps', 'characters', 'weapons']}
        for mode in ['humans', 'computer']:
            password = uuid.uuid4().hex
            current = request('main', '/rooms', {'kind': 'hotlinemiami', 'name': '热线发布验收' + uuid.uuid4().hex[:6], 'password': password, 'capacity': 2, 'options': {'map': 'no-talk', 'roundSeconds': 180}})
            identities['roomIds'].append(current['id'])
            private_save()
            if mode == 'humans':
                request('peer', '/rooms/' + current['id'], {'type': 'join', 'version': room()['version'], 'nonce': uuid.uuid4().hex}, expected=(400,))
                request('peer', '/rooms/' + current['id'], {'type': 'join', 'version': room()['version'], 'nonce': uuid.uuid4().hex, 'password': password})
            else:
                command('main', 'add_bot')
            wait_for(lambda: len(image_audit('main')) >= 10 and all(image['loaded'] for image in image_audit('main')), 60)
            screenshot('main', 'production-' + mode + '-waiting')
            command('main', 'ready')
            if mode == 'humans':
                command('peer', 'ready')
            command('main', 'start')
            wait_for(loaded, 90)
            wait_for(lambda: input_ready('main'), 60)
            browser('main', 'set', 'viewport', 1440, 1000)
            browser('main', 'scrollintoview', '[data-hotlinemiami-board]')
            entry = {'production': True, 'mode': mode, 'players': 2, 'keyboard': native('main', 'production-' + mode + '-keyboard', samples=8)}
            REPORT['cases'].append(entry)
            assert_no_errors('main')
            entry['observedNetwork'] = [seat.get('network') for seat in room()['seats']]
            screenshot('main', 'production-' + mode + '-desktop')
            browser('main', 'scrollintoview', 'button[aria-label="全屏"]')
            browser('main', 'click', 'button[aria-label="全屏"]')
            wait_for(lambda: evaluate('main', '!!document.querySelector("[data-room-fullscreen]")'))
            screenshot('main', 'production-' + mode + '-fullscreen')
            browser('main', 'click', 'button[aria-label="退出全屏"]')
            browser('main', 'set', 'viewport', 320, 760)
            browser('main', 'scrollintoview', '[data-hotlinemiami-board]')
            entry['touch'] = native('main', 'production-' + mode + '-touch', touch=True, samples=8)
            entry['mobile'] = viewport_audit('main')
            assert not entry['mobile']['overflow']
            entry['contrast'] = visual_audit('main')
            assert all(item['contrast'] >= 4.5 for item in entry['contrast'])
            entry['audioResources'] = evaluate('main', 'performance.getEntriesByType("resource").filter(entry=>/\\.flac(?:$|\\?)/.test(entry.name)).length')
            assert entry['audioResources'] > 0, 'production audio assets were not loaded'
            assert_no_errors('main')
            screenshot('main', 'production-' + mode + '-mobile')
            command('main', 'close')
            if mode == 'humans':
                command('peer', 'leave')
            command('main', 'leave')
            save()
        identities['complete'] = True
    finally:
        for client in SESSIONS:
            try:
                state = request(client, '/state')
                current = state.get('room')
                if current and current['id'] in identities['roomIds']:
                    if client == 'main' and current['status'] == 'playing':
                        command(client, 'close')
                    command(client, 'leave')
                request(client, '/logout', {})
            except Exception:
                REPORT.setdefault('cleanupNeedsReview', []).append(client)
        private_save()
        REPORT['productionTemporaryRecordsNeedExactIDCleanup'] = True


try:
    if os.environ.get('HOTLINE_BROWSER_PRODUCTION_CONFIG'):
        production_run()
        REPORT['passed'] = not REPORT['failures']
        raise SystemExit(0)
    REPORT['testFirst'] = {'scenarios': 'games/hotlinemiami/tests/browser-scenarios.md', 'red': '.local/hotlinemiami/browser/reports/fixture-red.log'}
    REPORT['sources'] = {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted((ROOT / 'games/hotlinemiami').rglob('*')) if path.is_file() and path.suffix in ['.go', '.ts', '.tsx', '.json', '.py', '.mjs', '.css']}
    for client in ['main', 'peer']:
        result = subprocess.run(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'hm' if client == 'main' else 'hp'], cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=10)
        assert result.returncode == 0, result.stderr
        SESSIONS[client] = result.stdout.strip()
        set_network(client)
    for name in ['urls.json', 'stop']:
        (BASE / name).unlink(missing_ok=True)
    log = (OUT / 'server.log').open('w')
    SERVER = subprocess.Popen(['go', 'test', '-count=1', '-timeout=26m', '-run', '^TestHotlineBrowserFixture$', './games/hotlinemiami/tests'], cwd=ROOT, env=ENV, stdout=log, stderr=subprocess.STDOUT)
    wait_for(lambda: (BASE / 'urls.json').exists(), 240)
    URLS = json.loads((BASE / 'urls.json').read_text())
    for client in ['main', 'peer']:
        browser(client, 'open', URLS[client])
        request(client, '/register', {'name': '热线' + client + uuid.uuid4().hex[:6], 'password': 'hotline-local-test', 'invite': 'hotline-browser'})
        browser(client, 'reload')
        browser(client, 'wait', '--fn', 'document.body.innerText.includes("创建房间")')
        evaluate(client, '''(() => {
          window.__hotlineTrustedEvents = [];
          for (const type of ['keydown','keyup']) window.addEventListener(type, event => {
            if (!event.isTrusted) return;
            window.__hotlineTrustedEvents.push({type:event.type,at:performance.now(),trusted:event.isTrusted,key:event.code});
            if (window.__hotlineTrustedEvents.length > 512) window.__hotlineTrustedEvents.shift();
          }, true);
          return true;
        })()''')
    catalogue = evaluate('main', 'fetch("/hotlinemiami/manifest.json").then(response=>response.json()).then(manifest=>({maps:manifest.maps,characters:manifest.characters,weapons:manifest.weapons}))')
    assert [len(catalogue[key]) for key in ['maps', 'characters', 'weapons']] == [6, 8, 12], 'expanded content catalogue is incomplete'
    for key in ['maps', 'characters', 'weapons']:
        assert len({entry['id'] for entry in catalogue[key]}) == len(catalogue[key]), 'duplicate content identity: ' + key
    REPORT['catalogue'] = {key: [{'id': entry['id'], 'name': entry['name']} for entry in catalogue[key]] for key in ['maps', 'characters', 'weapons']}
    cases = [(entry['id'], [2, 3, 4][index % 3], [1440, 390, 320][index % 3], [1000, 844, 760][index % 3]) for index, entry in enumerate(catalogue['maps'])]
    selected_maps = set(filter(None, os.environ.get('HOTLINE_BROWSER_MAP_FILTER', '').split(',')))
    assert selected_maps.issubset({entry['id'] for entry in catalogue['maps']}), 'unknown filtered map'
    REPORT['partial'] = bool(selected_maps)
    REPORT['selectedMaps'] = sorted(selected_maps) if selected_maps else [entry['id'] for entry in catalogue['maps']]
    for index, (map_id, count, width, height) in enumerate(cases):
        if selected_maps and map_id not in selected_maps:
            continue
        set_network('main', 40, 10)
        set_network('peer', 40, 10)
        browser('main', 'set', 'viewport', width, height)
        browser('peer', 'set', 'viewport', 1440, 1000)
        if index == 0:
            browser('main', 'find', 'role', 'button', 'click', '--name', '创建房间', '--exact')
            browser('main', 'find', 'role', 'button', 'click', '--name', '迈阿密热线')
            browser('main', 'wait', '[data-create-step=setup]')
            wait_for(lambda: evaluate('main', '[...document.querySelectorAll("[data-hotlinemiami-map] img")].filter(image=>image.complete&&image.naturalWidth>0).length===6'))
            screenshot('main', 'create-map-previews')
            browser('main', 'click', '[data-hotlinemiami-map="' + map_id + '"]')
            browser('main', 'select', '[aria-label="座位上限"]', str(count))
            browser('main', 'click', '[data-create-step=setup] button[type=submit]')
            wait_for(lambda: request('main', '/state').get('room'))
        else:
            request('main', '/rooms', {'kind': 'hotlinemiami', 'name': '热线浏览器验收', 'capacity': count, 'options': {'map': map_id, 'roundSeconds': 180}})
        current = room()
        request('peer', '/rooms/' + current['id'], {'type': 'join', 'version': current['version'], 'nonce': uuid.uuid4().hex})
        for bot in range(count - 2):
            command('main', 'add_bot')
        wait_for(lambda: len(image_audit('main')) >= 6 + count * 2 and all(image['loaded'] for image in image_audit('main')))
        assert evaluate('main', 'document.querySelectorAll(' + json.dumps('[data-hotlinemiami-character][data-seat="0"]') + ').length') == 8
        assert evaluate('main', 'document.querySelectorAll(' + json.dumps('[data-hotlinemiami-weapon][data-seat="0"]') + ').length') == 12
        if index == 0:
            for kind, picker in [('characters', '.hm-picker:not(.hm-weapon-picker)'), ('weapons', '.hm-weapon-picker')]:
                summary = '.hm-seat-choices:nth-child(1) ' + picker + ' summary'
                browser('main', 'scrollintoview', summary)
                browser('main', 'click', summary)
                browser('main', 'scrollintoview', '.hm-seat-choices:nth-child(1) ' + picker + ' .hm-picker-grid')
                screenshot('main', 'expanded-' + kind + '-references')
                browser('main', 'scrollintoview', summary)
                browser('main', 'click', summary)
            for character in catalogue['characters']:
                select_loadout('main', 0, character=character['id'])
            for weapon in catalogue['weapons']:
                select_loadout('main', 0, weapon=weapon['id'])
            browser('main', 'find', 'role', 'button', 'click', '--name', '玩法速查', '--exact')
            wait_for(lambda: evaluate('main', '!!document.querySelector("[role=dialog]")'))
            rule_rows = evaluate('main', 'Object.fromEntries(["map","character","weapon"].map(kind=>[kind,[...document.querySelectorAll("[data-hotlinemiami-rule-"+kind+"]")].map(row=>({id:row.getAttribute("data-hotlinemiami-rule-"+kind),description:row.querySelector("dd")?.textContent}))]))')
            assert [len(rule_rows[key]) for key in ['map', 'character', 'weapon']] == [6, 8, 12]
            assert all(row['description'] and len(row['description']) >= 6 for rows in rule_rows.values() for row in rows)
            for group in range(1, 6):
                selector = '[role=dialog] .hm-rule-group:nth-of-type(' + str(group) + ') summary'
                browser('main', 'scrollintoview', selector)
                browser('main', 'click', selector)
            rules = evaluate('main', 'document.querySelector("[role=dialog]").innerText')
            assert all(keyword in rules for keyword in ['地图', '造型', '武器', '弹药', '投掷', '复活', '混战', 'MP5', '双管', '斧', '砍']), 'expanded rules are incomplete'
            REPORT['rules'] = rules
            REPORT['ruleRows'] = rule_rows
            screenshot('main', 'expanded-rules')
            browser('main', 'scrollintoview', '[role=dialog] button[aria-label="关闭"]')
            browser('main', 'click', '[role=dialog] button[aria-label="关闭"]')
        character = catalogue['characters'][index % 8]['id']
        weapon = ['uzi', 'magnum', 'mp5', 'double-barrel', 'm16', 'shotgun'][index]
        select_loadout('main', 0, character=character, weapon=weapon)
        select_loadout('peer', 1, character=catalogue['characters'][(index + 6) % 8]['id'], weapon=['axe', 'machete', 'katana', 'knife', 'bat', 'pistol'][index])
        for seat in range(2, count):
            select_loadout('main', seat, character=catalogue['characters'][(index + seat) % 8]['id'])
        options = room()['options']
        denied_selector = '[data-hotlinemiami-character][data-seat="0"],[data-hotlinemiami-weapon][data-seat="0"]'
        denied = evaluate('peer', '([...document.querySelectorAll(' + json.dumps(denied_selector) + ')].every(button=>button.disabled))')
        assert denied
        assert options['map'] == map_id
        preview_images = image_audit('main')
        screenshot('main', map_id + '-waiting-' + str(width))
        command('main', 'ready')
        command('peer', 'ready')
        command('main', 'start')
        wait_for(loaded, 60)
        wait_for(lambda: input_ready('main') and input_ready('peer'), 60)
        initial_view = room()['game']['data']
        assert initial_view['players'][0]['character'] == character and initial_view['players'][0]['weapon'] == weapon
        peer_attack = native('peer', map_id + '-peer-attack')
        browser('main', 'scrollintoview', '[data-hotlinemiami-board]')
        audit = viewport_audit('main')
        assert not audit['overflow'], audit
        contrast = visual_audit('main')
        assert contrast and all(item['contrast'] >= 4.5 for item in contrast), contrast
        before = room()['game']['data']
        assert before['players'][0]['character'] == character
        entry = {'map': map_id, 'players': count, 'viewport': [width, height], 'loadout': {'character': character, 'weapon': weapon}, 'peerAttack': peer_attack, 'images': preview_images, 'layout': audit, 'contrast': contrast, 'epoch': before['epoch'], 'transport': {'injectedRTTMS': 40, 'perDirectionJitterMS': 10, 'httpRoundTripMS': measured_http_round_trip('main'), 'httpProbeIncludesApplicationAndBrowserQueue': True}}
        REPORT['cases'].append(entry)
        save()
        if width >= 650:
            entry['normal'] = native('main', map_id + '-normal', unarmed=index == 0)
        else:
            entry['touch'] = native('main', map_id + '-touch', touch=True)
        entry['observedNetwork'] = [seat.get('network') for seat in room()['seats']]
        assert_no_errors('main')
        screenshot('main', map_id + '-play-' + str(width))
        evaluate('main', 'window.__hotlineOriginalCanvas=document.querySelector("[data-hotlinemiami-canvas]");true')
        browser('main', 'scrollintoview', 'button[aria-label="全屏"]')
        browser('main', 'click', 'button[aria-label="全屏"]')
        wait_for(lambda: evaluate('main', '!!document.querySelector("[data-room-fullscreen]")'))
        full = viewport_audit('main')
        assert not full['overflow'] and full['canvas']['height'] > 100
        assert evaluate('main', 'window.__hotlineOriginalCanvas===document.querySelector("[data-hotlinemiami-canvas]")')
        entry['fullscreen'] = full
        screenshot('main', map_id + '-fullscreen')
        browser('main', 'click', 'button[aria-label="退出全屏"]')
        wait_for(lambda: not evaluate('main', '!!document.querySelector("[data-room-fullscreen]")'))
        browser('main', 'scrollintoview', '[data-hotlinemiami-board]')
        if index == len(cases) - 1:
            set_network('peer', 250, 15)
            peer_epoch = room('peer')['game']['data']['epoch']
            entry['slowPeerTransport'] = {'injectedRTTMS': 250, 'perDirectionJitterMS': 15, 'httpRoundTripMS': measured_http_round_trip('peer'), 'httpProbeIncludesApplicationAndBrowserQueue': True}
            wait_for(lambda: (room()['seats'][1].get('network', {}).get('rttMs') or 0) >= 180, 12)
            entry['slowPeerTransport']['observedNetwork'] = [seat.get('network') for seat in room()['seats']]
            wait_for(lambda: input_ready('main'))
            entry['fastWithSlowPeer'] = native('main', map_id + '-fast-with-slow', touch=True)
            frame_before = room()['game']['data']['frame']
            evaluate('peer', 'window.__hotlinePeerCanvas=document.querySelector("[data-hotlinemiami-canvas]");true')
            browser('peer', 'keydown', 'd')
            set_network('peer', offline=True)
            wait_for(lambda: not input_ready('peer'), 15)
            offline_frame = evaluate('peer', 'document.querySelector("[data-hotlinemiami-canvas]").__hotlineFrame')
            assert offline_frame['pending'] == 0, offline_frame
            time.sleep(0.3)
            assert room()['game']['data']['frame'] > frame_before
            assert not room()['seats'][1].get('autoPlay', False)
            set_network('peer', 40, 10)
            wait_for(lambda: input_ready('peer'), 30)
            browser('peer', 'keyup', 'd')
            assert room('peer')['game']['data']['epoch'] == peer_epoch
            same_canvas = evaluate('peer', 'window.__hotlinePeerCanvas===document.querySelector("[data-hotlinemiami-canvas]")')
            assert same_canvas
            browser('peer', 'reload')
            wait_for(lambda: input_ready('peer'), 30)
            assert room()['game']['data']['epoch'] == before['epoch'] and not room().get('loading')
            entry['recovery'] = {'sameEpoch': True, 'sameCanvasAfterNetwork': same_canvas, 'clearedPending': True, 'noAutomaticTakeover': True, 'fastContinued': True, 'reloadDidNotPause': True}
            command('main', 'autoplay', enabled=True)
            command('peer', 'autoplay', enabled=True)
            wait_for(lambda: room()['game']['data']['frame'] > frame_before + 60)
            assert_no_errors('main')
            assert_no_errors('peer')
            epoch = room()['game']['data']['epoch']
            command('main', 'autoplay', enabled=False)
            assert room()['game']['data']['epoch'] == epoch and not room()['seats'][0].get('autoPlay', False)
            command('main', 'autoplay', enabled=True)
            time.sleep(0.2)
            assert_no_errors('main')
            entry['takeover'] = {'manualReclaimPreservedEpoch': True}
            if os.environ.get('HOTLINE_BROWSER_SKIP_FINISH') != '1':
                wait_for(lambda: room()['game']['data']['finished'], 210)
                finished = room()
                assert finished['game']['finished']
                assert sum(player['kills'] for player in finished['game']['data']['players']) > 0
                assert finished['result']['rated'] is False
                entry['naturalFinish'] = finished['result']
                entry['finalPlayers'] = finished['game']['data']['players']
                save()
                wait_for(lambda: evaluate('main', '!!document.querySelector("[role=dialog]")'))
                screenshot('main', 'natural-finished-result')
                browser('main', 'click', '[role=dialog] button[aria-label="关闭"]')
                wait_for(lambda: not evaluate('main', '!!document.querySelector("[role=dialog]")'))
                browser('main', 'click', 'button.result-reopen')
                wait_for(lambda: evaluate('main', '!!document.querySelector("[role=dialog]")'))
        set_network('main')
        set_network('peer')
        save()
        cleanup_room()
    REPORT['passed'] = not REPORT['failures']
except Exception as error:
    REPORT['error'] = str(error)
    for client in SESSIONS:
        try:
            screenshot(client, client + '-failure')
            (OUT / (client + '-failure-dom.txt')).write_text(str(browser(client, 'snapshot')))
        except Exception:
            pass
    raise
finally:
    save()
    for client in SESSIONS:
        try:
            browser(client, 'close')
        except Exception:
            pass
    (BASE / 'stop').write_text('stop\n')
    if SERVER is not None:
        try:
            SERVER.wait(timeout=15)
        except subprocess.TimeoutExpired:
            SERVER.terminate()
            SERVER.wait(timeout=10)
    for client in ['main', 'peer']:
        profile = BASE / (client + '-profile')
        if profile.is_dir() and profile.is_relative_to(BASE):
            shutil.rmtree(profile)
    print(json.dumps({'passed': REPORT['passed'], 'report': str(OUT / 'report.json'), 'cases': len(REPORT['cases'])}, ensure_ascii=False))
