import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import time
from urllib.request import Request, urlopen
import uuid

ROOT = Path(__file__).resolve().parents[3]
assert len(sys.argv) == 2 and re.fullmatch(r'[a-z][a-z0-9-]{0,39}', sys.argv[1])
OUT = ROOT / '.local/pal/performance' / ('battle-recovery-' + sys.argv[1])
assert not (OUT / 'result.json').exists()
BASE = (OUT / 'url').read_text().strip()
PROFILE = OUT / 'profile'
PROFILE.mkdir()
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/prb'), AGENT_BROWSER_PROFILE=str(PROFILE), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
SESSION = subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','prb'],cwd=ROOT,env=ENV,text=True).strip() + '-' + uuid.uuid4().hex[:6]
REPORT = {'passed': False, 'scope': 'authenticated solo PAL 98 original battle fixture; actual TCP interruption and hidden tab; desktop key and 320px native touch', 'cases': []}


def process(*argv, **options):
    return subprocess.Popen(argv, cwd=ROOT, env=ENV, text=True, **options)


def rpc(proc, message):
    proc.stdin.write(json.dumps(message) + '\n')
    proc.stdin.flush()
    return json.loads(proc.stdout.readline())


def browser(*argv):
    result = subprocess.run(['agent-browser','--session',SESSION,*map(str,argv),'--json'], cwd=ROOT, env=ENV, text=True, capture_output=True, timeout=30)
    assert result.returncode == 0, result.stdout + result.stderr
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']


def js(expression):
    return browser('eval', expression)['result']


def wait(test, reason, timeout=20):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        result = test()
        if result:
            return result
        time.sleep(.04)
    raise AssertionError(reason)


def record(check, **extra):
    row = dict(check=check, **extra)
    REPORT['cases'].append(row)
    print(json.dumps(row, ensure_ascii=False), flush=True)


def drive(kind, **fields):
    return rpc(driver, dict(type=kind, **fields))


def network(**fields):
    return rpc(proxy, fields)


def api(path, data=None):
    headers = {'Cookie': 'wb_session=' + cookie}
    if data is not None:
        headers['Content-Type'] = 'application/json'
    request = Request(BASE + '/api' + path, data=None if data is None else json.dumps(data).encode(), headers=headers)
    with urlopen(request, timeout=4) as response:
        return json.load(response)


def room():
    return api('/state')['room']


def view():
    return room()['game']['data']


def battle():
    return view().get('battle', {})


def command(kind):
    state = room()
    return api('/rooms/' + state['id'], {'type':kind,'version':state['version'],'nonce':uuid.uuid4().hex})


def role(index):
    b = battle()
    return b.get('phase') == 0 and b.get('state') == 1 and b.get('menu') == 0 and b.get('current') == index


def glyph(d, char, x, ymin, ymax):
    raster = d.get('raster')
    if not raster:
        rows = [op for op in d.get('overlay', []) if op['kind'] == 0 and op['id'] == ord(char)]
    else:
        rows = []
        pending = [raster['root']]
        seen = set()
        while pending:
            i = pending.pop()
            if i <= 0 or i > len(raster['nodes']) or i in seen:
                continue
            seen.add(i)
            node = raster['nodes'][i-1]
            pending.extend(node[1:3])
            if node[0] == 2 and node[5] == ord(char):
                rows.append(dict(x=node[6], y=node[7], color=node[8]))
    return next((row for row in rows if row['x'] == x and ymin <= row['y'] <= ymax and row['color'] > 0), None)


def touch(x, y):
    px, py = js(f'(()=>{{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+{x}*r.width/320,r.y+{y}*r.height/200]}})()')
    subprocess.run(['node','games/pal/tests/screen-touch.mjs',endpoint,URL,'tap',*map(str,[px,py,px,py])], cwd=ROOT, env=ENV, check=True, timeout=10)


def key(code, down, repeat=False):
    return drive('key', code=code, down=down, repeat=repeat)


