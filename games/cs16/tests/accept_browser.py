from pathlib import Path
import csv
import gzip
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import uuid
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent
BASE = ROOT / '.local/cs16/browser'
REPORT_NAME = os.environ.get('CS16_BROWSER_REPORT', 'current')
assert re.fullmatch(r'[a-zA-Z0-9_-]{1,40}', REPORT_NAME)
OUT = BASE / 'reports' / REPORT_NAME
SOCKETS = ROOT / '.local/cs16/s'
for directory in [BASE, OUT, ROOT / '.local/cs16/tmp', ROOT / '.local/cs16/go-cache', ROOT / '.local/cs16/go-mod', SOCKETS, BASE / 'cache']:
    directory.mkdir(parents=True, exist_ok=True)
ENV = dict(os.environ, TMPDIR=str(ROOT / '.local/cs16/tmp'), GOTMPDIR=str(ROOT / '.local/cs16/tmp'), GOCACHE=str(ROOT / '.local/cs16/go-cache'), GOMODCACHE=str(ROOT / '.local/cs16/go-mod'), GOTOOLCHAIN='local', GOTELEMETRY='off', AGENT_BROWSER_SOCKET_DIR=str(SOCKETS), XDG_CACHE_HOME=str(BASE / 'cache'), AGENT_BROWSER_NO_VERSION_CHECK='1', WIRE_BOARD_CS16_BROWSER='1')
REPORT = {'passed': False, 'scope': 'production HTTP/SQLite/WebSocket, two native Chromium player pages plus eight bots and one spectator page; original maps and models, no rule state injection', 'touchBoundary': 'Chromium touch simulation, not physical mobile hardware', 'diagnostic': os.environ.get('CS16_BROWSER_DIAGNOSTIC') == '1', 'cases': [], 'failures': [], 'performanceFailures': [], 'budgets': {'keyboardP95MS': 33, 'touchP95MS': 50, 'frameP95MS': 20, 'minimumVisibleFPS': 55}, 'testFirst': {'scenarios': 'games/cs16/tests/browser-scenarios.md', 'red': '.local/cs16/browser/reports/current/fixture-red.log'}}
SERVER = None
SESSIONS = {}
URLS = {}
CAPTURES = []
FIREARMS = set('glock18 usp p228 deagle elite fiveseven m3 xm1014 mp5navy tmp p90 mac10 ump45 galil famas ak47 m4a1 sg552 aug scout awp g3sg1 sg550 m249'.split())
CLASSIC_MAPS = ('de_dust2', 'de_inferno', 'de_aztec', 'cs_assault')
ADDED_MAPS = ('fy_iceworld', 'cs_bloodstrike')
MAPS = CLASSIC_MAPS + ADDED_MAPS
REPORT['expectedMaps'] = list(MAPS)
REPORT['expectedCaseCount'] = len(MAPS)
REPORT['caseCount'] = 0
REPORT['lifecycle'] = {'fixtureMinutes': 60, 'goTestTimeoutMinutes': 61, 'nativeProcessSeconds': 60, 'nextLifeSeconds': 120, 'denseCombatMaximumSeconds': 45, 'naturalFinishSeconds': 240}
MAP_MODES = {'de_dust2': 'bomb', 'de_inferno': 'bomb', 'de_aztec': 'bomb', 'cs_assault': 'hostage', 'fy_iceworld': 'elimination', 'cs_bloodstrike': 'elimination'}
ADDED_MAP_NAMES = {'fy_iceworld': '冰雪世界', 'cs_bloodstrike': '血战回廊'}


def check_catalogue(manifest):
    assert set(manifest['maps']) == set(MAPS), {'expectedMaps': list(MAPS), 'actualMaps': list(manifest['maps'])}
    assert len(manifest['players']) == 8 and FIREARMS.issubset(set(manifest['weapons']))
    assert {key: value['mode'] for key, value in manifest['maps'].items()} == MAP_MODES
    assert all(manifest['maps'][key]['name'] == name for key, name in ADDED_MAP_NAMES.items())


def check_case_coverage(cases, selected, skip_finish=False):
    expected = selected or set(MAPS)
    actual = [case['map'] for case in cases]
    assert len(actual) == len(expected) and set(actual) == expected, {'expectedMaps': sorted(expected), 'actualMaps': actual}
    for case in cases:
        assert case['players'] == 10
        if case['map'] in ADDED_MAPS:
            required = {'mapRules', 'desktopNative', 'touchNative', 'slowPlayerInput', 'slowPeerFastInput', 'denseWire', 'naturalFinish'}
            if skip_finish:
                required -= {'denseWire', 'naturalFinish'}
            assert required.issubset(case), {'map': case['map'], 'missingCases': sorted(required - set(case))}
            if not skip_finish:
                assert case['naturalFinish']['status'] == 'finished'


def map_rules_audit(map_id, initial):
    manifest = json.loads((ROOT / 'web/dist/cs16/manifest.json').read_text())
    entry = manifest['maps'][map_id]
    resource = entry['rules']
    path = ROOT / 'web/dist/cs16' / resource['file']
    assert path.resolve().is_relative_to((ROOT / 'web/dist/cs16').resolve())
    carrier = path.read_bytes()
    assert len(carrier) == resource['bytes'] and hashlib.sha256(carrier).hexdigest() == resource['sha256']
    decoded = gzip.decompress(carrier) if resource.get('encoding') == 'gzip' else carrier
    rules = json.loads(decoded)
    assert rules['id'] == map_id and rules['mode'] == MAP_MODES[map_id]
    assert hashlib.sha256(decoded).hexdigest() == initial['rulesSHA256'] == resource.get('decodedSHA256', resource['sha256'])
    if map_id in ADDED_MAPS:
        spawn_count = 12 if map_id == 'fy_iceworld' else 10
        assert len(rules['spawnsT']) == len(rules['spawnsCT']) == spawn_count
        assert not rules['bombsites'] and not rules['hostages'] and not initial['hostages']
        assert not any(item['id'] == 'c4' for player in initial['players'] for item in player.get('inventory', []))
        assert len(rules['buyZones']) == (1 if map_id == 'fy_iceworld' else 0)
        weapons = rules.get('worldWeapons', [])
        assert len(weapons) == (24 if map_id == 'fy_iceworld' else 0)
        if map_id == 'fy_iceworld':
            expected = set('mp5navy tmp p90 mac10 ak47 sg552 m4a1 aug awp m3 xm1014 m249'.split())
            assert {weapon['weapon'] for weapon in weapons} == expected
            assert all(sum(item['weapon'] == weapon for item in weapons) == 2 for weapon in expected)
            assert len({weapon['id'] for weapon in weapons}) == 24 and all(weapon['count'] == 1 for weapon in weapons)
    return {'mode': rules['mode'], 'rulesSHA256': initial['rulesSHA256'], 'sourceSHA256': entry['sourceSHA256'], 'spawnsT': len(rules['spawnsT']), 'spawnsCT': len(rules['spawnsCT']), 'buyZones': rules['buyZones'], 'worldWeapons': rules.get('worldWeapons', [])}


