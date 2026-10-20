#!/usr/bin/env python3
"""Bubbleparty refactor E2E. See BROWSER_REFACTOR_SCENARIOS.md (written first).
Production HTTP/SQLite/clock; native CDP keyboard and touch, public state reads.
Run: python3 scripts/bubbleparty/check-refactor.py [--build]
"""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import traceback
import urllib.request
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--build', action='store_true')
parser.add_argument('--out', default='')
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
run = 'r' + uuid.uuid4().hex[:6]
out = (root / (args.out or '.local/bubbleparty/refactor-browser-' + run)).resolve()
assert out.is_relative_to(root) and out != root
assert not out.exists(), 'Use a new output directory; previous evidence is never overwritten'
out.mkdir(parents=True)
temp = out / 'temporary'
temp.mkdir()
socket_dir = root / '.s' / run
socket_dir.mkdir(parents=True)
env = dict(os.environ)
for key, path in {
    'AGENT_BROWSER_SOCKET_DIR': socket_dir,
    'AGENT_BROWSER_PROFILE': temp / 'profile',
    'TMPDIR': temp / 'tmp', 'GOTMPDIR': temp / 'tmp',
    'GOCACHE': root / '.local/go-build', 'GOMODCACHE': root / '.local/go-mod',
    'npm_config_cache': root / '.local/npm',
}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(AGENT_BROWSER_SESSION=run, AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',
           WIRE_BOARD_BUBBLEPARTY_BROWSER='1', WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out), GOTOOLCHAIN='local')
report = {'passed': False, 'checks': [], 'screenshots': [], 'output': str(out.relative_to(root)),
          'scope': '泡泡堂 only; real HTTP/SQLite/clock and native CDP input; no state injection'}
server = None
opened = False
log = None
layout_positions = {}


def browser(*args):
    p = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, text=True, capture_output=True, timeout=40)
    if p.returncode:
        raise RuntimeError(p.stdout + p.stderr)
    result = json.loads(p.stdout)
    assert result['success'], result
    return result['data']


