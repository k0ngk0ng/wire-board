#!/usr/bin/env python3
"""Starfort real-browser acceptance (specified before production UI).
Failure scenarios: unpublished game leaks; solo create fails; wrong options;
unauthorized spectator controls; invisible resources on 320px; nonfunctional
build/upgrade/sell; all sprites missing; frozen canvas; pause time still passes;
old media replays on reload; muted combat still sounds; false win on defeat.
Uses a production build and an explicitly opted-in HTTP fixture. No UI mocks.
Run . scripts/env.sh && WIRE_BOARD_STARFORT_ASSETS=... python3 -B games/starfort/tests/accept_ui.py
Results/screenshots/commands: .local/starfort/browser/.
"""
import hashlib
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
out = root / '.local/starfort/browser'
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
for key, relative in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/ab-starfort', 'AGENT_BROWSER_PROFILE': '.local/starfort/browser/profile'}.items():
    path = root / relative
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_STARFORT_BROWSER='1', AGENT_BROWSER_SESSION='starfort-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
# A fresh owned profile prevents stale test tabs from previous servers accumulating.
profile=out/'profiles'/env['AGENT_BROWSER_SESSION']
profile.mkdir(parents=True,exist_ok=True)
env['AGENT_BROWSER_PROFILE']=str(profile)
report = {'passed': False, 'checks': [], 'command': 'python3 -B games/starfort/tests/accept_ui.py', 'assets': env.get('WIRE_BOARD_STARFORT_ASSETS', ''), 'scope': 'real Chromium, HTTP, websocket, production Canvas UI and native media; browser hardware output muted'}
server = None
opened = False

def run(args, log):
    with (out / log).open('w') as f:
        subprocess.run(args, cwd=root, env=env, stdout=f, stderr=subprocess.STDOUT, check=True)

def b(*args):
    if args[0] in ['click','select','fill']:
        target=json.dumps(args[1])
        if js('(()=>{const d=document.querySelector("[role=dialog]"),e=document.querySelector('+target+');return !!d&&!!e&&!d.contains(e)})()'):
            b('click','[role=dialog] [aria-label="关闭"]')
        summaries=js('(()=>{let e=document.querySelector('+target+'),a=[];for(let p=e?.parentElement;p;p=p.parentElement)if(p.tagName==="DETAILS"&&!p.open&&p.querySelector(":scope > summary")!==e){const id="accept-detail-"+a.length;p.dataset.acceptDetail=id;a.push("[data-accept-detail="+id+"] > summary")}return a.reverse()})()')
        for summary in summaries: b('click',summary)
    if args[0]=='click':
        selector=json.dumps(args[1])
        b('wait','--fn',f'(()=>{{const e=document.querySelector({selector});return !!e&&!e.disabled}})()')
        # Camera pads are children of a fixed-height clipped viewport. Center
        # that viewport, so its upper pads cannot scroll under the sticky header.
        b('scrollintoview','[data-starfort-canvas]' if args[1].startswith('[data-sf-pad=') else args[1])
    r = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=45)
    if r.returncode: raise RuntimeError(r.stdout + r.stderr)
    data = json.loads(r.stdout)
    assert data['success'], data
    return data['data']

def js(code): return b('eval', code)['result']
def wait(code): b('wait', '--fn', code)
def click(name):
    label=json.dumps(name)
    wait(f'[...document.querySelectorAll("button")].some(e=>(e.textContent.trim()==={label}||e.getAttribute("aria-label")==={label})&&!e.disabled)')
    b('find', 'role', 'button', 'click', '--name', name, '--exact')