def source_snapshot():
    files = set()
    for relative in ['games/cs16', 'scripts/cs16', 'web/src/game-api', 'web/src/controllers']:
        directory = ROOT / relative
        files.update(source for source in directory.rglob('*') if source.is_file() and source.suffix in ['.go', '.ts', '.tsx', '.json', '.py', '.mjs', '.css'])
    for relative in ['internal/gameapi', 'internal/server']:
        files.update(source for source in (ROOT / relative).glob('*.go') if not source.name.endswith('_test.go'))
    files.update(source for source in (ROOT / 'games').glob('*.go') if not source.name.endswith('_test.go'))
    files.update((ROOT / 'web/src').glob('audio*.ts'))
    for relative in ['go.mod', 'go.sum', 'web/src/realtime-stream.ts', 'web/src/main.tsx', 'web/src/ui/room-fullscreen.tsx', 'web/src/ui/game-control-guard.ts', 'web/src/ui/game-control-guard.css', 'web/vite.config.ts', 'web/scripts/game-load-manifest.mjs', 'web/scripts/public-assets.mjs', 'web/scripts/cdn-base.mjs']:
        files.add(ROOT / relative)
    result = {}
    for source in sorted(files):
        assert source.resolve().is_relative_to(ROOT) and not source.is_symlink(), source
        result[str(source.relative_to(ROOT))] = hashlib.sha256(source.read_bytes()).hexdigest()
    return result


def artifact_snapshot():
    root = ROOT / 'web/dist'
    load = json.loads((root / 'game-load-manifest.json').read_text())
    manifest = json.loads((root / 'cs16/manifest.json').read_text())
    assert load['build'] == hashlib.sha256((root / 'index.html').read_bytes()).hexdigest(), 'load manifest does not match built index'
    files = {root / 'index.html', root / 'game-load-manifest.json', root / 'cs16/manifest.json'}
    for item in load['games']['cs16'] + load['home']['files']:
        selected = urlsplit(item['url']).path
        assert '/assets/' in selected, selected
        file = root / selected[selected.index('/assets/') + 1:]
        assert file.stat().st_size == item['bytes'], item
        files.add(file)
    for key in ['wasm', 'script', 'license']:
        item = manifest['runtime'].get(key)
        if item:
            selected = root / 'cs16' / item['file']
            assert selected.resolve().is_relative_to(root.resolve()), selected
            body = selected.read_bytes()
            assert len(body) == item['bytes'] and hashlib.sha256(body).hexdigest() == item['sha256'], item
            files.add(selected)
    mismatched = []
    for item in manifest['runtime'].get('sourceFiles', []):
        selected = ROOT / item['path']
        assert selected.resolve().is_relative_to(ROOT) and not selected.is_symlink(), selected
        if hashlib.sha256(selected.read_bytes()).hexdigest() != item['sha256']:
            mismatched.append(item['path'])
    assert REPORT['diagnostic'] or not mismatched, mismatched
    REPORT['runtimeSourceMismatches'] = mismatched
    return {str(source.relative_to(ROOT)): hashlib.sha256(source.read_bytes()).hexdigest() for source in sorted(files)}


def save():
    (OUT / 'report.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')


def wait_for(predicate, seconds=30):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        if SERVER is not None and SERVER.poll() is not None:
            raise AssertionError((OUT / 'server.log').read_text()[-8000:])
        try:
            result = predicate()
            if result:
                return result
        except (AssertionError, KeyError, TypeError) as error:
            last = str(error)
        time.sleep(0.1)
    raise AssertionError('condition timed out: ' + str(last))


def browser(client, *arguments, input_source=None):
    result = subprocess.run(['agent-browser', '--session', SESSIONS[client], '--profile', str(BASE / (client + '-profile')), *map(str, arguments), '--json'], cwd=ROOT, env=ENV, input=input_source, text=True, capture_output=True, timeout=45)
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
    return request(client, '/state').get('room')


def command(client, kind, **extra):
    nonce = uuid.uuid4().hex
    for attempt in range(12):
        current = room(client)
        response = evaluate(client, '(async()=>{const response=await fetch(' + json.dumps('/api/rooms/' + current['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(dict(type=kind, version=current['version'], nonce=nonce, **extra), ensure_ascii=False) + ')});return {status:response.status,body:await response.json()}})()')
        if response['status'] == 409:
            continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('room version never stabilized for ' + kind)


def set_network(client, rtt=0, jitter=0, offline=False):
    (BASE / ('network-' + client + '.json')).write_text(json.dumps({'rtt': rtt, 'jitter': jitter, 'offline': offline}))


def frame(client='main'):
    return evaluate(client, 'document.querySelector("[data-cs16-canvas]")?.__cs16Frame')


def loaded():
    for client in ['main', 'peer']:
        error = evaluate(client, 'document.querySelector(".game-loading [role=alert]")?.textContent')
        assert not error, client + ': ' + str(error)
    return not room().get('loading') and all(evaluate(client, '!!document.querySelector("[data-cs16-board][data-ready=true]")') for client in ['main', 'peer'])


