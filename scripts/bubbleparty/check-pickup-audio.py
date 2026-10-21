#!/usr/bin/env python3
"""Real four-seat HTTP/SQLite game with owner/ally/enemy/observer browsers.
Failure inventory (before implementation): drop sound, pickup broadcast to allies,
enemies or observers, seat-zero exclusion, duplicate snapshots, muted catch-up,
reload/reconnect replay. No game-state injection; media playback is observed only.
"""
import argparse, collections, concurrent.futures, hashlib, http.cookiejar, json, os
from pathlib import Path
import shutil, subprocess, time, traceback, urllib.request, uuid

root = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
out = (root / args.out).resolve()
assert out.is_relative_to(root) and out != root and not out.exists()
out.mkdir(parents=True)
temp = out / 'temporary'
run = 'pa' + uuid.uuid4().hex[:6]
sockets = root / '.s' / run
env = dict(os.environ)
for key, path in {'TMPDIR': temp/'tmp', 'GOTMPDIR': temp/'tmp', 'GOCACHE': root/'.local/go-build', 'GOMODCACHE': root/'.local/go-mod', 'AGENT_BROWSER_SOCKET_DIR': sockets}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_BUBBLEPARTY_BROWSER='1', WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out), WIRE_BOARD_REALTIME_NETWORK_CONTROL='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
roles = ['owner', 'ally', 'enemy', 'observer']
opened = []
observers = []
server = log = None
report = {'passed': False, 'checks': [], 'scope': 'real HTTP/SQLite; four human seats; four independent browser sessions; passive HTMLMediaElement playback log; legal input only', 'distSHA256': hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()}
report['sources'] = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'games/bubbleparty/web/Board.tsx', root/'games/bubbleparty/web/pickup-audio.ts', Path(__file__).resolve()]}
clients = {}

def browser(role, *args):
    owned = dict(env, AGENT_BROWSER_SESSION=run+role, AGENT_BROWSER_PROFILE=str(temp/role))
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=owned, text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stdout + result.stderr
    parsed = json.loads(result.stdout)
    assert parsed['success'], parsed
    return parsed['data']

def js(role, code):
    return browser(role, 'eval', code)['result']

