#!/usr/bin/env python3
"""Pre-implementation real-room acceptance for Conquest.

Failure matrix: FAILURE_SCENARIOS_UI.md. Uses production game registration,
normal starts and native Chromium mouse/touch. No game-state mutation or mocks.
Run python3 -B games/conquest/tests/accept_ui.py. Optional CONQUEST_BASE_URL
connects to an existing isolated acceptance server; never point at production.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/conquest/browser'
OUT.mkdir(parents=True, exist_ok=True)
ENV = dict(os.environ)
RUN = uuid.uuid4().hex[:8]
PROFILE = OUT / ('profile-' + RUN)
SOCKET = ROOT / '.ab'
for key, directory in {'GOCACHE': ROOT/'.local/go-build', 'GOMODCACHE': ROOT/'.local/go-mod', 'GOTMPDIR': ROOT/'.local/tmp', 'TMPDIR': ROOT/'.local/tmp', 'npm_config_cache': ROOT/'.local/npm', 'AGENT_BROWSER_SOCKET_DIR': SOCKET, 'AGENT_BROWSER_PROFILE': PROFILE}.items():
    directory.mkdir(parents=True, exist_ok=True)
    ENV[key] = str(directory)
ENV.update(GOTOOLCHAIN='local', WIRE_BOARD_CONQUEST_BROWSER='1', AGENT_BROWSER_SESSION='cq' + RUN[:4], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
REPORT = {'passed': False, 'command': 'python3 -B games/conquest/tests/accept_ui.py', 'scope': 'Production UI, natural initial states, real HTTP and native mouse/touch', 'checks': [], 'geometry': [], 'inputs': []}
BASE = ENV.get('CONQUEST_BASE_URL', '')
INVITE = ENV.get('CONQUEST_INVITE', 'conquest-browser')
PASSWORD = 'conquest-browser-only'
server = None
opened = False
handles = []


def browser(*args):
    p = subprocess.run(['agent-browser', *args, '--json'], cwd=ROOT, env=ENV, capture_output=True, text=True, timeout=45)
    if p.returncode:
        raise RuntimeError(p.stdout + p.stderr)
    result = json.loads(p.stdout)
    assert result['success'], result
    return result['data']


def js(code):
    return browser('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js('fetch(' + json.dumps('/api' + path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= result['status'] < 300, result
    return result['body']


def room():
    return api('/state')['room']


def view():
    return room()['game']['data']


def command(kind, **extra):
    r = room()
    return api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))


def wait(code):
    browser('wait', '--fn', code)


def click(name):
    label=json.dumps(name)
    wait('(()=>{const e=[...document.querySelectorAll("button")].find(b=>(b.textContent.trim()==='+label+'||b.getAttribute("aria-label")==='+label+')&&!b.disabled&&!b.closest("[inert]"));return !!e})()')
    browser('find', 'role', 'button', 'click', '--name', name, '--exact')


class Peer:
    def __init__(self, name):
        self.name = name
        self.client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.api('/register', dict(name=name, password=PASSWORD, invite=INVITE))

    def api(self, path, body=None):
        request = urllib.request.Request(BASE + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers={'Content-Type': 'application/json'})
        with self.client.open(request, timeout=20) as response:
            return json.load(response)

    def command(self, kind, r=None, **extra):
        r = r or self.api('/state')['room']
        return self.api('/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))

    def ready_load(self):
        r = self.api('/state')['room']
        if r.get('loading'):
            gate = r['loading']
            client = 'headless-peer-' + RUN
            self.command('load_begin', loadToken=gate['token'], loadBuild=gate['build'], loadClient=client)
            self.command('load_ready', loadToken=gate['token'], loadBuild=gate['build'], loadClient=client)


def snapshot(name):
    assert js('document.documentElement.scrollWidth<=innerWidth+2'), 'horizontal overflow at ' + name
    errors = browser('errors').get('errors', [])
    assert not errors, errors
    browser('screenshot', str(OUT / (name + '.png')), '--full')
    (OUT / (name + '.state.json')).write_text(json.dumps(api('/state'), ensure_ascii=False, indent=2))
    REPORT['checks'].append(name)
    if js('!!document.querySelector("[data-conquest-board]")'): contrast(name)
    print('PASS', name, flush=True)



def contrast(name):
    rows=js(r'''(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('.cq-heading span,.cq-heading strong,.cq-board button,.cq-board select,.cq-board input[type=number],.cq-budget span,.cq-action-heading strong,.cq-move-bottom>span,.cq-board .cq-phase,.cq-board .cq-notice')].filter(e=>e.getClientRects().length&&e.textContent.trim()||e.matches('input[type=number]')).map(e=>{
        let bg=[255,255,255],chain=[];for(let p=e;p;p=p.parentElement)chain.unshift(p);
        for(const p of chain){const c=rgb(getComputedStyle(p).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a));}
        const style=getComputedStyle(e),a=lum(rgb(style.color)),b=lum(bg);
        return {text:(e.textContent||e.value).trim(),color:style.color,background:bg,ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()''')
    (OUT/(name+'-contrast.json')).write_text(json.dumps(rows,ensure_ascii=False,indent=2))
    assert rows and all(row['ratio']>=4.5 for row in rows),rows


def frames_start():
    js('(()=>{window.__cqFrames=[];window.__cqCapture=true;const sample=()=>{if(!window.__cqCapture)return;const b=document.querySelector("[data-conquest-board]"),r=document.querySelector(".cq-viewport")?.getBoundingClientRect(),a=document.querySelector(".cq-actions")?.getBoundingClientRect();window.__cqFrames.push({at:performance.now(),phase:b?.dataset.phase,round:b?.dataset.round,map:r?.toJSON(),actions:a?.toJSON(),gold:(()=>{const e=document.querySelector("[data-cq-gold]"),r=e?.getBoundingClientRect();return {text:e?.textContent,rect:r?.toJSON(),visible:!!r&&r.top>=0&&r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})(),events:[...document.querySelectorAll("[data-cq-event]")].map(e=>({id:e.dataset.cqEvent,circle:e.querySelector("circle")?.getBoundingClientRect().toJSON()}))});if(window.__cqFrames.length<2000)requestAnimationFrame(sample)};sample()})()')


def frames_stop(name):
    frames=js('window.__cqCapture=false;window.__cqFrames')
    assert len(frames)>5,frames
    baseline=frames[0]
    for frame in frames:
        for part in ['map','actions']:
            for field in ['x','y','width','height']:
                assert abs(frame[part][field]-baseline[part][field])<1,(name,part,field,baseline,frame)
    (OUT/(name+'-frames.json')).write_text(json.dumps(frames,ensure_ascii=False,indent=2))
    REPORT['checks'].append(name+'-stable-frames')
    return frames


def settle_results(map_id,peer):
    # Two natural resignations across restart exercise platform result lifecycle.
    for iteration in range(2):
        browser('set','viewport','320','740')
        browser('scrollintoview','.cq-viewport')
        before=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
        d=peer.api('/state')['room']['game']['data']
        peer.command('action',action={'type':'resign','session':d['session'],'epoch':d['epoch']})
        browser('wait','[role=dialog][aria-label="本局结果"]')
        assert view()['phase']=='finished'
        snapshot(map_id+'-result-'+str(iteration))
        click('关闭')
        wait('!document.querySelector(".match-result-modal")')
        after=js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
        assert abs(before['y']-after['y'])<1,(before,after)
        # Changing platform muted preference and ordinary state reads cannot reopen.
        api('/state');browser('set','viewport','320','741');browser('set','viewport','320','740')
        assert not js('!!document.querySelector(".match-result-modal")')
        click('查看结果');browser('wait','.match-result-modal');click('关闭')
        snapshot(map_id+'-final-board-'+str(iteration))
        if iteration==0:
            command('rematch');browser('wait','.waiting-actions')
            command('ready');peer.command('ready');command('start');peer.ready_load()
            browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
            assert view()['phase']=='planning'



def zero_resource_case(peer):
    for width in [1440,320]:
        browser('set','viewport',str(width),'1000' if width==1440 else '740')
        new_game('europe',peer)
        d=view();owned=next(p for p in d['provinces'] if p['owner']==0)
        m=json.loads((ROOT/'games/conquest/maps/europe.json').read_text())
        target=next(p for p in m['provinces'] if p['id']==owned['id'])['neighbors'][0]
        browser('select','select[aria-label="选择地区"]',owned['id'])
        browser('find','role','tab','click','--name','征募','--exact')
        browser('fill','input[aria-label="征募数量"]','5000');click('加入征募计划')
        wait('document.querySelector(".cq-budget strong")?.textContent==="0"')
        assert view()['availableGold']==0
        snapshot('zero-budget-'+str(width))
        browser('find','role','tab','click','--name','行军','--exact')
        browser('select','select[aria-label="目标地区"]',target)
        browser('fill','input[aria-label="出兵数量"]','1000');click('加入行军计划')
        click('提交回合');wait('document.querySelector("[data-conquest-board]").dataset.submitted==="true"')
        browser('scrollintoview','[data-cq-gold="0"]');frames_start()
        before_round=view()['round'];d=peer.api('/state')['room']['game']['data']
        peer.command('action',action={'type':'submit','session':d['session'],'epoch':d['epoch']})
        wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(before_round))
        frames=frames_stop('zero-gold-'+str(width))
        zero=[frame for frame in frames if frame['gold']['text']=='0']
        assert len(zero)>=2 and all(frame['gold']['visible'] for frame in zero),frames
        assert int(view()['players'][0]['gold'])!=5000,'public gold never updated'
        REPORT['checks'].append('zero-gold-visible-'+str(width))
        snapshot('zero-gold-after-action-'+str(width))
        finish(peer)


def extra_players_and_fog():
    api('/rooms',dict(kind='conquest',name='八席迷雾验收',capacity=8,options={'mapId':'world','fog':True,'difficulty':'normal','turnSeconds':180}))
    for _ in range(7):command('add_bot')
    command('ready');command('start');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    assert len(view()['players'])==8 and view()['options']['fog']
    assert any(p['troops']==-1 for p in view()['provinces'])
    for width in [1440,320]:
        browser('set','viewport',str(width),'1000' if width==1440 else '740')
        resources('eight-fog-'+str(width));snapshot('eight-fog-'+str(width))
    assert js('[...document.querySelectorAll(".cq-troop-label text")].some(e=>e.textContent==="?")')
    own=next(p for p in view()['provinces'] if p['owner']==0)
    browser('select','select[aria-label="选择地区"]',own['id'])
    browser('find','role','tab','click','--name','建设','--exact')
    assert js('!!document.querySelector("button[aria-label=修建瞭望塔]")')
    snapshot('eight-fog-building')
    click('开启托管');wait('!!document.querySelector(".autoplay-notice")')
    assert js('[...document.querySelectorAll(".cq-turn-actions button")].every(b=>b.disabled)'), 'autoplay still exposes manual turn controls'
    browser('click','.autoplay-control')
    wait('!document.querySelector(".autoplay-notice")')
    snapshot('eight-fog-manual-reclaimed')
    spectator=Peer('征服观众'+RUN)
    spectator.api('/rooms/'+room()['id']+'/watch',{})
    api('/logout',{});api('/login',dict(name=spectator.name,password=PASSWORD));browser('reload');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    assert room()['spectating'] and view()['orders']==[]
    assert js('[...document.querySelectorAll(".cq-turn-actions button")].every(b=>b.disabled)')
    snapshot('spectator-readonly')
    api('/logout',{});api('/login',dict(name='征服验收'+RUN,password=PASSWORD));browser('reload');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    command('close');command('leave')


def geometry(name):
    value = js('(()=>{const box=s=>document.querySelector(s)?.getBoundingClientRect().toJSON();return {map:box(".cq-viewport"),actions:box(".cq-actions"),board:box("[data-conquest-board]"),width:innerWidth,height:innerHeight}})()')
    value['name'] = name
    REPORT['geometry'].append(value)
    assert value['map']['width'] > 250 and value['map']['height'] > 230, value
    assert value['map']['left'] >= 0 and value['map']['right'] <= value['width'] + 1, value
    assert value['actions']['left'] >= 0 and value['actions']['right'] <= value['width'] + 1, value
    return value


def native(mode, x, y, tx=None, ty=None):
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    args = ['node', 'scripts/browser_touch.mjs', endpoint, BASE, mode, str(x), str(y), str(tx if tx is not None else x), str(ty if ty is not None else y)]
    subprocess.run(args, cwd=ROOT, env=ENV, check=True, timeout=20)
    REPORT['inputs'].append({'type': 'native-touch-' + mode, 'from': [x, y], 'to': [tx, ty]})


def gesture_checks(prefix):
    browser('scrollintoview', '.cq-viewport')
    r = js('document.querySelector(".cq-viewport").getBoundingClientRect().toJSON()')
    x, y = round(r['x'] + r['width'] / 2), round(r['y'] + r['height'] / 2)
    state = view()
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    initial = js('Number(document.querySelector("[data-conquest-board]").dataset.zoom)')
    subprocess.run(['node', 'scripts/browser_wheel.mjs', endpoint, BASE, str(x), str(y), '-320'], cwd=ROOT, env=ENV, check=True, timeout=20)
    wait('Number(document.querySelector("[data-conquest-board]").dataset.zoom)>' + str(initial))
    before = js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    selected = js('document.querySelector("[data-conquest-board]").dataset.selected')
    browser('mouse', 'move', str(x), str(y)); browser('mouse', 'down')
    browser('mouse', 'move', str(x - 65), str(y - 40)); browser('mouse', 'up')
    after = js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    assert before != after, 'native mouse did not pan'
    assert js('document.querySelector("[data-conquest-board]").dataset.selected') == selected, 'drag selected region'
    assert view() == state, 'gesture changed authoritative game state'
    REPORT['inputs'].append({'type': 'native-mouse-wheel-and-drag', 'map': prefix})
    snapshot(prefix + '-mouse-map')
    click('全图')
    native('pinch', x, y)
    wait('Number(document.querySelector("[data-conquest-board]").dataset.zoom)>1.1')
    before = js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    native('drag', x, y, x - 45, y - 35)
    after = js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    assert before != after, 'native touch did not pan'
    native('cancel', x, y, x + 20, y + 20)
    assert view() == state, 'touch gesture changed game state'
    snapshot(prefix + '-touch-map')
    # Mini-map must support continuous dragging, not just click.
    browser('scrollintoview', '.cq-minimap')
    r = js('document.querySelector(".cq-minimap").getBoundingClientRect().toJSON()')
    x,y=round(r['x']+r['width']*.35),round(r['y']+r['height']*.4)
    browser('mouse','move',str(x),str(y));browser('mouse','down')
    before=js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    browser('mouse','move',str(x+round(r['width']*.3)),str(y+round(r['height']*.2)))
    after=js('document.querySelector(".cq-map-layer").getAttribute("transform")')
    browser('mouse','up');assert before!=after, 'minimap drag did not move viewport'
    actual=js('(()=>{const t=document.querySelector(".cq-map-layer").transform.baseVal.consolidate().matrix,r=document.querySelector(".cq-viewport").getBoundingClientRect(),v=document.querySelector(".cq-minimap").viewBox.baseVal;return {x:(r.width/2-t.e)/t.a,y:(r.height/2-t.f)/t.a,scale:t.a,w:r.width,h:r.height,mw:v.width,mh:v.height}})()')
    end_x=x+round(r['width']*.3);end_y=y+round(r['height']*.2)
    wanted=[(end_x-r['x'])/r['width']*actual['mw'],(end_y-r['y'])/r['height']*actual['mh']]
    for axis,extent,visible,want in [('x',actual['mw'],actual['w'],wanted[0]),('y',actual['mh'],actual['h'],wanted[1])]:
        half=visible/actual['scale']/2
        expected=extent/2 if 2*half>=extent else max(half,min(extent-half,want))
        assert abs(actual[axis]-expected)<2,(axis,actual,expected)
    REPORT['inputs'].append({'type':'minimap-geographic-centre','result':actual,'wanted':wanted})
    click('全图')


def resources(name):
    browser('scrollintoview', '.module-players')
    rows = js(r'''(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
      const lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('[data-cq-gold]')].map(e=>{
        const r=e.getBoundingClientRect();let bg=[255,255,255];const chain=[];
        for(let p=e;p;p=p.parentElement)chain.unshift(p);
        for(const p of chain){const c=rgb(getComputedStyle(p).backgroundColor),a=c[3]??1;bg=bg.map((v,i)=>c[i]*a+v*(1-a));}
        const a=lum(rgb(getComputedStyle(e).color)),b=lum(bg);
        return {seat:e.dataset.cqGold,text:e.textContent,rect:r.toJSON(),horizontal:r.left>=0&&r.right<=innerWidth,contrast:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()''')
    assert len(rows) == len(room()['seats']) and all(r['horizontal'] and r['contrast'] >= 4.5 for r in rows), rows
    for row in rows:
        selector = '[data-cq-gold="' + row['seat'] + '"]'
        browser('scrollintoview', selector)
        visible = js('(()=>{const e=document.querySelector(' + json.dumps(selector) + '),r=e.getBoundingClientRect();return r.width>0&&r.top>=0&&r.bottom<=innerHeight&&e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))})()')
        assert visible, row
    (OUT / (name + '-resources.json')).write_text(json.dumps(rows, ensure_ascii=False, indent=2))
    REPORT['checks'].append(name + '-resources')


def new_game(map_id, peer):
    api('/rooms', dict(kind='conquest', name='征服时代验收 ' + map_id, capacity=2, options={'mapId':map_id,'difficulty':'normal','turnSeconds':180}))
    peer.command('join', room()); peer.command('ready'); command('ready'); command('start')
    peer.ready_load()
    browser('wait', '[data-conquest-board]')
    wait('document.querySelector("[data-conquest-board]").dataset.map===' + json.dumps(map_id))
    wait('!document.querySelector(".game-loading")')
    assert view()['options']['mapId'] == map_id
    (OUT / (map_id + '-initial.json')).write_text(json.dumps(room(), ensure_ascii=False, indent=2))


def finish(peer):
    r = api('/state').get('room')
    if r and r['status'] == 'playing': command('close')
    if r: command('leave')
    if peer.api('/state').get('room'): peer.command('leave')
    browser('wait', '.room-list')



def exercise_orders(map_id, peer):
    browser('set','viewport','1440','1000')
    initial=view()
    owned=next(p for p in initial['provinces'] if p['owner']==0 and p['mobile']>1)
    map_data=json.loads((ROOT/'games/conquest/maps'/f'{map_id}.json').read_text())
    geo=next(p for p in map_data['provinces'] if p['id']==owned['id'])
    target=next(pid for pid in geo['neighbors'] if next(p for p in initial['provinces'] if p['id']==pid)['owner']!=0)
    browser('select','select[aria-label="选择地区"]',owned['id'])
    browser('select','select[aria-label="目标地区"]',target)
    browser('fill','input[aria-label="出兵数量"]','1')
    geometry(map_id+'-selected')
    snapshot(map_id+'-plan-preview')
    click('加入行军计划')
    wait('document.querySelector("[data-conquest-board]").dataset.orders==="1"')
    planned=view()
    assert planned['provinces']==initial['provinces'],'plan immediately altered actual battlefield'
    assert planned['orders'][0]['to']==target
    assert peer.api('/state')['room']['game']['data']['orders']==[], 'opponent received private order'
    click('查看行动计划')
    order=planned['orders'][0]
    click('撤销指令 '+str(order['id']))
    wait('document.querySelector("[data-conquest-board]").dataset.orders==="0"')
    click('关闭')
    browser('find','role','tab','click','--name','征募','--exact')
    browser('fill','input[aria-label="征募数量"]','10')
    click('加入征募计划')
    wait('document.querySelector("[data-conquest-board]").dataset.orders==="1"')
    assert view()['availableGold']<initial['availableGold']
    saved=view();browser('reload');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    assert view()==saved,'private plan lost after reload'
    snapshot(map_id+'-planned-reloaded')
    REPORT.setdefault('loadedResources',{})[map_id]=js(r'performance.getEntriesByType("resource").map(r=>r.name).filter(n=>/\/(world|europe)-[^/]+\.js/.test(n))')
    assert REPORT['loadedResources'][map_id] and all('/'+map_id+'-' in url for url in REPORT['loadedResources'][map_id]), REPORT['loadedResources'][map_id]
    js('window.__cqRequests=[];const originalFetch=window.fetch;window.fetch=async(...args)=>{const r=await originalFetch(...args);if(args[1]?.body)window.__cqRequests.push({url:String(args[0]),body:args[1].body,status:r.status,response:await r.clone().text()});return r}')
    click('提交回合')
    wait('document.querySelector("[data-conquest-board]").dataset.submitted==="true"')
    assert view()['round']==initial['round'],'round advanced before all players submitted'
    click('撤回提交')
    wait('document.querySelector("[data-conquest-board]").dataset.submitted==="false"')
    click('提议暂停')
    r=peer.api('/state')['room'];d=r['game']['data']
    peer.command('action',action={'type':'pause','enabled':True,'session':d['session'],'epoch':d['epoch']})
    wait('document.querySelector("[data-conquest-board]").dataset.phase==="paused"')
    paused=view();snapshot(map_id+'-paused')
    browser('reload');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
    assert view()==paused,'paused game changed on reload'
    click('同意恢复')
    d=peer.api('/state')['room']['game']['data']
    peer.command('action',action={'type':'resume','enabled':True,'session':d['session'],'epoch':d['epoch']})
    wait('document.querySelector("[data-conquest-board]").dataset.phase==="planning"')
    assert view()['orders']==saved['orders'],'pause/resume discarded plans'
    # Natural recruitment and attack guarantee an actual resolving animation.
    browser('select','select[aria-label="选择地区"]',owned['id'])
    browser('find','role','tab','click','--name','征募','--exact')
    browser('fill','input[aria-label="征募数量"]','1000');click('加入征募计划')
    wait('document.querySelector("[data-conquest-board]").dataset.orders==="2"')
    browser('find','role','tab','click','--name','行军','--exact')
    browser('select','select[aria-label="目标地区"]',target)
    browser('fill','input[aria-label="出兵数量"]','1000');click('加入行军计划')
    wait('document.querySelector("[data-conquest-board]").dataset.orders==="3"')
    js('window.__cqSounds=[];const originalPlay=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return originalPlay.call(this).then(()=>window.__cqSounds.push(src))}')
    browser('scrollintoview','.cq-turn-actions');frames_start()
    click('提交回合')
    d=peer.api('/state')['room']['game']['data']
    peer.command('action',action={'type':'submit','session':d['session'],'epoch':d['epoch']})
    wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(initial['round']))
    assert view()['round']==initial['round']+1
    desktop_frames=frames_stop(map_id+'-round-desktop')
    assert any(f['phase']=='resolving' for f in desktop_frames),'resolving phase not observed'
    moving=[f for f in desktop_frames if f['events']]
    assert len(moving)>=2 and len({str(f['events'][0]['circle']) for f in moving})>1,'actual march animation not observed'
    assert next(p for p in view()['provinces'] if p['id']==target)['owner']==0,'natural attack did not capture target'
    REPORT.setdefault('audio',{})[map_id]=js('window.__cqSounds')
    assert any('/conquest/v1/audio/' in path for path in REPORT['audio'][map_id]),REPORT['audio']
    snapshot(map_id+'-round-resolved')
    browser('set','viewport','320','740')
    browser('select','select[aria-label="选择地区"]',owned['id'])
    browser('scrollintoview','.cq-viewport')
    q=json.dumps('[data-cq-label="'+owned['id']+'"]')
    r=js('document.querySelector('+q+').getBoundingClientRect().toJSON()')
    native('tap',round(r['x']+r['width']/2),round(r['y']+r['height']/2))
    assert js('document.querySelector("[data-conquest-board]").dataset.selected')==owned['id']
    geometry(map_id+'-selected-mobile');resources(map_id+'-after-action');snapshot(map_id+'-mobile-selected')
    # Keyboard selects a real region; confirmation remains a separate action.
    browser('focus','[data-cq-province="'+owned['id']+'"]');browser('press','Enter')
    assert js('document.querySelector("[data-conquest-board]").dataset.selected')==owned['id']
    browser('find','role','tab','click','--name','行军','--exact')
    target2=next(pid for pid in geo['neighbors'] if pid!=target)
    browser('select','select[aria-label="目标地区"]',target2)
    browser('fill','input[aria-label="出兵数量"]','1');click('加入行军计划')
    click('静音');sound_count=js('window.__cqSounds.length')
    # Keep the action in view before recording to exclude deliberate scrolls.
    browser('scrollintoview','.cq-turn-actions');frames_start()
    before_round=view()['round'];click('提交回合')
    d=peer.api('/state')['room']['game']['data']
    peer.command('action',action={'type':'submit','session':d['session'],'epoch':d['epoch']})
    wait('Number(document.querySelector("[data-conquest-board]").dataset.round)>'+str(before_round))
    mobile_frames=frames_stop(map_id+'-round-mobile')
    assert any(f['phase']=='resolving' for f in mobile_frames),'mobile resolving phase not observed'
    assert js('window.__cqSounds.length')==sound_count,'muted action played sound'
    click('取消静音');assert js('window.__cqSounds.length')==sound_count,'unmute replayed old sound'
    snapshot(map_id+'-mobile-round-resolved')

try:
    if not BASE:
        if ENV.get('CONQUEST_SKIP_BUILD') != '1':
            with (OUT/'build.log').open('w') as h:
                subprocess.run(['npm','--prefix','web','run','build'],cwd=ROOT,env=ENV,stdout=h,stderr=subprocess.STDOUT,check=True)
        for f in ['url','stop']:(OUT/f).unlink(missing_ok=True)
        h=(OUT/'server.log').open('w');handles.append(h)
        server=subprocess.Popen(['go','test','-count=1','-timeout=20m','-run','^TestConquestBrowserFixture$','./games/conquest/tests'],cwd=ROOT,env=ENV,stdout=h,stderr=subprocess.STDOUT)
        until=time.monotonic()+120
        while not (OUT/'url').exists():
            if server.poll() is not None or time.monotonic()>until:raise RuntimeError('Conquest fixture did not start; see server.log')
            time.sleep(.1)
        BASE=(OUT/'url').read_text().strip()
    browser('open',BASE);opened=True
    api('/register',dict(name='征服验收'+RUN,password=PASSWORD,invite=INVITE))
    peer=Peer('征服对手'+RUN)
    browser('reload');browser('wait','.room-list')
    assert 'conquest' in api('/state')['availableGames'], 'new game not published by default'
    browser('set','viewport','1440','1000')
    click('创建房间');browser('wait','[data-create-step=choose]')
    browser('fill','input[type=search][placeholder*="游戏"]','征服时代')
    browser('find','role','button','click','--name','征服时代','--exact')
    browser('wait','select[aria-label="征服地图"]')
    browser('select','select[aria-label="征服地图"]','europe')
    snapshot('create-europe-desktop')
    browser('set','viewport','320','740');snapshot('create-europe-320')
    click('关闭')
    for map_id in ['europe','world']:
        browser('set','viewport','1440','1000')
        new_game(map_id,peer)
        geometry(map_id+'-desktop');resources(map_id+'-desktop');snapshot(map_id+'-desktop')
        saved=view();browser('reload');browser('wait','[data-conquest-board]');wait('!document.querySelector(".game-loading")')
        assert view()==saved,'reload changed saved game'
        assert not js('document.querySelector("[data-cq-event]")'), 'reload replayed old action'
        for width in [1440,320]:
            browser('set','viewport',str(width),'1000' if width==1440 else '740')
            geometry(map_id+'-'+str(width));gesture_checks(map_id+'-'+str(width));resources(map_id+'-'+str(width));snapshot(map_id+'-'+str(width))
        # Full orders/commit/pause assertions are tied to the frozen server contract
        # in exercise_orders below. This hook is required, never silently skipped.
        exercise_orders(map_id,peer)
        settle_results(map_id,peer)
        finish(peer)
    zero_resource_case(peer)
    extra_players_and_fog()
    REPORT['passed']=True
except Exception as error:
    import traceback
    traceback.print_exc();REPORT['error']=repr(error)
    if opened:
        try:
            REPORT['state']=api('/state');REPORT['snapshot']=browser('snapshot','-i');REPORT['requests']=js('window.__cqRequests||[]')
            browser('screenshot',str(OUT/'failure.png'),'--full')
        except Exception:pass
finally:
    if opened:
        try:browser('close')
        except Exception:pass
    if server:
        (OUT/'stop').write_text('stop')
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    for handle in handles:handle.close()
    shutil.rmtree(PROFILE,ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    if not REPORT['passed']:raise SystemExit(1)
