#!/usr/bin/env python3
"""Real Chromium regression for combat timing, nonmodal tower tools and chat.

Failure scenarios were written before implementation in the adjacent markdown.
Run with --skip-build after the matching production frontend build; --baseline
records original failures without representing them as successful acceptance.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import shutil
import uuid

ROOT = Path(__file__).resolve().parents[3]
BASELINE_COMBAT = '--baseline-combat' in sys.argv
BASELINE = '--baseline' in sys.argv or BASELINE_COMBAT
os.environ['WIRE_BOARD_STARFORT_BROWSER_OUT'] = str(ROOT / '.local/starfort-followup/browser' / ('before-combat' if BASELINE_COMBAT else 'before' if BASELINE else 'central' if '--central-case' in sys.argv else 'after-mobile' if '--mobile-only' in sys.argv else 'after-pc' if '--pc-only' in sys.argv else 'after-combat' if '--combat-only' in sys.argv else 'after'))
import accept_world_ui as w

w.MAIN = '战斗控件守望者'
for key, path in {'GOTMPDIR': w.OUT/'tmp', 'TMPDIR': w.OUT/'tmp',
                  'AGENT_BROWSER_SOCKET_DIR': Path(os.environ.get('STARFORT_FOLLOWUP_SOCKET_DIR', str(ROOT/'.local/s')))}.items():
    path.mkdir(parents=True, exist_ok=True)
    w.ENV[key] = str(path)
for key in ['GOCACHE', 'GOMODCACHE']:
    if os.environ.get(key): w.ENV[key] = os.environ[key]
w.ENV['AGENT_BROWSER_SESSION'] = 'sfc-' + w.SESSION.rsplit('-', 1)[-1]

w.REPORT = {'passed': False, 'baseline': BASELINE, 'checks': [], 'failures': [],
            'command': ' '.join(sys.argv),
            'assetsBaseURL': w.ENV.get('WIRE_BOARD_STARFORT_ASSETS',''),
            'selectedScope': 'combat' if '--combat-only' in sys.argv or BASELINE_COMBAT else 'ui' if '--ui-only' in sys.argv or BASELINE else 'ui-and-combat',
            'scope': 'Starfort production Canvas, real HTTP/WebSocket room, native Chromium mouse and touch, platform fullscreen and chat.',
            'limitations': ['Milestone fixtures are not a natural full match.', '320px Chromium touch emulation, not a physical phone or Safari.']}
b, js, wait = w.b, w.js, w.wait


def verify(name, passed, evidence=None):
    entry = {'name': name, 'passed': bool(passed)}
    if evidence is not None: entry['evidence'] = evidence
    w.REPORT['checks'].append(entry)
    if not passed: w.REPORT['failures'].append(entry)
    print(('PASS ' if passed else 'FAIL ') + name, flush=True)
    if not passed and not BASELINE: raise AssertionError((name, evidence))


def screenshot(name):
    b('screenshot', str(w.OUT / (name + '.png')))


def rect(selector):
    return js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')


def stable():
    js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))')


def setup():
    for name in ['url', 'stop']: (w.OUT/name).unlink(missing_ok=True)
    with (w.OUT/'server.log').open('w') as log:
        w.SERVER = subprocess.Popen(['go','test','-count=1','-timeout=20m','-run','^TestStarfortBrowserFixture$','./games/starfort/tests'],cwd=ROOT,env=w.ENV,stdout=log,stderr=subprocess.STDOUT)
    deadline=time.monotonic()+120
    while not (w.OUT/'url').exists():
        if w.SERVER.poll() is not None or time.monotonic()>deadline: raise RuntimeError('Fixture startup failed; see server.log')
        time.sleep(.1)
    w.BASE=(w.OUT/'url').read_text()
    if not BASELINE or BASELINE_COMBAT:
        atlases={v:json.loads((ROOT/'games/starfort/assets/public/starfort'/v/'atlas.json').read_text()) for v in ['v1','world-v2']}
        probe=w.OUT/'combat-init.js'
        probe.write_text('window.__sfProbeConfig='+json.dumps({'atlases':atlases})+';\n'+(ROOT/'games/starfort/tests/combat_draw_probe.js').read_text())
        w.ENV['AGENT_BROWSER_INIT_SCRIPTS']=str(probe)
    b('open',w.BASE);w.OPENED=True
    w.REPORT['browserUserAgent']=js('navigator.userAgent')
    w.api('/register',{'name':w.MAIN,'password':'starfort-browser-test','invite':'starfort-browser'})
    w.login('上架管理员');w.label('管理后台');b('wait','input[aria-label="搜索管理游戏"]')
    b('fill','input[aria-label="搜索管理游戏"]','星垒')
    if '已下架' in js('document.querySelector(".admin-game-card").textContent'):
        w.click('.admin-game-card button');w.label('确认操作')
    wait('document.querySelector(".admin-game-card")?.textContent.includes("已上架")')
    w.login(w.MAIN)
    w.PEERS=[w.Peer('战斗控件同伴')]


def enter():
    if not js('!!document.querySelector("[data-fullscreen-enter]")'): b('click','.compact-room-tools')
    b('click','[data-fullscreen-enter]');wait('!!document.querySelector("[data-room-fullscreen]")');stable()


def leave(key=False):
    b('press','Escape') if key else b('click','[data-room-fullscreen] [data-fullscreen-toggle]')
    wait('!document.querySelector("[data-room-fullscreen]")&&!document.fullscreenElement');stable()


def visibility(selector):
    return js('''(()=>{const e=document.querySelector('''+json.dumps(selector)+''');if(!e)return {exists:false};const r=e.getBoundingClientRect(),s=getComputedStyle(e),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {exists:true,rect:r.toJSON(),visible:r.width>0&&r.height>0&&r.x>=0&&r.y>=0&&r.right<=innerWidth&&r.bottom<=innerHeight&&s.visibility!=="hidden"&&s.display!=="none",hit:!!hit&&e.contains(hit),inside:!!document.querySelector('[data-game-fullscreen-surface]')?.contains(e)}})()''')


def choose_visible_tower(prefer_kind=None):
    w.click('[data-sf-camera-base]');stable()
    state=w.data();owned=[t['pad'] for t in state['towers'] if t['owner']==0 and (prefer_kind is None or t['kind']==prefer_kind)]
    value=js('''(()=>{const owned='''+json.dumps(owned)+''',towers=[...document.querySelectorAll('.sf-pad-occupied:not([hidden])')];return towers.find(e=>{const r=e.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height*.35);return owned.includes(+e.dataset.sfPad)&&r.x>=0&&r.right<=innerWidth&&r.y>=0&&r.bottom<=innerHeight&&e.contains(hit)})?.dataset.sfPad})()''')
    if value is None:
        pad=next(p for p in state['map']['pads'] if p['id']==owned[0])
        point=js('''(()=>{const e=document.querySelector('[data-sf-minimap]'),p=e.createSVGPoint();p.x='''+str(pad['x'])+';p.y='+str(pad['y'])+''';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()''')
        w.mouse_click(point['x'],point['y']);stable()
        value=str(pad['id'])
        wait('!document.querySelector('+json.dumps('[data-sf-pad="'+value+'"]')+').hidden')
    assert value is not None, 'no visible tower'
    return '[data-sf-pad="'+value+'"]'


def range_probe():
    js('''(()=>{if(window.__rangeProbe)return;window.__rangeProbe=[];const p=CanvasRenderingContext2D.prototype,arc=p.arc,stroke=p.stroke;p.arc=function(x,y,r,...a){if(this.canvas.matches('[data-starfort-canvas]'))this.__rangeArc={x,y,r};return arc.call(this,x,y,r,...a)};p.stroke=function(...a){if(this.canvas.matches('[data-starfort-canvas]')&&this.getLineDash().join(',')==='8,8'&&this.__rangeArc?.r>60){window.__rangeProbe.push({...this.__rangeArc,at:performance.now()});if(window.__rangeProbe.length>300)window.__rangeProbe.shift()}return stroke.apply(this,a)};return true})()''')


def range_seen():
    return js('window.__rangeProbe.filter(r=>r.at>performance.now()-120).length>0')


def native(mode, *coordinates):
    endpoint=b('get','cdp-url')['cdpUrl']
    result=subprocess.run(['node','scripts/browser_touch.mjs',endpoint,w.BASE,mode,*map(str,coordinates)],cwd=ROOT,env=w.ENV,capture_output=True,text=True,timeout=20)
    with (w.OUT/'commands.jsonl').open('a') as log:
        log.write(json.dumps({'nativeInput':'touch','mode':mode,'coordinates':coordinates,'returncode':result.returncode,'stdout':result.stdout,'stderr':result.stderr})+'\n')
    if result.returncode: raise RuntimeError(result.stdout+result.stderr)


def activate(selector, touch=False):
    wait('!!document.querySelector('+json.dumps(selector)+')&&!document.querySelector('+json.dumps(selector)+').disabled')
    visible=visibility(selector)
    verify('native activation is visible and hittable: '+selector,visible.get('visible') and visible.get('hit'),visible)
    r=rect(selector)
    if touch:native('tap',r['x']+r['width']/2,r['y']+r['height']/2,0,0)
    else:w.mouse_click(r['x']+r['width']/2,r['y']+r['height']/2)
    stable()


def painted_tower_point(selector):
    point=js('''(()=>{const e=document.querySelector('''+json.dumps(selector)+'''),r=e.getBoundingClientRect(),frames=window.__sfCombatProbe?.export().frames||[];for(const f of frames.slice(-12).reverse()){const values=f.draws.filter(d=>d.name.startsWith('tower-')&&d.screenPoint&&e.contains(document.elementFromPoint(d.screenPoint.x,d.screenPoint.y)));if(values.length)return values.sort((a,b)=>Math.abs(a.screenX-r.x-r.width/2)-Math.abs(b.screenX-r.x-r.width/2))[0]}return null})()''')
    verify('actual opaque tower pixel available',bool(point) and point['screenPoint']['alpha']>=200,point)
    return point['screenPoint']


def button_geometry(name):
    selectors=['.chat-toggle','[data-sf-zoom-in]','[data-sf-zoom-out]','[data-sf-camera-base]','[data-sf-fit]','[data-sf-open-info]','.sf-select-pad']
    values={s:visibility(s) for s in selectors}
    verify(name+' buttons visible and clickable',all(v.get('visible') and v.get('hit') for v in values.values()),values)
    pairs=[]
    for i,a in enumerate(selectors):
        for other in selectors[i+1:]:
            r,s=values[a]['rect'],values[other]['rect']
            overlap=max(0,min(r['right'],s['right'])-max(r['left'],s['left']))*max(0,min(r['bottom'],s['bottom'])-max(r['top'],s['top']))
            if overlap>.5:pairs.append({'a':a,'b':other,'area':overlap})
    verify(name+' buttons do not overlap',not pairs,pairs)
    w.artifact(name+'-button-rects.json',values)


def hud_geometry(name):
    result=js(r'''(()=>{const box=s=>document.querySelector(s)?.getBoundingClientRect().toJSON();const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number),lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);const entries=['[data-sf-base]','[data-sf-wave]','[data-sf-own-gold]','[data-sf-countdown]'].map(selector=>{const e=document.querySelector(selector);if(!e)return {selector,exists:false};const range=document.createRange();range.selectNodeContents(e);const r=range.getBoundingClientRect(),s=getComputedStyle(e),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);let bg=[255,255,255],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const a=rgb(getComputedStyle(n).backgroundColor),o=a[3]??1;bg=a.slice(0,3).map((v,i)=>v*o+bg[i]*(1-o))}const color=rgb(s.color),fg=lum(color.slice(0,3)),back=lum(bg);return {selector,exists:true,text:e.textContent,rect:r.toJSON(),visible:r.width>0&&r.height>0&&r.x>=0&&r.right<=innerWidth+1&&r.y>=0&&r.bottom<=innerHeight+1,hit:!!hit&&(e.contains(hit)||hit.contains(e)),color:s.color,background:bg,ratio:(Math.max(fg,back)+.05)/(Math.min(fg,back)+.05)}});return {hud:box('.sf-hud'),overview:box('.sf-overview'),banner:box('.sf-wave-banner'),canvas:box('[data-starfort-canvas]'),width:innerWidth,height:innerHeight,entries}})()''')
    w.artifact(name+'-hud.json',result)
    hud=result['hud'];limit=80 if result['width']<=850 else 48
    verify(name+' compact HUD height',bool(hud) and hud['height']<=limit,{'height':hud['height'] if hud else None,'maximum':limit})
    required=[e for e in result['entries'] if e['selector']!='[data-sf-countdown]']
    verify(name+' resources visibly readable',all(e.get('visible') and e.get('hit') and e.get('ratio',0)>=4.5 for e in required),required)
    if w.data()['phase']=='prepare':
        countdown=result['entries'][-1]
        verify(name+' actual countdown text visible',countdown.get('visible') and countdown.get('hit') and countdown.get('ratio',0)>=4.5,countdown)


def tower_scenarios(width,height):
    touch=width==320
    b('set','viewport',str(width),str(height));w.scenario('battle',2);enter()
    hud_geometry('after-fullscreen-'+str(width))
    selector=choose_visible_tower();r=rect(selector);range_probe()
    pad=int(js('document.querySelector('+json.dumps(selector)+').dataset.sfPad'))
    tower=next(t for t in w.data()['towers'] if t['pad']==pad)
    if not touch:
        b('mouse','move',r['x']+r['width']/2,r['y']+r['height']*.35);stable()
        verify('PC hover paints range',range_seen())
        verify('PC hover does not open tools or modal',not js('!!document.querySelector("[data-sf-tower-tools],.modal-backdrop")'))
        screenshot('pc-hover-range')
        b('mouse','move',1,1);b('focus',selector);stable()
        verify('PC keyboard focus paints range',range_seen())
        b('press','Enter')
    else:
        point=painted_tower_point(selector)
        native('tap',point['x'],point['y'],0,0)
    wait('!!document.querySelector("[data-sf-tower-tools]")')
    verify(str(width)+' tower click has no modal backdrop',not js('!!document.querySelector(".modal-backdrop")'))
    verify(str(width)+' selected range is drawn',range_seen())
    verify(str(width)+' tools inside fullscreen',visibility('[data-sf-tower-tools]').get('inside'))
    screenshot('tower-tools-'+str(width))
    gold=w.data()['players'][0]['gold'];activate('[data-sf-tower-tools] [data-sf-upgrade="level"]',touch)
    wait('document.querySelector("[data-sf-tower-tools]")?.textContent.includes("2")')
    state=w.data();upgraded=next(t for t in state['towers'] if t['id']==tower['id'])
    verify(str(width)+' upgrade changes real tower and gold',upgraded['level']==tower['level']+1 and state['players'][0]['gold']<gold,upgraded)
    wait('document.querySelector("[data-sf-own-gold]")?.textContent.trim()==='+json.dumps(str(state['players'][0]['gold'])))
    verify(str(width)+' compact gold updates after upgrade',visibility('[data-sf-own-gold]').get('visible'))
    activate('[data-sf-tower-more]',touch);wait('!!document.querySelector(".modal-backdrop")')
    verify(str(width)+' more dialog stays in fullscreen',js('document.querySelector("[data-game-fullscreen-surface]").contains(document.querySelector(".modal-backdrop"))'))
    screenshot('tower-more-'+str(width));activate('.modal-backdrop [aria-label="关闭"]',touch)
    activate('[data-sf-tower-tools] [data-sf-sell]',touch)
    wait('!document.querySelector('+json.dumps(selector)+')?.classList.contains("sf-pad-occupied")')
    verify(str(width)+' sell restores empty pad',not any(t['id']==tower['id'] for t in w.data()['towers']) and js('!!document.querySelector('+json.dumps(selector+' > span')+')'))
    verify(str(width)+' sell clears tools',not js('!!document.querySelector("[data-sf-tower-tools]")'))
    # Reopen the empty pad through actual touch/mouse; build remains a dialog.
    activate(selector,touch);wait('!!document.querySelector("[data-sf-build=arrow]")')
    verify(str(width)+' build dialog stays in fullscreen',js('document.querySelector("[data-game-fullscreen-surface]").contains(document.querySelector(".modal-backdrop"))'))
    activate('[data-sf-build=arrow]',touch);wait('document.querySelector('+json.dumps(selector)+')?.classList.contains("sf-pad-occupied")')
    if js('!!document.querySelector(".modal-backdrop")'):activate('.modal-backdrop [aria-label="关闭"]',touch)
    if js('!!document.querySelector("[data-sf-tower-close]")'):activate('[data-sf-tower-close]',touch)
    r=rect(selector);x=r['x']+r['width']/2;y=r['y']+r['height']*.35
    before=w.camera()
    if touch:native('drag',x,y,x+40,y+35)
    else:
        b('mouse','move',x,y);b('mouse','down');b('mouse','move',x+40,y+35);b('mouse','up')
    stable()
    verify(str(width)+' dragging tower does not open tools or modal',not js('!!document.querySelector("[data-sf-tower-tools],.modal-backdrop")'))
    verify(str(width)+' dragging tower moves camera',abs(w.camera()['x']-before['x'])+abs(w.camera()['y']-before['y'])>1)
    button_geometry('fullscreen-'+str(width));screenshot('tower-after-actions-'+str(width));leave(key=not touch)
    verify(str(width)+' exit clears inert and scroll lock',js('!document.querySelector("[inert]")&&document.documentElement.style.overflow!=="hidden"'))


def chat_scenarios(width,height):
    touch=width==320
    b('set','viewport',str(width),str(height));w.scenario('battle',2)
    if touch:b('scrollintoview','.chat-toggle')
    activate('.chat-toggle',touch);b('fill','textarea[aria-label="聊天消息"]','全屏草稿 '+str(width))
    activate('.chat-heading [aria-label="收起聊天"]',touch);enter()
    value=visibility('.chat-toggle');verify(str(width)+' fullscreen chat visible and hittable',value.get('inside') and value.get('visible') and value.get('hit'),value)
    button_geometry('chat-closed-'+str(width));activate('.chat-toggle',touch)
    verify(str(width)+' draft survives entering fullscreen',js('document.querySelector("textarea[aria-label=聊天消息]").value')=='全屏草稿 '+str(width))
    verify(str(width)+' chat dialog inside fullscreen',visibility('.chat-panel').get('inside'))
    verify(str(width)+' chat input receives focus',js('document.activeElement===document.querySelector("textarea[aria-label=聊天消息]")'))
    b('press','Space')
    verify(str(width)+' typing space keeps fullscreen',js('!!document.querySelector("[data-room-fullscreen]")'))
    draft=js('document.querySelector("textarea[aria-label=聊天消息]").value')
    leave();verify(str(width)+' draft survives exit',js('document.querySelector("textarea[aria-label=聊天消息]").value')==draft)
    enter();verify(str(width)+' open draft survives reentry',js('document.querySelector("textarea[aria-label=聊天消息]").value')==draft)
    message='原生全屏消息 '+str(width)+' '+uuid.uuid4().hex[:8]
    b('fill','textarea[aria-label="聊天消息"]',message);b('press','Enter')
    wait('document.querySelector(".chat-messages")?.textContent.includes('+json.dumps(message)+')')
    peer_messages=w.PEERS[0].api('/state')['room']['chat']
    verify(str(width)+' peer receives exactly one real message',sum(m['text']==message for m in peer_messages)==1)
    screenshot('fullscreen-chat-'+str(width))
    activate('.chat-message .player-link',touch);wait('!!document.querySelector(".modal-backdrop")')
    verify(str(width)+' chat player profile inside fullscreen',js('document.querySelector("[data-game-fullscreen-surface]").contains(document.querySelector(".modal-backdrop"))'))
    activate('.modal-backdrop [aria-label="关闭"]',touch)
    activate('.chat-heading [aria-label="收起聊天"]',touch)
    reply='同伴回信 '+str(width)
    w.PEERS[0].api('/rooms/'+w.room()['id']+'/chat',{'text':reply,'nonce':uuid.uuid4().hex})
    wait('document.querySelector(".chat-toggle b")?.textContent==="1"')
    verify(str(width)+' unread appears in fullscreen',visibility('.chat-toggle').get('hit'))
    activate('.chat-toggle',touch);wait('document.querySelector(".chat-messages")?.textContent.includes('+json.dumps(reply)+')')
    b('press','Escape');stable()
    # Native Chrome may consume the first Escape to exit OS fullscreen without
    # dispatching a DOM key. Verify that exit, then close the retained composer.
    if js('!!document.querySelector(".chat-panel")'):
        verify(str(width)+' browser consumed ESC to exit native fullscreen',not js('!!document.querySelector("[data-room-fullscreen]")||!!document.fullscreenElement'))
        b('focus','textarea[aria-label="聊天消息"]');b('press','Escape');stable()
    verify(str(width)+' ESC closes chat',not js('!!document.querySelector(".chat-panel")'))
    if js('!!document.querySelector("[data-room-fullscreen]")'):leave(key=True)
    verify(str(width)+' ESC exit restores document',js('!document.querySelector("[data-room-fullscreen],[inert]")&&document.documentElement.style.overflow!=="hidden"'))


def countdown_scenarios():
    for width,height in [(1366,900),(320,740)]:
        b('set','viewport',str(width),str(height));w.scenario('natural');enter()
        hud_geometry('prepare-'+str(width));screenshot('prepare-hud-'+str(width))
        activate('[data-sf-pause]',width==320);wait('document.querySelector(".starfort-board").dataset.phase==="paused"')
        before=js('document.querySelector("[data-sf-countdown]")?.textContent')
        b('wait','1200')
        verify(str(width)+' paused HUD stops countdown',before==js('document.querySelector("[data-sf-countdown]")?.textContent') and ('暂停' in (before or '')))
        leave()
    w.scenario('zero-gold');enter();hud_geometry('mobile-zero-gold')
    verify('zero gold remains visible',js('document.querySelector("[data-sf-own-gold]")?.textContent.trim()==="0"'))
    screenshot('mobile-zero-gold');leave()


def baseline_scenarios():
    for width,height in [(1366,900),(320,740)]:
        b('set','viewport',str(width),str(height));w.scenario('battle',2)
        selector=choose_visible_tower();r=rect(selector);range_probe()
        b('mouse','move',r['x']+r['width']/2,r['y']+r['height']*.35)
        stable();verify(str(width)+' hover paints range without clicking',range_seen());screenshot('before-hover-'+str(width))
        w.mouse_click(r['x']+r['width']/2,r['y']+r['height']*.35)
        wait('!!document.querySelector("[role=dialog]")')
        verify(str(width)+' tower click has no modal backdrop',not js('!!document.querySelector(".modal-backdrop")'))
        w.artifact('before-range-'+str(width)+'.json',{'dialog':js('document.querySelector("[role=dialog]")?.outerHTML'),'modalClasses':js('[...document.querySelectorAll("[role=dialog]")].map(e=>e.parentElement.className)')})
        screenshot('before-range-modal-'+str(width))
        b('click','[role=dialog] [aria-label="关闭"]')
        enter();hud_geometry('before-fullscreen-'+str(width));value=visibility('.chat-toggle')
        verify(str(width)+' chat remains visible in fullscreen',value.get('inside') and value.get('visible') and value.get('hit'),value)
        screenshot('before-fullscreen-chat-'+str(width));leave()


def hashes(paths):
    return {str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.is_file()}


def source_hashes():
    paths=list((ROOT/'games/starfort/web').glob('*'))+list((ROOT/'games/starfort/server').glob('*.go'))
    paths += [ROOT/p for p in ['web/src/chat.tsx','web/src/main.tsx','web/src/ui/room-fullscreen.tsx','web/src/ui/room-fullscreen.css','games/starfort/tests/browser_fixture_test.go','games/starfort/tests/accept_world_ui.py','games/starfort/tests/cadence_test.go','games/starfort/tests/accept_combat_controls_ui.py','games/starfort/tests/combat_draw_probe.js','games/starfort/tests/analyze_combat_draw_probe.py','scripts/browser_touch.mjs']]
    return hashes(paths)


def capture_combat(name, seconds=12):
    # Reload anchors the production cursor to its initial snapshot. The recorder
    # runs before fetch/WebSocket/Canvas initialization; it never injects events.
    b('reload');wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
    enter()
    target=w.data()['enemies'][0]
    point=js('''(()=>{const e=document.querySelector('[data-sf-minimap]'),p=e.createSVGPoint();p.x='''+str(target['x'])+';p.y='+str(target['y'])+''';const r=p.matrixTransform(e.getScreenCTM());return {x:r.x,y:r.y}})()''')
    w.mouse_click(point['x'],point['y']);stable()
    b('wait',str(seconds*1000))
    raw=js('window.__sfCombatProbe.export()')
    w.artifact(name+'-draws.json',raw)
    from analyze_combat_draw_probe import analyze_combat_probe
    result=analyze_combat_probe(raw)
    w.artifact(name+'-analysis.json',result)
    for check in result['checks']:verify(name+' '+check['name'],check['passed'],check.get('evidence'))
    screenshot(name);leave()


def combat_scenarios():
    b('reload');b('set','viewport','1366','900');w.scenario('cadence')
    capture_combat('cadence-level1')
    selector=choose_visible_tower('arrow');activate(selector)
    activate('[data-sf-tower-tools] [data-sf-upgrade="level"]')
    if js('!!document.querySelector("[data-sf-tower-close]")'):activate('[data-sf-tower-close]')
    verify('cadence real upgrade reaches level2',any(t['kind']=='arrow' and t['level']==2 for t in w.data()['towers']))
    capture_combat('cadence-level2-reconnect')
    w.scenario('cadence-speed');capture_combat('cadence-speed')


def main():
    try:
        if '--skip-build' not in sys.argv: w.run(['npm','--prefix','web','run','build'],'build.log')
        w.REPORT['buildSHA256']=hashes((ROOT/'web/dist').rglob('*'))
        assert w.REPORT['buildSHA256'], 'Production frontend build required'
        source_start=source_hashes()
        w.REPORT['sourceSHA256AtStart']=source_start
        setup()
        if BASELINE_COMBAT:
            w.REPORT['limitations'].append('Published old frontend with current test-server scheduler: isolates client batching; not evidence of original server cooldowns.')
            b('reload');b('set','viewport','1366','900');w.scenario('cadence');capture_combat('before-cadence')
        elif BASELINE: baseline_scenarios()
        else:
            if '--combat-only' not in sys.argv:
                sizes=[(320,740)] if '--mobile-only' in sys.argv else [(1366,900)] if '--pc-only' in sys.argv else [(1366,900),(320,740)]
                countdown_scenarios()
                for width,height in sizes:
                    b('reload');wait('document.querySelector(".sf-battlefield")?.dataset.assetsReady==="true"')
                    tower_scenarios(width,height);chat_scenarios(width,height)
            if '--ui-only' not in sys.argv:combat_scenarios()
        verify('no uncaught browser errors',not b('errors')['errors'],b('errors'))
        w.REPORT['passed']=not w.REPORT['failures'];w.REPORT['baselineComplete']=BASELINE
    except Exception as error:
        import traceback
        traceback.print_exc();w.REPORT['error']=repr(error)
        if w.OPENED:
            try: w.REPORT['snapshot']=b('snapshot','-i');screenshot('failure')
            except Exception: pass
    finally:
        if w.OPENED:
            try:b('close')
            except Exception:pass
        if w.SERVER:
            (w.OUT/'stop').write_text('stop')
            try:w.SERVER.wait(timeout=15)
            except subprocess.TimeoutExpired:w.SERVER.terminate();w.SERVER.wait(timeout=10)
        w.REPORT['sourceSHA256']=source_hashes()
        w.REPORT['sourceUnchangedDuringRun']=w.REPORT.get('sourceSHA256AtStart')==w.REPORT['sourceSHA256']
        if not BASELINE and not w.REPORT['sourceUnchangedDuringRun']:
            w.REPORT['passed']=False
            w.REPORT['failures'].append({'name':'sources remain unchanged during acceptance','passed':False})
        w.artifact('result.json',w.REPORT)
        for path in [w.OUT/'profiles',w.OUT/'tmp']:
            if path.is_dir():shutil.rmtree(path)
        for name in ['url','stop','combat-init.js']:(w.OUT/name).unlink(missing_ok=True)
        if w.REPORT.get('passed') or w.REPORT.get('baselineComplete'):
            for name in ['server.log','failure.png']:(w.OUT/name).unlink(missing_ok=True)
        print('Artifact:',w.OUT/'result.json',flush=True)
        if not w.REPORT['passed'] and not w.REPORT.get('baselineComplete'):raise SystemExit(1)


if __name__ == '__main__':
    main()