def js(code):
    return browser('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    response = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert response['status'] in (200, 201), response
    return response['body']


def state():
    return api('/state')['room']


def view():
    return state()['game']['data']


def me(v=None):
    return (v or view())['players'][0]


def free_direction(v):
    p = me(v)
    for key, opposite, dx, dy in [('ArrowRight','ArrowLeft',1,0), ('ArrowDown','ArrowUp',0,1), ('ArrowLeft','ArrowRight',-1,0), ('ArrowUp','ArrowDown',0,-1)]:
        x, y = p['x'] + dx, p['y'] + dy
        if 0 <= x < 15 and 0 <= y < 13 and v['cells'][y*15+x] == 0:
            return key, opposite, (0 if dx else 1), dx or dy
    raise AssertionError('No clear neighboring tile for native movement check')


def pos(v=None):
    p = me(v)
    assert 'px' in p and 'py' in p, 'public continuous coordinates missing'
    return [p['px'], p['py']]


def command(kind, **extra):
    r = state()
    return api('/rooms/' + r['id'], {'type': kind, 'version': r['version'], 'nonce': uuid.uuid4().hex, **extra})


def check(name, **evidence):
    report['checks'].append({'name': name, **evidence})
    print('PASS', name, flush=True)


def wait(predicate, timeout=15, label='condition'):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(.1)
    raise TimeoutError(label)


def shot(name, full=True):
    assert js('document.documentElement.scrollWidth<=innerWidth+2'), 'horizontal overflow'
    errors = browser('errors')['errors']
    assert not errors, errors
    path = out / (name + '.png')
    browser('screenshot', str(path), *(['--full'] if full else []))
    assert path.exists() and path.stat().st_size > 1000
    report['screenshots'].append(str(path.relative_to(root)))


def visible(selector, min_size=0):
    data = js('''(()=>{const e=document.querySelector(SEL);if(!e)return null;const r=e.getBoundingClientRect(),s=getComputedStyle(e);return {x:r.x,y:r.y,width:r.width,height:r.height,viewport:[innerWidth,innerHeight],visible:s.visibility==='visible'&&s.display!=='none',hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})()'''.replace('SEL', json.dumps(selector)))
    assert data and data['visible'] and data['x'] >= -1 and data['y'] >= -1, (selector, data)
    assert data['x'] + data['width'] <= data['viewport'][0] + 1 and data['y'] + data['height'] <= data['viewport'][1] + 1, (selector, data)
    assert data['width'] >= min_size and data['height'] >= min_size and data['hit'], (selector, data)
    return data


def contrast():
    return js(r"""(()=>{
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
      const lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
      const bg=e=>{let b=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor),a=c[3]??1;b=b.map((v,i)=>v*(1-a)+(c[i]??0)*a)}return b};
      return ['.bubble-status strong','.bubble-mine','.bubble-team span','.bubble-hint','.bubble-player-summary span','.bubble-dpad button','.bubble-bomb','.bubble-bomb small'].map(selector=>{const e=document.querySelector(selector);if(!e)throw Error('missing contrast target '+selector);const s=getComputedStyle(e),a=lum(rgb(s.color)),b=lum(bg(e));return {selector,foreground:s.color,background:bg(e),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})
    })()""")


def disabled_contrast():
    return js(r"""(()=>{
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
      const over=(front,back,opacity=1)=>back.map((v,i)=>v*(1-(front[3]??1)*opacity)+(front[i]??0)*(front[3]??1)*opacity);
      const lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
      return ['.bubble-dpad .up','.bubble-bomb','.bubble-bomb small'].map(selector=>{const e=document.querySelector(selector),button=e.closest('button'),s=getComputedStyle(e),bs=getComputedStyle(button),opacity=Number(bs.opacity);let backdrop=[255,255,255],chain=[];for(let n=button.parentElement;n;n=n.parentElement)chain.unshift(n);for(const n of chain)backdrop=over(rgb(getComputedStyle(n).backgroundColor),backdrop);const bg=over(rgb(bs.backgroundColor),backdrop,opacity),fg=over(rgb(s.color),backdrop,opacity),a=lum(bg),b=lum(fg);return {selector,opacity,foreground:fg,background:bg,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)}})
    })()""")


# Uses the same owned agent-browser target. Native Input events are the only
# writes; Runtime.evaluate reads the production HTTP snapshot after each step.
CDP = r'''
const [endpoint,prefix,raw]=process.argv.slice(2), steps=JSON.parse(raw);
const ws=new WebSocket(endpoint);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});
let n=0;const pending=new Map(),networkEvents=[];ws.onmessage=({data})=>{const m=JSON.parse(data);if(m.method==='Network.loadingFailed'||m.method==='Network.requestWillBeSent')networkEvents.push({method:m.method,params:m.params});if(!m.id)return;const p=pending.get(m.id);pending.delete(m.id);m.error?p.j(Error(JSON.stringify(m.error))):p.r(m.result)};
const call=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++n;pending.set(id,{r,j});ws.send(JSON.stringify({id,method,params,sessionId}))});
let sessionId;const evidence=[];let lastInputAt=Date.now();
try {
 const {targetInfos}=await call('Target.getTargets');const t=targetInfos.find(t=>t.type==='page'&&t.url.startsWith(prefix));if(!t)throw Error('owned page missing');
 ({sessionId}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true}));
 const touch=steps.some(s=>s.touch);if(touch)await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
 for(const step of steps){
  if(step.blockSprites){await call('Network.enable',{},sessionId);await call('Network.setCacheDisabled',{cacheDisabled:true},sessionId);await call('Network.setBlockedURLs',{urls:['*bubbleparty/v2/sprites/*','*bubbleparty/v3/avatars/*']},sessionId);await call('Page.reload',{ignoreCache:true},sessionId);await new Promise(r=>setTimeout(r,1200));}
  if(step.key){lastInputAt=Date.now();const code=step.key===' '?'Space':step.key;const vk={'ArrowLeft':37,'ArrowUp':38,'ArrowRight':39,'ArrowDown':40,' ':32}[step.key];await call('Input.dispatchKeyEvent',{type:step.up?'keyUp':'keyDown',key:step.key,code,windowsVirtualKeyCode:vk,nativeVirtualKeyCode:vk},sessionId)}
  if(step.mouse){lastInputAt=Date.now();await call('Input.dispatchMouseEvent',{type:step.mouse,x:step.x,y:step.y,button:'left',buttons:step.mouse==='mouseReleased'?0:1,clickCount:1},sessionId);}
  if(step.touch){lastInputAt=Date.now();await call('Input.dispatchTouchEvent',{type:step.touch,touchPoints:step.touch==='touchEnd'||step.touch==='touchCancel'?[]:[{x:step.x,y:step.y,id:1}]},sessionId);}
  if(step.wait)await new Promise(r=>setTimeout(r,step.wait));
  if(step.read){
   const read=async()=>{const r=await call('Runtime.evaluate',{expression:'fetch("/api/state").then(r=>r.json()).then(s=>s.room)',awaitPromise:true,returnByValue:true},sessionId);if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value};
   let polls=1;const readStarted=Date.now();let room=await read();
   if(step.until){const deadline=Date.now()+2000, u=step.until, origin=u.previous?evidence.at(-1).room.game.data.players[0][u.axis]:u.origin;
    while(true){const p=room.game.data.players[0];if((!u.move||p.applied.move===u.move)&&(!u.axis||(p[u.axis]-origin)*u.sign>0.01))break;if(Date.now()>=deadline)throw Error('native input state timeout '+JSON.stringify({step,player:p,origin}));await new Promise(r=>setTimeout(r,25));room=await read();polls++}
   }
   evidence.push({label:step.read,elapsedSinceInputMS:Date.now()-lastInputAt,pollWaitMS:Date.now()-readStarted,polls,room});
  }
 }
 if(touch)await call('Emulation.setTouchEmulationEnabled',{enabled:false},sessionId);
 if(steps.some(s=>s.blockSprites))evidence.push({label:'blocked-sprites-network',events:networkEvents.filter(e=>e.method==='Network.loadingFailed'||(e.params.request?.url.includes('/bubbleparty/v2/sprites/')||e.params.request?.url.includes('/bubbleparty/v3/avatars/')))});
 console.log(JSON.stringify(evidence));
}finally{if(sessionId)await call('Target.detachFromTarget',{sessionId});ws.close()}
'''


def native(steps):
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    url = browser('get', 'url')['url']
    p = subprocess.run(['node', '--input-type=module', '-', endpoint, url, json.dumps(steps)], input=CDP,
                       cwd=root, env=env, text=True, capture_output=True, timeout=35)
    assert p.returncode == 0, p.stdout + p.stderr
    return json.loads(p.stdout)


def key(key, duration=150):
    return native([{'key': key}, {'wait': duration}, {'key': key, 'up': True}, {'wait': 180, 'read': 'released'}])[-1]['room']['game']['data']


def dead_patch():
    return js("""(async()=>{const v=(await(await fetch('/api/state')).json()).room.game.data,p=v.players[0],c=document.querySelector('[data-bubbleparty-canvas]'),transform=c.getContext('2d').getTransform(),sx=transform.a,sy=transform.d,x=Math.max(0,Math.floor(((p.px??p.x)*24+12-20)*sx+transform.e)),y=Math.max(0,Math.floor(((p.py??p.y)*24+12-60)*sy+transform.f)),w=Math.min(c.width-x,Math.floor(40*sx)),h=Math.min(c.height-y,Math.floor(82*sy));const data=c.getContext('2d').getImageData(x,y,w,h).data,hash=await crypto.subtle.digest('SHA-256',data);return {alive:p.alive,epoch:v.epoch,rect:{x,y,w,h},sha256:Array.from(new Uint8Array(hash),b=>b.toString(16).padStart(2,'0')).join('')}})()""")


def record(name, value):
    (out / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


try:
    if args.build:
        subprocess.run(['npm', '--prefix', 'web', 'run', 'build'], cwd=root, env=env, check=True)
    assert (root / 'web/dist/index.html').exists(), 'Build frontend before acceptance'
    log = (out / 'server.log').open('w')
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=12m', '-run', '^TestBubblepartyBrowserFixture$', './games/bubbleparty/tests'], cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT)
    deadline = time.monotonic() + 120
    while not (out / 'url').exists():
        assert server.poll() is None, 'fixture exited; inspect server.log'
        assert time.monotonic() < deadline, 'fixture start timeout'
        time.sleep(.1)
    base = (out / 'url').read_text()
    browser('open', base)
    opened = True
    api('/login', {'name': '泡泡验收管理员', 'password': 'bubbleparty-browser-test-only'})
    assert 'bubbleparty' in api('/state')['availableGames']
    browser('reload')
    browser('wait', '.room-list')
    browser('set', 'viewport', '1280', '1000')
    browser('click', '.lobby-create')
    browser('fill', 'input[aria-label=搜索可选游戏]', '泡泡堂')
    browser('wait', '.choose-games button')
    assert js('document.querySelector(".choose-games")?.textContent.includes("泡泡堂")')
    browser('click', '.choose-games button')
    browser('wait', '[data-create-step=setup]:not([hidden])')
    assert js('[...document.querySelectorAll("select[aria-label=座位上限] option")].map(e=>e.value)') == ['2','4','6','8']
    browser('select', 'select[aria-label=座位上限]', '2')
    browser('select', 'select[aria-label=水泡战场地图]', 'community10')
    browser('fill', 'input[name=name]', '泡泡堂重构验收')
    browser('click', '[role=dialog] button[type=submit]')
    browser('wait', '.waiting-actions')
    check('renamed-create-and-default-published')

    # A real second authenticated human stays still; no bot randomness can
    # make the input and trapped-refresh checks pass or fail accidentally.
    opponent = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    def other(path, body=None):
        req = urllib.request.Request(base + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
        with opponent.open(req, timeout=10) as res:
            return json.load(res)
    other('/register', {'name': '静止验收对手', 'password': 'bubbleparty-browser-test-only', 'invite': 'bubbleparty-browser'})
    room = state()
    other('/rooms/' + room['id'], {'type':'join','version':room['version'],'nonce':uuid.uuid4().hex})
    room = other('/state')['room']
    other('/rooms/' + room['id'], {'type':'ready','version':room['version'],'nonce':uuid.uuid4().hex})
    for width in [1280, 320]:
        browser('set', 'viewport', str(width), '1000')
        shot('waiting-' + str(width))
    command('ready')
    command('start')
    browser('wait', '.bubble-board')
    wait(lambda: view()['phase'] == 'playing', label='playing phase')
    initial = view()
    record('initial-state', initial)
    assert isinstance(initial['cells'], list) and all(isinstance(x, int) for x in initial['cells'])
    assert 1 in initial['cells'] and 2 in initial['cells']
    for width in [1280, 320]:
        browser('set', 'viewport', str(width), '1000')
        browser('scroll', 'up', '10000')
        shot('top-viewport-' + str(width), full=False)
        top = js('(()=>{const r=document.querySelector("[data-bubbleparty-canvas]").getBoundingClientRect();return {top:r.top,bottom:r.bottom,viewport:innerHeight}})()')
        report.setdefault('viewport_observations', []).append({'width':width, **top})
        browser('scrollintoview', '[data-bubbleparty-canvas]')
        canvas = visible('[data-bubbleparty-canvas]')
        layout_positions[width] = js('document.querySelector("[data-bubbleparty-canvas]").getBoundingClientRect().top+scrollY')
        assert 1.05 <= canvas['width'] / canvas['height'] <= 1.4, canvas
        shot('playing-' + str(width))
        browser('scrollintoview', '.bubble-controls')
        controls = {s: visible(s, 44) for s in ['.bubble-dpad .left', '.bubble-dpad .right', '[data-bubbleparty-bomb]']}
        shot('controls-' + str(width))
        check('actual-visible-' + str(width), canvas=canvas, controls=controls)

    # Compare actual pixel patches for each public map class. A number-array
    # assertion alone cannot detect a broken or absent renderer.
    pixels = js('''(async()=>{const s=(await(await fetch('/api/state')).json()).room.game.data,c=document.querySelector('[data-bubbleparty-canvas]'),ctx=c.getContext('2d'),transform=ctx.getTransform();return [0,1,2].map(type=>{const i=s.cells.findIndex((v,i)=>v===type&&i%15>2&&i%15<12&&Math.floor(i/15)>2&&Math.floor(i/15)<10);if(i<0)throw Error('missing map class');const x=i%15,y=Math.floor(i/15),a=ctx.getImageData(Math.floor((x+.3)*24*transform.a+transform.e),Math.floor((y+.3)*24*transform.d+transform.f),Math.max(1,Math.floor(24*transform.a*.4)),Math.max(1,Math.floor(24*transform.d*.4))).data;const rgb=[0,0,0];for(let j=0;j<a.length;j+=4)for(let k=0;k<3;k++)rgb[k]+=a[j+k];return {type,x,y,average:rgb.map(v=>v/(a.length/4)),colors:new Set(Array.from({length:a.length/4},(_,j)=>Array.from(a.slice(j*4,j*4+3)).join(','))).size}})})()''')
    for a in pixels:
        for b in pixels:
            if a['type'] != b['type']:
                assert sum((x-y)**2 for x,y in zip(a['average'],b['average'])) ** .5 > 18, pixels
    ratios = contrast()
    assert all(r['ratio'] >= 4.5 for r in ratios), ratios
    check('computed-text-contrast', ratios=ratios)
    record('canvas-pixels', pixels)
    check('wall-crate-ground-different-real-pixels', pixels=pixels)

    browser('set', 'viewport', '1280', '1000')
    browser('scrollintoview', '[data-bubbleparty-canvas]')
    browser('click', '[data-bubbleparty-canvas]')
    start = view()
    before = pos(start)
    time.sleep(3.6)
    idle = state()
    assert pos(idle['game']['data']) == before, (before, pos(idle['game']['data']))
    assert not idle['seats'][0].get('autoPlay'), idle['seats'][0]
    check('idle-over-three-seconds-keeps-human-control', position=before)

    # Opposite directions share the clear spawn corridor. Pressing the second
    # changes direction; releasing it must resume the first held direction.
    forward, reverse, axis, sign = free_direction(start)
    coordinate = ['px', 'py'][axis]
    evidence = native([{'key':forward}, {'read':'forward','until':{'move':forward.replace('Arrow','').lower(),'axis':coordinate,'origin':before[axis],'sign':sign}},
                       {'key':reverse}, {'read':'reverse-overrides','until':{'move':reverse.replace('Arrow','').lower(),'axis':coordinate,'previous':True,'sign':-sign}},
                       {'key':reverse,'up':True}, {'read':'forward-resumes','until':{'move':forward.replace('Arrow','').lower(),'axis':coordinate,'previous':True,'sign':sign}},
                       {'key':forward,'up':True}, {'read':'all-released','until':{'move':'none'}},
                       {'wait':350,'read':'remains-stopped'}])
    record('native-key-overlap', evidence)
    movement = [x['room']['game']['data']['players'][0] for x in evidence]
    assert (movement[0][coordinate] - before[axis]) * sign > 0, movement
    assert (movement[1][coordinate] - movement[0][coordinate]) * sign < 0, movement
    assert (movement[2][coordinate] - movement[1][coordinate]) * sign > 0, movement
    assert [movement[3]['px'],movement[3]['py']] == [movement[4]['px'],movement[4]['py']], movement
    check('real-keyboard-overlap-release-and-stop', positions=[[p['px'],p['py']] for p in movement], elapsedSinceInputMS=[x['elapsedSinceInputMS'] for x in evidence])

    browser('set', 'viewport', '320', '1000')
    browser('scrollintoview', '.bubble-controls')
    selector = '.bubble-dpad .' + reverse.replace('Arrow','').lower()
    rect = visible(selector, 44)
    before = pos()
    opposite_rect = visible('.bubble-dpad .' + forward.replace('Arrow','').lower(), 44)
    evidence = native([{'touch':'touchStart','x':rect['x']+rect['width']/2,'y':rect['y']+rect['height']/2},
                       {'read':'touch-held','until':{'move':reverse.replace('Arrow','').lower(),'axis':coordinate,'origin':before[axis],'sign':-sign}},
                       {'touch':'touchMove','x':opposite_rect['x']+opposite_rect['width']/2,'y':opposite_rect['y']+opposite_rect['height']/2},
                       {'read':'touch-drag-changes-direction','until':{'move':forward.replace('Arrow','').lower(),'axis':coordinate,'previous':True,'sign':sign}},
                       {'touch':'touchCancel'}, {'read':'touch-cancelled','until':{'move':'none'}},
                       {'wait':350,'read':'cancel-remains-stopped'}])
    record('native-touch', evidence)
    held = pos(evidence[0]['room']['game']['data'])
    dragged = pos(evidence[1]['room']['game']['data'])
    stopped = pos(evidence[2]['room']['game']['data'])
    after = pos(evidence[3]['room']['game']['data'])
    assert (held[axis] - before[axis]) * sign < 0, (before, held)
    assert (dragged[axis] - held[axis]) * sign > 0, (held, dragged)
    assert stopped == after, (stopped, after)
    check('native-touch-moves-drags-and-cancels', before=before, held=held, dragged=dragged, stopped=stopped,
          elapsedSinceInputMS=[x['elapsedSinceInputMS'] for x in evidence])

    browser('set', 'viewport', '1280', '1000')
    browser('scrollintoview', '[data-bubbleparty-canvas]')
    browser('click', '[data-bubbleparty-canvas]')
    mouse_view = view()
    mouse_forward, _, mouse_axis, mouse_sign = free_direction(mouse_view)
    mouse_coordinate = ['px','py'][mouse_axis]
    mouse_before = pos(mouse_view)
    mouse_rect = visible('.bubble-dpad .' + mouse_forward.replace('Arrow','').lower(),44)
    mouse_x, mouse_y = mouse_rect['x']+mouse_rect['width']/2, mouse_rect['y']+mouse_rect['height']/2
    mouse_evidence = native([{'mouse':'mousePressed','x':mouse_x,'y':mouse_y},
                             {'read':'mouse-held','until':{'move':mouse_forward.replace('Arrow','').lower(),'axis':mouse_coordinate,'origin':mouse_before[mouse_axis],'sign':mouse_sign}},
                             {'mouse':'mouseReleased','x':mouse_x,'y':mouse_y},
                             {'read':'mouse-released','until':{'move':'none'}},{'wait':350,'read':'mouse-remains-stopped'}])
    record('native-mouse',mouse_evidence)
    mouse_moved = pos(mouse_evidence[0]['room']['game']['data'])
    mouse_stopped = pos(mouse_evidence[1]['room']['game']['data'])
    assert (mouse_moved[mouse_axis]-mouse_before[mouse_axis])*mouse_sign > 0
    assert pos(mouse_evidence[2]['room']['game']['data']) == mouse_stopped
    check('native-mouse-direction-moves-and-release-stops',before=mouse_before,moved=mouse_moved,stopped=mouse_stopped,
          elapsedSinceInputMS=[x['elapsedSinceInputMS'] for x in mouse_evidence])
    browser('set','media','light','reduced-motion')
    assert js('matchMedia("(prefers-reduced-motion: reduce)").matches')
    before = view()
    browser('reload')
    browser('wait', '.bubble-board')
    after = view()
    assert before['epoch'] == after['epoch'] and pos(before) == pos(after), (before, after)
    check('reduced-motion-refresh-preserves-epoch-and-stopped-position', epoch=after['epoch'], position=pos(after))

    browser('set','media','light')
    assert not js('matchMedia("(prefers-reduced-motion: reduce)").matches')
    browser('reload')
    browser('wait','.bubble-board')

    # A short native pulse must create a bomb. Moving out and returning must
    # stop at its boundary while it still exists; its real blast then traps us.
    browser('click', '[data-bubbleparty-canvas]')
    forward, reverse, axis, sign = free_direction(view())
    evidence = native([{'key':' '}, {'wait':10}, {'key':' ','up':True},
                       {'wait':160,'read':'bomb-created'}, {'key':forward},
                       {'wait':240,'read':'left-bomb-tile'}, {'key':forward,'up':True},
                       {'key':reverse}, {'wait':240}, {'key':reverse,'up':True},
                       {'wait':140,'read':'blocked-return'}])
    bomb_view = evidence[0]['room']['game']['data']
    bomb = next((b for b in bomb_view['bombs'] if b['seat'] == 0), None)
    assert bomb, bomb_view
    departed = evidence[1]['room']['game']['data']
    blocked = evidence[2]['room']['game']['data']
    assert [me(departed)['x'],me(departed)['y']] != [bomb['x'],bomb['y']], evidence
    assert any(b['id'] == bomb['id'] for b in blocked['bombs']), evidence
    assert [me(blocked)['x'],me(blocked)['y']] != [bomb['x'],bomb['y']], evidence
    record('short-bomb-and-blocked-return', evidence)
    check('short-space-creates-bomb-and-bomb-blocks-reentry', bomb=bomb,
          departed=pos(departed), blocked=pos(blocked))
    trapped = wait(lambda: (lambda v: v if me(v)['trappedUntil'] > v['serverAt'] else None)(view()), timeout=5, label='real water bomb traps human')
    assert me(trapped)['alive'], trapped
    assert js('document.querySelector(".bubble-status")?.textContent.includes("困")'), 'trapped state absent from visible status'
    disabled = disabled_contrast()
    record('disabled-control-contrast', disabled)
    assert all(r['ratio'] >= 4.5 for r in disabled), disabled
    check('disabled-controls-actual-opacity-composited-contrast', ratios=disabled)
    deadline = me(trapped)['trappedUntil']
    position = pos(trapped)
    shot('trapped-desktop')
    browser('reload')
    browser('wait', '.bubble-board')
    restored = view()
    assert restored['epoch'] == trapped['epoch'] and me(restored)['trappedUntil'] == deadline and pos(restored) == position
    check('trapped-refresh-preserves-epoch-position-deadline', deadline=deadline, position=position)
    browser('set', 'viewport', '320', '1000')
    trapped_top = js('document.querySelector("[data-bubbleparty-canvas]").getBoundingClientRect().top+scrollY')
    assert abs(trapped_top-layout_positions[320]) <= 1, (layout_positions[320],trapped_top)
    check('320-trapped-status-keeps-board-position', before=layout_positions[320], trapped=trapped_top)
    shot('trapped-320')
    record('trapped-state', restored)
    browser('click','[data-bubbleparty-avatar-open]')
    browser('wait','[role=dialog][aria-label=选择人物]')
    wait(lambda: js('!!document.querySelector(".match-result-modal")'), timeout=12, label='real trap expiry result modal')
    wait(lambda:js('!document.querySelector("[role=dialog][aria-label=选择人物]")'),label='appearance closed on match end')
    assert js('!!document.activeElement?.closest(".match-result-modal")'), 'result modal lost focus to the board'
    check('appearance-to-result-keeps-focus-inside-result')
    result_top = js('document.querySelector("[data-bubbleparty-canvas]").getBoundingClientRect().top+scrollY')
    shot('result-320')
    browser('click', '.modal-head button[aria-label=关闭]')
    browser('wait', '.result-reopen')
    closed_top = js('document.querySelector("[data-bubbleparty-canvas]").getBoundingClientRect().top+scrollY')
    shot('result-closed-final-board-320')
    check_positions = {'trapped':trapped_top,'result':result_top,'closed':closed_top}
    record('result-board-positions',check_positions)
    assert max(check_positions.values())-min(check_positions.values()) <= 1, check_positions
    time.sleep(1.2)
    assert not js('!!document.querySelector(".match-result-modal")'), 'closed result reopened on snapshot'
    time.sleep(2.1)
    final_patch = dead_patch()
    assert not final_patch['alive']
    shot('settled-final-death-before-refresh-320')
    browser('reload')
    browser('wait','.bubble-board')
    wait(lambda: js('performance.getEntriesByType("resource").filter(e=>e.name.includes("/bubbleparty/v2/sprites/")&&e.responseEnd>0).length>=21'),timeout=5,label='reload sprite resources complete')
    js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
    refreshed_patch = dead_patch()
    time.sleep(.5)
    snapshot_patch = dead_patch()
    record('death-final-frame-pixels',{'final':final_patch,'refreshed':refreshed_patch,'snapshot':snapshot_patch})
    assert final_patch == refreshed_patch == snapshot_patch, (final_patch,refreshed_patch,snapshot_patch)
    if js('!!document.querySelector(".match-result-modal")'):
        browser('click','.modal-head button[aria-label=关闭]')
    shot('refreshed-final-death-320')
    check('normal-motion-ended-death-does-not-replay-after-refresh-or-snapshot',pixels=final_patch)
    browser('click', '.result-reopen')
    browser('wait', '.match-result-modal')
    browser('set', 'viewport', '1280', '1000')
    shot('result-desktop')
    check('320-result-close-preserves-final-board-position',positions=check_positions)
    check('real-result-close-stays-closed-and-reopens')

    # A separate real eight-seat room exercises mixed human/computer seats,
    # public resource visibility, and the platform takeover controls.
    browser('click', '.modal-head button[aria-label=关闭]')
    command('leave')
    api('/rooms', {'kind':'bubbleparty','name':'泡泡堂八人电脑混合验收','capacity':8,
                   'options':{'map':'community10','roundSeconds':180}})
    for _ in range(7):
        command('add_bot')
    browser('set','viewport','320','844')
    browser('scroll','up','10000')
    command('ready')
    command('start')
    wait(lambda: view()['phase'] == 'playing', label='eight-seat round playing')
    initial_canvas = visible('[data-bubbleparty-canvas]')
    initial_controls = {s:visible(s,44) for s in ['.bubble-dpad .up','.bubble-dpad .down','.bubble-dpad .left','.bubble-dpad .right','[data-bubbleparty-bomb]']}
    initial_scroll = js('scrollY')
    shot('eight-player-initial-playable-320x844',full=False)
    time.sleep(.5)
    assert abs(js('scrollY')-initial_scroll) <= 1, 'snapshot changed scroll position'
    check('eight-player-320x844-initial-playable-without-test-scroll',canvas=initial_canvas,controls=initial_controls,scrollY=initial_scroll)
    assert len(view()['players']) == 8
    browser('set','viewport','1280','1000')
    browser('scrollintoview','.autoplay-control')
    browser('click', '.autoplay-control')
    wait(lambda: state()['seats'][0].get('autoPlay'), label='explicit autoplay enabled')
    browser('click', '.autoplay-control')
    wait(lambda: not state()['seats'][0].get('autoPlay'), label='explicit autoplay disabled')
    stationary = pos()
    time.sleep(.8)
    assert pos() == stationary
    check('platform-takeover-and-manual-reclaim-stop-bot', position=stationary)
    for width in [1280,320]:
        browser('set','viewport',str(width),'1000')
        browser('scroll','up','10000')
        shot('eight-player-top-viewport-' + str(width), full=False)
        browser('scrollintoview','[data-bubbleparty-canvas]')
        bounds = visible('[data-bubbleparty-canvas]')
        shot('eight-player-' + str(width))
        resource_bounds = []
        for seat in range(8):
            selector = '[data-bubbleparty-seat="' + str(seat) + '"]'
            browser('scrollintoview',selector)
            resource_bounds.append(visible(selector))
        check('eight-player-real-visible-' + str(width), canvas=bounds, resources=resource_bounds)
    for map_id in ['waterpark','forest','desert']:
        if state()['status'] == 'playing':
            command('close')
        command('leave')
        api('/rooms', {'kind':'bubbleparty','name':'泡泡堂地图验收 '+map_id,'capacity':2,
                       'options':{'map':map_id,'roundSeconds':180}})
        command('add_bot')
        command('ready')
        command('start')
        wait(lambda: view()['phase'] == 'playing', label=map_id+' playing')
        rendered = view()
        assert rendered['mapId'] == map_id and len(rendered['players']) == 2
        browser('set','viewport','1280','1000')
        browser('scroll','up','10000')
        shot(map_id+'-top-viewport', full=False)
        browser('scrollintoview','[data-bubbleparty-canvas]')
        bounds = visible('[data-bubbleparty-canvas]')
        shot(map_id+'-desktop')
        browser('set','viewport','320','1000')
        browser('scrollintoview','[data-bubbleparty-canvas]')
        mobile = visible('[data-bubbleparty-canvas]')
        shot(map_id+'-320')
        check(map_id+'-human-bot-render', desktop=bounds, mobile=mobile)
    browser('set','viewport','1280','1000')
    browser('scroll','up','10000')
    media = js('[...document.querySelectorAll("audio[data-platform-audio]")].map(e=>({src:e.currentSrc||e.src,paused:e.paused,muted:e.muted,music:e.hasAttribute("data-platform-music"),readyState:e.readyState}))')
    assert any('/bubbleparty/v2/audio/' in m['src'] and m['music'] and not m['paused'] for m in media), media
    browser('click','.sound-toggle[aria-label=静音]')
    wait(lambda: js('[...document.querySelectorAll("audio[data-platform-audio]")].every(e=>e.paused&&e.muted)'),label='platform mute stops all media')
    assert js('document.querySelector(".sound-preview").disabled')
    muted = js('[...document.querySelectorAll("audio[data-platform-audio]")].map(e=>({src:e.currentSrc||e.src,paused:e.paused,muted:e.muted}))')
    check('original-background-music-and-platform-mute', playing=media, muted=muted)

    # Fault injection is restricted to resource requests. No game state is
    # substituted: the same public snapshot must remain playable/renderable.
    before = view()
    blocked_evidence = native([{'blockSprites':True},{'read':'image-failure-view'}])
    sprite_request_ids = {e['params']['requestId'] for e in blocked_evidence[-1]['events'] if e['method'] == 'Network.requestWillBeSent' and ('/bubbleparty/v2/sprites/' in e['params']['request']['url'] or '/bubbleparty/v3/avatars/' in e['params']['request']['url'])}
    failures = [e for e in blocked_evidence[-1]['events'] if e['method'] == 'Network.loadingFailed' and e['params']['requestId'] in sprite_request_ids]
    assert len(failures) == len(sprite_request_ids) >= 20 and all(e['params'].get('blockedReason') == 'inspector' for e in failures), blocked_evidence
    record('blocked-sprite-requests', blocked_evidence)
    browser('wait','.bubble-board')
    assert js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"&&document.querySelector(".sound-preview").disabled')
    assert view()['epoch'] == before['epoch']
    fallback = js("""(()=>{const c=document.querySelector('[data-bubbleparty-canvas]'),d=c.getContext('2d').getImageData(0,0,c.width,c.height).data;let opaque=0;const colors=new Set();for(let i=0;i<d.length;i+=4){if(d[i+3]>200)opaque++;colors.add([d[i],d[i+1],d[i+2]].join(','))}return {opaqueRatio:opaque/(d.length/4),colors:colors.size,width:c.width,height:c.height}})()""")
    assert fallback['opaqueRatio'] > .95 and fallback['colors'] > 20, fallback
    browser('scrollintoview','[data-bubbleparty-canvas]')
    shot('blocked-sprites-fallback-desktop')
    browser('set','viewport','320','1000')
    browser('scrollintoview','[data-bubbleparty-canvas]')
    shot('blocked-sprites-fallback-320')
    record('fallback-pixels', fallback)
    check('blocked-sprites-render-visible-fallback-and-muted-reload', canvas=fallback)
    report['passed'] = True
except BaseException:
    report['error'] = traceback.format_exc()
    print(report['error'], flush=True)
    if opened:
        try:
            shot('failure')
            record('failure-state', state())
        except BaseException:
            pass
finally:
    if opened:
        try:
            browser('close')
        except BaseException:
            report['cleanup_error'] = traceback.format_exc()
            report['passed'] = False
    if server is not None:
        (out / 'stop').write_text('stop')
        try:
            server.wait(timeout=20)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait(timeout=5)
            report['passed'] = False
        if server.returncode != 0:
            report['passed'] = False
    if log:
        log.close()
    cleaned = []
    for path in [temp, socket_dir, out / 'data']:
        assert path.is_relative_to(root) and path != root
        if path.exists():
            shutil.rmtree(path)
            cleaned.append(str(path.relative_to(root)))
    for name in ['url','stop']:
        (out / name).unlink(missing_ok=True)
    report['cleaned'] = cleaned
    record('result', report)
    print(out / 'result.json', flush=True)
    if not report['passed']:
        raise SystemExit(1)