def icon(index):
    xy = [(39,147),(12,162),(66,162),(39,177)][index]
    if battle()['selected'] != index:
        touch(*xy)
        wait(lambda:battle()['selected'] == index, 'battle icon not selected')
    touch(*xy)


def defend(index):
    wait(lambda:role(index), 'next original role not ready')
    icon(3)
    wait(lambda:battle()['menu'] == 4, 'original battle misc menu did not open')
    read = lambda:glyph(view(), '防', 16, 32, 104)
    item = wait(read, 'original defense glyph missing')
    if not 249 <= item['color'] <= 254:
        touch(item['x']+8,item['y']+8)
        wait(lambda:249 <= (read() or {}).get('color',0) <= 254, 'defense not selected')
    touch(item['x']+8,item['y']+8)
    wait(lambda:battle()['players'][index]['action'] == 1, 'defense not accepted')


def trace():
    return drive('trace')['events']


def reconnect(start):
    def state():
        tail = trace()[start:]
        first = next((i for i,e in enumerate(tail) if e['kind']=='closed'), None)
        if first is None:
            return
        opened = next((i for i,e in enumerate(tail[first+1:], first+1) if e['kind']=='connected'), None)
        if opened is not None and any(e['kind']=='state' and e.get('room')==room_id and not e['loading'] for e in tail[opened+1:]):
            return tail
    return wait(state, 'WebSocket did not close, reconnect and send the actual room snapshot')


def surface():
    status = drive('status')
    assert status['sameCanvas'] and status['page'] == original_page and not status['loading'] and status['ready'] == 'true' and not status.get('error'), status
    state = room()
    assert state['id'] == room_id and state['matchId'] == match_id and not state.get('loading') and not state['seats'][0].get('autoPlay'), state['seats']
    return status


def pixels():
    return js('(async()=>{const {authorityPixelsMatch}=await import("/pal-renderer-probe.js");const r=await fetch("/api/state");return authorityPixelsMatch((await r.json()).room.game.data);})()')


def selection():
    d = view()
    g = glyph(d, '气', 35, 54, 104)
    return d['battle']['menu'], d['battle']['state'], d['battle']['selected'], d.get('menu',{}).get('selected'), g and [g['x'],g['y'],249 <= g['color'] <= 254]


def offensive_inputs(events):
    return [e for e in events if e['kind'] == 'sent' and e['action'].get('keys')]


def original_spell(object_id):
    original = ROOT/'.local/pal/original-98'
    sss, data = [(original/name).read_bytes() for name in ('sss.mkf','data.mkf')]
    object_offset = struct.unpack_from('<I',sss,8)[0]
    magic_offset = struct.unpack_from('<I',data,16)[0]
    magic, _, success, *_ = struct.unpack_from('<7H',sss,object_offset+object_id*14)
    cost = struct.unpack_from('<H',data,magic_offset+magic*32+24)[0]
    script_offset = struct.unpack_from('<I',sss,16)[0]
    op, actor, recovery, unused = struct.unpack_from('<4H',sss,script_offset+success*8)
    assert (object_id,magic,cost,success,op,actor,recovery,unused) == (296,33,6,43016,27,0,75,0)
    return {'object':object_id,'magic':magic,'costMP':cost,'maxRecoveryHP':recovery,'successScript':success,'dataSHA256':hashlib.sha256(data).hexdigest(),'sssSHA256':hashlib.sha256(sss).hexdigest()}


