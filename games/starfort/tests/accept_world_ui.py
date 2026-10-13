#!/usr/bin/env python3
"""Growing world real-browser E2E; failure cases precede implementation in
world-ui-acceptance.md. No production state mutation or mocked game responses.
Fixtures only position a real HTTP room at milestones; real actions/timers settle.
Run: . scripts/env.sh && WIRE_BOARD_STARFORT_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B games/starfort/tests/accept_world_ui.py
Use --skip-build only after the matching production frontend build succeeds.
Artifacts: .local/starfort-world/browser/{result.json,commands.jsonl,*.png,*.json}.
"""
import hashlib
import http.cookiejar
import json
import math
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / os.environ.get('WIRE_BOARD_STARFORT_BROWSER_OUT', '.local/starfort-world/browser')
OUT.mkdir(parents=True, exist_ok=True)
ENV = dict(os.environ)
for key, relative in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/ab-starfort-world'}.items():
    directory = ROOT / relative
    directory.mkdir(parents=True, exist_ok=True)
    ENV[key] = str(directory)
SESSION = 'sf-world-' + uuid.uuid4().hex[:8]
PROFILE = OUT / 'profiles' / SESSION
PROFILE.mkdir(parents=True, exist_ok=True)
ENV.update(GOTOOLCHAIN='local', WIRE_BOARD_STARFORT_BROWSER='1', WIRE_BOARD_STARFORT_BROWSER_OUT=str(OUT), AGENT_BROWSER_SESSION=SESSION, AGENT_BROWSER_PROFILE=str(PROFILE), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
REPORT = {'passed': False, 'checks': [], 'command': ' '.join(sys.argv), 'assets': ENV.get('WIRE_BOARD_STARFORT_ASSETS', ''), 'scope': 'real Chromium DOM clicks, HTTP, websocket, server timers and production Canvas. Mouse/viewport are browser emulation; pinch is synthetic PointerEvents, not physical phone input.', 'limitations': ['Physical phone gestures and audio hardware are not exercised.'], 'scenarioNotes': 'Milestone fixture skips earlier waves. objectives-clear skips combat and places convoy 10 units from home on second ready; real production timer moves it, settles mission rewards and advances waves.'}
SERVER = None
SOURCE_START = {}
OPENED = False
BASE = ''
PEERS = []
MAIN = '扩展主守望者'
SPECTATOR = None
(OUT / 'commands.jsonl').write_text('')


def artifact(name, value):
    (OUT / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def source_hashes():
    files=list((ROOT/'games/starfort').rglob('*.go'))+list((ROOT/'games/starfort/web').glob('*'))+[Path(__file__)]
    files += [ROOT/'games/starfort/assets/public/starfort'/version/'atlas.json' for version in ['v1','world-v2']]
    return {str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest() for path in files if path.is_file()}


def run(args, log):
    with (OUT / log).open('w') as handle:
        subprocess.run(args, cwd=ROOT, env=ENV, stdout=handle, stderr=subprocess.STDOUT, check=True)


def b(*args):
    if args[0] in ['select','fill']:
        selector=json.dumps(args[1])
        wait('!!document.querySelector('+selector+')')
        summaries=js('(()=>{let e=document.querySelector('+selector+'),a=[];for(let p=e.parentElement;p;p=p.parentElement)if(p.tagName==="DETAILS"&&!p.open){const id="accept-detail-"+a.length;p.dataset.acceptDetail=id;a.push("[data-accept-detail="+id+"] > summary")}return a.reverse()})()')
        for summary in summaries: b('click',summary)
    if args[:2] == ('mouse', 'move'):
        args = ('mouse', 'move', str(round(float(args[2]))), str(round(float(args[3]))))
    started = time.monotonic()
    result = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=ENV, capture_output=True, text=True, timeout=45)
    with (OUT / 'commands.jsonl').open('a') as handle:
        handle.write(json.dumps({'args': args, 'returncode': result.returncode, 'elapsed': round(time.monotonic()-started, 3), 'stdout': result.stdout, 'stderr': result.stderr}, ensure_ascii=False) + '\n')
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    data = json.loads(result.stdout)
    assert data['success'], data
    return data['data']


def js(code): return b('eval', code)['result']
def wait(code): b('wait', '--fn', code)

def click(selector):
    if ('data-sf-route-choice' in selector or 'data-sf-boon-choice' in selector) and not js('!!document.querySelector('+json.dumps(selector)+')'):
        b('click','[data-sf-open-choice]')
    if js('(()=>{const d=document.querySelector("[role=dialog]"),e=document.querySelector('+json.dumps(selector)+');return !!d&&!!e&&!d.contains(e)})()'):
        b('click','[role=dialog] [aria-label="关闭"]')
    wait('(()=>{const e=document.querySelector(' + json.dumps(selector) + ');return !!e&&!e.disabled})()')
    summaries=js('(()=>{let e=document.querySelector('+json.dumps(selector)+'),a=[];for(let p=e?.parentElement;p;p=p.parentElement)if(p.tagName==="DETAILS"&&!p.open){const id="accept-detail-"+a.length;p.dataset.acceptDetail=id;a.push("[data-accept-detail="+id+"] > summary")}return a.reverse()})()')
    for summary in summaries: b('click',summary)
    if selector.startswith('[data-sf-pad=') and js('document.querySelector('+json.dumps(selector)+').hidden'):
        pad_id=int(js('document.querySelector('+json.dumps(selector)+').dataset.sfPad'))
        pad=next(p for p in data()['map']['pads'] if p['id']==pad_id)
        b('scrollintoview','[data-sf-minimap]')
        point=js('(()=>{const e=document.querySelector("[data-sf-minimap]"),p=e.createSVGPoint();p.x='+str(pad['x'])+';p.y='+str(pad['y'])+';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()')
        mouse_click(point['x'],point['y'])
        wait('!document.querySelector('+json.dumps(selector)+').hidden')
    b('scrollintoview', '[data-starfort-canvas]' if selector.startswith('[data-sf-pad=') else selector)
    b('click', selector)


def label(name):
    wait('[...document.querySelectorAll("button")].some(e=>e.textContent.trim()===' + json.dumps(name) + '&&!e.disabled)')
    b('find', 'role', 'button', 'click', '--name', name, '--exact')


def api(path, body=None, status=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('fetch(' + json.dumps('/api' + path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert value['status'] == status if status else 200 <= value['status'] < 300, value
    return value['body']


def room(): return api('/state')['room']
def data(): return room()['game']['data']


def command(kind, **extra):
    for _ in range(20):
        current = room()
        payload = dict(type=kind, version=current['version'], nonce=uuid.uuid4().hex, **extra)
        value = js('fetch(' + json.dumps('/api/rooms/' + current['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(payload) + ')}).then(async r=>({status:r.status,body:await r.json()}))')
        if value['status'] == 409: continue
        assert value['status'] == 200, value
        return value['body']
    raise AssertionError('unresolved stale command')


def login(name):
    api('/logout', {})
    api('/login', {'name': name, 'password': 'starfort-browser-test'})
    b('open', BASE)


class Peer:
    def __init__(self, name):
        self.name = name
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.api('/register', {'name': name, 'password': 'starfort-browser-test', 'invite': 'starfort-browser'})
    def api(self, path, body=None):
        req = urllib.request.Request(BASE + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
        with self.opener.open(req, timeout=10) as response: return json.load(response)
    def command(self, kind, room_id=None, version=None, **extra):
        for _ in range(20):
            current = self.api('/state').get('room')
            try:
                return self.api('/rooms/' + (room_id or current['id']), dict(type=kind, version=version if version is not None else current['version'], nonce=uuid.uuid4().hex, **extra))
            except urllib.error.HTTPError as error:
                if error.code != 409: raise
        raise AssertionError('peer stale command')
    def action(self, kind, **extra):
        return self.command('action', action=dict(type=kind, epoch=self.api('/state')['room']['game']['data']['epoch'], **extra))


def scenario(name, players=1, mode='endless'):
    if api('/state')['user']['name'] != MAIN: login(MAIN)
    current = api('/state').get('room')
    if current:
        if current['status'] == 'playing': command('close')
        command('leave')
    for peer in PEERS:
        if peer.api('/state').get('room'): peer.command('leave')
    created = api('/rooms', {'kind': 'starfort', 'name': '扩展验收 ' + name, 'capacity': players, 'options': {'scenario': name, 'mode': mode, 'waves': 20, 'difficulty': 'normal', 'prepSeconds': 45}})
    for peer in PEERS[:players-1]:
        peer.command('join', room_id=created['id'], version=room()['version'])
        peer.command('ready')
    command('ready'); command('start')
    b('wait', '.starfort-board')
    wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
    return data()


def checkpoint(name, state=True):
    wait('[...document.querySelectorAll(".modal")].every(e=>getComputedStyle(e).opacity==="1")')
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth<=innerWidth+2'), 'horizontal overflow: ' + name
    assert js('[...document.images].every(i=>!i.complete||i.naturalWidth>0)'), 'broken images: ' + name
    contrast = js(r"""(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('.sf-choice-card p,.sf-choice-card button,.sf-objective-card p,.sf-objective-card button,.sf-objective-health span,.sf-world-event p,.sf-event-forecast span,.sf-tower-stats dd,.sf-target-hint,.sf-camera-toolbar button,.sf-entry-list button,.sf-overview-hint')].filter(e=>e.getBoundingClientRect().width>0).map(e=>{
        let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);
        for(const n of chain){const a=rgb(getComputedStyle(n).backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o))}
        const style=getComputedStyle(e),a=rgb(style.color),opacity=chain.reduce((s,n)=>s*Number(getComputedStyle(n).opacity),a[3]??1),fg=lum(a.slice(0,3).map((v,i)=>v*opacity+bg[i]*(1-opacity))),back=lum(bg);
        return {text:e.textContent.trim(),color:style.color,background:bg,fontSize:parseFloat(style.fontSize),opacity,ratio:(Math.max(fg,back)+.05)/(Math.min(fg,back)+.05)};
      });
    })()""")
    assert all(v['ratio'] >= 4.5 for v in contrast), ('unreadable world controls', [v for v in contrast if v['ratio'] < 4.5])
    artifact(name + '.contrast.json', contrast)
    b('screenshot', str(OUT / (name + '.png')), '--full')
    artifact(name + '.snapshot.json', b('snapshot', '-i'))
    if state: artifact(name + '.state.json', api('/state'))
    REPORT['checks'].append(name)
    print('PASS', name, flush=True)


def resources(name):
    current = data()
    for index, player in enumerate(current['players']):
        wait('document.querySelector(' + json.dumps(f'[data-sf-gold="{index}"]') + ')?.textContent.includes(' + json.dumps(str(player['gold'])) + ')')
    values = js(r'''(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('[data-sf-gold],strong[data-sf-base]')].map(e=>{
        let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);
        for(const n of chain){const a=rgb(getComputedStyle(n).backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o))}
        const style=getComputedStyle(e),fg=lum(rgb(style.color)),back=lum(bg),r=e.getBoundingClientRect();
        const visible=r.width>0&&r.height>0&&r.left>=0&&r.right<=innerWidth&&style.visibility!=='hidden';
        return {text:e.textContent,visible,fontSize:parseFloat(style.fontSize),ratio:(Math.max(fg,back)+.05)/(Math.min(fg,back)+.05)};
      });
    })()''')
    assert len(values) == len(current['players'])+1 and all(v['visible'] and v['ratio'] >= 4.5 for v in values), values
    artifact(name + '.resources.json', values)


def camera():
    return js('(()=>{const c=document.querySelector("canvas[data-starfort-canvas]"),r=c.getBoundingClientRect();return {x:+c.dataset.cameraX,y:+c.dataset.cameraY,zoom:+c.dataset.zoom,width:r.width,height:r.height,left:r.left,top:r.top}})()')


def mouse_click(x, y):
    b('mouse', 'move', str(round(x, 2)), str(round(y, 2)))
    b('mouse', 'down'); b('mouse', 'up')


def pause_all(players):
    click('[data-sf-pause]')
    for peer in PEERS[:players-1]: peer.action('pause')
    wait('document.querySelector(".starfort-board").dataset.phase==="paused"')


def resume_all(players):
    click('[data-sf-resume]')
    for peer in PEERS[:players-1]: peer.action('resume')
    wait('document.querySelector(".starfort-board").dataset.phase!=="paused"')


def preview_proof(choice):
    proof = []
    for option in choice['options']:
        selector = f'[data-sf-route-choice="{option["id"]}"]'
        if not js('!!document.querySelector("[role=dialog]")'): click('[data-sf-open-choice]')
        found = js('(()=>{const a=document.querySelector(' + json.dumps(selector) + ').closest(".sf-choice-card");return {routes:[...a.querySelectorAll("polyline")].map(e=>e.getAttribute("points")),circles:[...a.querySelectorAll("svg circle")].map(e=>[+e.getAttribute("cx"),+e.getAttribute("cy")]),facts:a.querySelector(".sf-choice-facts").textContent}})()')
        expected = [' '.join(f'{p["x"]},{p["y"]}' for p in route['points']) for route in option['map']['routes']]
        assert found['routes'] == expected, (option['id'], found, expected)
        for pad in option['map']['pads']: assert [pad['x'], pad['y']] in found['circles']
        assert str(len(option['map']['pads'])) in found['facts']
        click(f'[data-sf-preview="{option["id"]}"]')
        minimap = js('[...document.querySelectorAll("[data-sf-minimap] polyline")].map(e=>e.getAttribute("points"))')
        assert minimap == expected, 'Canvas preview minimap differs from offered real map'
        proof.append({'option': option, 'rendered': found, 'minimap': minimap})
    artifact('candidate-previews.json', proof)


def tower_identity(value):
    pads = {pad['id']: pad for pad in value['map']['pads']}
    return [{'id': t['id'], 'owner': t['owner'], 'pad': t['pad'], 'kind': t['kind'], 'level': t['level'], 'x': pads[t['pad']]['x'], 'y': pads[t['pad']]['y']} for t in value['towers']]


def instrument_draws():
    # Record the actual native draw calls, so glowing base animation cannot
    # satisfy moving-enemy assertions. No getImageData/toDataURL on CDN canvas.
    lookup = {}
    for suffix in ['v1', 'world-v2']:
        atlas = json.loads((ROOT / 'games/starfort/assets/public/starfort' / suffix / 'atlas.json').read_text())
        for key, frame in atlas['frames'].items():
            if key.startswith('enemy-'): lookup[suffix + ':' + str(frame['x']) + ':' + str(frame['y'])] = key
    js('window.__worldEnemyFrames=' + json.dumps(lookup))
    js(r'''(() => {
      const c=document.querySelector('canvas[data-starfort-canvas]').getContext('2d');
      if(!c.__worldDraw){c.__worldDraw=c.drawImage;c.__worldClear=c.clearRect;
        c.clearRect=function(...a){window.__worldFrame=(window.__worldFrame||0)+1;window.__worldIndex=0;return this.__worldClear(...a)};
        c.drawImage=function(...a){if(a.length===9){const m=this.getTransform(),p=m.transformPoint({x:a[5],y:a[6]}),q=m.transformPoint({x:a[5]+a[7],y:a[6]+a[8]});window.__worldDraws.push({at:performance.now(),frame:window.__worldFrame,index:window.__worldEnemyFrames[(a[0].src.includes('/world-v2/')?'world-v2':'v1')+':'+a[1]+':'+a[2]]?window.__worldIndex++:-1,sx:a[1],sy:a[2],sw:a[3],sh:a[4],x:a[5],y:a[6],w:a[7],h:a[8],px:p.x,py:p.y,right:q.x,bottom:q.y,cw:this.canvas.width,ch:this.canvas.height,src:a[0].src});if(window.__worldDraws.length>18000)window.__worldDraws.splice(0,2000)}return this.__worldDraw(...a)};
      }
      window.__worldDraws=[];
    })()''')


def segment_distance(point, a, b):
    vx, vy = b['x']-a['x'], b['y']-a['y']
    squared = vx*vx+vy*vy
    t = max(0, min(1, ((point[0]-a['x'])*vx+(point[1]-a['y'])*vy)/squared)) if squared else 0
    return math.hypot(point[0]-a['x']-t*vx, point[1]-a['y']-t*vy)


def movement_proof(name):
    before = data()
    instrument_draws()
    wait("(()=>{const a=window.__worldDraws.filter(d=>d.index===0);if(a.length<20)return false;const f=a[0],z=a.at(-1);return Math.hypot((z.x+z.w/2)-(f.x+f.w/2),(z.y+z.h*.8)-(f.y+f.h*.8))>5})()")
    after = data()
    draws = js('window.__worldDraws')
    artifact(name + '.draws.json', {'before': before, 'after': after, 'draws': draws})
    frames = {}
    for suffix in ['v1', 'world-v2']:
        atlas = json.loads((ROOT / 'games/starfort/assets/public/starfort' / suffix / 'atlas.json').read_text())
        for key, frame in atlas['frames'].items():
            if key.startswith('enemy-'): frames[(suffix, frame['x'], frame['y'])] = key
    enemies = []
    for draw in draws:
        suffix = 'world-v2' if '/world-v2/' in draw['src'] else 'v1'
        frame = frames.get((suffix, draw['sx'], draw['sy']))
        if frame:
            draw['sprite'] = frame
            enemies.append(draw)
    assert len(enemies) > 20, 'no actual enemy sprite drawings'
    assert any(e['right']>0 and e['bottom']>0 and e['px']<e['cw'] and e['py']<e['ch'] for e in enemies), 'enemy drawings all outside viewport'
    assert any('/world-v2/' in d['src'] for d in draws), 'new world atlas is not drawn'
    if ENV.get('WIRE_BOARD_STARFORT_ASSETS'):
        assert all(d['src'].startswith(ENV['WIRE_BOARD_STARFORT_ASSETS']) for d in draws), 'unexpected silent CDN fallback'
    first = [draw for draw in enemies if draw['index'] == 0]
    assert len(first)>10 and len({(round(d['x'], 3), round(d['y'], 3)) for d in first})>10, 'enemy position did not interpolate between server ticks'
    assert len({d['sprite'] for d in first}) > 1, 'walking sprite poses did not change'
    enemy = before['enemies'][0]
    route = next(route for route in before['map']['routes'] if route['id'] == enemy['route'])
    distances = [min(segment_distance((d['x']+d['w']/2, d['y']+d['h']*.8), a, z) for a,z in zip(route['points'], route['points'][1:])) for d in first]
    assert max(distances) < 2.1, ('enemy drawn off saved route', max(distances), route['id'])
    travel = math.hypot(first[-1]['x']-first[0]['x'], first[-1]['y']-first[0]['y'])
    assert travel>3 and after['tick']>before['tick'], 'server/movement frozen'
    artifact(name + '.draws.json', {'before': before, 'after': after, 'draws': draws, 'firstEnemyRouteDistance': distances, 'travel': travel})


def overview_checks(name):
    click('[data-sf-fit]')
    wait('document.querySelector("[data-starfort-canvas]").dataset.overview==="true"')
    assert js('document.querySelectorAll("[data-sf-pad]:not([hidden])").length')==0, 'overview has overlapping clickable pad targets'
    assert js('document.querySelector(".sf-overview-hint").textContent.includes("点选区域放大")')
    value=data();initial_selected=js('document.querySelector(".sf-pad-selected")?.dataset.sfPad ?? null')
    instrument_draws();wait('window.__worldDraws.length>20')
    compact=js('window.__worldDraws')
    assert all(d['index']==-1 and not(d['sw']==192 and d['sh']==224) for d in compact), 'overview still paints full-size tower/enemy sprites'
    checkpoint(name+'-compact-overview')
    b('scrollintoview','[data-starfort-canvas]');before=camera()
    tapx,tapy=before['left']+before['width']*.55,before['top']+before['height']*.5
    focusx=(tapx-before['left']-before['width']/2)/before['zoom']+before['x']
    focusy=(tapy-before['top']-before['height']/2)/before['zoom']+before['y']
    time.sleep(.35)  # previous navigation's ghost-click suppression has elapsed
    mouse_click(tapx,tapy)
    wait('document.querySelector("[data-starfort-canvas]").dataset.overview==="false"')
    after=camera()
    assert after['zoom']>before['zoom'], 'overview tap did not enter detailed scale'
    assert abs(after['x']-focusx)<5 and abs(after['y']-focusy)<5, ('overview tap centered wrong world point',before,after,focusx,focusy)
    assert js('document.querySelector(".sf-pad-selected")?.dataset.sfPad ?? null')==initial_selected, 'overview tap accidentally selected a pad'
    assert tower_identity(data())==tower_identity(value), 'overview tap built or changed a tower'
    assert js('document.querySelectorAll("[data-sf-pad]:not([hidden])").length')>0
    artifact(name+'.overview.json',{'before':before,'after':after,'focus':{'x':focusx,'y':focusy},'compactDraws':compact,'originalTowers':tower_identity(value)})


def camera_checks(name):
    if js('!!document.querySelector("[role=dialog]")'): click('[role=dialog] [aria-label="关闭"]')
    initial_selected=js('document.querySelector(".sf-pad-selected")?.dataset.sfPad ?? null')
    b('scrollintoview', '[data-starfort-canvas]')
    click('[data-sf-fit]'); cfit = camera()
    click('[data-sf-zoom-in]'); cin = camera()
    assert cin['zoom'] > cfit['zoom'], (cfit, cin)
    click('[data-sf-zoom-out]'); cout = camera()
    assert cout['zoom'] < cin['zoom']
    for _ in range(3): click('[data-sf-zoom-in]')
    b('scrollintoview', '[data-starfort-canvas]'); start = camera()
    x, y = start['left']+start['width']*.5, start['top']+start['height']*.5
    b('mouse', 'move', x, y); b('mouse', 'down')
    b('mouse', 'move', x+70, y+30); b('mouse', 'up')
    end = camera()
    assert math.hypot(start['x']-end['x'], start['y']-end['y'])>10, 'native mouse drag did not pan'
    assert js('document.querySelector(".sf-pad-selected")?.dataset.sfPad ?? null')==initial_selected, 'drag accidentally selected/build a pad'
    value = data(); assert len(value['towers'])>0
    # Actual mouse hit at world-transformed coordinate must select that pad.
    visible = js('[...document.querySelectorAll("[data-sf-pad]")].filter(e=>{const r=e.getBoundingClientRect();return !e.hidden&&!e.classList.contains("sf-pad-occupied")&&document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.closest("[data-sf-pad]")===e}).map(e=>({id:+e.dataset.sfPad,r:e.getBoundingClientRect().toJSON()}))')
    assert visible, 'all pads disappeared after pan'
    hit = visible[0]; pad = next(p for p in value['map']['pads'] if p['id']==hit['id'])
    expected_x = (pad['x']-end['x'])*end['zoom']+end['width']/2+end['left']
    expected_y = (pad['y']-end['y'])*end['zoom']+end['height']/2+end['top']
    assert abs(hit['r']['x']+hit['r']['width']/2-expected_x)<1 and abs(hit['r']['y']+hit['r']['height']/2-expected_y)<1, 'pad DOM/render transforms diverged'
    # A new intentional tap follows the production 300ms ghost-click guard.
    time.sleep(.35)
    mouse_click(expected_x, expected_y)
    wait('document.querySelector(".sf-pad-selected")?.dataset.sfPad===' + json.dumps(str(hit['id'])))
    click('[role=dialog] [aria-label=关闭]')
    # Release a real drag over a visible pad; it must not select or build.
    c = camera(); x,y=c['left']+c['width']*.6,c['top']+c['height']*.6
    b('mouse','move',x,y);b('mouse','down');b('mouse','move',x-65,y-35)
    visible = js('[...document.querySelectorAll("[data-sf-pad]")].filter(e=>!e.hidden&&e.dataset.sfPad!==' + json.dumps(str(hit['id'])) + ').map(e=>({id:e.dataset.sfPad,r:e.getBoundingClientRect().toJSON()}))')
    if visible:
        r=visible[0]['r'];b('mouse','move',r['x']+r['width']/2,r['y']+r['height']/2)
    b('mouse','up')
    assert js('document.querySelector(".sf-pad-selected")?.dataset.sfPad ?? null') is None, 'drag release selected/builds a pad'
    assert tower_identity(data())==tower_identity(value), 'drag mutated towers'
    last=value['map']['routes'][-1]
    click(f'[data-sf-entry="{last["id"]}"]'); centry=camera()
    click('[data-sf-camera-base]'); cbase=camera()
    assert math.hypot(centry['x']-cbase['x'],centry['y']-cbase['y'])>30, 'entry/base jump did not move camera'
    click('.sf-entry-menu > summary')
    b('scrollintoview','[data-sf-minimap]')
    point = js('(()=>{const e=document.querySelector("[data-sf-minimap]"),p=e.createSVGPoint();p.x=' + str(last['points'][0]['x']) + ';p.y=' + str(last['points'][0]['y']) + ';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()')
    mouse_click(point['x'],point['y']); cmini=camera()
    assert math.hypot(cmini['x']-cbase['x'],cmini['y']-cbase['y'])>30, 'minimap click failed'
    # CLI has no touch injection. This explicitly scoped synthetic event check
    # exercises the production two-pointer branch without claiming native touch.
    b('scrollintoview','[data-starfort-canvas]'); cp=camera()
    js('''(()=>{const e=document.querySelector('.sf-battlefield'),r=e.getBoundingClientRect(),x=r.left+r.width/2,y=r.top+r.height/2;
      const emit=(type,id,dx)=>e.dispatchEvent(new PointerEvent(type,{bubbles:true,pointerId:id,pointerType:'touch',isPrimary:id===41,clientX:x+dx,clientY:y,buttons:type==='pointerup'?0:1}));
      emit('pointerdown',41,-35);emit('pointerdown',42,35);emit('pointermove',41,-60);emit('pointermove',42,60);emit('pointerup',41,-60);emit('pointerup',42,60);})()''')
    pinch=camera();assert pinch['zoom']>cp['zoom'], 'two-pointer pinch handler failed'
    artifact(name+'.camera-transforms.json', {'fit':cfit,'zoomIn':cin,'zoomOut':cout,'beforeDrag':start,'afterDrag':end,'padHit':hit,'entry':centry,'base':cbase,'minimap':cmini,'beforeSyntheticPinch':cp,'afterSyntheticPinch':pinch})


def main():
    global SOURCE_START, SERVER, BASE, PEERS, SPECTATOR, OPENED
    try:
        if '--skip-build' not in sys.argv: run(['npm','--prefix','web','run','build'],'build.log')
        SOURCE_START=source_hashes()
        for name in ['url','stop']: (OUT/name).unlink(missing_ok=True)
        with (OUT/'server.log').open('w') as handle:
            SERVER=subprocess.Popen(['go','test','-count=1','-timeout=20m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=ROOT,env=ENV,stdout=handle,stderr=subprocess.STDOUT)
        deadline=time.monotonic()+120
        while not (OUT/'url').exists():
            if SERVER.poll() is not None or time.monotonic()>deadline: raise RuntimeError('fixture failed: see server.log')
            time.sleep(.1)
        BASE=(OUT/'url').read_text()
        PEERS=[Peer('扩展同伴'+str(index)) for index in range(1,4)]
        SPECTATOR=Peer('扩展观战者')
        b('open',BASE);OPENED=True
        api('/register',{'name':MAIN,'password':'starfort-browser-test','invite':'starfort-browser'})
        b('reload');b('wait','.game-selection')
        assert 'starfort' not in api('/state')['availableGames']
        api('/rooms',{'kind':'starfort','name':'默认下架','capacity':1},403)
        login('上架管理员');label('管理后台');b('wait','input[aria-label="搜索管理游戏"]')
        b('fill','input[aria-label="搜索管理游戏"]','星垒');click('.admin-game-card button');b('wait','[role=dialog]');label('确认操作')
        wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
        checkpoint('admin-published',False)
        login(MAIN)

        # Four real players choose after wave 5; inspect all previews while paused,
        # then click each player's votes through the actual DOM on their account.
        scenario('growth-vote',4)
        b('wait','[data-sf-route-choice]'); original=data(); old_towers=tower_identity(original)
        option=original['world']['choice']['options'][1]
        click(f'[data-sf-route-choice="{option["id"]}"]');click('[data-sf-boon-choice="reach"]')
        assert data()['world']['choice']['votes'][0]==option['id']
        pause_all(4)
        preview_proof(original['world']['choice'])
        assert js('[...document.querySelectorAll("[data-sf-route-choice],[data-sf-boon-choice],[data-sf-ready]")].every(e=>e.disabled)')
        assert '秒后出波' not in js('document.querySelector(".sf-wave-banner").textContent')
        b('set','viewport','320','844');resources('growth-mobile');checkpoint('growth-preview-mobile')
        paused=data();b('reload');b('wait','.starfort-board')
        assert data()['world']['choice']['votes']==paused['world']['choice']['votes']
        assert data()['world']['map']==paused['world']['map']
        wait('document.querySelector("[data-sf-route-choice='+option['id']+']")?.getAttribute("aria-pressed")==="true"')
        watched=room()['id'];SPECTATOR.api('/rooms/'+watched+'/watch',{})
        login(SPECTATOR.name);b('wait','.starfort-board')
        assert js('[...document.querySelectorAll("[data-sf-route-choice],[data-sf-boon-choice],[data-sf-objective]")].every(e=>e.disabled)')
        resources('spectator-growth-mobile');checkpoint('spectator-growth-readonly')
        login(MAIN);b('wait','.starfort-board');SPECTATOR.api('/rooms/'+watched+'/watch',{'leave':True})
        resume_all(4)
        assert js('document.querySelector("[data-sf-ready]").disabled')
        for index,peer in enumerate(PEERS):
            login(peer.name);b('wait','.starfort-board')
            chosen=original['world']['choice']['options'][0]['id'] if index==0 else option['id']
            click(f'[data-sf-route-choice="{chosen}"]')
            click('[data-sf-boon-choice="'+('power' if index==0 else 'reach')+'"]')
        expanded=data();assert expanded['world']['choice'] is None and expanded['world']['boonChoice'] is None
        assert expanded['world']['stage']==2 and expanded['world']['boons']['reach']==1
        assert expanded['map']['routes']==option['map']['routes']
        assert tower_identity(expanded)==old_towers
        assert expanded['nextAt']-expanded['serverAt']>=35000, 'choices did not renew preparation time'
        artifact('growth-vote-proof.json',{'before':original,'paused':paused,'after':expanded})
        login(MAIN);b('wait','.starfort-board');pause_all(4)
        b('set','viewport','1440','1000');checkpoint('growth-majority-preserved-tower')

        # All expanded sizes, entrances, special enemy artwork and native draws.
        for stage,ratio in [(2,1.5),(3,2.0),(4,3.0)]:
            state=scenario('stage'+str(stage),4)
            actual=state['map']['width']*state['map']['height']/(960*640)
            assert abs(actual-ratio)<.03, (stage,actual)
            assert len(state['map']['routes'])>=stage
            b('set','viewport','1440','1000');click('[data-sf-fit]')
            if js('document.querySelector("[data-starfort-canvas]").dataset.overview')=='true':
                checkpoint('stage'+str(stage)+'-desktop-overview')
                click('[data-sf-entry=main]')
            movement_proof('stage'+str(stage));checkpoint('stage'+str(stage)+'-desktop')
        state=data()
        for event in state['world']['forecast']:
            assert event['label'] in js('document.querySelector(".sf-event-forecast").textContent')
            assert str(event['wave']) in js('document.querySelector(".sf-event-forecast").textContent')
        assert state['world']['event']['description'] in js('document.querySelector("[data-sf-wave-event]").textContent')
        overview_checks('desktop')
        camera_checks('desktop')
        b('set','viewport','320','844');resources('stage4-mobile');overview_checks('mobile');camera_checks('mobile');checkpoint('stage4-mobile-navigate')
        click('[data-sf-fit]');checkpoint('stage4-mobile-full-map')
        saved=data();b('reload');b('wait','.starfort-board')
        restored=data();assert restored['world']['seed']==saved['world']['seed'] and restored['map']==saved['map']
        assert js('document.querySelector("[data-sf-camera-base]")&&!document.querySelector("[data-sf-camera-base]").disabled')
        click('[data-sf-camera-base]');checkpoint('reload-expanded-world')

        # Ownership and actual server-computed tower stats are presented verbatim.
        b('set','viewport','1440','1000');click('[data-sf-entry=main]')
        own=next(t for t in data()['towers'] if t['owner']==0)
        click('[data-sf-pad="'+str(own['pad'])+'"]');b('select','[data-sf-target]','support')
        wait('document.querySelector("[data-sf-target]")?.value==="support"')
        own=next(t for t in data()['towers'] if t['id']==own['id']);assert own['priority']=='support'
        stats=js('[...document.querySelectorAll(".sf-tower-stats dd")].map(e=>e.textContent)')
        expected=[str(round(own['effectiveDamage'],1)).removesuffix('.0'),str(round(own['effectiveRange'],1)).removesuffix('.0'),f'{own["effectiveInterval"]/1000:.2f} 秒']
        assert stats==expected,(stats,expected)
        other=next(t for t in data()['towers'] if t['owner']!=0)
        click('[data-sf-pad="'+str(other['pad'])+'"]');assert js('document.querySelector("[data-sf-target]").disabled')
        checkpoint('tower-ownership-effective-stats')
        watched=room()['id'];SPECTATOR.api('/rooms/'+watched+'/watch',{})
        login(SPECTATOR.name);b('wait','.starfort-board');click('[data-sf-entry=main]');click('[data-sf-pad="'+str(own['pad'])+'"]')
        assert js('document.querySelector("[data-sf-target]").disabled')
        checkpoint('spectator-tower-priority-readonly')
        login(MAIN);b('wait','.starfort-board');SPECTATOR.api('/rooms/'+watched+'/watch',{'leave':True})

        # Mission costs, active progress, success and failure use production actions
        # and timers. Only preceding combat is skipped by the explicitly named fixture.
        state=scenario('objectives',2)
        pause_all(2)
        assert js('[...document.querySelectorAll("[data-sf-objective]")].every(e=>e.disabled)'), 'paused objectives remain actionable'
        resume_all(2)
        b('select','select[aria-label="接收金币的队友"]','1')
        b('fill','input[aria-label="赠予金币数量"]',str(state['players'][0]['gold']))
        click('[data-sf-gift]')
        wait('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.players[0].gold===0)')
        wait('[...document.querySelectorAll("[data-sf-objective]")].every(e=>e.disabled)')
        assert data()['players'][0]['gold']==0
        b('set','viewport','320','844');resources('objectives-insufficient-gold')
        checkpoint('objectives-paused-and-insufficient-gold')
        watched=room()['id'];SPECTATOR.api('/rooms/'+watched+'/watch',{})
        login(SPECTATOR.name);b('wait','.starfort-board')
        assert js('[...document.querySelectorAll("[data-sf-objective]")].every(e=>e.disabled)')
        checkpoint('spectator-objectives-readonly')
        login(MAIN);b('wait','.starfort-board');SPECTATOR.api('/rooms/'+watched+'/watch',{'leave':True})
        b('set','viewport','1440','1000')
        state=scenario('objectives-clear')
        objectives=state['world']['objectives'];assert {o['kind'] for o in objectives}>={'mine','beacon','convoy'}
        gold=state['players'][0]['gold']
        for objective in objectives:
            if objective['status']!='available':continue
            selector='[data-sf-objective="'+objective['id']+'"]'
            assert str(objective['cost']) in js('document.querySelector('+json.dumps(selector)+').textContent')
            click(selector);gold-=objective['cost'];assert data()['players'][0]['gold']==gold
        assert all(o['status']=='active' for o in data()['world']['objectives'])
        checkpoint('objectives-cost-and-active')
        click('[data-sf-ready]');wait('document.querySelector(".starfort-board").dataset.phase==="prepare"&&document.querySelectorAll("[data-sf-objective-state=completed]").length>=2')
        first=data();assert all(o['status']=='completed' for o in first['world']['objectives'] if o['kind']!='convoy')
        assert next(o for o in first['world']['objectives'] if o['kind']=='convoy')['status']=='active'
        convoy=next(o for o in first['world']['objectives'] if o['kind']=='convoy')
        assert convoy['progress']>0 and convoy['hp']>0
        assert '护送进度' in js('document.querySelector(".sf-objective-progress").textContent')
        assert js('document.querySelectorAll("[data-sf-objective-health]").length')==len(first['world']['objectives'])
        checkpoint('objectives-one-wave-progress')
        click('[data-sf-ready]');wait('[...document.querySelectorAll("[data-sf-objective-state]")].every(e=>e.dataset.sfObjectiveState==="completed")')
        completed=data();assert completed['baseHP']==24 and all(o['status']=='completed' for o in completed['world']['objectives'])
        checkpoint('objectives-rewards-completed')
        # A separate unshortened battle proves the new moving objective is drawn
        # continuously on its server route and preserves progress on reconnect.
        state=scenario('objectives')
        convoy=next(o for o in state['world']['objectives'] if o['kind']=='convoy')
        click('[data-sf-objective="'+convoy['id']+'"]');click('[data-sf-ready]')
        wait('document.querySelector(".starfort-board").dataset.phase==="battle"')
        click('[data-sf-entry="'+convoy['route']+'"]')
        before=data();instrument_draws()
        cart=json.loads((ROOT/'games/starfort/assets/public/starfort/world-v2/atlas.json').read_text())['frames']['prop-cart']
        cart_filter='d=>d.src.includes("/world-v2/")&&d.sx==='+str(cart['x'])+'&&d.sy==='+str(cart['y'])
        wait('(()=>{const a=window.__worldDraws.filter('+cart_filter+');return a.length>20&&new Set(a.map(d=>d.x.toFixed(3))).size>20&&Math.abs(a.at(-1).x-a[0].x)>12})()')
        draws=js('window.__worldDraws.filter('+cart_filter+')');after=data()
        original=next(o for o in before['world']['objectives'] if o['id']==convoy['id'])
        current=next(o for o in after['world']['objectives'] if o['id']==convoy['id'])
        route=next(r for r in after['map']['routes'] if r['id']==current['route'])
        distances=[min(segment_distance((d['x']+d['w']/2,d['y']+d['h']),a,z) for a,z in zip(route['points'],route['points'][1:])) for d in draws]
        assert max(distances)<2 and current['progress']>original['progress'], 'convoy sprite off route or server motion frozen'
        assert len({round(d['x'],3) for d in draws})>12, 'convoy jumps between snapshots without interpolation'
        assert any(d['right']>0 and d['bottom']>0 and d['px']<d['cw'] and d['py']<d['ch'] for d in draws)
        if ENV.get('WIRE_BOARD_STARFORT_ASSETS'):assert all(d['src'].startswith(ENV['WIRE_BOARD_STARFORT_ASSETS']) for d in draws)
        # Pause is intentionally available only between waves. Reconnection during
        # battle must retain the server's advancing objective, not replay its start.
        saved=data();b('reload');b('wait','.starfort-board')
        reconnected=data();restored_convoy=next(o for o in reconnected['world']['objectives'] if o['id']==convoy['id'])
        saved_convoy=next(o for o in saved['world']['objectives'] if o['id']==convoy['id'])
        assert restored_convoy['progress']>=saved_convoy['progress'] and restored_convoy['route']==saved_convoy['route'], 'reconnect reset convoy journey'
        artifact('convoy-native-draws.json',{'before':before,'after':after,'draws':draws,'routeDistance':distances,'saved':saved,'reconnected':reconnected})
        checkpoint('convoy-continuous-draw-reconnect')
        state=scenario('objectives-fail')
        click('[data-sf-objective]');click('[data-sf-ready]')
        wait('!!document.querySelector("[data-sf-objective-state=failed]")');checkpoint('objective-failure-visible')
        state=scenario('endless-rotation');assert state['wave']>20 and state['world']['stage']==4
        active=[r['id'] for r in state['map']['routes'] if r['active']]
        rendered=js('[...document.querySelectorAll("[data-sf-entry][data-active=true]")].map(e=>e.dataset.sfEntry)')
        assert rendered==active and len(active)>=1
        artifact('endless-routes.json',{'world':state['world'],'enemies':state['enemies'],'renderedActive':rendered})
        checkpoint('endless-rotated-entrances')
        scenario('victory-ready',mode='campaign');b('wait','.results')
        assert js('[...document.querySelectorAll("[data-sf-ready],[data-sf-route-choice],[data-sf-boon-choice],[data-sf-objective],[data-sf-gift],[data-sf-pause]")].every(e=>e.disabled)')
        checkpoint('ended-no-action-controls')
        REPORT['passed']=True
    except Exception as error:
        import traceback
        traceback.print_exc();REPORT['error']=repr(error)
        if OPENED:
            try:
                REPORT['snapshot']=b('snapshot','-i');REPORT['state']=api('/state');REPORT['errors']=b('errors')
                b('screenshot',str(OUT/'failure.png'),'--full')
            except Exception as diagnostic:REPORT['diagnosticError']=repr(diagnostic)
    finally:
        if OPENED:
            try:b('close')
            except Exception:pass
        if SERVER:
            (OUT/'stop').write_text('stop')
            try:SERVER.wait(timeout=15)
            except subprocess.TimeoutExpired:SERVER.terminate();SERVER.wait(timeout=10)
        REPORT['commit']=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
        REPORT['sourceSHA256']=source_hashes()
        REPORT['sourceUnchangedDuringRun']=SOURCE_START==REPORT['sourceSHA256']
        if SOURCE_START and not REPORT['sourceUnchangedDuringRun']:
            REPORT['passed']=False;REPORT['sourceChangedDuringRun']=True
        built=[ROOT/'web/dist/index.html']+list((ROOT/'web/dist/assets').glob('*.js'))+list((ROOT/'web/dist/assets').glob('*.css'))
        REPORT['buildSHA256']={str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest() for path in built if path.is_file()}
        artifact('result.json',REPORT)
        print('Artifact:',OUT/'result.json',flush=True)
        if not REPORT['passed']:raise SystemExit(1)


if __name__ == "__main__":
    main()