def endpoint(client):
    value = browser(client, 'get', 'cdp-url')
    return value if isinstance(value, str) else value.get('url', value.get('cdpUrl', value.get('value')))


def sample_summary(observations):
    responses = [response for observation in observations for response in observation.get('responses', [])]
    groups = [observation.get('frames', []) for observation in observations]
    intervals = [frame['at'] - frames[index]['at'] for frames in groups for index, frame in enumerate(frames[1:])]
    elapsed = sum(frames[-1]['at'] - frames[0]['at'] for frames in groups if len(frames) > 1)
    updates = sum(frame['drawCount'] != frames[index]['drawCount'] for frames in groups for index, frame in enumerate(frames[1:]))
    def percentile(values, percent=95):
        return sorted(values)[(len(values) * percent + 99) // 100 - 1] if values else None
    return {'responses': len(responses), 'keyboardP95MS': percentile([item['milliseconds'] for item in responses if item['kind'] == 'key']), 'mouseP95MS': percentile([item['milliseconds'] for item in responses if item['kind'] == 'mouse']), 'touchP95MS': percentile([item['milliseconds'] for item in responses if item['kind'] == 'touch']), 'frameP95MS': percentile(intervals), 'frameP99MS': percentile(intervals, 99), 'visibleFPS': updates * 1000 / elapsed if elapsed else None}


def check_sample_budgets(summary):
    for field, limit in [('keyboardP95MS', 33), ('mouseP95MS', 33), ('touchP95MS', 50), ('frameP95MS', 20), ('frameP99MS', 33)]:
        assert summary[field] is None or summary[field] <= limit, {'performanceBudgetFailure': field, 'limit': limit, 'actual': summary[field]}
    assert summary['visibleFPS'] is None or summary['visibleFPS'] >= 55, summary


def native(client, name, touch=False, allow_life_interruption=False, **extra):
    selected = OUT / (name + '-native.json')
    control = BASE / ('network-' + client + '.json')
    original_control = control.read_bytes()
    inputs = {'endpoint': endpoint(client), 'pageURL': URLS[client], 'options': dict(touch=touch, output=str(selected), diagnostic=REPORT['diagnostic'], networkControl=str(control), **extra)}
    try:
        result = subprocess.run(['node', str(HERE / 'native-input.mjs')], cwd=ROOT, env=ENV, input=json.dumps(inputs), text=True, capture_output=True, timeout=60)
    finally:
        control.write_bytes(original_control)
    (OUT / (name + '-native.log')).write_text(result.stdout + result.stderr)
    observation = json.loads(selected.read_text()) if selected.exists() else {}
    if observation.get('frames'):
        keys = ['at', 'drawCount', 'x', 'y', 'z', 'yaw', 'pitch', 'serverFrame', 'predictedFrame', 'receivedSeq', 'appliedSeq', 'inputWindow', 'pending', 'inputEnabled', 'weapon', 'life']
        with (OUT / (name + '-frames.csv')).open('w', newline='') as output:
            writer = csv.DictWriter(output, fieldnames=keys, extrasaction='ignore')
            writer.writeheader()
            writer.writerows(observation['frames'])
    if observation.get('responses'):
        with (OUT / (name + '-responses.csv')).open('w', newline='') as output:
            writer = csv.DictWriter(output, fieldnames=['kind', 'inputAt', 'visibleAt', 'milliseconds', 'pixelsChanged'], extrasaction='ignore')
            writer.writeheader()
            writer.writerows(observation['responses'])
    accepted = result.returncode == 0 and (observation.get('passed') or REPORT['diagnostic'] and observation.get('functionalPassed'))
    if allow_life_interruption:
        measured = sample_summary([observation])
        check_sample_budgets(measured)
        assert not observation.get('performanceFailures'), observation['performanceFailures']
        authority = observation.get('network', {}).get('lastAuthority', {})
        bootstrap = observation.get('network', {}).get('measurementStartIdentity', {})
        lives = {entry['life'] for entry in observation.get('frames', []) if isinstance(entry.get('life'), int)}
        replaced = authority.get('epoch') == bootstrap.get('epoch') and isinstance(authority.get('life'), int) and isinstance(bootstrap.get('life'), int) and authority['life'] != bootstrap['life']
        interrupted = authority.get('player', {}).get('alive') is False or replaced
        interruption_errors = ('native modifier combination', 'native weapon selection', 'too few real pixel responses', 'ongoing movement prevented', 'two-thumb fire was not applied', 'mobile crouch toggle', 'both rapid selections', 'rapid Digit3 and Q', 'getBoundingClientRect', 'native transport baseline was interrupted by authoritative death')
        eligible = any(message in observation.get('error', '') for message in interruption_errors)
        assert not observation.get('layoutFailures') and not observation.get('network', {}).get('internalFieldLeaks'), observation
        if not accepted and interrupted and eligible:
            return {'interrupted': True, 'rawAuthority': authority, 'observedLives': sorted(lives), 'report': str(selected.relative_to(ROOT)), 'allMeasuredSamples': measured}
    if not accepted and REPORT['diagnostic']:
        failure = {'case': name, 'nativeFailure': observation.get('error', result.stderr)}
        REPORT['failures'].append(failure)
        save()
        return {'failed': True, 'error': failure['nativeFailure'], **observation.get('summary', {})}
    assert accepted, json.dumps(observation.get('summary', observation.get('error', result.stderr)))
    if observation.get('performanceFailures'):
        REPORT['performanceFailures'].append({'case': name, 'errors': observation['performanceFailures']})
    if observation.get('layoutFailures'):
        REPORT['failures'].append({'case': name, 'nativeLayoutFailures': observation['layoutFailures']})
        save()
    return observation['summary']


def elimination_native(client, map_id, name, touch=False):
    attempts = []
    observations = []
    for attempt in range(3):
        browser(client, 'scrollintoview', '[data-cs16-board]')
        browser(client, 'click', '[data-cs16-board]')
        wait_for(lambda: frame(client) and frame(client).get('inputEnabled') and room(client)['game']['data']['phase'] == 'live', 120)
        label = map_id + '-' + name + ('-attempt' + str(attempt + 1) if attempt else '')
        result = native(client, label, touch=touch, allow_life_interruption=True, allowAmmoPurchase=map_id != 'fy_iceworld')
        observations.append(json.loads((OUT / (label + '-native.json')).read_text()))
        measured = sample_summary(observations)
        check_sample_budgets(measured)
        attempts.append(result)
        if not result.get('interrupted'):
            return {'attempts': attempts, 'summary': result, 'allMeasuredSamples': measured}
        REPORT.setdefault('lifeInterruptions', []).append({'map': map_id, 'client': client, 'case': label, 'evidence': result})
        save()
    raise AssertionError({'map': map_id, 'client': client, 'error': 'three native groups were interrupted by authoritative death or life replacement', 'attempts': attempts})


def begin_native_capture(client, name, milliseconds=12000):
    selected = OUT / (name + '-native.json')
    ready = OUT / (name + '-ready.json')
    ready.unlink(missing_ok=True)
    inputs = {'endpoint': endpoint(client), 'pageURL': URLS[client], 'options': {'observeOnly': True, 'observeMS': milliseconds, 'readyFile': str(ready), 'output': str(selected), 'diagnostic': REPORT['diagnostic']}}
    log = (OUT / (name + '-native.log')).open('w')
    process = subprocess.Popen(['node', str(HERE / 'native-input.mjs')], cwd=ROOT, env=ENV, stdin=subprocess.PIPE, stdout=log, stderr=subprocess.STDOUT, text=True)
    process.stdin.write(json.dumps(inputs))
    process.stdin.close()
    value = {'process': process, 'file': selected, 'ready': ready, 'log': log, 'name': name}
    CAPTURES.append(value)
    wait_for(lambda: ready.exists() or process.poll() is not None, 15)
    assert ready.exists(), (OUT / (name + '-native.log')).read_text()
    return value


def finish_native_capture(value):
    value['process'].wait(timeout=60)
    value['log'].close()
    observation = json.loads(value['file'].read_text())
    value['ready'].unlink(missing_ok=True)
    if not observation.get('passed'):
        REPORT['failures'].append({'case': value['name'], 'wireCaptureFailure': observation.get('error')})
        assert REPORT['diagnostic'], observation
    return {'summary': observation.get('summary'), 'wire': observation.get('wire'), 'passed': observation.get('passed')}


def screenshot(client, name):
    browser(client, 'screenshot', OUT / (name + '.png'))


def check_action_error(client, stage):
    value = evaluate(client, '({toast:document.querySelector(".toast.error")?.textContent,banner:document.querySelector(".cs16-error-banner[role=alert]")?.textContent})')
    assert not value.get('banner'), value
    if not value.get('toast'):
        return
    REPORT['failures'].append({'client': client, 'stage': stage, 'unexpectedActionError': value['toast']})
    save()
    assert REPORT['diagnostic'], value
    browser(client, 'click', 'button[aria-label="关闭错误"]')


def viewport_expression():
    return '''(() => {
      const rect=element=>element.getBoundingClientRect().toJSON();
      const visible=element=>{const box=element.getBoundingClientRect();if(!box.width||!box.height||element.closest('[inert]')||document.fullscreenElement&&!document.fullscreenElement.contains(element))return false;const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);return !!hit&&(hit===element||element.contains(hit));};
      const controls=[...document.querySelectorAll('[data-cs16-action],[data-fullscreen-toggle],.chat-toggle:not([aria-hidden=true])')].filter(visible).map(element=>({kind:element.dataset.cs16Action??(element.matches('.chat-toggle')?'platform-chat':'platform-fullscreen'),rect:rect(element)}));
      const hud=[...document.querySelectorAll('.cs16-vitals,.cs16-ammo,.cs16-match-hud,.cs16-inventory')].map(element=>({kind:element.className,text:element.textContent,rect:rect(element),color:getComputedStyle(element).color,background:getComputedStyle(element).backgroundColor}));
      const overlap=(a,b)=>a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top;
      return {width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth,canvas:document.querySelector('[data-cs16-canvas]')?.getBoundingClientRect().toJSON(),sticks:[...document.querySelectorAll('[data-cs16-stick]')].map(element=>({kind:element.dataset.cs16Stick,rect:rect(element)})),controls,hud,overlaps:hud.flatMap(item=>controls.filter(control=>overlap(item.rect,control.rect)).map(control=>({hud:item.kind,control:control.kind})))};
    })()'''


def viewport(client):
    return evaluate(client, viewport_expression())


def fullscreen_viewport(client, stage=''):
    value = evaluate(client, '''new Promise(resolve => {
      const audit=()=>''' + viewport_expression() + ''';
      const canvas=document.querySelector('[data-cs16-canvas]');
      const board=document.querySelector('[data-cs16-board]');
      const sample=frame=>({frame,at:performance.now(),drawCount:canvas?.__cs16Frame?.drawCount??0,chatInset:board.style.getPropertyValue('--cs16-chat-right'),computedChatInset:getComputedStyle(board).getPropertyValue('--cs16-chat-right'),audit:audit()});
      const trace=[sample(0)];
      let frame=0;
      const next=()=>{frame++;if(frame===1||frame===3)trace.push(sample(frame));if(frame===3)resolve({trace,audit:trace.at(-1).audit});else requestAnimationFrame(next);};
      requestAnimationFrame(next);
    })''')
    REPORT.setdefault('fullscreenFrameTraces', []).append({'client': client, 'stage': stage, **value})
    save()
    assert [item['frame'] for item in value['trace']] == [0, 1, 3], value
    assert value['trace'][-1]['drawCount'] - value['trace'][0]['drawCount'] >= 3, value
    return value


def check_layout(audit, stage):
    assert not audit['overflow'], audit
    if not audit['overlaps']:
        return
    REPORT['failures'].append({'stage': stage, 'layoutOverlaps': audit['overlaps']})
    save()
    assert REPORT['diagnostic'], audit['overlaps']


def buy_audit(client, map_id, mobile=False):
    assert map_id in CLASSIC_MAPS + ('cs_bloodstrike',), {'map': map_id, 'error': 'map uses original ground equipment and does not allow spawn purchasing'}
    browser(client, 'scrollintoview', '[data-cs16-board]')
    browser(client, 'focus', '[data-cs16-board]')
    browser(client, 'press', 'b')
    wait_for(lambda: evaluate(client, 'document.querySelectorAll("[data-cs16-buy]").length>=24'))
    entries = evaluate(client, '[...document.querySelectorAll("[data-cs16-buy]")].map(button=>({id:button.dataset.cs16Buy,disabled:button.disabled,text:button.textContent}))')
    expected = FIREARMS
    assert expected.issubset({entry['id'] for entry in entries}), entries
    current = room(client)
    own = current['game']['data']['players'][current['you']]
    other = 'fiveseven' if own['team'] == 't' else 'elite'
    assert next(entry for entry in entries if entry['id'] == other)['disabled']
    assert next(entry for entry in entries if entry['id'] == 'awp')['disabled']
    scroll = native(client, map_id + '-buy-scroll', touch=True, scrollOnly=True) if mobile else None
    screenshot(client, map_id + '-buy-menu')
    browser(client, 'click', '[data-cs16-buy="deagle"]')
    wait_for(lambda: room(client)['game']['data']['players'][current['you']]['weapon'] == 'deagle')
    ammunition = native(client, map_id + '-buy-ammo-scroll', touch=True, scrollOnly=True, buyItem='secammo') if mobile else None
    browser(client, 'click', 'button[aria-label="关闭购买菜单"]')
    wait_for(lambda: frame(client).get('weapon') == 'deagle')
    return {'catalogue': entries, 'team': own['team'], 'moneyBefore': own['money'], 'purchased': 'deagle', 'authoritativeAndVisible': True, 'scroll': scroll, 'ammunition': ammunition}


def ground_equipment_audit(client, map_id):
    current = room(client)
    data = current['game']['data']
    own = next(player for player in data['players'] if player['seat'] == current['you'])
    assert data.get('buyAllowed') is False and own['weapon'] in FIREARMS and any(item['id'] in {'glock18', 'usp'} for item in own['inventory']), {'map': map_id, 'player': own, 'buyAllowed': data.get('buyAllowed')}
    browser(client, 'focus', '[data-cs16-board]')
    browser(client, 'press', 'b')
    wait_for(lambda: evaluate(client, 'document.querySelectorAll("[data-cs16-buy]").length>=24'))
    entries = evaluate(client, '[...document.querySelectorAll("[data-cs16-buy]")].map(button=>({id:button.dataset.cs16Buy,disabled:button.disabled}))')
    assert FIREARMS.issubset({entry['id'] for entry in entries}) and all(entry['disabled'] for entry in entries), entries
    screenshot(client, map_id + '-spawn-buy-disabled')
    browser(client, 'click', 'button[aria-label="关闭购买菜单"]')
    return {'source': 'original spawn sidearm and world armoury; no spawn purchase', 'player': own, 'catalogue': entries, 'buyAllowedAtSpawn': False, 'initialDrops': data['drops']}


def fullscreen_audit(client, map_id):
    original = room(client)['game']['data']
    evaluate(client, 'window.__cs16OriginalCanvas=document.querySelector("[data-cs16-canvas]");true')
    modes = []
    for fallback in [False, True]:
        check_action_error(client, map_id + '-fullscreen')
        if fallback:
            evaluate(client, '(()=>{const surface=document.querySelector("[data-game-fullscreen-surface]");window.__cs16FullscreenDescriptor=Object.getOwnPropertyDescriptor(surface,"requestFullscreen");Object.defineProperty(surface,"requestFullscreen",{configurable:true,value:()=>Promise.reject(Error("acceptance fallback"))});return true})()')
        browser(client, 'scrollintoview', 'button[aria-label="全屏"]')
        browser(client, 'click', 'button[aria-label="全屏"]')
        mode = wait_for(lambda: evaluate(client, 'document.querySelector("[data-room-fullscreen]")?.dataset.roomFullscreen'))
        assert mode == 'page' if fallback else mode in ['native', 'page']
        assert evaluate(client, 'window.__cs16OriginalCanvas===document.querySelector("[data-cs16-canvas]")')
        transition = fullscreen_viewport(client, map_id + '-fullscreen-' + mode)
        audit = transition['audit']
        assert not audit['overflow'] and audit['canvas']['width'] > 0 and audit['canvas']['height'] > 0, audit
        check_layout(audit, map_id + '-fullscreen-' + mode)
        screenshot(client, map_id + '-fullscreen-' + mode)
        modes.append({'mode': mode, 'simulatedNativeRefusal': fallback, 'viewport': audit, 'sameCanvas': True, 'frameTrace': transition['trace']})
        if fallback:
            browser(client, 'click', 'button[aria-label="退出全屏"]')
        else:
            browser(client, 'press', 'Escape')
        wait_for(lambda: not evaluate(client, '!!document.querySelector("[data-room-fullscreen]")'))
        if fallback:
            evaluate(client, '(()=>{const surface=document.querySelector("[data-game-fullscreen-surface]");if(window.__cs16FullscreenDescriptor)Object.defineProperty(surface,"requestFullscreen",window.__cs16FullscreenDescriptor);else delete surface.requestFullscreen;return true})()')
    after = room(client)['game']['data']
    assert original['epoch'] == after['epoch'] and original['startAt'] == after['startAt']
    return modes


def cleanup_room():
    current = room()
    if current and current['status'] == 'playing':
        check_action_error('main', 'before-close')
        browser('main', 'find', 'role', 'button', 'click', '--name', '结束牌桌', '--exact')
        browser('main', 'find', 'role', 'button', 'click', '--name', '确认结束', '--exact')
        wait_for(lambda: room()['status'] != 'playing')
    for client in ['watch', 'peer', 'main']:
        if evaluate(client, '!!document.querySelector("button.result-review")'):
            browser(client, 'click', 'button.result-review')
        current = room(client)
        if current and current.get('spectating'):
            browser(client, 'find', 'role', 'button', 'click', '--name', '离开观战', '--exact')
        elif current:
            browser(client, 'find', 'role', 'button', 'click', '--name', '离开房间', '--exact')
            browser(client, 'find', 'role', 'button', 'click', '--name', '确认离开', '--exact')
        wait_for(lambda: room(client) is None)
        if evaluate(client, '!!document.querySelector("button[aria-label=关闭提示]")'):
            browser(client, 'click', 'button[aria-label=关闭提示]')
    wait_for(lambda: room() is None)


def prepare_room(map_id, mobile=False, dual=False, natural=False):
    browser('main', 'set', 'viewport', 320 if mobile else 1440, 760 if mobile else 1000)
    browser('peer', 'set', 'viewport', 320 if dual else 1440, 760 if dual else 1000)
    if map_id == 'de_dust2':
        browser('main', 'find', 'role', 'button', 'click', '--name', '创建房间', '--exact')
        browser('main', 'find', 'role', 'button', 'click', '--name', '反恐精英 1.6', '--exact')
        browser('main', 'wait', '[data-create-step=setup]')
        browser('main', 'wait', '[data-create-step=setup] select[aria-label="座位上限"]')
        capacity = evaluate('main', 'document.querySelector("[aria-label=座位上限]").value')
        assert capacity == '10', capacity
        assert evaluate('main', 'document.querySelector("[data-cs16-setting=friendly-fire]")?.checked') is False
        browser('main', 'fill', '[data-create-step=setup] input[name=name]', 'CS十席原图验收')
        browser('main', 'select', '[aria-label="比赛局数"]', '2')
        browser('main', 'click', '[data-create-step=setup] button[type=submit]')
        created = wait_for(lambda: room() if room() and room()['status'] == 'waiting' else None)
        REPORT['defaultTenSeatCreationUI'] = True
    else:
        created = request('main', '/rooms', {'kind': 'cs16', 'name': 'CS十席原图验收', 'capacity': 10, 'options': {'map': map_id, 'roundSeconds': 60 if map_id in ('de_inferno', 'cs_assault') + ADDED_MAPS else 120, 'maxRounds': 30 if map_id in ADDED_MAPS and not natural else 2}})
    request('peer', '/rooms/' + created['id'], {'type': 'join', 'version': room()['version'], 'nonce': uuid.uuid4().hex})
    for index in range(8):
        command('main', 'add_bot')
    current = room()
    assert len(current['seats']) == 10 and sum(bool(seat.get('bot')) for seat in current['seats']) == 8, current['seats']
    assert current['options']['teams'].count('t') == current['options']['teams'].count('ct') == 5
    friendly_fire = map_id == 'de_inferno'
    wait_for(lambda: evaluate('main', '!!document.querySelector("[data-cs16-setting=friendly-fire]")'))
    assert evaluate('main', 'document.querySelector("[data-cs16-setting=friendly-fire]").disabled') is False
    assert evaluate('peer', 'document.querySelector("[data-cs16-setting=friendly-fire]").disabled') is True
    assert current['options'].get('friendlyFire', False) is False
    if friendly_fire:
        browser('main', 'check', '[data-cs16-setting=friendly-fire]')
        wait_for(lambda: room()['options'].get('friendlyFire') is True)
    wait_for(lambda: evaluate('main', 'document.querySelectorAll("[data-cs16-map]").length===' + str(len(MAPS))))
    images = evaluate('main', '[...document.querySelectorAll("[data-cs16-map] img")].map(image=>({map:image.closest("[data-cs16-map]").dataset.cs16Map,label:image.closest("[data-cs16-map]").textContent,loaded:image.complete&&image.naturalWidth>0,src:image.src,rect:image.getBoundingClientRect().toJSON()}))')
    assert len(images) == len(MAPS) and {image['map'] for image in images} == set(MAPS) and all(image['loaded'] for image in images), images
    for image in images:
        if image['map'] in ADDED_MAP_NAMES:
            assert ADDED_MAP_NAMES[image['map']] in image['label'] and '人质' not in image['label'] and '爆破' not in image['label'], image
    screenshot('main', map_id + '-waiting')
    if mobile:
        browser('main', 'set', 'viewport', 310, 760)
        assert not viewport('main')['overflow'], viewport('main')
        screenshot('main', map_id + '-waiting-310')
        browser('main', 'set', 'viewport', 320, 760)
    for client in ['main', 'peer']:
        command(client, 'ready')
    command('main', 'start')
    request('watch', '/rooms/' + created['id'] + '/watch', {})
    wait_for(loaded, 180)
    browser('main', 'scrollintoview', '[data-cs16-board]')
    browser('main', 'click', '[data-cs16-board]')
    wait_for(lambda: frame() and frame().get('inputEnabled'), 60)
    browser('main', 'scrollintoview', '[data-cs16-board]')
    browser('peer', 'scrollintoview', '[data-cs16-board]')
    screenshot('main', map_id + '-first-frame')
    data = room()['game']['data']
    assert data['mapId'] == map_id and len(data['players']) == 10
    assert data['options'].get('friendlyFire', False) is friendly_fire, data['options']
    assert sum(player['team'] == 't' for player in data['players']) == 5
    assert sum(player['team'] == 'ct' for player in data['players']) == 5
    if map_id == 'cs_assault':
        assert len(room('watch')['game']['data']['hostages']) == 4
    assert not viewport('main')['overflow'], viewport('main')
    return data


def elimination_match(map_id):
    initial = prepare_room(map_id, natural=True)
    command('main', 'autoplay', enabled=True)
    command('peer', 'autoplay', enabled=True)
    capture = native('watch', map_id + '-natural-dense-wire', observeOnly=True, observeMS=10000, denseExpected=True)
    if map_id == 'fy_iceworld':
        assert capture['wire']['eventCounts'].get('pickup', 0) > 0, {'map': map_id, 'error': 'natural raw wire did not observe an original-world weapon pickup', 'capture': capture}
    finished = wait_for(lambda: room() if room()['status'] == 'finished' or room().get('game', {}).get('finished') else None, 240)
    assert len(finished['seats']) == 10 and sum(bool(seat.get('bot')) for seat in finished['seats']) == 8
    assert all(seat.get('autoPlay') for seat in finished['seats'] if not seat.get('bot'))
    screenshot('main', map_id + '-natural-finish')
    return {'initial': initial, 'denseWire': capture, 'finished': finished}


if os.environ.get('CS16_BROWSER_PREFLIGHT_ONLY') == '1':
    REPORT['partial'] = True
    REPORT['scope'] = 'offline six-map catalogue preflight; no browser or server started'
    try:
        check_catalogue(json.loads((ROOT / 'web/dist/cs16/manifest.json').read_text()))
        REPORT['preflightPassed'] = True
    except AssertionError as error:
        REPORT['failures'].append({'cataloguePreflight': str(error)})
        save()
        raise SystemExit(1)
    save()
    raise SystemExit(0)


try:
    REPORT['commands'] = {'browser': 'python3 -B games/cs16/tests/accept_browser.py', 'server': 'WIRE_BOARD_CS16_BROWSER=1 go test -count=1 -timeout=61m -run ^TestCS16BrowserFixture$ ./games/cs16/tests', 'options': {key: value for key, value in os.environ.items() if key.startswith('CS16_BROWSER_')}}
    REPORT['environment'] = {'platform': sys.platform, 'python': sys.version.split()[0], 'workspace': str(ROOT)}
    REPORT['sources'] = source_snapshot()
    REPORT['artifacts'] = artifact_snapshot()
    REPORT['sourceIdentitySHA256'] = hashlib.sha256(json.dumps(REPORT['sources'], sort_keys=True).encode()).hexdigest()
    for client in ['main', 'peer', 'watch']:
        (BASE / (client + '-profile')).mkdir(parents=True, exist_ok=True)
    for client, prefix in [('main', 'cm'), ('peer', 'cp'), ('watch', 'cw')]:
        result = subprocess.run(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', prefix], cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=10)
        assert result.returncode == 0, result.stderr
        SESSIONS[client] = result.stdout.strip()
        set_network(client)
    for name in ['urls.json', 'stop']:
        (BASE / name).unlink(missing_ok=True)
    log = (OUT / 'server.log').open('w')
    SERVER = subprocess.Popen(['go', 'test', '-count=1', '-timeout=61m', '-run', '^TestCS16BrowserFixture$', './games/cs16/tests'], cwd=ROOT, env=ENV, stdout=log, stderr=subprocess.STDOUT)
    wait_for(lambda: (BASE / 'urls.json').exists(), 180)
    URLS = json.loads((BASE / 'urls.json').read_text())
    REPORT['serverBinary'] = json.loads((BASE / 'server-binary.json').read_text())
    for client in ['main', 'peer', 'watch']:
        browser(client, 'open', URLS[client])
        request(client, '/register', {'name': 'CS原生' + client + uuid.uuid4().hex[:6], 'password': 'cs16-browser-password', 'invite': 'cs16-browser'})
        browser(client, 'reload')
        browser(client, 'wait', '--fn', 'document.body.innerText.includes("创建房间")')
    REPORT['controllers'] = request('main', '/controllers?game=cs16')
    manifest = evaluate('main', 'fetch("/cs16/manifest.json").then(response=>response.json()).then(manifest=>({maps:manifest.maps,players:manifest.players,weapons:manifest.weapons,runtime:manifest.runtime}))')
    check_catalogue(manifest)
    REPORT['catalogue'] = {'maps': list(manifest['maps']), 'players': list(manifest['players']), 'weapons': list(manifest['weapons'])}
    REPORT['bindings'] = {'runtime': manifest['runtime'], 'indexSHA256': hashlib.sha256((ROOT / 'web/dist/index.html').read_bytes()).hexdigest()}
    selected = set(filter(None, os.environ.get('CS16_BROWSER_MAP_FILTER', '').split(',')))
    assert selected.issubset(set(manifest['maps']))
    REPORT['partial'] = bool(selected) or REPORT['diagnostic']
    for map_id in MAPS:
        if selected and map_id not in selected:
            continue
        set_network('main', 40, 5)
        set_network('peer', 40, 5)
        added = map_id in ADDED_MAPS
        initial = prepare_room(map_id, mobile=map_id == 'de_aztec', dual=added)
        case = {'map': map_id, 'players': len(initial['players']), 'viewport': viewport('main'), 'initial': initial}
        case['mapRules'] = map_rules_audit(map_id, initial)
        case['friendlyFire'] = {'selected': map_id == 'de_inferno', 'initial': initial['options'].get('friendlyFire', False), 'waitingHostOnly': True}
        case['purchase'] = ground_equipment_audit('main', map_id) if map_id == 'fy_iceworld' else buy_audit('main', map_id, mobile=map_id == 'de_aztec')
        wait_for(lambda: room()['game']['data']['phase'] == 'live', 30)
        measured_client = 'peer' if map_id == 'cs_assault' else 'main'
        case['measuredClient'] = measured_client
        case['assetAudit'] = evaluate(measured_client, 'document.querySelector("[data-cs16-canvas]")?.__cs16Assets')
        assert case['assetAudit'] and case['assetAudit']['texturedMaterialCount'] > 0 and not case['assetAudit']['missingTextures'], case['assetAudit']
        if added:
            case['desktopNative'] = elimination_native('main', map_id, 'desktop-input')
            case['touchNative'] = elimination_native('peer', map_id, 'touch-input', touch=True)
            case['native'] = case['desktopNative']['summary']
            case['touchAssetAudit'] = evaluate('peer', 'document.querySelector("[data-cs16-canvas]")?.__cs16Assets')
            assert case['touchAssetAudit'] and not case['touchAssetAudit']['missingTextures']
        else:
            case['native'] = native(measured_client, map_id + '-input', touch=map_id == 'de_aztec', visualAudit=map_id == 'de_dust2')
        case['measuredViewport'] = viewport(measured_client)
        screenshot(measured_client, map_id + '-measured-client')
        check_action_error('main', map_id + '-after-native')
        case['activeLayout'] = viewport('main')
        check_layout(case['activeLayout'], map_id + '-active')
        screenshot('main', map_id + '-active-controls')
        case['actualWebSocketNetwork'] = request('main', '/state').get('network')
        if map_id in ['de_dust2', 'de_aztec']:
            case['fullscreen'] = fullscreen_audit('main', map_id)
        if map_id == 'de_dust2':
            case['background'] = native('main', map_id + '-background', backgroundOnly=True)
            before = room()['game']['data']
            command('peer', 'autoplay', enabled=True)
            command('peer', 'autoplay', enabled=False)
            after = room()['game']['data']
            assert before['epoch'] == after['epoch'] and before['startAt'] == after['startAt']
            case['handoff'] = {'beforeFrame': before['frame'], 'afterFrame': after['frame'], 'sameMatch': True}
            set_network('peer', 250, 20)
            case['slowPlayerInput'] = native('peer', map_id + '-slow-player')
            case['slowPeerFastInput'] = native('main', map_id + '-slow-peer')
            recovery_capture = begin_native_capture('peer', map_id + '-recovery-wire')
            fast_before = room()['game']['data']['frame']
            set_network('peer', 0, 0, True)
            time.sleep(0.5)
            fast_after = room()['game']['data']['frame']
            assert fast_after - fast_before >= 30
            assert not room()['seats'][1].get('autoPlay')
            set_network('peer', 40, 5)
            browser('peer', 'reload')
            wait_for(lambda: frame('peer') and frame('peer').get('inputEnabled'), 120)
            case['disconnect'] = {'fastFrames': fast_after - fast_before, 'humanNotAutoTaken': True, 'recovered': True}
            case['recoveryWire'] = finish_native_capture(recovery_capture)
        if added:
            set_network('peer', 250, 20)
            case['slowPlayerInput'] = elimination_native('peer', map_id, 'slow-touch-input', touch=True)
            case['slowPeerFastInput'] = elimination_native('main', map_id, 'slow-peer-desktop-input')
            set_network('peer', 40, 5)
        spectator = room('watch')
        assert spectator.get('spectating')
        case['spectator'] = {'readOnly': spectator['game']['data'].get('canAct') is False}
        wait_for(lambda: evaluate('watch', '!!document.querySelector("[data-cs16-board][data-ready=true]")'), 180)
        screenshot('watch', map_id + '-spectator')
        if map_id == 'de_inferno':
            command('main', 'autoplay', enabled=True)
            command('peer', 'autoplay', enabled=True)
            round_result = wait_for(lambda: room()['game']['data'] if room()['game']['data']['phase'] == 'round_end' or room()['game']['data']['round'] > 1 else None, 150)
            assert round_result['options'].get('friendlyFire') is True
            case['friendlyFire']['naturalRoundResult'] = round_result
            screenshot('main', map_id + '-friendly-fire-round')
        if map_id == 'cs_assault' and os.environ.get('CS16_BROWSER_SKIP_FINISH') != '1':
            command('main', 'autoplay', enabled=True)
            command('peer', 'autoplay', enabled=True)
            case['denseWire'] = native('watch', map_id + '-dense-wire', observeOnly=True, observeMS=10000, denseExpected=True)
            finished = wait_for(lambda: room() if room()['status'] == 'finished' or room().get('game', {}).get('finished') else None, 240)
            case['naturalFinish'] = finished
            screenshot('main', map_id + '-natural-finish')
        elif map_id == 'cs_assault':
            REPORT['partial'] = True
        if added and os.environ.get('CS16_BROWSER_SKIP_FINISH') != '1':
            case['manualMatchEpoch'] = initial['epoch']
            cleanup_room()
            natural_match = elimination_match(map_id)
            case['naturalStart'] = natural_match['initial']
            case['denseWire'] = natural_match['denseWire']
            case['naturalFinish'] = natural_match['finished']
        elif added:
            REPORT['partial'] = True
        REPORT['cases'].append(case)
        REPORT['caseCount'] = len(REPORT['cases'])
        save()
        cleanup_room()
    check_case_coverage(REPORT['cases'], selected, os.environ.get('CS16_BROWSER_SKIP_FINISH') == '1')
    after_sources = source_snapshot()
    REPORT['sourceChangedDuringRun'] = [name for name in set(REPORT['sources']) | set(after_sources) if REPORT['sources'].get(name) != after_sources.get(name)]
    after_artifacts = artifact_snapshot()
    REPORT['artifactsChangedDuringRun'] = [name for name in set(REPORT['artifacts']) | set(after_artifacts) if REPORT['artifacts'].get(name) != after_artifacts.get(name)]
    assert REPORT['diagnostic'] or not REPORT['sourceChangedDuringRun'], REPORT['sourceChangedDuringRun']
    assert REPORT['diagnostic'] or not REPORT['artifactsChangedDuringRun'], REPORT['artifactsChangedDuringRun']
    REPORT['passed'] = not REPORT['failures'] and not REPORT['performanceFailures']
except Exception as error:
    REPORT['failures'].append({'error': str(error)})
    for client in SESSIONS:
        try:
            screenshot(client, client + '-failure')
            (OUT / (client + '-failure-dom.txt')).write_text(str(browser(client, 'snapshot')))
        except Exception:
            pass
    raise
finally:
    save()
    for capture in CAPTURES:
        if capture['process'].poll() is None:
            capture['process'].terminate()
            capture['process'].wait(timeout=10)
        capture['log'].close()
    for client in SESSIONS:
        try:
            browser(client, 'close')
        except Exception:
            pass
    (BASE / 'stop').write_text('stop\n')
    if SERVER is not None:
        try:
            SERVER.wait(timeout=20)
        except subprocess.TimeoutExpired:
            SERVER.terminate()
            SERVER.wait(timeout=10)
    for client in ['main', 'peer', 'watch']:
        directory = BASE / (client + '-profile')
        if directory.exists() and directory.is_relative_to(ROOT / '.local/cs16'):
            shutil.rmtree(directory)