proxy = driver = device = cookie = None
try:
    proxy = process('node','games/pal/tests/performance-proxy.mjs',BASE,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
    URL = json.loads(proxy.stdout.readline())['url']
    browser('open',URL)
    browser('set','viewport',1280,900)
    registration = {'name':'战斗恢复'+uuid.uuid4().hex[:8],'password':'pal-recovery-case-123','invite':'test-invite'}
    assert js('(async()=>{const r=await fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(registration)+')});return r.ok;})()')
    endpoint = browser('get','cdp-url')['cdpUrl']
    driver = process('node','games/pal/tests/lifecycle-driver.mjs',endpoint,URL,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
    assert driver.stdout.readline().strip() == 'ready'
    cookie = drive('session-cookie')
    assert cookie
    api('/rooms', {'kind':'pal','name':'原版战斗恢复','capacity':1})
    command('ready')
    command('start')
    browser('reload')
    wait(lambda:not room().get('loading') and role(0), 'original solo battle not ready', 60)
    browser('wait','[data-pal-ready=true]')
    browser('click','[data-fullscreen-enter]')
    browser('wait','[data-room-fullscreen]')
    room_id, match_id = room()['id'],room()['matchId']
    drive('remember-canvas')
    original_page = drive('status')['page']
    manifest = json.loads((OUT/'web-dist/game-load-manifest.json').read_bytes())
    scripts = [manifest['home']['files'][0]['url'],manifest['games']['pal'][0]['url']]
    resources = js('[...new Set(performance.getEntriesByType("resource").map(r=>new URL(r.name).pathname))]')
    assert all(path in resources for path in scripts), scripts
    REPORT['browserBundles'] = []
    for path in sorted(scripts):
        payload = urlopen(URL+path, timeout=4).read()
        assert payload == (OUT/'web-dist'/path.lstrip('/')).read_bytes()
        REPORT['browserBundles'].append({'path':path,'bytes':len(payload),'sha256':hashlib.sha256(payload).hexdigest()})

    icon(1)
    wait(lambda:battle()['menu'] == 1 and glyph(view(),'气',35,54,104), 'original spell menu missing')
    wait(pixels, 'opened spell menu pixels do not match current authority')
    before = selection()
    seq = view()['seq']
    browser('screenshot',OUT/'magic-before.png')
    start = len(trace())
    network(pauseUp=True,reset=True)
    key('ArrowDown', True)
    dropped = wait(lambda:next(iter(offensive_inputs(trace()[start:])),None), 'held key never entered transport')
    stalled = network()
    assert stalled['stats']['uploaded'] == 0 and stalled['stats']['maxQueueBytes'] > 0, stalled
    assert view()['seq'] == seq and selection() == before
    network(pauseUp=False,pauseDown=True,disconnect=True)
    wait(lambda:any(e['kind']=='closed' for e in trace()[start:]), 'WebSocket did not close')
    key('Enter',True)
    key('Enter',False)
    assert view()['seq'] == seq and selection() == before
    network(pauseDown=False)
    reconnected = reconnect(start)
    key('ArrowDown', True, True)
    time.sleep(.34)
    key('ArrowDown', False)
    assert not any(e['kind']=='ack' and e['nonce']==dropped['nonce'] for e in trace())
    assert len(offensive_inputs(trace()[start:])) == 1 and view()['seq'] == seq and selection() == before
    wait(pixels, 'reconnected original spell menu did not converge in the real canvas')
    record('unacknowledged native hold discarded', state=surface(), network=stalled, selection=before, reconnect=reconnected)

    key('ArrowDown',True)
    wait(lambda:view()['seq'] > seq,'fresh desktop keyboard input did not reach authority')
    hidden = drive('background')
    assert hidden['hidden'] and not hidden['focused'], hidden
    marker = len(trace())
    time.sleep(.42)
    background_selection = selection()
    visible = drive('foreground')
    assert not visible['hidden'] and visible['focused'], visible
    key('ArrowDown',True,True)
    time.sleep(.32)
    key('ArrowDown',False)
    assert not offensive_inputs(trace()[marker:]) and selection() == background_selection
    wait(pixels, 'foreground spell menu pixels did not converge')
    record('real background and foreground reject old key repeat', hidden=hidden, foreground=visible, state=surface())

    browser('set','viewport',320,820)
    device = process('node','scripts/browser_device.mjs',endpoint,URL,'320','820','--keep',stdout=subprocess.PIPE)
    assert device.stdout.readline().strip() == 'ready'
    if not js('!!document.querySelector("[data-room-fullscreen]")'):
        browser('click','[data-fullscreen-enter]')
        browser('wait','[data-room-fullscreen]')
    geometry = js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height,matchMedia("(pointer:coarse)").matches,document.querySelectorAll("[data-pal-controls] button").length]})()')
    assert geometry == [320,200,True,0], geometry
    healing = wait(lambda:glyph(view(),'气',35,54,104),'original healing glyph missing')
    if not 249 <= healing['color'] <= 254:
        touch(healing['x']+8,healing['y']+8)
        wait(lambda:249 <= (glyph(view(),'气',35,54,104) or {}).get('color',0) <= 254,'native phone did not select healing spell')
    touch(healing['x']+8,healing['y']+8)
    wait(lambda:battle()['state']==3,'native phone touch did not open original ally target selection')
    if battle()['target'] != 1:
        touch(198,180)
        wait(lambda:battle()['target']==1,'original Ling HP panel did not select her')
    initial_party = view()['party']
    assert initial_party[1]['hp'] == 28
    start = len(trace())
    network(pauseDown=True,reset=True)
    touch(198,180)
    wait(lambda:(p if (p:=battle()['players'][0])['state']==2 else None),'authoritative healing command not committed')
    initial_command = battle()['players'][0]
    assert initial_command['action']==3 and initial_command['target']==1, initial_command
    spell = original_spell(initial_command['object'])
    committed = wait(lambda:next((e for e in offensive_inputs(trace()[start:]) if e['action']['keys'] == 2),None),'native healing confirmation absent on stream')
    stalled = network()
    assert stalled['stats']['downloaded'] == 0 and stalled['stats']['uploaded'] > 0, stalled
    assert not any(e['kind']=='ack' and e['nonce']==committed['nonce'] for e in trace())
    network(pauseDown=False,disconnect=True)
    reconnected = reconnect(start)
    assert not any(e['kind']=='ack' and e['nonce']==committed['nonce'] for e in trace())
    p = battle()['players'][0]
    assert [p[k] for k in ('state','action','target','object')] == [initial_command[k] for k in ('state','action','target','object')] and role(1), p
    assert view()['party'] == initial_party and len(offensive_inputs(trace()[start:])) == 1
    browser('screenshot',OUT/'committed-after.png')
    record('committed before ACK; original command survives reconnect', state=surface(), geometry=geometry, command=p, network=stalled, reconnect=reconnected)

    defend(1)
    defend(2)
    wait(lambda:role(0) and view()['party'][1]['hp'] > initial_party[1]['hp'],'committed healing did not execute',60)
    healed = view()['party']
    assert initial_party[1]['hp'] < healed[1]['hp'] <= initial_party[1]['hp']+spell['maxRecoveryHP'], healed
    assert initial_party[0]['mp']-healed[0]['mp'] == spell['costMP'], healed
    browser('screenshot',OUT/'healed.png')
    defend(0)
    defend(1)
    wait(lambda:role(2),'third role not ready after reconnect')
    icon(0)
    wait(lambda:view()['cash']==3,'native final attack did not grant three-cash reward',60)
    browser('screenshot',OUT/'victory.png')
    status = surface()
    assert view()['cash'] == 3
    record('one healing and one original reward', before=initial_party, healed=healed, spell=spell, cash=view()['cash'], state=status)
    REPORT['trace'] = trace()
    assert not status['interruptions'], status['interruptions']
    REPORT['passed'] = True
except Exception as error:
    REPORT['error'] = str(error)
    if driver and driver.poll() is None:
        REPORT['debugSocket'] = trace()
        REPORT['debugSurface'] = drive('status')
        if cookie:
            REPORT['debugBattle'] = battle()
        browser('screenshot',OUT/'failure.png')
    raise
finally:
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    if device:
        device.terminate()
        device.wait(timeout=5)
    if driver:
        driver.stdin.close()
        driver.wait(timeout=5)
    if proxy:
        proxy.terminate()
        proxy.wait(timeout=5)
    try:
        browser('close')
    finally:
        shutil.rmtree(PROFILE)
        (OUT/'stop').touch()