def request(role, path, body=None):
    req = urllib.request.Request(base + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
    with clients[role].open(req, timeout=10) as response:
        return json.load(response)

def room(role='owner'):
    return request(role, '/state')['room']

def view():
    return room()['game']['data']

def command(role, kind, **extra):
    r = room(role)
    return request(role, '/rooms/' + r['id'], {'type': kind, 'version': r['version'], 'nonce': uuid.uuid4().hex, **extra})

def action(move='none', bomb=False):
    r = room(); v = r['game']['data']; p = v['players'][0]
    assert p['alive'] and not p['trappedUntil'], p
    return command('owner', 'action', action={'type': 'input', 'epoch': v['epoch'], 'seq': p['seq'] + 1, 'move': move, 'bomb': bomb})

def wait(fn, seconds=10):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        value = fn()
        if value:
            return value
        time.sleep(.03)
    raise AssertionError('condition timed out')

def check(name, **evidence):
    report['checks'].append({'name': name, **evidence})
    print('PASS', name, flush=True)

def parallel(fn):
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        return dict(zip(roles, pool.map(fn, roles)))

def logs():
    return parallel(lambda role: js(role, 'JSON.parse(localStorage.getItem("pickup-audio-audit")||"[]")'))

def item_counts():
    return {role: sum('/item.wav' in item['src'] or '/ItemEaten.wav' in item['src'] for item in values) for role, values in logs().items()}

# Capturing native play events does not replace play(), source selection or the
# platform audio channel. Install before reloads as well to detect stale replays.
recorder = '''(()=>{if(window.__pickupAudit)return;window.__pickupAudit=true;document.addEventListener('play',e=>{if(!(e.target instanceof HTMLMediaElement))return;const a=JSON.parse(localStorage.getItem('pickup-audio-audit')||'[]');a.push({at:Date.now(),src:(e.target.currentSrc||e.target.src).startsWith('data:')?'data:audio':e.target.currentSrc||e.target.src,muted:e.target.muted});localStorage.setItem('pickup-audio-audit',JSON.stringify(a));},true);})()'''
cdp_install = r'''const [endpoint,url,source]=process.argv.slice(2);
const ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let id=0;
async function call(method,params={},sessionId){const n=++id;return new Promise((r,j)=>{const t=setTimeout(()=>j(Error('CDP timeout')),10000);const h=e=>{const v=JSON.parse(e.data);if(v.id!==n)return;clearTimeout(t);ws.removeEventListener('message',h);v.error?j(Error(JSON.stringify(v.error))):r(v.result)};ws.addEventListener('message',h);ws.send(JSON.stringify({id:n,method,params,sessionId}))})}
let sessionId;try{const {targetInfos}=await call('Target.getTargets');const target=targetInfos.find(x=>x.type==='page'&&x.url.startsWith(url));if(!target)throw Error('owned target missing');({sessionId}=await call('Target.attachToTarget',{targetId:target.targetId,flatten:true}));await call('Page.enable',{},sessionId);await call('Page.addScriptToEvaluateOnNewDocument',{source},sessionId);await call('Runtime.evaluate',{expression:source},sessionId);console.log('READY');process.stdin.resume();await new Promise(r=>process.stdin.on('end',r))}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}
'''

def install_recorder(role):
    endpoint = browser(role, 'get', 'cdp-url')['cdpUrl']
    script = temp / ('recorder-' + role + '.mjs')
    script.write_text(cdp_install)
    observer = subprocess.Popen(['node', str(script), endpoint, base, recorder], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, cwd=root, text=True)
    observers.append(observer)
    assert observer.stdout.readline().strip() == 'READY', 'audio recorder failed to attach'

DIRS = [(1, 0, 'right'), (-1, 0, 'left'), (0, 1, 'down'), (0, -1, 'up')]

def paths(v, start, blocked=()):
    found = {start: []}; queue = collections.deque([start])
    occupied = {(p['x'], p['y']) for p in v['players'][1:] if p['alive']}
    while queue:
        x, y = queue.popleft()
        for dx, dy, move in DIRS:
            q = x+dx, y+dy
            if not (0 <= q[0] < 15 and 0 <= q[1] < 13) or q in found or q in occupied or q in blocked:
                continue
            if v['cells'][q[1]*15+q[0]] != 0:
                continue
            found[q] = found[x, y] + [(q, move)]
            queue.append(q)
    return found

def walk(path):
    for target, move in path:
        action(move)
        axis = 'px' if move in ('left', 'right') else 'py'
        sign = 1 if move in ('right', 'down') else -1
        goal = target[0 if axis == 'px' else 1]
        wait(lambda: sign * (view()['players'][0][axis] - goal) >= -.12, 3)
        action()
    action()

def reveal_items():
    for attempt in range(20):
        v = view(); p = v['players'][0]; start = p['x'], p['y']; routes = paths(v, start)
        reachable = [it for it in v['items'] if (it['x'], it['y']) in routes]
        if reachable:
            return reachable
        candidates = []
        for at, route in routes.items():
            if not any(0 <= at[0]+dx < 15 and 0 <= at[1]+dy < 13 and v['cells'][(at[1]+dy)*15+at[0]+dx] == 2 for dx, dy, _ in DIRS):
                continue
            escape_routes = paths(v, at)
            safe = [(q, path) for q, path in escape_routes.items() if path and (q[0] != at[0] and q[1] != at[1] or abs(q[0]-at[0])+abs(q[1]-at[1]) > p['range'])]
            if safe:
                escape = min(safe, key=lambda pair: len(pair[1]))[1]
                candidates.append((route, escape))
        assert candidates, 'no reachable crate with escape route'
        route, escape = min(candidates, key=lambda pair: len(pair[0]))
        walk(route); action(bomb=True); walk(escape)
        wait(lambda: not view()['bombs'] and not view()['blasts'], 8)
        report.setdefault('bombAttempts', []).append({'route': route, 'escape': escape})
    raise AssertionError('no reachable dropped item after 20 crates')

try:
    log = (out/'server.log').open('w')
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=12m', '-run', '^TestBubblepartyBrowserFixture$', './games/bubbleparty/tests'], cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT)
    wait(lambda: (out/'url').exists(), 120)
    base = (out/'url').read_text()
    for role in roles + ['fourth']:
        clients[role] = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        name = '泡泡验收管理员' if role == 'owner' else '拾取音效验收' + role
        request(role, '/login' if role == 'owner' else '/register', {'name': name, 'password': 'bubbleparty-browser-test-only', 'invite': 'bubbleparty-browser'})
    made = request('owner', '/rooms', {'kind': 'bubbleparty', 'name': '拾取声音仅本人', 'capacity': 4, 'options': {'map': 'forest', 'roundSeconds': 300}})
    for role in ['ally', 'enemy', 'fourth']:
        request(role, '/rooms/'+made['id'], {'type': 'join', 'version': room()['version'], 'nonce': uuid.uuid4().hex})
    for role in ['owner', 'ally', 'enemy', 'fourth']:
        command(role, 'ready')
    command('owner', 'start')
    selection = view()
    assert selection['phase'] == 'select', selection['phase']
    for seat, role in enumerate(['owner', 'ally', 'enemy', 'fourth']):
        command(role, 'action', action={'type': 'character', 'epoch': selection['epoch'], 'character': seat, 'confirm': True})
    request('observer', '/rooms/'+made['id']+'/watch', {})
    for role in roles:
        browser(role, 'open', base); opened.append(role)
        name = '泡泡验收管理员' if role == 'owner' else '拾取音效验收' + role
        js(role, '(async()=>{const r=await fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name': name, 'password': 'bubbleparty-browser-test-only'})+')});if(!r.ok)throw Error(await r.text());return true})()')
        install_recorder(role)
        browser(role, 'reload'); browser(role, 'wait', '.bubble-board')
        assert js(role, 'window.__pickupAudit===true'), 'playback observer not installed after reload'
        browser(role, 'set', 'viewport', '1280', '1000')
        if js(role, 'document.querySelector(".sound-toggle").getAttribute("aria-label")==="取消静音"'):
            browser(role, 'click', '.sound-toggle')
        browser(role, 'click', '.sound-preview')
        assert not js(role, 'document.hidden')
    wait(lambda: view()['phase'] == 'playing')
    players = view()['players']
    assert players[0]['team'] == players[1]['team'] != players[2]['team']
    assert room('observer')['spectating']
    check('owner ally enemy observer are independent real clients', seats=players, observer=room('observer')['you'])
    before = item_counts()
    available = reveal_items()
    assert item_counts() == before, (before, item_counts())
    check('destroying crates and revealing items is silent', items=available, counts=before)
    v = view(); p = v['players'][0]; routes = paths(v, (p['x'], p['y']))
    chosen = min(available, key=lambda it: len(routes[it['x'], it['y']]))
    walk(routes[chosen['x'], chosen['y']])
    picked = wait(lambda: next((e for e in view()['events'] if e['type'] == 'item' and e['seat'] == 0 and e.get('value') == chosen['id']), None))
    wait(lambda: item_counts()['owner'] > before['owner'])
    after = item_counts()
    assert after['owner'] == before['owner'] + 1 and all(after[role] == before[role] for role in roles[1:]), (before, after)
    check('pickup plays once only for owner, never ally enemy observer', event=picked, before=before, after=after)
    time.sleep(1)
    assert item_counts() == after
    with urllib.request.urlopen(urllib.request.Request(base+'/_test/disconnect', data=b'', method='POST')) as response:
        disconnected = json.load(response)
    time.sleep(2)
    assert item_counts() == after
    browser('owner', 'reload'); browser('owner', 'wait', '.bubble-board')
    time.sleep(1)
    assert item_counts() == after
    check('duplicate snapshots reconnect and reload do not replay pickups', disconnected=disconnected, counts=after)
    # A second natural pickup while muted must not be caught up after unmuting.
    available = reveal_items()
    browser('owner', 'click', '.sound-toggle[aria-label=静音]')
    v = view(); p = v['players'][0]; routes = paths(v, (p['x'], p['y']))
    chosen = min(available, key=lambda it: len(routes[it['x'], it['y']]))
    walk(routes[chosen['x'], chosen['y']])
    wait(lambda: next((e for e in view()['events'] if e['type'] == 'item' and e['seat'] == 0 and e.get('value') == chosen['id']), None))
    time.sleep(.5)
    assert item_counts() == after
    browser('owner', 'click', '.sound-toggle[aria-label=取消静音]')
    time.sleep(.5)
    assert item_counts() == after
    check('muted pickup remains silent after unmuting', counts=after)
    report['media'] = logs()
    for role in roles:
        browser(role, 'screenshot', str(out/(role+'.png')), '--full')
        assert not browser(role, 'errors')['errors']
    report['passed'] = True
except BaseException:
    report['error'] = traceback.format_exc(); print(report['error'], flush=True)
    if opened:
        try: report['media'] = logs(); report['lastView'] = view()
        except BaseException: pass
finally:
    for observer in observers:
        observer.stdin.close()
        try: observer.wait(timeout=10)
        except subprocess.TimeoutExpired: observer.kill(); observer.wait(); report['passed'] = False
    for role in opened:
        try: browser(role, 'close')
        except BaseException: report['passed'] = False
    if server:
        (out/'stop').write_text('stop')
        try: server.wait(timeout=20)
        except subprocess.TimeoutExpired: server.kill(); server.wait(); report['passed'] = False
        if server.returncode: report['passed'] = False
    if log: log.close()
    report['cleaned'] = []
    for path in [temp, sockets, out/'data']:
        if path.exists(): shutil.rmtree(path); report['cleaned'].append(str(path.relative_to(root)))
    for name in ['url', 'stop']: (out/name).unlink(missing_ok=True)
    (out/'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    print(out/'report.json', flush=True)
    if not report['passed']: raise SystemExit(1)