def api(path, body=None, status=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('fetch(' + json.dumps('/api' + path) + opts + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert r['status'] == status if status else 200 <= r['status'] < 300, r
    return r['body']
def room(): return api('/state')['room']
def command(kind, **extra):
    for _ in range(20):
        r = room()
        body = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        response = js('fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}).then(async r=>({status:r.status,body:await r.json()}))')
        if response['status'] == 409: continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('unresolved stale command')
def action(kind, **extra):
    return command('action', action=dict(type=kind, epoch=room()['game']['data']['epoch'], **extra))
def login(name):
    api('/logout', {})
    api('/login', {'name': name, 'password': 'starfort-browser-test'})
    b('open', base)
def close_room():
    r = api('/state').get('room')
    if r:
        if r['status'] == 'playing': command('close')
        command('leave')
    b('wait', '.room-list')
def scenario(name, mode='campaign', players=1):
    close_room()
    for peer in peers:
        if peer.api('/state').get('room'): peer.command('leave')
    created = api('/rooms', {'kind': 'starfort', 'name': '星垒 ' + name, 'capacity': players, 'options': {'scenario': name, 'mode': mode, 'waves': 10, 'difficulty': 'normal', 'prepSeconds': 30}})
    for peer in peers[:players-1]:
        peer.command('join', room_id=created['id'], version=room()['version'])
        peer.command('ready')
    command('ready'); command('start')
    b('wait', '.starfort-board')

class Peer:
    def __init__(self, base, name):
        self.base, self.name = base, name
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.api('/register', {'name':name,'password':'starfort-browser-test','invite':'starfort-browser'})
    def api(self, path, body=None):
        req=urllib.request.Request(self.base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
        with self.opener.open(req,timeout=10) as r:return json.load(r)
    def command(self, kind, room_id=None, version=None, **extra):
        r=self.api('/state').get('room')
        return self.api('/rooms/'+(room_id or r['id']),dict(type=kind,version=version if version is not None else r['version'],nonce=uuid.uuid4().hex,**extra))
    def act(self, kind, **extra):
        return self.command('action',action=dict(type=kind,epoch=self.api('/state')['room']['game']['data']['epoch'],**extra))

def watch_enemy_draws():
    frames={}
    for suffix in ['v1','world-v2']:
        atlas=json.loads((root/'games/starfort/assets/public/starfort'/suffix/'atlas.json').read_text())
        for key,frame in atlas['frames'].items():
            if key.startswith('enemy-'):frames[suffix+':'+str(frame['x'])+':'+str(frame['y'])]=key
    js('window.__sfEnemyFrames='+json.dumps(frames))
    js(r"""(() => {
      const c=document.querySelector('canvas[data-starfort-canvas]').getContext('2d');
      if(!c.__originalDraw){c.__originalDraw=c.drawImage;c.__originalClear=c.clearRect;
        c.clearRect=function(...a){window.__sfFrame=(window.__sfFrame||0)+1;return this.__originalClear(...a)};
        c.drawImage=function(...a){if(a.length===9&&a[3]===192&&a[4]===224){const t=this.getTransform(),p=t.transformPoint({x:a[5],y:a[6]}),q=t.transformPoint({x:a[5]+a[7],y:a[6]+a[8]});window.__sfTowers.push({x:p.x,y:p.y,right:q.x,bottom:q.y,width:this.canvas.width,height:this.canvas.height});if(window.__sfTowers.length>500)window.__sfTowers.shift()}if(a.length===9&&window.__sfEnemyFrames[(a[0].src.includes('/world-v2/')?'world-v2':'v1')+':'+a[1]+':'+a[2]]&&window.__sfLast!==window.__sfFrame){window.__sfLast=window.__sfFrame;window.__sfDraws.push({frame:window.__sfFrame,x:a[5],y:a[6],sx:a[1],sy:a[2],src:a[0].src});if(window.__sfDraws.length>200)window.__sfDraws.shift()}return this.__originalDraw(...a)};
      }
      window.__sfDraws=[];window.__sfTowers=[];
    })()""")

def checkpoint(name):
    wait('document.querySelector(".modal")?.getAnimations().every(a=>a.playState!=="running") ?? true')
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth<=innerWidth+2'), 'horizontal overflow: ' + name
    assert js('[...document.images].every(i=>!i.complete||i.naturalWidth>0)'), 'broken images: ' + name
    b('screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(b('snapshot', '-i'),ensure_ascii=False,indent=2))
    report['checks'].append(name)
    print('PASS', name, flush=True)

def resources():
    assert js('!!document.querySelector("[data-sf-base]")'), 'missing base health'
    for i, p in enumerate(room()['game']['data']['players']):
        selector = json.dumps(f'[data-sf-gold="{i}"]')
        wait(f'(()=>{{const e=document.querySelector({selector});return !!e&&e.textContent.includes("{p["gold"]}")}})()')
    checks = js(r"""(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('[data-sf-gold],[data-sf-base]')].map(e=>{
        let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);
        for(const n of chain){const a=rgb(getComputedStyle(n).backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o))}
        const s=getComputedStyle(e),fg=lum(rgb(s.color)),back=lum(bg),r=e.getBoundingClientRect();
        return {text:e.textContent,visible:r.width>0&&r.left>=0&&r.right<=innerWidth,fontSize:parseFloat(s.fontSize),ratio:(Math.max(fg,back)+.05)/(Math.min(fg,back)+.05)};
      });
    })()""")
    assert all(c['visible'] and c['ratio']>=4.5 for c in checks), checks
    (out / ('resources-' + str(js('innerWidth')) + '.json')).write_text(json.dumps(checks,ensure_ascii=False,indent=2))

try:
    run(['npm','--prefix','web','run','build'], 'build.log')
    for name in ['url','stop']: (out / name).unlink(missing_ok=True)
    with (out / 'server.log').open('w') as f:
        server = subprocess.Popen(['go','test','-count=1','-timeout=15m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
    deadline=time.monotonic()+100
    while not (out/'url').exists():
        if server.poll() is not None or time.monotonic()>deadline: raise RuntimeError('fixture failed: see server.log')
        time.sleep(.1)
    base=(out/'url').read_text()
    peers=[Peer(base,'守望同伴'+str(i)) for i in range(1,4)]
    spectator=Peer(base,'星垒观战')
    b('open',base);opened=True
    b('tab',next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/register',{'name':'星垒体验','password':'starfort-browser-test','invite':'starfort-browser'})
    b('reload');b('wait','.room-list')
    assert 'starfort' not in api('/state')['availableGames']
    api('/rooms',{'kind':'starfort','name':'未上架不能创建','capacity':1},403)
    checkpoint('default-unpublished')
    login('上架管理员');b('wait','--text','管理后台');click('管理后台')
    b('wait','input[aria-label="搜索管理游戏"]')
    b('fill','input[aria-label="搜索管理游戏"]','星垒')
    b('click','.admin-game-card button');b('wait','[role=dialog]');click('确认操作')
    wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
    checkpoint('admin-published')
    login('星垒体验');b('wait','.room-list')
    b('select','select[aria-label="按游戏筛选牌桌"]','starfort');b('click','.lobby-create')
    b('wait','[role=dialog]');wait('document.querySelector(".modal").getAnimations().every(a=>a.playState!=="running")')
    assert js('!!document.querySelector(".choose-games .selected .cover svg")')
    b('select','select[aria-label=座位上限]','1')
    b('select','select[aria-label=守望模式]','endless')
    b('select','select[aria-label=难度]','hard')
    b('select','select[aria-label=波间准备]','45')
    checkpoint('create-desktop')
    b('set','viewport','320','740');checkpoint('create-mobile')
    b('scrollintoview','[role=dialog] button[type=submit]');b('click','[role=dialog] button[type=submit]')
    b('wait','.waiting-layout')
    opts=room()['options'];assert opts['mode']=='endless' and opts['difficulty']=='hard' and opts['prepSeconds']==45
    b('select','select[aria-label=守望模式]','campaign20');b('select','select[aria-label=难度]','easy');b('select','select[aria-label=波间准备]','20');click('保存设置')
    wait('document.querySelector(".waiting-actions")!==null')
    opts=room()['options'];assert opts['waves']==20 and opts['difficulty']=='easy' and opts['prepSeconds']==20
    b('select','select[aria-label=守望模式]','campaign10');b('select','select[aria-label=难度]','normal');b('select','select[aria-label=波间准备]','30');click('保存设置')
    checkpoint('waiting-solo')
    click('我准备好了');click('开始游戏');b('wait','.starfort-board')
    assert len(room()['seats'])==1 and room()['game']['data']['phase']=='prepare'
    resources()
    assert js('document.querySelector(".module-players .player-panel").getBoundingClientRect().width>=document.querySelector(".module-players").clientWidth-4'), 'solo player card wastes half the row'
    checkpoint('initial-mobile')
    before=room()['game']['data']['players'][0]['gold']
    b('click','[data-sf-pad="0"]');b('click','[data-sf-build="arrow"]')
    wait('document.querySelector("[data-sf-upgrade]")!==null')
    data=room()['game']['data'];assert len(data['towers'])==1 and data['players'][0]['gold']<before
    b('click','[data-sf-upgrade]')
    assert room()['game']['data']['towers'][0]['level']==2
    checkpoint('build-upgrade-mobile')
    b('click','[data-sf-sell]');wait('document.querySelector("[data-sf-build=arrow]")!==null')
    assert len(room()['game']['data']['towers'])==0
    b('click','[data-sf-pause]');wait('document.querySelector(".starfort-board").dataset.phase==="paused"')
    tick=room()['game']['data']['tick'];time.sleep(2)
    assert room()['game']['data']['tick']==tick
    checkpoint('paused-no-ticks')
    b('click','[data-sf-resume]');wait('document.querySelector(".starfort-board").dataset.phase==="prepare"')
    b('click','[data-sf-pad="0"]');b('click','[data-sf-build="arrow"]')
    b('click','[data-sf-ready]');wait('document.querySelector(".starfort-board").dataset.phase==="battle"')
    wait('Number(document.querySelector("canvas[data-starfort-canvas]").dataset.enemies)>0')
    wait('document.querySelector(".sf-battlefield").dataset.assetsReady==="true"')
    checkpoint('natural-moving-battle')
    scenario('battle',players=4)
    wait('document.querySelector(".sf-battlefield").dataset.assetsReady==="true"')
    until=time.monotonic()+8
    while room()['game']['data']['tick']<5:
        assert time.monotonic()<until, 'battle timer did not advance'
        time.sleep(.1)
    watch_enemy_draws()
    # Broadcast cadence varies under concurrent Go checks; wait for actual
    # enemy travel instead of letting a fixed sleep mistake bobbing for motion.
    wait('window.__sfDraws.length>10&&Math.max(...window.__sfDraws.map(d=>d.x))-Math.min(...window.__sfDraws.map(d=>d.x))>4')
    draws=js('window.__sfDraws')
    (out/'enemy-animation.json').write_text(json.dumps(draws,indent=2))
    assert len(draws)>10 and max(d['x'] for d in draws)-min(d['x'] for d in draws)>1, 'enemy sprite does not travel'
    assert len({(d['sx'],d['sy']) for d in draws})>1, 'enemy walking frames do not change'
    if env.get('WIRE_BOARD_STARFORT_ASSETS'): assert all(d['src'].startswith(env['WIRE_BOARD_STARFORT_ASSETS']) for d in draws), 'Canvas silently used local fallback instead of OSS'
    (out/'enemy-animation.json').write_text(json.dumps(draws,indent=2))
    if env.get('WIRE_BOARD_STARFORT_ASSETS'):
        js('window.__starfortAudio=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return play.call(this).then(()=>{if(src.includes("/starfort/"))window.__starfortAudio.push(src)})}')
        wait('window.__starfortAudio.length>0')
        click('静音');count=js('window.__starfortAudio.length');time.sleep(2)
        assert js('window.__starfortAudio.length')==count
        assert js('document.querySelector("audio[data-platform-audio]").paused')
        checkpoint('muted-combat')
        b('reload');b('wait','.starfort-board')
        assert js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"')
        assert js('document.querySelector("audio[data-platform-audio]").paused')
        click('取消静音')
    for width in [1440,390,320]:
        b('set','viewport',str(width),'900' if width==1440 else '844')
        resources();checkpoint('battle-'+str(width))
    b('click','[data-sf-fit]')
    assert js('document.querySelector("[data-starfort-canvas]").dataset.overview==="false"'), 'stage-one full map lost construction controls'
    b('click','[data-sf-pad="12"]');b('click','[data-sf-build="arrow"]')
    watch_enemy_draws();time.sleep(.3)
    drawn=js('window.__sfTowers')
    assert len(drawn)>0 and all(d['x']>=0 and d['y']>=0 and d['right']<=d['width']+.1 and d['bottom']<=d['height']+.1 for d in drawn), 'top or edge tower sprite clipped'
    checkpoint('top-edge-tower-mobile')
    b('set','media','light','reduced-motion')
    js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))')
    watch_enemy_draws();time.sleep(.8)
    draws=js('window.__sfDraws')
    first_frames={(v['x'],v['y']) for k,v in json.loads((root/'games/starfort/assets/public/starfort/v1/atlas.json').read_text())['frames'].items() if k.startswith('enemy-') and k.endswith('-0')}
    assert len(draws)>1 and all((d['sx'],d['sy']) in first_frames for d in draws),'reduced motion still cycles sprite frames'
    (out/'reduced-motion.json').write_text(json.dumps(draws,indent=2))
    checkpoint('reduced-motion');b('set','media','light')
    watched=room()['id'];spectator.api('/rooms/'+watched+'/watch',{})
    login(spectator.name);b('wait','.starfort-board');resources()
    assert not room()['game']['data']['controls']['canAct']
    assert js('[...document.querySelectorAll("[data-sf-ready],[data-sf-build],[data-sf-upgrade],[data-sf-sell],[data-sf-gift]")].every(b=>b.disabled)')
    checkpoint('spectator-four-players')
    login('星垒体验');b('wait','.starfort-board');spectator.api('/rooms/'+watched+'/watch',{'leave':True})
    scenario('natural',players=2)
    original=[p['gold'] for p in room()['game']['data']['players']]
    b('select','select[aria-label=接收金币的队友]','1');b('fill','input[aria-label=赠予金币数量]','25');b('click','[data-sf-gift]')
    wait('document.querySelector('+json.dumps('[data-sf-gold="0"]')+').textContent.includes('+json.dumps(str(original[0]-25))+')')
    assert [p['gold'] for p in room()['game']['data']['players']]==[original[0]-25,original[1]+25]
    b('click','[data-sf-pause]');assert room()['game']['data']['phase']=='prepare'
    peers[0].act('pause');wait('document.querySelector(".starfort-board").dataset.phase==="paused"')
    checkpoint('team-gift-and-unanimous-pause')
    b('click','[data-sf-resume]');assert room()['game']['data']['phase']=='paused'
    peers[0].act('resume');wait('document.querySelector(".starfort-board").dataset.phase==="prepare"')
    scenario('advanced','endless')
    assert room()['game']['data']['wave']>20
    checkpoint('endless-beyond-20')
    scenario('zero-gold');resources();checkpoint('zero-gold-visible')
    scenario('victory-ready');b('wait','.results')
    assert room()['status']=='finished' and room()['game']['data']['phase']=='won'
    assert not room().get('result',{}).get('rated',False)
    checkpoint('cooperative-victory')
    b('reload');b('wait','.results');checkpoint('result-restored')
    scenario('defeat-ready');b('wait','.results')
    assert room()['game']['data']['phase']=='lost' and not room()['game']['winners']
    assert not room().get('result',{}).get('rated',False)
    assert not js('document.querySelector(".starfort-board").textContent.includes("从一座塔开始")'), 'finished empty state still invites initial construction'
    checkpoint('cooperative-defeat')
    report['passed']=True
except Exception as error:
    import traceback
    traceback.print_exc();report['error']=repr(error)
    if opened:
        try:
            report['snapshot']=b('snapshot','-i');report['state']=api('/state');report['errors']=b('errors');report['draws']=js('window.__sfDraws||[]')
            b('screenshot',str(out/'failure.png'),'--full')
        except Exception as e: report['diagnosticError']=repr(e)
finally:
    if opened:
        try:b('close')
        except Exception:pass
    if server:
        (out/'stop').write_text('stop')
        try:server.wait(timeout=15)
        except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
    report['commit']=subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()
    sources=list((root/'games/starfort').rglob('*.go'))+list((root/'games/starfort/web').glob('*'))+[Path(__file__)]
    report['sourceSHA256']={str(path.relative_to(root)):hashlib.sha256(path.read_bytes()).hexdigest() for path in sources if path.is_file()}
    built=[root/'web/dist/index.html']+list((root/'web/dist/assets').glob('*.js'))+list((root/'web/dist/assets').glob('*.css'))
    report['buildSHA256']={str(path.relative_to(root)):hashlib.sha256(path.read_bytes()).hexdigest() for path in built if path.is_file()}
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print('Artifact:',out/'result.json',flush=True)
    if not report['passed']:raise SystemExit(1)
