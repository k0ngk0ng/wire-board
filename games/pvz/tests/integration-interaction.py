#!/usr/bin/env python3
"""Real page input, two clients, delayed bytes, native touch and reconnect.
Budgets were declared in integration_FAILURE_SCENARIOS.md before measurement.
This is desktop Chromium with phone emulation, not a physical-phone claim.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/interaction'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ.get('PVZ_BROWSER_URL', '').rstrip('/')
SESSIONS = {}
CURRENT = 'host'
REPORT = {'passed': False, 'command': 'PVZ_BROWSER_URL=<fixture> python3 -B games/pvz/tests/integration-interaction.py',
          'scope': 'production module and browser Wasm; authenticated two-human room; real WebSocket bytes with 320ms test-only RTT; native mouse and CDP touch; phone viewport emulation',
          'budgets': {'inputToPixelP95MS': 100, 'renderIntervalP95MS': 35}, 'inputs': [], 'network': {}, 'rendering': {}}
for name in ['host', 'guest']:
    profile = OUT / 'profiles' / name
    profile.mkdir(parents=True, exist_ok=True)
    socket = ROOT / '.ab-pvz'
    socket.mkdir(exist_ok=True)
    tmp = OUT / 'tmp'
    tmp.mkdir(exist_ok=True)
    SESSIONS[name] = dict(os.environ, AGENT_BROWSER_SESSION='pvz' + name[0] + uuid.uuid4().hex[:4],
                          AGENT_BROWSER_SOCKET_DIR=str(socket), AGENT_BROWSER_PROFILE=str(profile), TMPDIR=str(tmp),
                          AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')

def b(*args):
    r = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=SESSIONS[CURRENT], text=True, capture_output=True, timeout=45)
    if r.returncode: raise RuntimeError(r.stdout + r.stderr)
    d = json.loads(r.stdout)
    assert d['success'], d
    return d['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + opts + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= r['status'] < 300, r
    return r['body']

def room(): return api('/state')['room']

def command(kind, **extra):
    for _ in range(8):
        r = room()
        q = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        value = js('(async()=>{let r=await fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(q) + ')});return {status:r.status,body:await r.json()}})()')
        if value['status'] == 409: continue
        assert value['status'] == 200, value
        return value['body']
    raise AssertionError('command version did not converge')

def wait(code, seconds=30):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if js(code): return
        time.sleep(.1)
    raise AssertionError('browser condition timed out: ' + code)

INIT = r'''(()=>{
 const Native=window.WebSocket;window.__pvzSockets=[];window.__pvzNet={game:[],acks:[],sent:{}};
 window.WebSocket=class extends Native{constructor(...args){super(...args);window.__pvzSockets.push(this);this.addEventListener('message',e=>{try{const p=JSON.parse(e.data),n=window.__pvzNet,at=performance.now();if(p.type==='game'){n.game.push({at,bytes:new TextEncoder().encode(e.data).length,tick:p.game?.data?.tick,full:!!p.game?.data?.snapshot,serverAt:p.game?.data?.serverAt});if(n.game.length>1000)n.game.shift()}if(p.type==='ack')n.acks.push({at,ms:at-(n.sent[p.nonce]??at),status:p.status})}catch{}})}send(raw){try{let p=JSON.parse(raw);if(p.type==='input')window.__pvzNet.sent[p.nonce]=performance.now()}catch{}return super.send(raw)}};
})()'''

PROBE = r'''(()=>{
 const c=document.querySelector('[data-pvz-canvas]');
 window.__pvzMeasures=[];window.__pvzDraw={frames:0,changed:0,intervals:[]};
 window.__pvzFocusLog=[];for(const type of ['focus','blur'])window.addEventListener(type,()=>{window.__pvzFocusLog.push({type,at:performance.now(),focused:document.hasFocus(),active:document.activeElement?.outerHTML.slice(0,160),selected:c.__pvzFrame?.selected});if(window.__pvzFocusLog.length>100)window.__pvzFocusLog.shift()});
 const hash=()=>{let a=c.getContext('2d').getImageData(0,0,c.width,c.height).data,h=2166136261;for(let i=0;i<a.length;i+=128)h=Math.imul(h^a[i],16777619);return h>>>0};
 let pending=null,lastAt=0,lastHash=-1;
 c.addEventListener('pointerdown',e=>{pending={at:performance.now(),trusted:e.isTrusted,before:c.__pvzFrame?.lastActionSeq??0,hash:hash(),selected:c.__pvzFrame?.selected,focused:document.hasFocus(),hit:document.elementFromPoint(e.clientX,e.clientY)?.outerHTML.slice(0,200)};window.__pvzPendingInput=pending;},{capture:true});
 function tick(at){const f=c.__pvzFrame,d=window.__pvzDraw;if(f&&f.renderAt!==lastAt){if(lastAt)d.intervals.push(f.renderAt-lastAt);lastAt=f.renderAt;d.frames++;const h=hash();if(h!==lastHash)d.changed++;lastHash=h;if(pending&&f.lastActionSeq>pending.before&&h!==pending.hash){window.__pvzMeasures.push({inputToPixelMS:(f.pixelAt??f.renderAt)-pending.at,trusted:pending.trusted,seq:f.lastActionSeq,tick:f.tick,at});pending=null}}requestAnimationFrame(tick)}requestAnimationFrame(tick);return true;
})()'''

def touch(x, y):
    value = b('get', 'cdp-url')
    endpoint = value if isinstance(value, str) else value.get('url') or value.get('cdpUrl') or value.get('cdp_url')
    assert endpoint, value
    commands = [{'method': 'Emulation.setTouchEmulationEnabled', 'params': {'enabled': True, 'maxTouchPoints': 1}},
                {'method': 'Input.dispatchTouchEvent', 'params': {'type': 'touchStart', 'touchPoints': [{'x': x, 'y': y, 'radiusX': 2, 'radiusY': 2, 'force': 1}]}},
                {'method': 'Input.dispatchTouchEvent', 'params': {'type': 'touchEnd', 'touchPoints': []}}]
    subprocess.run(['node', 'games/pvz/tests/integration-cdp.mjs', endpoint, BASE, json.dumps(commands)], cwd=ROOT, env=SESSIONS[CURRENT], check=True, capture_output=True, text=True, timeout=15)

def plant(kind, row, col, condition, mobile=False):
    selector = '[data-pvz-card="' + kind + '"]'
    REPORT.setdefault('cardSetup', []).append(js('(()=>{const e=document.querySelector(' + json.dumps(selector) + ');return{kind:' + json.dumps(kind) + ',disabled:e?.disabled,pressed:e?.getAttribute("aria-pressed"),card:e?.outerHTML.slice(0,400),focused:document.hasFocus(),frame:document.querySelector("[data-pvz-canvas]").__pvzFrame}})()'))
    b('click', selector)
    selected = 'document.querySelector(' + json.dumps(selector) + ').getAttribute("aria-pressed")==="true"'
    # Separate headless browser windows may consume their first native click
    # to acquire focus. Require a visibly selected card before measuring the
    # subsequent battlefield input, using another real click if needed.
    if not js(selected): b('click', selector)
    wait(selected, 5)
    b('scrollintoview', '[data-pvz-canvas]')
    rect = js('document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON()')
    x = round(rect['left'] + rect['width'] * (40 + 80 * (col + .5)) / 800)
    y = round(rect['top'] + rect['height'] * (80 + 100 * (row + .5)) / 600)
    REPORT.setdefault('inputSetup', []).append(js('({selected:' + selected + ',focused:document.hasFocus(),frame:document.querySelector("[data-pvz-canvas]").__pvzFrame,hit:document.elementFromPoint(' + str(x) + ',' + str(y) + ')?.outerHTML.slice(0,200)})'))
    before = js('window.__pvzMeasures.length')
    if mobile: touch(x, y)
    else:
        b('mouse', 'move', x, y); b('mouse', 'down'); b('mouse', 'up')
    REPORT.setdefault('afterNative', []).append(js('({selected:' + selected + ',focused:document.hasFocus(),frame:document.querySelector("[data-pvz-canvas]").__pvzFrame,pending:window.__pvzPendingInput,focusLog:window.__pvzFocusLog,error:document.querySelector(".pvz-action-error")?.textContent,hit:document.elementFromPoint(' + str(x) + ',' + str(y) + ')?.outerHTML.slice(0,200)})'))
    wait('window.__pvzMeasures.length>' + str(before), 10)
    measurement = js('window.__pvzMeasures.at(-1)')
    assert measurement['trusted'], measurement
    assert 0 <= measurement['inputToPixelMS'] <= REPORT['budgets']['inputToPixelP95MS'], measurement
    measurement.update(condition=condition, kind=kind, native='touch' if mobile else 'mouse')
    REPORT['inputs'].append(measurement)
    wait('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.plants.some(p=>p.kind===' + json.dumps(kind) + '&&p.row===' + str(row) + '&&p.col===' + str(col) + '))')

try:
    assert BASE, 'Set PVZ_BROWSER_URL to the opt-in production fixture'
    init_path = OUT / 'probe-init.js'
    init_path.write_text(INIT)
    for name in ['host', 'guest']:
        CURRENT = name
        b('open', '--init-script', init_path, BASE)
        api('/register', {'name': '花园' + name + uuid.uuid4().hex[:5], 'password': 'pvz-interaction-test', 'invite': 'pvz-browser'})
        b('reload'); b('set', 'viewport', 1440, 960)
    CURRENT = 'host'
    created = api('/rooms', {'kind': 'pvz', 'name': '原生操作延迟验收', 'capacity': 2, 'options': {'mode': 'duel', 'teams': [0, 1], 'prepSeconds': 30, 'roundSeconds': 360}})
    room_id = created['id']
    join_version = room()['version']
    CURRENT = 'guest'
    api('/rooms/' + room_id, {'type': 'join', 'version': join_version, 'nonce': uuid.uuid4().hex})
    command('ready')
    CURRENT = 'host'; command('ready'); command('start')
    for name in ['host', 'guest']:
        CURRENT = name; b('wait', '[data-pvz-board][data-ready="true"]'); wait('fetch("/api/state").then(r=>r.json()).then(s=>!s.room.loading)'); js(PROBE)
    CURRENT = 'host'
    plant('sunflower', 0, 0, 'normal')
    plant('peashooter', 0, 1, 'normal')
    REPORT['network']['normal'] = api('/state').get('network')
    js('document.cookie="pvz-test-rtt=320;path=/;SameSite=Lax";window.__pvzSockets.forEach(s=>s.close());true')
    wait('window.__pvzSockets.some(s=>s.readyState===1)')
    time.sleep(1)
    b('set', 'viewport', 320, 760)
    plant('wallnut', 1, 1, '320ms-rtt', True)
    plant('potatomine', 2, 1, '320ms-rtt', True)
    REPORT['network']['delayed'] = api('/state').get('network')
    wait('fetch("/api/state").then(r=>r.json()).then(s=>(s.network?.rttMs??0)>=250)', 15)
    REPORT['network']['delayed'] = api('/state').get('network')
    # The fast peer keeps its own transport and receives an advancing clock.
    CURRENT = 'guest'
    wait('document.querySelector("[data-pvz-board]").dataset.phase==="battle"', 45)
    fast_before = js('document.querySelector("[data-pvz-canvas]").__pvzFrame.tick')
    b('click', '[data-pvz-card="zombie"]')
    rect = js('document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON()')
    x = round(rect['left'] + rect['width'] * 780 / 800)
    y = round(rect['top'] + rect['height'] * 130 / 600)
    b('mouse', 'move', x, y); b('mouse', 'down'); b('mouse', 'up')
    time.sleep(3)
    fast_after = js('document.querySelector("[data-pvz-canvas]").__pvzFrame.tick')
    assert fast_after - fast_before >= 45, (fast_before, fast_after)
    REPORT['network']['fastPeer'] = {'before': fast_before, 'after': fast_after, 'info': api('/state').get('network')}
    for name in ['host', 'guest']:
        CURRENT = name
        report = js('({render:window.__pvzDraw,network:window.__pvzNet,frame:document.querySelector("[data-pvz-canvas]").__pvzFrame})')
        intervals = sorted(report['render']['intervals'])
        p95 = intervals[min(len(intervals) - 1, int(len(intervals) * .95))]
        report['render']['intervalP95MS'] = p95
        assert p95 <= REPORT['budgets']['renderIntervalP95MS'], report['render']
        assert any(not p['full'] for p in report['network']['game']), 'only full snapshots were sent'
        REPORT['rendering'][name] = report
        b('screenshot', OUT / (name + '-battle.png'))
    CURRENT = 'host'
    before = room()['game']['data']
    js('document.cookie="pvz-test-rtt=;Max-Age=0;path=/";true')
    b('reload'); b('wait', '[data-pvz-board][data-ready="true"]')
    after = room()['game']['data']
    assert after['epoch'] == before['epoch'] and after['players'][0]['seq'] >= before['players'][0]['seq']
    assert not room()['seats'][0].get('autoPlay'), 'reconnect automatically took over human'
    REPORT['reconnect'] = {'epoch': after['epoch'], 'seq': after['players'][0]['seq']}
    REPORT['passed'] = True
finally:
    if not REPORT['passed']:
        try: REPORT['page'] = js('document.body.innerText'); REPORT['failureFrame'] = js('document.querySelector("[data-pvz-canvas]")?.__pvzFrame'); REPORT['failureInput'] = js('({pending:window.__pvzPendingInput,focusLog:window.__pvzFocusLog,error:document.querySelector(".pvz-action-error")?.textContent})'); b('screenshot', OUT / 'failure.png', '--full')
        except Exception: pass
    for name in SESSIONS:
        CURRENT = name
        try: b('close')
        except Exception: pass
    shutil.rmtree(OUT / 'profiles', ignore_errors=True)
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed'], 'inputs': REPORT['inputs']}, ensure_ascii=False))
